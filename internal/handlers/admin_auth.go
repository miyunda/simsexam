package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"simsexam/internal/config"
)

const adminSessionCookieName = "simsexam_admin_session"
const adminSessionTTL = 24 * time.Hour
const adminLoginMaxFailures = 5
const adminLoginWindow = 10 * time.Minute
const adminLoginBlockDuration = 10 * time.Minute

var defaultAdminLoginRateLimiter = newAdminLoginRateLimiter(
	time.Now,
	adminLoginMaxFailures,
	adminLoginWindow,
	adminLoginBlockDuration,
)

type adminLoginPageData struct {
	Error string
}

type adminLoginFailureRecord struct {
	FailCount    int
	FirstFailAt  time.Time
	LastFailAt   time.Time
	BlockedUntil time.Time
}

type adminLoginRateLimiter struct {
	mu            sync.Mutex
	now           func() time.Time
	maxFailures   int
	window        time.Duration
	blockDuration time.Duration
	records       map[string]adminLoginFailureRecord
}

func AdminLoginForm(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminAccessConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderTemplate(w, "admin_login.html", adminLoginPageData{
				Error: "Admin access is not configured.",
			})
			return
		}
		renderTemplate(w, "admin_login.html", adminLoginPageData{})
	}
}

func AdminLoginSubmit(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminAccessConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderTemplate(w, "admin_login.html", adminLoginPageData{
				Error: "Admin access is not configured.",
			})
			return
		}

		clientIP := clientIPFromRequest(r)
		if allowed, blockedUntil := defaultAdminLoginRateLimiter.Allow(clientIP); !allowed {
			logAdminLoginAttempt("blocked", clientIP, r, blockedUntil)
			w.WriteHeader(http.StatusTooManyRequests)
			renderTemplate(w, "admin_login.html", adminLoginPageData{
				Error: "Too many failed login attempts. Please try again later.",
			})
			return
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			renderTemplate(w, "admin_login.html", adminLoginPageData{
				Error: "Failed to parse login form.",
			})
			return
		}

		password := r.FormValue("password")
		if password != cfg.AdminPassword {
			_, blockedUntil := defaultAdminLoginRateLimiter.RegisterFailure(clientIP)
			logAdminLoginAttempt("invalid_password", clientIP, r, blockedUntil)
			w.WriteHeader(http.StatusUnauthorized)
			renderTemplate(w, "admin_login.html", adminLoginPageData{
				Error: "Invalid password.",
			})
			return
		}
		defaultAdminLoginRateLimiter.Clear(clientIP)

		token, err := signAdminSession(time.Now().Add(adminSessionTTL), cfg.AdminSessionSecret)
		if err != nil {
			http.Error(w, "Failed to create admin session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     adminSessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(adminSessionTTL),
			MaxAge:   int(adminSessionTTL.Seconds()),
		})

		http.Redirect(w, r, "/admin/subjects", http.StatusSeeOther)
	}
}

func AdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func AdminAuthMiddleware(cfg config.ServerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !adminAccessConfigured(cfg) {
				http.Error(w, "Admin access is not configured.", http.StatusServiceUnavailable)
				return
			}
			cookie, err := r.Cookie(adminSessionCookieName)
			if err != nil || !verifyAdminSession(cookie.Value, cfg.AdminSessionSecret, time.Now()) {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func adminAccessConfigured(cfg config.ServerConfig) bool {
	return cfg.AdminPassword != "" && cfg.AdminSessionSecret != ""
}

func newAdminLoginRateLimiter(now func() time.Time, maxFailures int, window, blockDuration time.Duration) *adminLoginRateLimiter {
	return &adminLoginRateLimiter{
		now:           now,
		maxFailures:   maxFailures,
		window:        window,
		blockDuration: blockDuration,
		records:       make(map[string]adminLoginFailureRecord),
	}
}

func (l *adminLoginRateLimiter) Allow(clientIP string) (bool, time.Time) {
	if clientIP == "" {
		clientIP = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	record, ok := l.records[clientIP]
	if !ok {
		return true, time.Time{}
	}
	if record.BlockedUntil.After(now) {
		return false, record.BlockedUntil
	}
	if !record.FirstFailAt.IsZero() && now.Sub(record.FirstFailAt) > l.window {
		delete(l.records, clientIP)
	}
	return true, time.Time{}
}

func (l *adminLoginRateLimiter) RegisterFailure(clientIP string) (int, time.Time) {
	if clientIP == "" {
		clientIP = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	record := l.records[clientIP]
	if record.FirstFailAt.IsZero() || now.Sub(record.FirstFailAt) > l.window {
		record = adminLoginFailureRecord{
			FirstFailAt: now,
		}
	}

	record.FailCount++
	record.LastFailAt = now
	if record.FailCount >= l.maxFailures {
		record.BlockedUntil = now.Add(l.blockDuration)
	}
	l.records[clientIP] = record
	return record.FailCount, record.BlockedUntil
}

func (l *adminLoginRateLimiter) Clear(clientIP string) {
	if clientIP == "" {
		clientIP = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, clientIP)
}

func clientIPFromRequest(r *http.Request) string {
	if ip := normalizeIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			if ip := normalizeIP(part); ip != "" {
				return ip
			}
		}
	}
	if ip := normalizeIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		if ip := normalizeIP(host); ip != "" {
			return ip
		}
	}
	return normalizeIP(r.RemoteAddr)
}

func normalizeIP(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func logAdminLoginAttempt(outcome, clientIP string, r *http.Request, blockedUntil time.Time) {
	log.Printf(
		"admin login %s client_ip=%q remote_addr=%q cf_connecting_ip=%q x_forwarded_for=%q x_real_ip=%q blocked_until=%q",
		outcome,
		clientIP,
		r.RemoteAddr,
		r.Header.Get("CF-Connecting-IP"),
		r.Header.Get("X-Forwarded-For"),
		r.Header.Get("X-Real-IP"),
		blockedUntil.Format(time.RFC3339),
	)
}

func signAdminSession(expiresAt time.Time, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("empty admin session secret")
	}
	payload := strconv.FormatInt(expiresAt.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", err
	}
	signature := hex.EncodeToString(mac.Sum(nil))
	token := payload + "." + signature
	return base64.RawURLEncoding.EncodeToString([]byte(token)), nil
}

func verifyAdminSession(token, secret string, now time.Time) bool {
	if token == "" || secret == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 2 {
		return false
	}

	expiresAtUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if now.Unix() > expiresAtUnix {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(parts[0])); err != nil {
		return false
	}
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}
