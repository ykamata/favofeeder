package parser_test

import (
	"testing"

	"github.com/ykamata/favofeeder/internal/parser"
)

const validJSON = `{
  "items": [
    {
      "title": "テストアニメ 新シーズン発表",
      "summary": "2026年秋に新シーズンの放送が決定しました。",
      "source_url": "https://example.com/news/1",
      "published_date": "2026-05-01",
      "content_type": "news"
    },
    {
      "title": "テストアニメ Blu-ray発売",
      "summary": "Blu-rayボックスが発売されます。",
      "source_url": "https://example.com/news/2",
      "published_date": null,
      "content_type": "release"
    }
  ]
}`

func TestParse_ValidJSON(t *testing.T) {
	items, err := parser.Parse(validJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Title != "テストアニメ 新シーズン発表" {
		t.Errorf("unexpected title: %q", items[0].Title)
	}
	if items[0].PublishedDate != "2026-05-01" {
		t.Errorf("unexpected date: %q", items[0].PublishedDate)
	}
	if items[1].PublishedDate != "" {
		t.Errorf("null date should normalize to empty string, got %q", items[1].PublishedDate)
	}
}

func TestParse_JSONWithPreamble(t *testing.T) {
	output := "以下が結果です:\n\n" + validJSON + "\n\n以上です。"
	items, err := parser.Parse(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
}

func TestParse_MarkdownCodeFence(t *testing.T) {
	output := "結果:\n```json\n" + validJSON + "\n```"
	items, err := parser.Parse(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
}

func TestParse_SkipsItemsMissingURLOrSummary(t *testing.T) {
	raw := `{
  "items": [
    {"title": "valid", "summary": "ok", "source_url": "https://example.com", "content_type": "news"},
    {"title": "no url", "summary": "ok", "source_url": "", "content_type": "news"},
    {"title": "no summary", "summary": "", "source_url": "https://example.com/2", "content_type": "news"}
  ]
}`
	items, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 valid item, got %d", len(items))
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := parser.Parse("これはJSONではありません")
	if err == nil {
		t.Fatal("want error for invalid JSON, got nil")
	}
}

func TestParse_InvalidDate(t *testing.T) {
	raw := `{
  "items": [
    {"title": "t", "summary": "s", "source_url": "https://example.com", "published_date": "not-a-date", "content_type": "news"}
  ]
}`
	items, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].PublishedDate != "" {
		t.Errorf("invalid date should normalize to empty, got %q", items[0].PublishedDate)
	}
}
