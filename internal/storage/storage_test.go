package storage_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/ykamata/favofeeder/internal/parser"
	"github.com/ykamata/favofeeder/internal/storage"
)

// openTestDB opens a real MySQL DB for integration tests.
// Set DATABASE_TEST_DSN to run; otherwise the test is skipped.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_TEST_DSN")
	if dsn == "" {
		t.Skip("DATABASE_TEST_DSN not set; skipping DB integration test")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestHash_Deterministic(t *testing.T) {
	item := parser.ContentItem{SourceURL: "https://example.com/1", Summary: "テスト要約"}
	if h1, h2 := storage.Hash(item), storage.Hash(item); h1 != h2 {
		t.Error("hash should be deterministic")
	}
}

func TestHash_URLOnly(t *testing.T) {
	a := storage.Hash(parser.ContentItem{SourceURL: "https://example.com/1", Summary: "summary A"})
	b := storage.Hash(parser.ContentItem{SourceURL: "https://example.com/1", Summary: "summary B"})
	if a != b {
		t.Error("same URL with different summary should produce same hash")
	}
}

func TestHash_TrailingSlashNormalized(t *testing.T) {
	a := storage.Hash(parser.ContentItem{SourceURL: "https://example.com/article"})
	b := storage.Hash(parser.ContentItem{SourceURL: "https://example.com/article/"})
	if a != b {
		t.Error("trailing slash should be normalized away")
	}
}

func TestHash_DifferentURLs(t *testing.T) {
	a := storage.Hash(parser.ContentItem{SourceURL: "https://a.com/1"})
	b := storage.Hash(parser.ContentItem{SourceURL: "https://b.com/2"})
	if a == b {
		t.Error("different URLs should produce different hashes")
	}
}

func TestSaveItems_Deduplication(t *testing.T) {
	db := openTestDB(t)

	items := []parser.ContentItem{
		{Title: "記事1", Summary: "内容1", SourceURL: "https://example.com/1", ContentType: "news"},
		{Title: "記事2", Summary: "内容2", SourceURL: "https://example.com/2", ContentType: "news"},
	}

	ctx := context.Background()

	res, err := storage.SaveItems(ctx, db, "テスト", "anime", "website", items)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if res.New != 2 || res.Dup != 0 {
		t.Errorf("first save: want new=2 dup=0, got new=%d dup=%d", res.New, res.Dup)
	}

	res, err = storage.SaveItems(ctx, db, "テスト", "anime", "website", items)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if res.New != 0 || res.Dup != 2 {
		t.Errorf("second save: want new=0 dup=2, got new=%d dup=%d", res.New, res.Dup)
	}
}

func TestLogCrawl(t *testing.T) {
	db := openTestDB(t)

	ctx := context.Background()
	id, err := storage.LogCrawlStart(ctx, db)
	if err != nil {
		t.Fatalf("log start: %v", err)
	}
	if id <= 0 {
		t.Errorf("want positive id, got %d", id)
	}

	if err := storage.LogCrawlEnd(ctx, db, id, 5, 2, nil); err != nil {
		t.Fatalf("log end: %v", err)
	}
}
