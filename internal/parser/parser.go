package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type ContentItem struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	SourceURL     string `json:"source_url"`
	PublishedDate string `json:"published_date"` // "YYYY-MM-DD" or ""
	ContentType   string `json:"content_type"`   // news/release/review/other
}

type codexResponse struct {
	Items []ContentItem `json:"items"`
}

var jsonBlock = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")
var jsonRaw = regexp.MustCompile("(?s)(\\{[^{}]*\"items\"[^{}]*(?:\\{[^{}]*\\}[^{}]*)*\\})")

// Parse extracts ContentItems from Codex CLI output.
// It tolerates extra text before/after JSON and markdown code fences.
func Parse(output string) ([]ContentItem, error) {
	jsonStr := extractJSON(output)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in codex output")
	}

	var resp codexResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal codex response: %w\nraw: %s", err, jsonStr)
	}

	items := make([]ContentItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		normalized := normalize(item)
		if normalized.SourceURL == "" || normalized.Summary == "" {
			continue
		}
		items = append(items, normalized)
	}
	return items, nil
}

func extractJSON(s string) string {
	// Try markdown code fence first
	if m := jsonBlock.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	// Try raw JSON object containing "items"
	if m := jsonRaw.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	// Try the whole string as JSON
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		return s
	}
	return ""
}

func normalize(item ContentItem) ContentItem {
	item.Title = strings.TrimSpace(item.Title)
	item.Summary = strings.TrimSpace(item.Summary)
	item.SourceURL = strings.TrimSpace(item.SourceURL)
	item.ContentType = strings.ToLower(strings.TrimSpace(item.ContentType))

	if item.ContentType == "" {
		item.ContentType = "other"
	}

	// Validate and normalize published_date
	if item.PublishedDate != "" && item.PublishedDate != "null" {
		if _, err := time.Parse("2006-01-02", item.PublishedDate); err != nil {
			item.PublishedDate = ""
		}
	} else {
		item.PublishedDate = ""
	}

	return item
}
