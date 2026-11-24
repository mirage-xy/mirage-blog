// Package migrations 只做一件事：把 SQL 迁移文件嵌进二进制，并按版本号排好序交出去。
//
// //go:embed 只能嵌入「当前包目录及其子目录」下的文件，所以这段指令必须和
// .sql 文件待在同一个目录里——写在 cmd/mirage-server 下是嵌不到仓库根的
// migrations/ 的。
package migrations

import (
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// sqlFS 是编译期就把迁移文件打进来的只读文件系统。
//
//go:embed *.sql
var sqlFS embed.FS

// File 是一个待执行的迁移。Version 取自文件名开头的序号。
type File struct {
	Version int    // 1、2、3……
	Name    string // 文件名，如 001_init.sql
	SQL     string // 文件全部内容
}

// Load 读出全部迁移，按版本号升序返回。
//
// 版本号重复或文件名不合规都会直接报错——迁移是表结构的版本历史，
// 顺序错乱比少一个文件更难查。
func Load() ([]File, error) {
	entries, err := sqlFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	files := make([]File, 0, len(entries))
	seen := make(map[int]string, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}

		version, err := parseVersion(name)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("duplicate migration version %d: %s 和 %s", version, prev, name)
		}
		seen[version] = name

		body, err := sqlFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files = append(files, File{Version: version, Name: name, SQL: string(body)})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	return files, nil
}

// parseVersion 从 NNN_description.sql 里取出 NNN。
func parseVersion(name string) (int, error) {
	base := strings.TrimSuffix(name, ".sql")
	sep := strings.IndexByte(base, '_')
	if sep <= 0 {
		return 0, fmt.Errorf("migration %q: 命名要是 NNN_description.sql", name)
	}
	version, err := strconv.Atoi(base[:sep])
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration %q: %q 不是正整数版本号", name, base[:sep])
	}
	return version, nil
}
