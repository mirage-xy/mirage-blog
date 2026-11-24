// mirage-server 是 mirage 博客的 HTTP 服务入口。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"mirage/internal/config"
	"mirage/internal/store"
	"mirage/internal/web"
)

const usage = `用法: mirage-server [选项]

  -addr string     监听地址（环境变量 MIRAGE_ADDR，默认 127.0.0.1:8091）
  -db string       数据库路径（环境变量 MIRAGE_DB，默认 data.db）
  -uploads string  附件目录（环境变量 MIRAGE_UPLOADS，默认 uploads）
  -static string   静态资源根目录（环境变量 MIRAGE_STATIC，默认 static）
  -dev             开发模式：日志改用 Text，便于人读

配置优先级：命令行 flag > 环境变量 > 默认值。签名密钥只从 MIRAGE_SECRET 读，不给 flag。
`

func main() {
	cfg, err := config.Load(os.Args[1:], os.Getenv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(os.Stdout, usage)
			return
		}
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n\n%s", err, usage)
		os.Exit(2)
	}

	logger := newLogger(cfg.Dev)
	slog.SetDefault(logger)

	// M1 还没有签名逻辑，密钥缺失只提示；M5 起改成拒绝启动。
	if cfg.Secret == "" {
		logger.Warn("MIRAGE_SECRET is not set")
	}

	// listener 提前创建：端口被占时立刻失败，不用等第一个请求进来才发现。
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Error("listen", "addr", cfg.Addr, "err", err)
		os.Exit(1)
	}

	// 数据层：开库 + 把没跑过的迁移按顺序跑掉。
	// 迁移失败就拒绝启动，不带着半截 schema 继续跑（D20）。
	st, err := store.Open(context.Background(), cfg.DB)
	if err != nil {
		logger.Error("open database", "path", cfg.DB, "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		logger.Error("migrate database", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("server starting", "addr", ln.Addr().String(), "db", cfg.DB, "dev", cfg.Dev)
	srv := web.New(ln.Addr().String(), logger)
	if err := srv.Serve(ctx, ln); err != nil {
		logger.Error("server exit", "err", err)
		os.Exit(1)
	}
}

// newLogger 只往 stdout 写，轮转交给 journald。开发时用 Text，人眼好读。
func newLogger(dev bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if dev {
		opts.Level = slog.LevelDebug
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
