package store

import (
	"context"
	"fmt"
	"time"

	"mirage/migrations"
)

// schemaMigrationsDDL 由迁移器自己建，不写进迁移文件：
// 它得在「读已执行哪些迁移」之前就存在（D20 推论）。
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
)`

// Migrate 把还没跑过的迁移按版本号顺序执行。
//
// 开发和生产都自动跑；任何一条失败就返回错误，调用方据此拒绝启动，
// 不带着半截 schema 继续跑（D20）。
func (s *Store) Migrate(ctx context.Context) error {
	files, err := migrations.Load()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	return s.apply(ctx, files)
}

// apply 是 Migrate 的实现，单独拆出来是为了测试能塞进一份坏迁移，
// 验证「失败整体回滚」而不是只验证正常路径。
func (s *Store) apply(ctx context.Context, files []migrations.File) error {
	if _, err := s.writeDB.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := s.appliedVersions(ctx)
	if err != nil {
		return err
	}

	for _, f := range files {
		if applied[f.Version] {
			continue
		}
		if err := s.applyOne(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

// appliedVersions 读出已经执行过的版本号。
func (s *Store) appliedVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := s.writeDB.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read applied versions: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan applied version: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied versions: %w", err)
	}
	return applied, nil
}

// applyOne 在一个事务里执行一条迁移：DDL 是事务性的，失败整体回滚，
// 不会留下半截 schema。DSN 里的 _txlock=immediate 让这里一开就拿到写锁。
func (s *Store) applyOne(ctx context.Context, f migrations.File) error {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin %s: %w", f.Name, err)
	}
	defer tx.Rollback() // 已经 Commit 之后再回滚是 no-op

	// 事务里只用 tx.*，不要用 s.writeDB.*，否则会跑到另一条连接上，事务就失效了。
	if _, err := tx.ExecContext(ctx, f.SQL); err != nil {
		return fmt.Errorf("apply %s: %w", f.Name, err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		f.Version, time.Now().Unix(),
	); err != nil {
		return fmt.Errorf("record %s: %w", f.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", f.Name, err)
	}
	return nil
}
