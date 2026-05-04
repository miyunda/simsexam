package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"simsexam/internal/config"
	"simsexam/internal/database"
)

const userSessionCookieName = "simsexam_user_session"
const userSessionTTL = 30 * 24 * time.Hour
const passwordHashIterations = 210000

type userAuthPageData struct {
	Error string
	Email string
}

type accountPageData struct {
	Email       string
	DisplayName string
	Role        string
	CreatedAt   string
}

func RegisterForm(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !userAuthConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderTemplate(w, "register.html", userAuthPageData{
				Error: "User login is not configured.",
			})
			return
		}
		renderTemplate(w, "register.html", userAuthPageData{})
	}
}

func RegisterSubmit(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !userAuthConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderTemplate(w, "register.html", userAuthPageData{
				Error: "User login is not configured.",
			})
			return
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			renderTemplate(w, "register.html", userAuthPageData{Error: "Failed to parse registration form."})
			return
		}

		email := normalizeEmail(r.FormValue("email"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		displayName := strings.TrimSpace(r.FormValue("display_name"))
		if err := validateRegistration(email, password, confirmPassword); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			renderTemplate(w, "register.html", userAuthPageData{Error: err.Error(), Email: email})
			return
		}
		if displayName == "" {
			displayName = displayNameFromEmail(email)
		}

		passwordHash, err := hashPassword(password)
		if err != nil {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}

		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		res, err := tx.Exec(`
			INSERT INTO users (email, display_name, password_hash)
			VALUES (?, ?, ?)
		`, email, displayName, passwordHash)
		if err != nil {
			w.WriteHeader(http.StatusConflict)
			renderTemplate(w, "register.html", userAuthPageData{Error: "An account with this email already exists.", Email: email})
			return
		}
		userID, err := res.LastInsertId()
		if err != nil {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`
			INSERT INTO user_identities (user_id, provider, provider_user_id, provider_email)
			VALUES (?, 'email', ?, ?)
		`, userID, email, email); err != nil {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
		if err := claimAnonymousHistoryTx(r, tx, int(userID)); err != nil {
			http.Error(w, "Failed to claim learning history", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}

		setUserSessionCookie(w, cfg, int(userID), cfg.UserSessionSecret)
		http.Redirect(w, r, "/me", http.StatusSeeOther)
	}
}

func LoginForm(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !userAuthConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderTemplate(w, "login.html", userAuthPageData{
				Error: "User login is not configured.",
			})
			return
		}
		renderTemplate(w, "login.html", userAuthPageData{})
	}
}

func LoginSubmit(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !userAuthConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			renderTemplate(w, "login.html", userAuthPageData{
				Error: "User login is not configured.",
			})
			return
		}
		clientIP := clientIPFromRequest(r)
		if allowed, blockedUntil := defaultUserLoginRateLimiter.Allow(clientIP); !allowed {
			log.Printf("user login blocked client_ip=%q blocked_until=%q", clientIP, blockedUntil.Format(time.RFC3339))
			w.WriteHeader(http.StatusTooManyRequests)
			renderTemplate(w, "login.html", userAuthPageData{
				Error: "Too many failed login attempts. Please try again later.",
			})
			return
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			renderTemplate(w, "login.html", userAuthPageData{Error: "Failed to parse login form."})
			return
		}

		email := normalizeEmail(r.FormValue("email"))
		password := r.FormValue("password")
		var (
			userID       int
			passwordHash sql.NullString
			status       string
		)
		err := database.DB.QueryRow(`
			SELECT id, password_hash, status
			FROM users
			WHERE email = ?
		`, email).Scan(&userID, &passwordHash, &status)
		if err != nil || status != "active" || !passwordHash.Valid || !verifyPassword(password, passwordHash.String) {
			_, blockedUntil := defaultUserLoginRateLimiter.RegisterFailure(clientIP)
			log.Printf("user login invalid_password client_ip=%q email=%q blocked_until=%q", clientIP, email, blockedUntil.Format(time.RFC3339))
			w.WriteHeader(http.StatusUnauthorized)
			renderTemplate(w, "login.html", userAuthPageData{Error: "Invalid email or password.", Email: email})
			return
		}
		defaultUserLoginRateLimiter.Clear(clientIP)

		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Failed to sign in", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if err := claimAnonymousHistoryTx(r, tx, userID); err != nil {
			http.Error(w, "Failed to claim learning history", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to sign in", http.StatusInternalServerError)
			return
		}

		setUserSessionCookie(w, cfg, userID, cfg.UserSessionSecret)
		http.Redirect(w, r, "/me", http.StatusSeeOther)
	}
}

func UserLogout(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, expiredSessionCookie(cfg, userSessionCookieName))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func AccountPage(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r, cfg)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var data accountPageData
		err := database.DB.QueryRow(`
			SELECT email, COALESCE(display_name, ''), role, created_at
			FROM users
			WHERE id = ? AND status = 'active'
		`, userID).Scan(&data.Email, &data.DisplayName, &data.Role, &data.CreatedAt)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		renderTemplate(w, "account.html", data)
	}
}

func userAuthConfigured(cfg config.ServerConfig) bool {
	return cfg.UserSessionSecret != ""
}

func validateRegistration(email, password, confirmPassword string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("Enter a valid email address.")
	}
	if len(password) < 8 {
		return fmt.Errorf("Password must be at least 8 characters.")
	}
	if password != confirmPassword {
		return fmt.Errorf("Passwords do not match.")
	}
	return nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func displayNameFromEmail(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return email
	}
	return local
}

func setUserSessionCookie(w http.ResponseWriter, cfg config.ServerConfig, userID int, secret string) {
	token, err := signUserSession(userID, time.Now().Add(userSessionTTL), secret)
	if err != nil {
		return
	}
	http.SetCookie(w, newSessionCookie(cfg, userSessionCookieName, token, time.Now().Add(userSessionTTL), int(userSessionTTL.Seconds())))
}

func currentUserID(r *http.Request, cfg config.ServerConfig) (int, bool) {
	cookie, err := r.Cookie(userSessionCookieName)
	if err != nil {
		return 0, false
	}
	return verifyUserSession(cookie.Value, cfg.UserSessionSecret, time.Now())
}

func claimAnonymousHistoryTx(r *http.Request, tx *sql.Tx, userID int) error {
	cookie, err := r.Cookie(anonymousSessionCookieName)
	if err != nil {
		return nil
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return nil
	}

	var (
		sessionID     int
		claimedUserID sql.NullInt64
	)
	err = tx.QueryRow(`
		SELECT id, claimed_user_id
		FROM anonymous_sessions
		WHERE token_hash = ?
	`, anonymousSessionTokenHash(token)).Scan(&sessionID, &claimedUserID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if claimedUserID.Valid && claimedUserID.Int64 != int64(userID) {
		return nil
	}

	if _, err := tx.Exec(`
		UPDATE exams
		SET user_id = ?
		WHERE anonymous_session_id = ? AND user_id IS NULL
	`, userID, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE question_feedback
		SET user_id = ?
		WHERE anonymous_session_id = ? AND user_id IS NULL
	`, userID, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE anonymous_sessions
		SET claimed_user_id = COALESCE(claimed_user_id, ?),
			claimed_at = COALESCE(claimed_at, CURRENT_TIMESTAMP),
			last_seen_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID, sessionID); err != nil {
		return err
	}
	if err := rebuildUserQuestionStatsTx(tx, userID); err != nil {
		return err
	}
	return nil
}

func signUserSession(userID int, expiresAt time.Time, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("empty user session secret")
	}
	payload := fmt.Sprintf("%d.%d", userID, expiresAt.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", err
	}
	signature := hex.EncodeToString(mac.Sum(nil))
	token := payload + "." + signature
	return base64.RawURLEncoding.EncodeToString([]byte(token)), nil
}

func verifyUserSession(token, secret string, now time.Time) (int, bool) {
	if token == "" || secret == "" {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, false
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 3 {
		return 0, false
	}
	userID, err := strconv.Atoi(parts[0])
	if err != nil || userID <= 0 {
		return 0, false
	}
	expiresAtUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > expiresAtUnix {
		return 0, false
	}

	payload := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(payload)); err != nil {
		return 0, false
	}
	expected := hex.EncodeToString(mac.Sum(nil))
	return userID, hmac.Equal([]byte(parts[2]), []byte(expected))
}

func hashPassword(password string) (string, error) {
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", err
	}
	digest := pbkdf2SHA256([]byte(password), salt[:], passwordHashIterations, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s",
		passwordHashIterations,
		base64.RawURLEncoding.EncodeToString(salt[:]),
		base64.RawURLEncoding.EncodeToString(digest),
	), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := sha256.Size
	blocks := (keyLen + hashLen - 1) / hashLen
	var derived []byte
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		derived = append(derived, t...)
	}
	return derived[:keyLen]
}
