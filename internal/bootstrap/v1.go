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

const DefaultSeedName = "embedded:se-demo.md"

const defaultSeedContent = `# Subject: se-demo

## Meta
- slug: se-demo
- title: SE Demo Subject
- description: 用于验证导入器和首次运行 seed 的示例题库
- duration_minutes: 20
- question_count: 2
- access_level: free
- status: published
- version: 2026-04-23

---

## Question
key: demo-001
type: single

What color is the sky on a clear day?

- [x] Blue
- [ ] Green
- [ ] Red
- [ ] Yellow

### Explanation
Under ordinary daytime conditions, the sky usually appears blue.

---

## Question
key: demo-002
type: multiple

Which of the following are planets? Choose two.

- [x] Mars
- [x] Jupiter
- [ ] Moon
- [ ] Sun

### Explanation
Mars and Jupiter are planets. The Moon is a natural satellite and the Sun is a star.
`

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

	result := V1BootstrapResult{}
	if len(opts.SeedFiles) == 0 {
		applied, err := ImportSeedContent(ctx, db, DefaultSeedName, []byte(defaultSeedContent))
		if err != nil {
			return V1BootstrapResult{}, err
		}
		if applied {
			result.AppliedSeeds = append(result.AppliedSeeds, DefaultSeedName)
		} else {
			result.SkippedSeeds = append(result.SkippedSeeds, DefaultSeedName)
		}
		return result, nil
	}

	for _, path := range opts.SeedFiles {
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

	return ImportSeedContent(ctx, db, path, content)
}

func ImportSeedContent(ctx context.Context, db *sql.DB, sourceName string, content []byte) (bool, error) {
	checksum := checksum(content)
	return importSeedContent(ctx, db, sourceName, content, checksum)
}

func importSeedContent(ctx context.Context, db *sql.DB, sourceName string, content []byte, checksum string) (bool, error) {
	var existingCount int
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM import_jobs
		WHERE source_type = 'seed'
		  AND source_checksum = ?
		  AND status = 'imported'`,
		checksum,
	).Scan(&existingCount)
	if err != nil {
		return false, fmt.Errorf("check existing seed import %s: %w", sourceName, err)
	}
	if existingCount > 0 {
		return false, nil
	}

	doc, err := importer.ParseString(string(content))
	if err != nil {
		return false, fmt.Errorf("parse seed file %s: %w", sourceName, err)
	}

	if report := importer.ValidateDocument(doc); !report.Valid() {
		return false, fmt.Errorf("seed file %s failed validation", sourceName)
	}

	_, err = importer.ImportDocument(ctx, db, doc, importer.ImportOptions{
		SourceType:     "seed",
		SourceFilename: filepath.Base(sourceName),
		SourceChecksum: checksum,
		Activate:       true,
	})
	if err != nil {
		return false, fmt.Errorf("import seed file %s: %w", sourceName, err)
	}

	return true, nil
}

func checksum(content []byte) string {
	sum := sha256sum(content)
	return sum
}
