package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"simsexam/internal/bootstrap"
	"simsexam/internal/config"
	"simsexam/internal/database"
	"strings"
)

func main() {
	cfg := config.LoadRuntimeConfig()
	dsn := flag.String("dsn", cfg.DBPath, "SQLite database path")
	seedFilesFlag := flag.String("seed-files", "", "Comma-separated list of seed markdown files")
	flag.Parse()

	db, err := database.OpenSQLite(*dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	opts := bootstrap.V1BootstrapOptions{
		SeedFiles: parseSeedFiles(*seedFilesFlag),
	}

	result, err := bootstrap.PrepareV1Database(context.Background(), db, opts)
	if err != nil {
		log.Fatalf("prepare v1 database: %v", err)
	}

	fmt.Printf("Prepared v1 database at %s\n", *dsn)
	fmt.Printf("- applied seed files: %d\n", len(result.AppliedSeeds))
	for _, path := range result.AppliedSeeds {
		fmt.Printf("  - %s\n", filepath.Clean(path))
	}
	fmt.Printf("- skipped seed files: %d\n", len(result.SkippedSeeds))
	for _, path := range result.SkippedSeeds {
		fmt.Printf("  - %s\n", filepath.Clean(path))
	}
}

func parseSeedFiles(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		files = append(files, part)
	}
	return files
}
