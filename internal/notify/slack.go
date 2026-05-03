package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ykamata/favofeeder/internal/crawler"
)

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

type slackBlock struct {
	Type string      `json:"type"`
	Text *slackText  `json:"text,omitempty"`
	Elements []slackText `json:"elements,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

func (s *SlackNotifier) Notify(ctx context.Context, result crawler.Result) error {
	payload := buildPayload(result)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}

func buildPayload(result crawler.Result) slackPayload {
	var blocks []slackBlock

	blocks = append(blocks, slackBlock{
		Type: "header",
		Text: &slackText{Type: "plain_text", Text: "favofeeder 新着情報"},
	})

	for _, tr := range result.TargetResults {
		var sb strings.Builder
		emoji := categoryEmoji(tr.Category)
		fmt.Fprintf(&sb, "*%s %s* — %d件の新着\n", emoji, tr.Title, tr.New)
		for _, item := range tr.NewItems {
			summary := item.Summary
			if len([]rune(summary)) > 120 {
				summary = string([]rune(summary)[:120]) + "…"
			}
			if item.SourceURL != "" {
				fmt.Fprintf(&sb, "• <%s|%s> — %s\n", item.SourceURL, item.Title, summary)
			} else {
				fmt.Fprintf(&sb, "• %s — %s\n", item.Title, summary)
			}
		}
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: strings.TrimRight(sb.String(), "\n")},
		})
		blocks = append(blocks, slackBlock{Type: "divider"})
	}

	footerParts := []string{
		fmt.Sprintf("新着: %d件", result.TotalNew),
		fmt.Sprintf("重複スキップ: %d件", result.TotalDup),
		time.Now().Format("2006-01-02 15:04"),
	}
	if len(result.Errors) > 0 {
		footerParts = append(footerParts, fmt.Sprintf("エラー: %d件", len(result.Errors)))
	}
	blocks = append(blocks, slackBlock{
		Type:     "context",
		Elements: []slackText{{Type: "mrkdwn", Text: strings.Join(footerParts, " | ")}},
	})

	summaryText := fmt.Sprintf("favofeeder: %d件の新着情報", result.TotalNew)
	return slackPayload{Text: summaryText, Blocks: blocks}
}

func categoryEmoji(category string) string {
	switch category {
	case "game":
		return "🎮"
	case "anime":
		return "📺"
	case "manga":
		return "📚"
	case "novel":
		return "📖"
	default:
		return "📌"
	}
}
