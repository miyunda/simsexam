package bootstrap

import (
	"context"
	"database/sql"
	"path/filepath"
	"simsexam/internal/database"
	"testing"
)

func TestPrepareV1DatabaseImportsDefaultSeedOnce(t *testing.T) {
	db := openBootstrapTestDB(t)
	defer db.Close()

	first, err := PrepareV1Database(context.Background(), db, V1BootstrapOptions{})
	if err != nil {
		t.Fatalf("first PrepareV1Database failed: %v", err)
	}
	if len(first.AppliedSeeds) != 1 {
		t.Fatalf("expected 1 applied seed on first run, got %d", len(first.AppliedSeeds))
	}

	second, err := PrepareV1Database(context.Background(), db, V1BootstrapOptions{})
	if err != nil {
		t.Fatalf("second PrepareV1Database failed: %v", err)
	}
	if len(second.AppliedSeeds) != 0 {
		t.Fatalf("expected 0 applied seeds on second run, got %d", len(second.AppliedSeeds))
	}
	if len(second.SkippedSeeds) != 1 {
		t.Fatalf("expected 1 skipped seed on second run, got %d", len(second.SkippedSeeds))
	}

	var importCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM import_jobs WHERE source_type = 'seed' AND status = 'imported'`).Scan(&importCount); err != nil {
		t.Fatalf("count seed import jobs: %v", err)
	}
	if importCount != 1 {
		t.Fatalf("expected 1 imported seed job, got %d", importCount)
	}
}

func openBootstrapTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "bootstrap.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite returned error: %v", err)
	}
	return db
}
