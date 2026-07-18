package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ykamata/favofeeder/internal/codex"
	"github.com/ykamata/favofeeder/internal/config"
	"github.com/ykamata/favofeeder/internal/crawler"
	"github.com/ykamata/favofeeder/internal/notify"
	"github.com/ykamata/favofeeder/internal/search"
	"github.com/ykamata/favofeeder/internal/storage"
)

func filterResult(result crawler.Result, titles []string) crawler.Result {
	if len(titles) == 0 {
		return result
	}
	titleSet := make(map[string]bool, len(titles))
	for _, t := range titles {
		titleSet[t] = true
	}
	filtered := crawler.Result{Errors: result.Errors}
	for _, tr := range result.TargetResults {
		if titleSet[tr.Title] {
			filtered.TargetResults = append(filtered.TargetResults, tr)
			filtered.TotalNew += tr.New
			filtered.TotalDup += tr.Dup
		}
	}
	return filtered
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Load .env if present (local development). Silently ignored in production where
	// env vars are already set via docker-compose env_file or system environment.
	_ = godotenv.Load()

	fs := flag.NewFlagSet("favofeeder", flag.ContinueOnError)
	configPath := fs.String("config", "config/targets.yaml", "path to targets YAML")
	dbPath     := fs.String("db", "", "MySQL DSN or SQLite file path")
	local      := fs.Bool("local", false, "use SQLite for local development (default file: favofeeder.db)")
	codexPath  := fs.String("codex", "codex", "path to Codex CLI binary")
	model      := fs.String("model", "gpt-5.5", "Codex model to use")
	dryRun     := fs.Bool("dry-run", false, "print prompts without calling Codex or saving to DB")
	lookbackDays := fs.Int("since-days", 2, "fixed number of days to look back (2 = yesterday + today); dedup prevents re-notification")
	verbose    := fs.Bool("v", false, "verbose logging")
	braveKey   := fs.String("brave-key", os.Getenv("BRAVE_API_KEY"), "Brave Search API key")

	if err := fs.Parse(args); err != nil {
		return err
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	driver := "mysql"
	dsn := *dbPath
	if *local {
		driver = "sqlite"
		if dsn == "" {
			dsn = "favofeeder.db"
		}
	} else if dsn == "" {
		dsn = os.Getenv("DATABASE_DSN")
	}

	db, err := storage.Open(dsn, driver)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Fixed lookback window in JST: always fetch the last N days (default 2 =
	// yesterday + today), independent of the previous crawl time. An incremental
	// window (last successful crawl) drops "yesterday" on the second run of the
	// day, which is exactly the content we want. Duplicate items are skipped by
	// content-hash dedup, so a fixed overlapping window never re-notifies.
	jst := time.FixedZone("JST", 9*60*60)
	now := time.Now().In(jst)
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	sinceDate := startOfToday.AddDate(0, 0, -(*lookbackDays - 1))

	var searchClient *search.Client
	if *braveKey != "" {
		searchClient = search.NewClient(*braveKey)
	} else {
		slog.Warn("BRAVE_API_KEY not set; web search disabled")
	}

	client := codex.NewClient(*codexPath, *model, *verbose)
	c := crawler.New(client, searchClient, db, *dryRun, sinceDate)

	sinceStr := "none"
	if !sinceDate.IsZero() {
		sinceStr = sinceDate.Format("2006-01-02 15:04:05")
	}
	slog.Info("starting fetch", "targets", len(cfg.Targets), "dry_run", *dryRun, "since", sinceStr)
	result := c.Run(ctx, cfg.Targets)

	slog.Info("fetch complete",
		"new", result.TotalNew,
		"dup", result.TotalDup,
		"errors", len(result.Errors),
	)

	for _, webhook := range cfg.Slack.Webhooks {
		filtered := filterResult(result, webhook.Titles)
		if filtered.TotalNew == 0 && len(filtered.Errors) == 0 {
			continue
		}
		notifier := notify.NewSlackNotifier(webhook.WebhookURL)
		if err := notifier.Notify(ctx, filtered); err != nil {
			slog.Warn("slack notification failed", "webhook", webhook.WebhookURL, "err", err)
		} else {
			slog.Info("slack notification sent", "webhook", webhook.WebhookURL, "titles", webhook.Titles)
		}
	}

	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			slog.Error("fetch error", "err", e)
		}
		return fmt.Errorf("%d target(s) failed", len(result.Errors))
	}
	return nil
}
