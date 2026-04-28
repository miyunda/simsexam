package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"simsexam/internal/config"
)

func TestNewServerAppBootstrapsAndServesHome(t *testing.T) {
	changeToRepoRoot(t)

	cfg := config.ServerConfig{
		RuntimeConfig: config.RuntimeConfig{
			DBPath: filepath.Join(t.TempDir(), "app.db"),
		},
		Addr:               config.DefaultAddr,
		AdminPassword:      "admin-pass",
		AdminSessionSecret: "admin-session-secret",
	}

	serverApp, err := NewServerApp(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewServerApp failed: %v", err)
	}
	defer serverApp.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	serverApp.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func changeToRepoRoot(t *testing.T) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(original, "..", ".."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir to repo root failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
}
