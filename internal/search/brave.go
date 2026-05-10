package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiURL = "https://api.search.brave.com/res/v1/web/search"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type Result struct {
	Title       string
	URL         string
	Description string
	PublishedAt string // "YYYY-MM-DD" or empty
}

func (c *Client) Search(ctx context.Context, query string, count int) ([]Result, error) {
	params := url.Values{
		"q":                {query},
		"count":            {fmt.Sprintf("%d", count)},
		"search_lang":      {"jp"},
		"country":          {"JP"},
		"text_decorations": {"0"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var body struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				PageAge     string `json:"page_age"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]Result, 0, len(body.Web.Results))
	for _, r := range body.Web.Results {
		res := Result{
			Title:       strings.ToValidUTF8(r.Title, ""),
			URL:         r.URL,
			Description: strings.ToValidUTF8(strings.TrimSpace(r.Description), ""),
		}
		if r.PageAge != "" {
			for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
				if t, err := time.Parse(layout, r.PageAge); err == nil {
					res.PublishedAt = t.Format("2006-01-02")
					break
				}
			}
		}
		results = append(results, res)
	}
	return results, nil
}
