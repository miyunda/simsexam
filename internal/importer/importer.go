package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

type ImportOptions struct {
	SourceType      string
	SourceFilename  string
	SourceChecksum  string
	CreatedByUserID *int
	Activate        bool
	Now             time.Time
}

type ImportResult struct {
	SubjectID      int64
	QuestionSetID  int64
	ImportJobID    int64
	QuestionsCount int
	OptionsCount   int
}

func ImportDocument(ctx context.Context, db *sql.DB, doc Document, opts ImportOptions) (ImportResult, error) {
	report := ValidateDocument(doc)
	if !report.Valid() {
		return ImportResult{}, fmt.Errorf("document validation failed")
	}

	if opts.SourceType == "" {
		opts.SourceType = "markdown_import"
	}
	if opts.SourceFilename == "" {
		opts.SourceFilename = doc.Manifest.Slug + ".md"
	} else {
		opts.SourceFilename = filepath.Base(opts.SourceFilename)
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if !opts.Activate {
		opts.Activate = true
	}

	manifestJSON, err := json.Marshal(doc.Manifest)
	if err != nil {
		return ImportResult{}, fmt.Errorf("marshal manifest: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, fmt.Errorf("begin transaction: %w", err)
	}

	var committed bool
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	subjectID, err := upsertSubject(ctx, tx, doc, opts.Now)
	if err != nil {
		return ImportResult{}, err
	}

	importJobID, err := insertImportJob(ctx, tx, subjectID, manifestJSON, opts)
	if err != nil {
		return ImportResult{}, err
	}

	questionSetID, err := insertQuestionSet(ctx, tx, subjectID, doc, opts)
	if err != nil {
		return ImportResult{}, err
	}

	questionsCount, optionsCount, err := insertQuestions(ctx, tx, subjectID, questionSetID, doc, opts.Now)
	if err != nil {
		return ImportResult{}, err
	}

	if opts.Activate {
		if err := activateQuestionSet(ctx, tx, subjectID, questionSetID, opts.Now); err != nil {
			return ImportResult{}, err
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE import_jobs
		SET question_set_id = ?, status = 'imported'
		WHERE id = ?`,
		questionSetID,
		importJobID,
	); err != nil {
		return ImportResult{}, fmt.Errorf("update import job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ImportResult{}, fmt.Errorf("commit import: %w", err)
	}
	committed = true

	return ImportResult{
		SubjectID:      subjectID,
		QuestionSetID:  questionSetID,
		ImportJobID:    importJobID,
		QuestionsCount: questionsCount,
		OptionsCount:   optionsCount,
	}, nil
}

func upsertSubject(ctx context.Context, tx *sql.Tx, doc Document, now time.Time) (int64, error) {
	var subjectID int64
	err := tx.QueryRowContext(
		ctx,
		`SELECT id FROM subjects WHERE slug = ?`,
		doc.Manifest.Slug,
	).Scan(&subjectID)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("find subject: %w", err)
	}

	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(
			ctx,
			`INSERT INTO subjects (
				slug, title, description, duration_minutes, question_count, access_level, status, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.Manifest.Slug,
			doc.Manifest.Title,
			doc.Manifest.Description,
			doc.Manifest.DurationMinutes,
			doc.Manifest.QuestionCount,
			doc.Manifest.AccessLevel,
			doc.Manifest.Status,
			now,
			now,
		)
		if err != nil {
			return 0, fmt.Errorf("insert subject: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("subject last insert id: %w", err)
		}
		return id, nil
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE subjects
		SET title = ?, description = ?, duration_minutes = ?, question_count = ?, access_level = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		doc.Manifest.Title,
		doc.Manifest.Description,
		doc.Manifest.DurationMinutes,
		doc.Manifest.QuestionCount,
		doc.Manifest.AccessLevel,
		doc.Manifest.Status,
		now,
		subjectID,
	); err != nil {
		return 0, fmt.Errorf("update subject: %w", err)
	}

	return subjectID, nil
}

func insertImportJob(ctx context.Context, tx *sql.Tx, subjectID int64, manifestJSON []byte, opts ImportOptions) (int64, error) {
	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO import_jobs (
			subject_id, source_type, source_filename, source_checksum, status, manifest_json, created_by_user_id, created_at
		) VALUES (?, ?, ?, ?, 'pending', ?, ?, ?)`,
		subjectID,
		opts.SourceType,
		opts.SourceFilename,
		opts.SourceChecksum,
		string(manifestJSON),
		opts.CreatedByUserID,
		opts.Now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert import job: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("import job last insert id: %w", err)
	}
	return id, nil
}

func insertQuestionSet(ctx context.Context, tx *sql.Tx, subjectID int64, doc Document, opts ImportOptions) (int64, error) {
	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO question_sets (
			subject_id, version, source_type, source_name, source_checksum, is_active, created_by_user_id, created_at
		) VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		subjectID,
		nilIfEmpty(doc.Manifest.Version),
		opts.SourceType,
		opts.SourceFilename,
		nilIfEmpty(opts.SourceChecksum),
		opts.CreatedByUserID,
		opts.Now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert question set: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("question set last insert id: %w", err)
	}
	return id, nil
}

func insertQuestions(ctx context.Context, tx *sql.Tx, subjectID, questionSetID int64, doc Document, now time.Time) (int, int, error) {
	totalOptions := 0
	for _, q := range doc.Questions {
		res, err := tx.ExecContext(
			ctx,
			`INSERT INTO questions (
				subject_id, question_set_id, external_key, type, stem_markdown, explanation_markdown, status, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
			subjectID,
			questionSetID,
			q.Key,
			q.Type,
			q.Stem,
			emptyToNil(q.Explanation),
			now,
			now,
		)
		if err != nil {
			return 0, 0, fmt.Errorf("insert question %s: %w", q.Key, err)
		}

		questionID, err := res.LastInsertId()
		if err != nil {
			return 0, 0, fmt.Errorf("question last insert id for %s: %w", q.Key, err)
		}

		for idx, opt := range q.Options {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO question_options (
					question_id, option_key, content_markdown, sort_order, is_correct
				) VALUES (?, ?, ?, ?, ?)`,
				questionID,
				optionKey(idx),
				opt.Text,
				idx+1,
				boolToInt(opt.IsCorrect),
			); err != nil {
				return 0, 0, fmt.Errorf("insert option for question %s: %w", q.Key, err)
			}
			totalOptions++
		}
	}

	return len(doc.Questions), totalOptions, nil
}

func activateQuestionSet(ctx context.Context, tx *sql.Tx, subjectID, questionSetID int64, now time.Time) error {
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE question_sets SET is_active = 0 WHERE subject_id = ?`,
		subjectID,
	); err != nil {
		return fmt.Errorf("deactivate question sets: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE question_sets SET is_active = 1 WHERE id = ?`,
		questionSetID,
	); err != nil {
		return fmt.Errorf("activate question set: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE subjects SET current_question_set_id = ?, updated_at = ? WHERE id = ?`,
		questionSetID,
		now,
		subjectID,
	); err != nil {
		return fmt.Errorf("update subject current_question_set_id: %w", err)
	}

	return nil
}

func optionKey(idx int) string {
	if idx < 26 {
		return string(rune('A' + idx))
	}
	return fmt.Sprintf("OPT%d", idx+1)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func emptyToNil(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nilIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
