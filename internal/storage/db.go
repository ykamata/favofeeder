package storage

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// schema statements executed in order on every Open.
// Using CREATE TABLE IF NOT EXISTS + inline keys for idempotency.
var schema = []string{
	`CREATE TABLE IF NOT EXISTS content_items (
  id           INT          NOT NULL AUTO_INCREMENT,
  title        VARCHAR(255) NOT NULL,
  category     VARCHAR(50)  NOT NULL,
  source_url   TEXT         NOT NULL,
  source_type  VARCHAR(50)  NOT NULL,
  content      TEXT         NOT NULL,
  content_hash VARCHAR(64)  NOT NULL,
  crawled_at   DATETIME     NOT NULL,
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_content_hash (content_hash),
  KEY idx_title    (title(191)),
  KEY idx_category (category),
  KEY idx_crawled  (crawled_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS crawl_logs (
  id         INT         NOT NULL AUTO_INCREMENT,
  started_at DATETIME    NOT NULL,
  ended_at   DATETIME,
  status     VARCHAR(20) NOT NULL,
  items_new  INT         NOT NULL DEFAULT 0,
  items_dup  INT         NOT NULL DEFAULT 0,
  error_msg  TEXT,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

func Open(dsn string) (*sql.DB, error) {
	dsn = withParseTime(dsn)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate schema: %w", err)
		}
	}
	return db, nil
}

// withParseTime ensures parseTime=true is in the DSN so DATETIME columns
// scan correctly into time.Time.
func withParseTime(dsn string) string {
	if strings.Contains(dsn, "parseTime=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}
