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
	"github.com/ykamata/favofeeder/internal/storage"
)

type Crawler struct {
	client    *codex.Client
	db        *storage.DB
	dryRun    bool
	sinceDate time.Time // zero = no date filter
}

func New(client *codex.Client, db *storage.DB, dryRun bool, sinceDate time.Time) *Crawler {
	return &Crawler{client: client, db: db, dryRun: dryRun, sinceDate: sinceDate}
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

	prompt := codex.BuildPrompt(target.Title, target.Category, sources, c.sinceDate)

	slog.Debug("prompt", "target", target.Title, "prompt", prompt)

	slog.Info("calling codex", "target", target.Title, "sources", len(sources))
	output, err := c.client.Run(ctx, prompt)
	if err != nil {
		return storage.SaveResult{}, fmt.Errorf("codex run: %w", err)
	}

	items, err := parser.Parse(output)
	if err != nil {
		return storage.SaveResult{}, fmt.Errorf("parse output: %w\nraw output: %s", err, output)
	}

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
