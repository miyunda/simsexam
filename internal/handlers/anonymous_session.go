package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const anonymousSessionCookieName = "simsexam_anon_session"

func ensureAnonymousSessionTx(r *http.Request, tx *sql.Tx) (int, *http.Cookie, error) {
	if cookie, err := r.Cookie(anonymousSessionCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			sessionID, err := anonymousSessionIDByToken(tx, token)
			if err == nil {
				return sessionID, refreshAnonymousSessionCookie(token), nil
			}
			if err != sql.ErrNoRows {
				return 0, nil, err
			}
		}
	}

	token, err := newAnonymousSessionToken()
	if err != nil {
		return 0, nil, err
	}
	res, err := tx.Exec(`
		INSERT INTO anonymous_sessions (token_hash)
		VALUES (?)
	`, anonymousSessionTokenHash(token))
	if err != nil {
		return 0, nil, err
	}
	sessionID, err := res.LastInsertId()
	if err != nil {
		return 0, nil, err
	}
	return int(sessionID), refreshAnonymousSessionCookie(token), nil
}

func anonymousSessionIDByToken(tx *sql.Tx, token string) (int, error) {
	var sessionID int
	err := tx.QueryRow(`
		SELECT id
		FROM anonymous_sessions
		WHERE token_hash = ?
	`, anonymousSessionTokenHash(token)).Scan(&sessionID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`
		UPDATE anonymous_sessions
		SET last_seen_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, sessionID); err != nil {
		return 0, err
	}
	return sessionID, nil
}

func newAnonymousSessionToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate anonymous session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func anonymousSessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func refreshAnonymousSessionCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     anonymousSessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int((180 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
