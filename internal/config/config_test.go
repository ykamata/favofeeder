package config_test

import (
	"os"
	"testing"

	"github.com/ykamata/favofeeder/internal/config"
)

func TestLoad_Valid(t *testing.T) {
	yaml := `
targets:
  - title: "テストアニメ"
    category: anime
    sources:
      - type: x_account
        account: "@TestAccount"
      - type: website
        url: "https://example.com"
  - title: "テストゲーム"
    category: game
    sources:
      - type: website
        url: "https://game.example.com"
`
	f := writeTempYAML(t, yaml)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(cfg.Targets))
	}
	if cfg.Targets[0].Title != "テストアニメ" {
		t.Errorf("want テストアニメ, got %q", cfg.Targets[0].Title)
	}
	if cfg.Targets[0].Category != "anime" {
		t.Errorf("want anime, got %q", cfg.Targets[0].Category)
	}
	if len(cfg.Targets[0].Sources) != 2 {
		t.Errorf("want 2 sources, got %d", len(cfg.Targets[0].Sources))
	}
}

func TestLoad_InvalidCategory(t *testing.T) {
	yaml := `
targets:
  - title: "テスト"
    category: movie
    sources:
      - type: website
        url: "https://example.com"
`
	f := writeTempYAML(t, yaml)
	_, err := config.Load(f)
	if err == nil {
		t.Fatal("want error for invalid category, got nil")
	}
}

func TestLoad_MissingTitle(t *testing.T) {
	yaml := `
targets:
  - category: anime
    sources:
      - type: website
        url: "https://example.com"
`
	f := writeTempYAML(t, yaml)
	_, err := config.Load(f)
	if err == nil {
		t.Fatal("want error for missing title, got nil")
	}
}

func TestLoad_MissingAccount(t *testing.T) {
	yaml := `
targets:
  - title: "テスト"
    category: manga
    sources:
      - type: x_account
`
	f := writeTempYAML(t, yaml)
	_, err := config.Load(f)
	if err == nil {
		t.Fatal("want error for missing account, got nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("want error for missing file, got nil")
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}
