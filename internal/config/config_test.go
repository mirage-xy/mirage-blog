package config

import (
	"errors"
	"flag"
	"testing"
	"time"
)

func envOf(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(nil, envOf(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != "127.0.0.1:8091" {
		t.Errorf("Addr = %q, want 127.0.0.1:8091", cfg.Addr)
	}
	if cfg.DB != "data.db" {
		t.Errorf("DB = %q, want data.db", cfg.DB)
	}
	if cfg.Uploads != "uploads" {
		t.Errorf("Uploads = %q, want uploads", cfg.Uploads)
	}
	if cfg.Static != "static" {
		t.Errorf("Static = %q, want static", cfg.Static)
	}
	if cfg.Dev {
		t.Error("Dev = true, want false")
	}
	if cfg.Secret != "" {
		t.Errorf("Secret = %q, want empty", cfg.Secret)
	}
	if cfg.SessionTTL != 168*time.Hour {
		t.Errorf("SessionTTL = %s, want 168h", cfg.SessionTTL)
	}
}

func TestLoadEnvOverridesDefaults(t *testing.T) {
	env := map[string]string{
		"MIRAGE_ADDR":        "127.0.0.1:9000",
		"MIRAGE_DB":          "/tmp/mirage.db",
		"MIRAGE_UPLOADS":     "/tmp/uploads",
		"MIRAGE_STATIC":      "/tmp/static",
		"MIRAGE_SESSION_TTL": "1h30m",
		"MIRAGE_SECRET":      "s3cret",
	}
	cfg, err := Load(nil, envOf(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9000" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.DB != "/tmp/mirage.db" {
		t.Errorf("DB = %q", cfg.DB)
	}
	if cfg.Uploads != "/tmp/uploads" {
		t.Errorf("Uploads = %q", cfg.Uploads)
	}
	if cfg.Static != "/tmp/static" {
		t.Errorf("Static = %q", cfg.Static)
	}
	if cfg.SessionTTL != 90*time.Minute {
		t.Errorf("SessionTTL = %s, want 1h30m", cfg.SessionTTL)
	}
	if cfg.Secret != "s3cret" {
		t.Errorf("Secret = %q", cfg.Secret)
	}
}

func TestLoadFlagOverridesEnv(t *testing.T) {
	env := map[string]string{
		"MIRAGE_ADDR": "127.0.0.1:9000",
		"MIRAGE_DB":   "/tmp/mirage.db",
	}
	args := []string{"-addr", "127.0.0.1:9100", "-db", "flag.db", "-dev"}
	cfg, err := Load(args, envOf(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9100" {
		t.Errorf("Addr = %q, want flag value", cfg.Addr)
	}
	if cfg.DB != "flag.db" {
		t.Errorf("DB = %q, want flag value", cfg.DB)
	}
	if !cfg.Dev {
		t.Error("Dev = false, want true")
	}
}

func TestLoadSecretOnlyFromEnv(t *testing.T) {
	if _, err := Load([]string{"-secret", "x"}, envOf(nil)); err == nil {
		t.Fatal("Load with -secret should fail, secret must not be a flag")
	}

	cfg, err := Load(nil, envOf(map[string]string{"MIRAGE_SECRET": "abc"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Secret != "abc" {
		t.Errorf("Secret = %q, want abc", cfg.Secret)
	}
}

func TestLoadInvalidSessionTTL(t *testing.T) {
	if _, err := Load(nil, envOf(map[string]string{"MIRAGE_SESSION_TTL": "7 days"})); err == nil {
		t.Fatal("Load with invalid MIRAGE_SESSION_TTL should fail")
	}
}

func TestLoadRejectsEmptyAddr(t *testing.T) {
	if _, err := Load([]string{"-addr", ""}, envOf(nil)); err == nil {
		t.Fatal("Load with empty addr should fail")
	}
}

func TestLoadHelp(t *testing.T) {
	if _, err := Load([]string{"-h"}, envOf(nil)); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("Load(-h) err = %v, want flag.ErrHelp", err)
	}
}
