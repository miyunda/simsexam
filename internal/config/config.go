package config

import (
	"os"
	"strconv"
)

const (
	EnvAddr             = "SIMSEXAM_ADDR"
	EnvDBPath           = "SIMSEXAM_DB_PATH"
	EnvImportSourceType = "SIMSEXAM_IMPORT_SOURCE_TYPE"
	EnvAdminPassword    = "SIMSEXAM_ADMIN_PASSWORD"
	EnvAdminSessionKey  = "SIMSEXAM_ADMIN_SESSION_SECRET"
	EnvUserSessionKey   = "SIMSEXAM_USER_SESSION_SECRET"
	EnvCookieSecure     = "SIMSEXAM_COOKIE_SECURE"
)

const (
	DefaultAddr             = "127.0.0.1:6080"
	DefaultDBPath           = "./simsexam_v1.db"
	DefaultImportSourceType = "markdown_import"
)

type RuntimeConfig struct {
	DBPath string
}

type ServerConfig struct {
	RuntimeConfig
	Addr               string
	AdminPassword      string
	AdminSessionSecret string
	UserSessionSecret  string
	CookieSecure       bool
}

type ImportConfig struct {
	RuntimeConfig
	SourceType string
}

func LoadServerConfig() ServerConfig {
	return ServerConfig{
		RuntimeConfig:      LoadRuntimeConfig(),
		Addr:               envOrDefault(EnvAddr, DefaultAddr),
		AdminPassword:      os.Getenv(EnvAdminPassword),
		AdminSessionSecret: os.Getenv(EnvAdminSessionKey),
		UserSessionSecret:  os.Getenv(EnvUserSessionKey),
		CookieSecure:       envBoolOrDefault(EnvCookieSecure, false),
	}
}

func LoadRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		DBPath: envOrDefault(EnvDBPath, DefaultDBPath),
	}
}

func LoadImportConfig() ImportConfig {
	return ImportConfig{
		RuntimeConfig: LoadRuntimeConfig(),
		SourceType:    envOrDefault(EnvImportSourceType, DefaultImportSourceType),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
