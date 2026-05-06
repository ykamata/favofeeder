package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with the driver name so queries can be adapted per dialect.
type DB struct {
	*sql.DB
	driver string
}

var mysqlSchema = []string{
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

var sqliteSchema = []string{
	`CREATE TABLE IF NOT EXISTS content_items (
  id           INTEGER NOT NULL PRIMARY KEY,
  title        TEXT    NOT NULL,
  category     TEXT    NOT NULL,
  source_url   TEXT    NOT NULL,
  source_type  TEXT    NOT NULL,
  content      TEXT    NOT NULL,
  content_hash TEXT    NOT NULL,
  crawled_at   TEXT    NOT NULL,
  created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  UNIQUE (content_hash)
)`,
	`CREATE INDEX IF NOT EXISTS idx_title    ON content_items (title)`,
	`CREATE INDEX IF NOT EXISTS idx_category ON content_items (category)`,
	`CREATE INDEX IF NOT EXISTS idx_crawled  ON content_items (crawled_at)`,
	`CREATE TABLE IF NOT EXISTS crawl_logs (
  id         INTEGER NOT NULL PRIMARY KEY,
  started_at TEXT    NOT NULL,
  ended_at   TEXT,
  status     TEXT    NOT NULL,
  items_new  INTEGER NOT NULL DEFAULT 0,
  items_dup  INTEGER NOT NULL DEFAULT 0,
  error_msg  TEXT
)`,
}

// Open opens a database connection for the given driver ("mysql" or "sqlite").
func Open(dsn, driver string) (*DB, error) {
	switch driver {
	case "sqlite":
		return openSQLite(dsn)
	default:
		return openMySQL(dsn)
	}
}

func openMySQL(dsn string) (*DB, error) {
	dsn = withParseTime(dsn)
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return applySchema(&DB{DB: sqlDB, driver: "mysql"}, mysqlSchema)
}

func openSQLite(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return applySchema(&DB{DB: sqlDB, driver: "sqlite"}, sqliteSchema)
}

func applySchema(db *DB, stmts []string) (*DB, error) {
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate schema: %w", err)
		}
	}
	return db, nil
}

// insertIgnorePrefix returns the dialect-specific INSERT IGNORE variant.
func (db *DB) insertIgnorePrefix() string {
	if db.driver == "sqlite" {
		return "INSERT OR IGNORE"
	}
	return "INSERT IGNORE"
}

// timeVal formats a time.Time for use in query parameters.
// SQLite has no native datetime type; values are stored as RFC3339 text.
func (db *DB) timeVal(t time.Time) interface{} {
	if db.driver == "sqlite" {
		return t.UTC().Format(time.RFC3339)
	}
	return t.UTC()
}

// scanTime scans a DATETIME/TEXT column from row into time.Time.
// SQLite stores datetimes as RFC3339 text, MySQL returns time.Time directly.
func (db *DB) scanTime(row *sql.Row) (time.Time, error) {
	if db.driver == "sqlite" {
		var s string
		if err := row.Scan(&s); err != nil {
			return time.Time{}, err
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse sqlite time %q: %w", s, err)
		}
		return t, nil
	}
	var t time.Time
	err := row.Scan(&t)
	return t, err
}

// withParseTime ensures parseTime=true is in the MySQL DSN so DATETIME columns
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
