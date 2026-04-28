package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"simsexam/internal/config"
)

func TestClientIPFromRequestPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.10")
	req.Header.Set("X-Forwarded-For", "203.0.113.20, 127.0.0.1")
	req.Header.Set("X-Real-IP", "203.0.113.30")

	got := clientIPFromRequest(req)
	if got != "198.51.100.10" {
		t.Fatalf("expected CF-Connecting-IP to win, got %q", got)
	}
}

func TestClientIPFromRequestParsesXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", " bad-value , 203.0.113.20, 10.0.0.1 ")

	got := clientIPFromRequest(req)
	if got != "203.0.113.20" {
		t.Fatalf("expected first valid X-Forwarded-For IP, got %q", got)
	}
}

func TestAdminLoginRateLimiterBlocksAfterFailures(t *testing.T) {
	current := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	limiter := newAdminLoginRateLimiter(func() time.Time { return current }, 5, 10*time.Minute, 10*time.Minute)

	for i := 0; i < 5; i++ {
		failCount, blockedUntil := limiter.RegisterFailure("203.0.113.10")
		if failCount != i+1 {
			t.Fatalf("expected fail count %d, got %d", i+1, failCount)
		}
		if i < 4 && !blockedUntil.IsZero() {
			t.Fatalf("did not expect blocked_until before threshold, got %v", blockedUntil)
		}
	}

	allowed, blockedUntil := limiter.Allow("203.0.113.10")
	if allowed {
		t.Fatal("expected client to be blocked after threshold failures")
	}
	if blockedUntil.IsZero() {
		t.Fatal("expected non-zero blocked_until")
	}
}

func TestAdminLoginRateLimiterClearRemovesFailureState(t *testing.T) {
	current := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	limiter := newAdminLoginRateLimiter(func() time.Time { return current }, 5, 10*time.Minute, 10*time.Minute)

	limiter.RegisterFailure("203.0.113.10")
	limiter.RegisterFailure("203.0.113.10")
	limiter.Clear("203.0.113.10")

	allowed, blockedUntil := limiter.Allow("203.0.113.10")
	if !allowed {
		t.Fatalf("expected client to be allowed after clear, blocked until %v", blockedUntil)
	}
}

func TestAdminLoginSubmitBlocksAfterRepeatedFailures(t *testing.T) {
	changeToRepoRootForAdminAuthTest(t)

	cfg := config.ServerConfig{
		Addr:               config.DefaultAddr,
		AdminPassword:      "admin-pass",
		AdminSessionSecret: "admin-session-secret",
	}

	current := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	oldLimiter := defaultAdminLoginRateLimiter
	defaultAdminLoginRateLimiter = newAdminLoginRateLimiter(func() time.Time { return current }, 5, 10*time.Minute, 10*time.Minute)
	defer func() {
		defaultAdminLoginRateLimiter = oldLimiter
	}()

	handler := AdminLoginSubmit(cfg)
	for i := 0; i < 5; i++ {
		form := url.Values{}
		form.Set("password", "wrong-pass")
		req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("CF-Connecting-IP", "198.51.100.50")
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rec.Code)
		}
	}

	form := url.Values{}
	form.Set("password", "wrong-pass")
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("CF-Connecting-IP", "198.51.100.50")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after threshold is reached, got %d", rec.Code)
	}
}

func TestAdminLoginSubmitSuccessClearsFailureState(t *testing.T) {
	changeToRepoRootForAdminAuthTest(t)

	cfg := config.ServerConfig{
		Addr:               config.DefaultAddr,
		AdminPassword:      "admin-pass",
		AdminSessionSecret: "admin-session-secret",
	}

	current := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	oldLimiter := defaultAdminLoginRateLimiter
	defaultAdminLoginRateLimiter = newAdminLoginRateLimiter(func() time.Time { return current }, 5, 10*time.Minute, 10*time.Minute)
	defer func() {
		defaultAdminLoginRateLimiter = oldLimiter
	}()

	handler := AdminLoginSubmit(cfg)
	defaultAdminLoginRateLimiter.RegisterFailure("198.51.100.60")
	defaultAdminLoginRateLimiter.RegisterFailure("198.51.100.60")

	form := url.Values{}
	form.Set("password", "admin-pass")
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("CF-Connecting-IP", "198.51.100.60")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful login, got %d", rec.Code)
	}

	allowed, blockedUntil := defaultAdminLoginRateLimiter.Allow("198.51.100.60")
	if !allowed {
		t.Fatalf("expected limiter state to clear after successful login, blocked until %v", blockedUntil)
	}
}

func changeToRepoRootForAdminAuthTest(t *testing.T) {
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
