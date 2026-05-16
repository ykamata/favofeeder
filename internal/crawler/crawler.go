package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ykamata/favofeeder/internal/codex"
	"github.com/ykamata/favofeeder/internal/config"
	"github.com/ykamata/favofeeder/internal/parser"
	"github.com/ykamata/favofeeder/internal/search"
	"github.com/ykamata/favofeeder/internal/storage"
)

type Crawler struct {
	client       *codex.Client
	searchClient *search.Client // nil = web search disabled
	db           *storage.DB
	dryRun       bool
	sinceDate    time.Time // zero = no date filter
}

func New(client *codex.Client, searchClient *search.Client, db *storage.DB, dryRun bool, sinceDate time.Time) *Crawler {
	return &Crawler{client: client, searchClient: searchClient, db: db, dryRun: dryRun, sinceDate: sinceDate}
}

type TargetResult struct {
	Title    string
	Category string
	New      int
	Dup      int
	NewItems []parser.ContentItem
}

type Result struct {
	TotalNew      int
	TotalDup      int
	Errors        []error
	TargetResults []TargetResult
}

// Run fetches content for all targets and saves to DB.
func (c *Crawler) Run(ctx context.Context, targets []config.Target) Result {
	logID, err := storage.LogCrawlStart(ctx, c.db)
	if err != nil {
		slog.Warn("failed to log crawl start", "err", err)
	}

	var res Result
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("context cancelled: %w", err))
			break
		}
		r, err := c.fetchTarget(ctx, target)
		if err != nil {
			slog.Error("fetch failed", "target", target.Title, "err", err)
			res.Errors = append(res.Errors, fmt.Errorf("%s: %w", target.Title, err))
			continue
		}
		res.TotalNew += r.New
		res.TotalDup += r.Dup
		res.TargetResults = append(res.TargetResults, TargetResult{
			Title:    target.Title,
			Category: target.Category,
			New:      r.New,
			Dup:      r.Dup,
			NewItems: r.NewItems,
		})
		slog.Info("fetched", "target", target.Title, "new", r.New, "dup", r.Dup)
	}

	var crawlErr error
	if len(res.Errors) > 0 {
		msgs := make([]string, len(res.Errors))
		for i, e := range res.Errors {
			msgs[i] = e.Error()
		}
		crawlErr = fmt.Errorf("%s", strings.Join(msgs, "; "))
	}

	if err := storage.LogCrawlEnd(ctx, c.db, logID, res.TotalNew, res.TotalDup, crawlErr); err != nil {
		slog.Warn("failed to log crawl end", "err", err)
	}

	return res
}

func (c *Crawler) fetchTarget(ctx context.Context, target config.Target) (storage.SaveResult, error) {
	sources := make([]string, len(target.Sources))
	for i, s := range target.Sources {
		switch s.Type {
		case "x_account":
			account := strings.TrimPrefix(s.Account, "@")
			sources[i] = fmt.Sprintf("https://x.com/%s", account)
		case "website":
			sources[i] = s.URL
		}
	}

	searchResults := c.runSearches(ctx, target.Title, target.Sources)
	prompt := codex.BuildPrompt(target.Title, target.Category, sources, searchResults, c.sinceDate)

	slog.Debug("prompt", "target", target.Title, "prompt", prompt)

	slog.Info("calling codex", "target", target.Title, "sources", len(sources), "search_results", strings.Count(searchResults, "\n["))
	output, err := c.client.Run(ctx, prompt)
	if err != nil {
		return storage.SaveResult{}, fmt.Errorf("codex run: %w", err)
	}

	items, err := parser.Parse(output)
	if err != nil {
		return storage.SaveResult{}, fmt.Errorf("parse output: %w\nraw output: %s", err, output)
	}

	items = c.filterByDate(items)

	if c.dryRun {
		slog.Info("[dry-run] response", "output", output, "items", len(items))
		return storage.SaveResult{}, nil
	}

	// Determine source_type for DB (use first source type)
	sourceType := "website"
	if len(target.Sources) > 0 {
		sourceType = target.Sources[0].Type
	}

	return storage.SaveItems(ctx, c.db, target.Title, target.Category, sourceType, items)
}

// runSearches calls Brave Search API with multiple queries and returns formatted results.
// Returns empty string if search client is not configured or no results found.
func (c *Crawler) runSearches(ctx context.Context, title string, sources []config.Source) string {
	if c.searchClient == nil {
		return ""
	}

	const freshness = "pd"

	queries := []struct {
		q         string
		freshness string
	}{
		{title + " 最新情報", freshness},
		{title + " ニュース", freshness},
		{title + " アップデート", freshness},
	}
	for _, s := range sources {
		if s.Type != "x_account" {
			continue
		}
		account := strings.TrimPrefix(s.Account, "@")
		queries = append(queries, struct {
			q         string
			freshness string
		}{"site:x.com @" + account, "pd"})
	}

	since := c.sinceDate.Truncate(24 * time.Hour)
	seen := map[string]bool{}
	var results []search.Result

	for _, entry := range queries {
		res, err := c.searchClient.Search(ctx, entry.q, 10, entry.freshness)
		if err != nil {
			slog.Warn("search failed", "query", entry.q, "err", err)
			continue
		}
		slog.Debug("search", "query", entry.q, "count", len(res))
		for _, r := range res {
			if seen[r.URL] {
				continue
			}
			seen[r.URL] = true
			// Skip results with a known date older than sinceDate
			if !since.IsZero() && r.PublishedAt != "" {
				if d, err := time.Parse("2006-01-02", r.PublishedAt); err == nil && d.Before(since) {
					continue
				}
			}
			results = append(results, r)
		}
	}

	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "[%d] %s\n", i+1, r.Title)
		fmt.Fprintf(&sb, "URL: %s\n", r.URL)
		if r.PublishedAt != "" {
			fmt.Fprintf(&sb, "日付: %s\n", r.PublishedAt)
		}
		if r.Description != "" {
			desc := r.Description
			if len(desc) > 300 {
				desc = desc[:300] + "..."
			}
			fmt.Fprintf(&sb, "概要: %s\n", desc)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// filterByDate removes items whose published_date is known and older than sinceDate.
// Items with no published_date are kept as their date cannot be verified.
func (c *Crawler) filterByDate(items []parser.ContentItem) []parser.ContentItem {
	if c.sinceDate.IsZero() {
		return items
	}
	since := c.sinceDate.Truncate(24 * time.Hour)
	filtered := items[:0]
	for _, item := range items {
		if item.PublishedDate == "" {
			filtered = append(filtered, item)
			continue
		}
		d, err := time.Parse("2006-01-02", item.PublishedDate)
		if err != nil || !d.Before(since) {
			filtered = append(filtered, item)
			continue
		}
		slog.Debug("filtered out old item", "url", item.SourceURL, "published", item.PublishedDate, "since", since.Format("2006-01-02"))
	}
	return filtered
}
