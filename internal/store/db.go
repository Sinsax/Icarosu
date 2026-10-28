package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	if driver == "sqlite" {
		dsn = sqliteDSN(dsn)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping store: %w", err)
	}

	if driver == "sqlite" {
		// 这些 PRAGMA 是连接级的；必须写进 DSN，否则连接池中新开的连接
		// 不会继承 foreign_keys/busy_timeout。
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(4)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func sqliteDSN(dsn string) string {
	if dsn == "" {
		dsn = "data/bot.db"
	}
	if dsn == ":memory:" {
		dsn = "file::memory:?cache=shared"
	}
	if !strings.HasPrefix(dsn, "file:") && !strings.Contains(dsn, "mode=memory") {
		dsn = "file:" + dsn
	}
	if !strings.Contains(dsn, "_pragma=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	}
	return dsn
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec("CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)"); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var version int
	if err := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	migrations := []struct {
		version    int
		statements []string
	}{
		{1, allSchemaStatements()},
		{2, v2SchemaStatements()},
		{3, v3SchemaStatements()},
		{4, v4SchemaStatements()},
	}
	for _, m := range migrations {
		if version >= m.version {
			continue
		}
		if err := s.applyMigration(m.version, m.statements); err != nil {
			return err
		}
		version = m.version
	}
	return nil
}

// applyMigration 在单个事务里执行一个版本的 DDL + 版本号写入。
// 失败会整体回滚，避免留下“迁移到一半”的 schema。
func (s *Store) applyMigration(version int, statements []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migrate v%d: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrate store to v%d: %w", version, err)
		}
	}
	if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("write schema version v%d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrate v%d: %w", version, err)
	}
	return nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}
