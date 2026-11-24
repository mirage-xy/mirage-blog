package store

import (
	"context"
	"path/filepath"
	"testing"

	"mirage/migrations"
)

// newTestStore 在临时目录里开一个真实的库文件。
// 不用 :memory:——那玩意每个连接一份独立内存库，连接池里换条连接就 "no such table"。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func tableExists(t *testing.T, s *Store, name string) bool {
	t.Helper()
	var n int
	err := s.readDB.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name,
	).Scan(&n)
	if err != nil {
		t.Fatalf("query sqlite_master(%s): %v", name, err)
	}
	return n > 0
}

func countRows(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.readDB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func TestOpenAndMigrateCreateSchema(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, name := range []string{"users", "sessions", "sections", "posts", "schema_migrations"} {
		if !tableExists(t, s, name) {
			t.Errorf("表 %s 没建出来", name)
		}
	}

	if n := countRows(t, s, "SELECT COUNT(*) FROM schema_migrations"); n != 1 {
		t.Errorf("schema_migrations 有 %d 条记录，想要 1 条", n)
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM schema_migrations WHERE version = 1"); n != 1 {
		t.Errorf("001_init 没被记进 schema_migrations")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("第一次 Migrate: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("第二次 Migrate 应该什么都不做、也不报错: %v", err)
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM schema_migrations"); n != 1 {
		t.Errorf("重跑之后记录数 = %d，想要还是 1", n)
	}
}

func TestJournalModeIsWAL(t *testing.T) {
	s := newTestStore(t)

	var mode string
	if err := s.readDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q，想要 wal", mode)
	}
}

func TestOpenRejectsMissingDirectory(t *testing.T) {
	// 父目录不存在就该当场报错，不在意外的地方建库。
	path := filepath.Join(t.TempDir(), "not-there", "data.db")
	if s, err := Open(context.Background(), path); err == nil {
		s.Close()
		t.Fatal("父目录不存在却 Open 成功了")
	}
}

func TestApplyRollsBackOnBadMigration(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// 前半句是合法 DDL，后半句是语法错误：如果 DDL 不是事务性的，
	// broken 这张表会残留下来。
	bad := []migrations.File{{
		Version: 7,
		Name:    "007_broken.sql",
		SQL:     "CREATE TABLE broken (id INTEGER);\nTHIS IS NOT VALID SQL;",
	}}

	if err := s.apply(ctx, bad); err == nil {
		t.Fatal("坏迁移竟然执行成功了")
	}
	if tableExists(t, s, "broken") {
		t.Error("坏迁移里建的表没被回滚，DDL 事务性没生效")
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM schema_migrations WHERE version = 7"); n != 0 {
		t.Errorf("坏迁移被记进了 schema_migrations（%d 条）", n)
	}
}
