package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"icarosu/internal/config"
)

// Store 是 SQLite 长期存储。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库。
func Open(cfg config.StoreConfig) (*Store, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = "sqlite"
	}
	dsn := cfg.DSN
	if dsn == "" {
		dsn = "data/bot.db"
	}

	// SQLite 需要确保数据库文件所在目录存在
	if dir := filepath.Dir(dsn); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create store dir %q: %w", dir, err)
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping store: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var version int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if version < 1 {
		for _, stmt := range allSchemaStatements() {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate store to v1: %w", err)
			}
		}
		if _, err := s.db.Exec(`INSERT INTO schema_version (version) VALUES (1)`); err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
		version = 1
	}

	if version < 2 {
		for _, stmt := range v2SchemaStatements() {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate store to v2: %w", err)
			}
		}
		if _, err := s.db.Exec(`INSERT INTO schema_version (version) VALUES (2)`); err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
		version = 2
	}

	if version < 3 {
		for _, stmt := range v3SchemaStatements() {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate store to v3: %w", err)
			}
		}
		if _, err := s.db.Exec(`INSERT INTO schema_version (version) VALUES (3)`); err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
	}

	if version < 4 {
		for _, stmt := range v4SchemaStatements() {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate store to v4: %w", err)
			}
		}
		if _, err := s.db.Exec(`INSERT INTO schema_version (version) VALUES (4)`); err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
	}

	return nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}