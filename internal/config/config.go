// Package config 负责配置加载，优先级是命令行 flag > 环境变量 > 代码默认值。
package config

import (
	"flag"
	"fmt"
	"io"
	"time"
)

// Config 是服务的全部运行配置。
type Config struct {
	Addr       string
	DB         string
	Uploads    string
	Static     string
	Dev        bool
	Secret     string
	SessionTTL time.Duration
}

// Load 读出配置。args 是去掉程序名之后的参数，getenv 用来读环境变量（测试时换成自己的实现）。
func Load(args []string, getenv func(string) string) (*Config, error) {
	cfg := &Config{
		Addr:       "127.0.0.1:8091",
		DB:         "data.db",
		Uploads:    "uploads",
		Static:     "static",
		SessionTTL: 168 * time.Hour,
	}

	cfg.Addr = envOr(getenv, "MIRAGE_ADDR", cfg.Addr)
	cfg.DB = envOr(getenv, "MIRAGE_DB", cfg.DB)
	cfg.Uploads = envOr(getenv, "MIRAGE_UPLOADS", cfg.Uploads)
	cfg.Static = envOr(getenv, "MIRAGE_STATIC", cfg.Static)
	cfg.Secret = getenv("MIRAGE_SECRET")

	if v := getenv("MIRAGE_SESSION_TTL"); v != "" {
		ttl, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse MIRAGE_SESSION_TTL %q: %w", v, err)
		}
		cfg.SessionTTL = ttl
	}

	// flag 的默认值取的是环境变量生效之后的值，所以没传 flag 时就保持环境变量的结果。
	fs := flag.NewFlagSet("mirage-server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	fs.StringVar(&cfg.DB, "db", cfg.DB, "database file")
	fs.StringVar(&cfg.Uploads, "uploads", cfg.Uploads, "upload directory")
	fs.StringVar(&cfg.Static, "static", cfg.Static, "static source directory")
	fs.BoolVar(&cfg.Dev, "dev", cfg.Dev, "development mode")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if cfg.Addr == "" {
		return nil, fmt.Errorf("addr must not be empty")
	}
	if cfg.SessionTTL <= 0 {
		return nil, fmt.Errorf("session ttl must be positive, got %s", cfg.SessionTTL)
	}
	return cfg, nil
}

func envOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}
