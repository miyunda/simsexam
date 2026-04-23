package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"simsexam/internal/database"
	"simsexam/internal/importer"
)

var DefaultSeedFiles = []string{
	filepath.Join("docs", "examples", "se-demo.md"),
}

type V1BootstrapOptions struct {
	SeedFiles []string
}

type V1BootstrapResult struct {
	AppliedSeeds []string
	SkippedSeeds []string
}

func PrepareV1Database(ctx context.Context, db *sql.DB, opts V1BootstrapOptions) (V1BootstrapResult, error) {
	if err := database.RunMigrations(db, database.V1Migrations); err != nil {
		return V1BootstrapResult{}, fmt.Errorf("run migrations: %w", err)
	}

	seedFiles := opts.SeedFiles
	if len(seedFiles) == 0 {
		seedFiles = DefaultSeedFiles
	}

	result := V1BootstrapResult{}
	for _, path := range seedFiles {
		applied, err := ImportSeedFile(ctx, db, path)
		if err != nil {
			return V1BootstrapResult{}, err
		}
		if applied {
			result.AppliedSeeds = append(result.AppliedSeeds, path)
		} else {
			result.SkippedSeeds = append(result.SkippedSeeds, path)
		}
	}

	return result, nil
}

func ImportSeedFile(ctx context.Context, db *sql.DB, path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read seed file %s: %w", path, err)
	}

	checksum := checksum(content)
	var existingCount int
	err = db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM import_jobs
		WHERE source_type = 'seed'
		  AND source_checksum = ?
		  AND status = 'imported'`,
		checksum,
	).Scan(&existingCount)
	if err != nil {
		return false, fmt.Errorf("check existing seed import %s: %w", path, err)
	}
	if existingCount > 0 {
		return false, nil
	}

	doc, err := importer.ParseString(string(content))
	if err != nil {
		return false, fmt.Errorf("parse seed file %s: %w", path, err)
	}

	if report := importer.ValidateDocument(doc); !report.Valid() {
		return false, fmt.Errorf("seed file %s failed validation", path)
	}

	_, err = importer.ImportDocument(ctx, db, doc, importer.ImportOptions{
		SourceType:     "seed",
		SourceFilename: filepath.Base(path),
		SourceChecksum: checksum,
		Activate:       true,
	})
	if err != nil {
		return false, fmt.Errorf("import seed file %s: %w", path, err)
	}

	return true, nil
}

func checksum(content []byte) string {
	sum := sha256sum(content)
	return sum
}
