package codex_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ykamata/favofeeder/internal/codex"
)

func TestBuildPrompt(t *testing.T) {
	sources := []string{"X: @TestAccount", "https://example.com"}
	prompt := codex.BuildPrompt("テストアニメ", "anime", sources, time.Time{})

	if !strings.Contains(prompt, "テストアニメ") {
		t.Error("prompt should contain title")
	}
	if !strings.Contains(prompt, "anime") {
		t.Error("prompt should contain category")
	}
	if !strings.Contains(prompt, "@TestAccount") {
		t.Error("prompt should contain source account")
	}
	if !strings.Contains(prompt, `"items"`) {
		t.Error("prompt should contain JSON template")
	}
	if strings.Contains(prompt, "収集対象期間") {
		t.Error("prompt should not contain date filter when sinceDate is zero")
	}
	if !strings.Contains(prompt, "日本語") {
		t.Error("prompt should contain Japanese language instruction")
	}
	if !strings.Contains(prompt, "site:x.com") {
		t.Error("prompt should contain X search query")
	}
	if !strings.Contains(prompt, "Web 検索") {
		t.Error("prompt should contain web search instruction")
	}
}

func TestBuildPrompt_WithSinceDate(t *testing.T) {
	since := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	sources := []string{"https://example.com"}
	prompt := codex.BuildPrompt("テスト", "game", sources, since)

	if !strings.Contains(prompt, "収集対象期間") {
		t.Error("prompt should contain date filter when sinceDate is set")
	}
	if !strings.Contains(prompt, "2026年4月2日") {
		t.Error("prompt should contain formatted since date")
	}
}
