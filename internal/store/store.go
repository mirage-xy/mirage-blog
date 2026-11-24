// Package store 是数据访问层：全项目唯一写 SQL 的地方。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // 纯 Go 的 SQLite 驱动，注册 "sqlite"
)

// readPoolSize 是读池的连接数。写池固定 1：SQLite 是整库级写锁，
// 同时只有一个写者，多开连接只会排队和互相打断。
const readPoolSize = 4

// Store 持有两条独立的连接池，指向同一个数据库文件。
// 读走 readDB，写走 writeDB——写成两条是为了让「写串行」由连接池保证，
// 而不是靠事务里排队。
type Store struct {
	readDB  *sql.DB
	writeDB *sql.DB
}

// Open 打开数据库文件，装配两条连接池，并确认 WAL 真的生效。
func Open(ctx context.Context, path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve db path: %w", err)
	}
	// 父目录必须已经存在。生产上那目录归部署脚本或 systemd 管；
	// 这里不自动 MkdirAll，是为了让「路径写错了」当场报错，
	// 而不是在意外的地方悄悄建出一个空库。
	if _, err := os.Stat(filepath.Dir(abs)); err != nil {
		return nil, fmt.Errorf("db directory: %w", err)
	}

	// 连接级 PRAGMA 一律写进 DSN。写成 db.Exec("PRAGMA ...") 只作用于
	// 当时借出的那一条连接，池里后来开的连接全都没生效——外键会随机失效。
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_txlock", "immediate") // 事务一开就拿写锁，不等第一次写
	dsn := "file:" + abs + "?" + q.Encode()

	readDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read db: %w", err)
	}
	readDB.SetMaxOpenConns(readPoolSize)
	readDB.SetMaxIdleConns(readPoolSize)

	writeDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		readDB.Close()
		return nil, fmt.Errorf("open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	s := &Store{readDB: readDB, writeDB: writeDB}
	if err := s.checkWAL(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// checkWAL 校验 journal_mode 真的是 wal。请求 WAL ≠ VFS 真的接受了，
// 所以要回读一次——否则性能假设全是空的。
func (s *Store) checkWAL(ctx context.Context) error {
	var mode string
	if err := s.readDB.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("WAL not enabled, got %q", mode)
	}
	return nil
}

// Close 关掉两条连接池。先关写池，让写事务有机会收尾。
func (s *Store) Close() error {
	return errors.Join(s.writeDB.Close(), s.readDB.Close())
}
