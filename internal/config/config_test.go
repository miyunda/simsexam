package config

import "testing"

func TestLoadServerConfigDefaults(t *testing.T) {
	t.Setenv(EnvAddr, "")
	t.Setenv(EnvDBPath, "")
	t.Setenv(EnvAdminPassword, "")
	t.Setenv(EnvAdminSessionKey, "")
	t.Setenv(EnvCookieSecure, "")

	cfg := LoadServerConfig()
	if cfg.Addr != DefaultAddr {
		t.Fatalf("expected default addr, got %q", cfg.Addr)
	}
	if cfg.DBPath != DefaultDBPath {
		t.Fatalf("expected default db path, got %q", cfg.DBPath)
	}
	if cfg.AdminPassword != "" {
		t.Fatalf("expected empty admin password by default, got %q", cfg.AdminPassword)
	}
	if cfg.AdminSessionSecret != "" {
		t.Fatalf("expected empty admin session secret by default, got %q", cfg.AdminSessionSecret)
	}
	if cfg.CookieSecure {
		t.Fatal("expected cookie secure to default false")
	}
}

func TestLoadServerConfigFromEnv(t *testing.T) {
	t.Setenv(EnvAddr, "127.0.0.1:7000")
	t.Setenv(EnvDBPath, "/tmp/se.db")
	t.Setenv(EnvAdminPassword, "admin-pass")
	t.Setenv(EnvAdminSessionKey, "session-secret")
	t.Setenv(EnvCookieSecure, "true")

	cfg := LoadServerConfig()
	if cfg.Addr != "127.0.0.1:7000" {
		t.Fatalf("expected env addr, got %q", cfg.Addr)
	}
	if cfg.DBPath != "/tmp/se.db" {
		t.Fatalf("expected env db path, got %q", cfg.DBPath)
	}
	if cfg.AdminPassword != "admin-pass" {
		t.Fatalf("expected env admin password, got %q", cfg.AdminPassword)
	}
	if cfg.AdminSessionSecret != "session-secret" {
		t.Fatalf("expected env admin session secret, got %q", cfg.AdminSessionSecret)
	}
	if !cfg.CookieSecure {
		t.Fatal("expected cookie secure true from env")
	}
}

func TestLoadRuntimeConfigDefaults(t *testing.T) {
	t.Setenv(EnvDBPath, "")

	cfg := LoadRuntimeConfig()
	if cfg.DBPath != DefaultDBPath {
		t.Fatalf("expected default db path, got %q", cfg.DBPath)
	}
}

func TestLoadImportConfigDefaults(t *testing.T) {
	t.Setenv(EnvDBPath, "")
	t.Setenv(EnvImportSourceType, "")

	cfg := LoadImportConfig()
	if cfg.DBPath != DefaultDBPath {
		t.Fatalf("expected default db path, got %q", cfg.DBPath)
	}
	if cfg.SourceType != DefaultImportSourceType {
		t.Fatalf("expected default source type, got %q", cfg.SourceType)
	}
}

func TestLoadImportConfigFromEnv(t *testing.T) {
	t.Setenv(EnvDBPath, "/tmp/import.db")
	t.Setenv(EnvImportSourceType, "seed")

	cfg := LoadImportConfig()
	if cfg.DBPath != "/tmp/import.db" {
		t.Fatalf("expected env db path, got %q", cfg.DBPath)
	}
	if cfg.SourceType != "seed" {
		t.Fatalf("expected env source type, got %q", cfg.SourceType)
	}
}
