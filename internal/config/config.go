package config

import "os"

const (
	EnvAddr             = "SIMSEXAM_ADDR"
	EnvDBPath           = "SIMSEXAM_DB_PATH"
	EnvImportSourceType = "SIMSEXAM_IMPORT_SOURCE_TYPE"
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
	Addr string
}

type ImportConfig struct {
	RuntimeConfig
	SourceType string
}

func LoadServerConfig() ServerConfig {
	return ServerConfig{
		RuntimeConfig: LoadRuntimeConfig(),
		Addr:          envOrDefault(EnvAddr, DefaultAddr),
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
