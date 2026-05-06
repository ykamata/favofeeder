package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ykamata/favofeeder/internal/parser"
)

type SaveResult struct {
	New      int
	Dup      int
	NewItems []parser.ContentItem
}

// SaveItems inserts ContentItems, skipping duplicates by content_hash.
func SaveItems(ctx context.Context, db *DB, title, category, sourceType string, items []parser.ContentItem) (SaveResult, error) {
	q := fmt.Sprintf(`%s INTO content_items
  (title, category, source_url, source_type, content, content_hash, crawled_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?)`, db.insertIgnorePrefix())

	now := time.Now().UTC()
	var res SaveResult

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return res, fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		hash := Hash(item)
		content := fmt.Sprintf("%s\n%s", item.Title, item.Summary)
		r, err := stmt.ExecContext(ctx, title, category, item.SourceURL, sourceType, content, hash, db.timeVal(now))
		if err != nil {
			return res, fmt.Errorf("insert item %q: %w", item.SourceURL, err)
		}
		n, _ := r.RowsAffected()
		if n > 0 {
			res.New++
			res.NewItems = append(res.NewItems, item)
		} else {
			res.Dup++
		}
	}

	return res, tx.Commit()
}

// LastSuccessfulCrawlTime returns the ended_at of the most recent successful crawl.
// Returns zero time if no successful crawl exists yet.
func LastSuccessfulCrawlTime(ctx context.Context, db *DB) (time.Time, error) {
	row := db.QueryRowContext(ctx,
		`SELECT ended_at FROM crawl_logs WHERE status='success' ORDER BY ended_at DESC LIMIT 1`,
	)
	t, err := db.scanTime(row)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("query last crawl time: %w", err)
	}
	return t, nil
}

// LogCrawlStart inserts a crawl_log with status=running and returns its ID.
func LogCrawlStart(ctx context.Context, db *DB) (int64, error) {
	r, err := db.ExecContext(ctx,
		`INSERT INTO crawl_logs (started_at, status) VALUES (?, 'running')`,
		db.timeVal(time.Now().UTC()),
	)
	if err != nil {
		return 0, fmt.Errorf("log crawl start: %w", err)
	}
	return r.LastInsertId()
}

// LogCrawlEnd updates a crawl_log with final status and counts.
func LogCrawlEnd(ctx context.Context, db *DB, id int64, newItems, dupItems int, crawlErr error) error {
	status := "success"
	var errMsg *string
	if crawlErr != nil {
		status = "failed"
		s := crawlErr.Error()
		errMsg = &s
	}
	_, err := db.ExecContext(ctx,
		`UPDATE crawl_logs SET ended_at=?, status=?, items_new=?, items_dup=?, error_msg=? WHERE id=?`,
		db.timeVal(time.Now().UTC()), status, newItems, dupItems, errMsg, id,
	)
	return err
}
