package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Slack   SlackConfig `yaml:"slack"`
	Targets []Target    `yaml:"targets"`
}

type SlackConfig struct {
	Webhooks []SlackWebhook `yaml:"webhooks"`
}

type SlackWebhook struct {
	WebhookURL string   `yaml:"webhook_url"`
	Titles     []string `yaml:"titles"` // empty = all targets
}

type Target struct {
	Title    string   `yaml:"title"`
	Category string   `yaml:"category"` // anime / game / manga
	Sources  []Source `yaml:"sources"`
}

type Source struct {
	Type    string `yaml:"type"`    // x_account / website
	Account string `yaml:"account"` // X account (e.g. "@NintendoJP")
	URL     string `yaml:"url"`     // website URL
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	for i, w := range c.Slack.Webhooks {
		if w.WebhookURL == "" {
			return fmt.Errorf("slack.webhooks[%d]: webhook_url is required", i)
		}
	}
	for i, t := range c.Targets {
		if t.Title == "" {
			return fmt.Errorf("targets[%d]: title is required", i)
		}
		switch t.Category {
		case "anime", "game", "manga", "novel":
		default:
			return fmt.Errorf("targets[%d]: category must be anime/game/manga, got %q", i, t.Category)
		}
		if len(t.Sources) == 0 {
			return fmt.Errorf("targets[%d] %q: at least one source is required", i, t.Title)
		}
		for j, s := range t.Sources {
			switch s.Type {
			case "x_account":
				if s.Account == "" {
					return fmt.Errorf("targets[%d].sources[%d]: account is required for x_account", i, j)
				}
			case "website":
				if s.URL == "" {
					return fmt.Errorf("targets[%d].sources[%d]: url is required for website", i, j)
				}
			default:
				return fmt.Errorf("targets[%d].sources[%d]: type must be x_account/website, got %q", i, j, s.Type)
			}
		}
	}
	return nil
}
