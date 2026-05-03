package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS content_items (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT    NOT NULL,
  category     TEXT    NOT NULL,
  source_url   TEXT    NOT NULL,
  source_type  TEXT    NOT NULL,
  content      TEXT    NOT NULL,
  content_hash TEXT    NOT NULL UNIQUE,
  crawled_at   DATETIME NOT NULL,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_content_items_title    ON content_items(title);
CREATE INDEX IF NOT EXISTS idx_content_items_category ON content_items(category);
CREATE INDEX IF NOT EXISTS idx_content_items_crawled  ON content_items(crawled_at);

CREATE TABLE IF NOT EXISTS crawl_logs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  started_at DATETIME NOT NULL,
  ended_at   DATETIME,
  status     TEXT    NOT NULL,
  items_new  INTEGER NOT NULL DEFAULT 0,
  items_dup  INTEGER NOT NULL DEFAULT 0,
  error_msg  TEXT
);
`

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", dsn, err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return db, nil
}
