package config

import "testing"

func TestLoadServerConfigDefaults(t *testing.T) {
	t.Setenv(EnvAddr, "")
	t.Setenv(EnvDBPath, "")

	cfg := LoadServerConfig()
	if cfg.Addr != DefaultAddr {
		t.Fatalf("expected default addr, got %q", cfg.Addr)
	}
	if cfg.DBPath != DefaultDBPath {
		t.Fatalf("expected default db path, got %q", cfg.DBPath)
	}
}

func TestLoadServerConfigFromEnv(t *testing.T) {
	t.Setenv(EnvAddr, "127.0.0.1:7000")
	t.Setenv(EnvDBPath, "/tmp/se.db")

	cfg := LoadServerConfig()
	if cfg.Addr != "127.0.0.1:7000" {
		t.Fatalf("expected env addr, got %q", cfg.Addr)
	}
	if cfg.DBPath != "/tmp/se.db" {
		t.Fatalf("expected env db path, got %q", cfg.DBPath)
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
