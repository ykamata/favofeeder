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
// sinceDate sets the preferred start date for news collection; zero value means no filter.
func BuildPrompt(title, category string, sources []string, sinceDate time.Time) string {
	sourceList := strings.Join(sources, "\n")

	var periodInstruction string
	if !sinceDate.IsZero() {
		periodInstruction = fmt.Sprintf(`
**収集対象期間**:
- published_date が判明している記事: %s 以降のもののみ収集する。それより古い記事は除外する。
- published_date が不明な記事: 最近の情報と判断できれば含める。`,
			sinceDate.Format("2006年1月2日"))
	}

	return fmt.Sprintf(`「%s」(%s)に関する最新情報を収集してください。

## 手順

### 1. 以下の公式ソースを確認する
%s

### 2. Web 検索で新着情報を探す
上記ソースに載っていない情報も拾うため、下記のようなクエリで検索してください:
- 「%s 最新情報」
- 「%s ニュース」
- 「%s アップデート」
- 「%s site:x.com」
%s

## ルール

- ソース確認 → Web 検索の順で、なるべく多くの新着情報を集めること
- クエリを変えながら複数回検索し、見落としを防ぐこと
- 日本語の情報を優先するが、重要な公式発表は英語でも含める
- 情報が本当に何も見つからなかった場合のみ items を空配列にする

## 出力（JSON のみ、前後に説明文を含めないこと）

{
  "items": [
    {
      "title": "タイトル名",
      "summary": "内容の要約（日本語）",
      "source_url": "情報源の URL（X の場合は個別投稿 URL: https://x.com/username/status/1234567890）",
      "published_date": "YYYY-MM-DD または null",
      "content_type": "news/release/review/other"
    }
  ]
}`, title, category, sourceList, title, title, title, title, periodInstruction)
}
