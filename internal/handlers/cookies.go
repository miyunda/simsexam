package handlers

import (
	"net/http"
	"time"

	"simsexam/internal/config"
)

func newSessionCookie(cfg config.ServerConfig, name, value string, expiresAt time.Time, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.CookieSecure,
		Expires:  expiresAt,
		MaxAge:   maxAge,
	}
}

func expiredSessionCookie(cfg config.ServerConfig, name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.CookieSecure,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}
