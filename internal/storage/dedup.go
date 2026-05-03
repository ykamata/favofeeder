package storage

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/ykamata/favofeeder/internal/parser"
)

// Hash computes a deduplication hash for a ContentItem.
// Keyed on normalized source URL only — Codex regenerates summaries each run,
// so including summary in the key would always produce a new hash.
func Hash(item parser.ContentItem) string {
	key := normalizeURL(item.SourceURL)
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", sum)
}

// normalizeURL lowercases and strips trailing slashes so minor URL variations
// (e.g. trailing /) don't create duplicate records.
func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.ToLower(u)
	u = strings.TrimRight(u, "/")
	return u
}
