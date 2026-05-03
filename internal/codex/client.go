package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 5 * time.Minute

// Client calls Codex CLI as a subprocess.
type Client struct {
	codexPath string
	model     string
	timeout   time.Duration
}

func NewClient(codexPath, model string) *Client {
	if codexPath == "" {
		codexPath = "codex"
	}
	return &Client{
		codexPath: codexPath,
		model:     model, // empty = use model from ~/.codex/config.toml
		timeout:   defaultTimeout,
	}
}

// Run executes `codex exec` non-interactively and returns the agent's last message.
func (c *Client) Run(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Write last message to a temp file; avoids mixing with JSONL progress output.
	tmp, err := os.CreateTemp("", "favofeeder-codex-*.txt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	args := []string{"exec", "--skip-git-repo-check", "--ephemeral", "-o", tmpPath}
	if c.model != "" {
		args = append(args, "-m", c.model)
	}
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, c.codexPath, args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("codex exec failed: %w\noutput: %s", err, string(out))
	}

	result, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read codex output: %w", err)
	}

	return strings.TrimSpace(string(result)), nil
}

// BuildPrompt constructs the prompt for Codex CLI from target info.
// sinceDate filters results to on-or-after that date ("YYYY-MM-DD"); zero value means no filter.
func BuildPrompt(title, category string, sources []string, sinceDate time.Time) string {
	sourceList := strings.Join(sources, "\n")

	var periodLine string
	if !sinceDate.IsZero() {
		periodLine = fmt.Sprintf("\n収集対象期間: %s 以降に公開・更新された情報のみ収集してください。それより古い情報は除外してください。", sinceDate.Format("2006年1月2日"))
	}

	return fmt.Sprintf(`以下の情報源から「%s」(%s)に関する最新情報を収集してください。
必ず確認するソース:
%s
%s
加えて、関連する検索やリンクも辿って追加情報を集めてください。
日本語で書かれた情報のみ収集してください。英語など外国語の記事・ページは除外してください。

結果は以下のJSON形式のみで出力してください（前後に説明文を含めないでください）:
{
  "items": [
    {
      "title": "タイトル名",
      "summary": "内容の要約（日本語）",
      "source_url": "情報源のURL（X/Twitterの場合はアカウントのタイムラインURLではなく、各投稿の個別URL（例: https://x.com/username/status/1234567890）を使用してください）",
      "published_date": "YYYY-MM-DD または null",
      "content_type": "news/release/review/other"
    }
  ]
}`, title, category, sourceList, periodLine)
}
