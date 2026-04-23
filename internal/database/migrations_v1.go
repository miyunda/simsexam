package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Migration struct {
	Name string
	SQL  string
}

var V1Migrations = []Migration{
	{
		Name: "0001_schema_v1_initial",
		SQL: `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schema_migrations (
	name TEXT PRIMARY KEY,
	applied_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL UNIQUE,
	display_name TEXT,
	avatar_url TEXT,
	role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	provider TEXT NOT NULL,
	provider_user_id TEXT NOT NULL,
	provider_email TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id),
	UNIQUE(provider, provider_user_id)
);

CREATE TABLE IF NOT EXISTS subjects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	slug TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	description TEXT,
	duration_minutes INTEGER NOT NULL CHECK (duration_minutes > 0),
	question_count INTEGER NOT NULL CHECK (question_count > 0),
	access_level TEXT NOT NULL CHECK (access_level IN ('free', 'paid', 'private')),
	status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
	current_question_set_id INTEGER,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS question_sets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	subject_id INTEGER NOT NULL,
	version TEXT,
	source_type TEXT NOT NULL CHECK (source_type IN ('seed', 'markdown_import', 'manual')),
	source_name TEXT,
	source_checksum TEXT,
	is_active INTEGER NOT NULL DEFAULT 0 CHECK (is_active IN (0, 1)),
	created_by_user_id INTEGER,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(subject_id) REFERENCES subjects(id),
	FOREIGN KEY(created_by_user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS questions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	subject_id INTEGER NOT NULL,
	question_set_id INTEGER NOT NULL,
	external_key TEXT NOT NULL,
	type TEXT NOT NULL CHECK (type IN ('single', 'multiple')),
	stem_markdown TEXT NOT NULL,
	explanation_markdown TEXT,
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(subject_id) REFERENCES subjects(id),
	FOREIGN KEY(question_set_id) REFERENCES question_sets(id),
	UNIQUE(question_set_id, external_key)
);

CREATE TABLE IF NOT EXISTS question_options (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	question_id INTEGER NOT NULL,
	option_key TEXT NOT NULL,
	content_markdown TEXT NOT NULL,
	sort_order INTEGER NOT NULL CHECK (sort_order > 0),
	is_correct INTEGER NOT NULL DEFAULT 0 CHECK (is_correct IN (0, 1)),
	FOREIGN KEY(question_id) REFERENCES questions(id),
	UNIQUE(question_id, option_key),
	UNIQUE(question_id, sort_order)
);

CREATE TABLE IF NOT EXISTS import_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	subject_id INTEGER,
	question_set_id INTEGER,
	source_type TEXT NOT NULL,
	source_filename TEXT NOT NULL,
	source_checksum TEXT,
	status TEXT NOT NULL CHECK (status IN ('pending', 'validated', 'imported', 'failed')),
	manifest_json TEXT,
	error_report TEXT,
	warning_report TEXT,
	created_by_user_id INTEGER,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(subject_id) REFERENCES subjects(id),
	FOREIGN KEY(question_set_id) REFERENCES question_sets(id),
	FOREIGN KEY(created_by_user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS question_revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	question_id INTEGER NOT NULL,
	editor_user_id INTEGER,
	change_summary TEXT,
	snapshot_json TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(question_id) REFERENCES questions(id),
	FOREIGN KEY(editor_user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS exams (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER,
	subject_id INTEGER NOT NULL,
	question_set_id INTEGER NOT NULL,
	mode TEXT NOT NULL CHECK (mode IN ('practice', 'formal')),
	status TEXT NOT NULL CHECK (status IN ('in_progress', 'submitted', 'expired')),
	started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	submitted_at DATETIME,
	expires_at DATETIME,
	score INTEGER CHECK (score IS NULL OR (score >= 0 AND score <= 100)),
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(subject_id) REFERENCES subjects(id),
	FOREIGN KEY(question_set_id) REFERENCES question_sets(id)
);

CREATE TABLE IF NOT EXISTS exam_questions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exam_id INTEGER NOT NULL,
	question_id INTEGER NOT NULL,
	position INTEGER NOT NULL CHECK (position > 0),
	FOREIGN KEY(exam_id) REFERENCES exams(id),
	FOREIGN KEY(question_id) REFERENCES questions(id),
	UNIQUE(exam_id, position),
	UNIQUE(exam_id, question_id)
);

CREATE TABLE IF NOT EXISTS exam_answers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exam_id INTEGER NOT NULL,
	question_id INTEGER NOT NULL,
	answered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	is_correct INTEGER NOT NULL DEFAULT 0 CHECK (is_correct IN (0, 1)),
	FOREIGN KEY(exam_id) REFERENCES exams(id),
	FOREIGN KEY(question_id) REFERENCES questions(id),
	UNIQUE(exam_id, question_id)
);

CREATE TABLE IF NOT EXISTS exam_answer_options (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exam_answer_id INTEGER NOT NULL,
	option_id INTEGER NOT NULL,
	FOREIGN KEY(exam_answer_id) REFERENCES exam_answers(id),
	FOREIGN KEY(option_id) REFERENCES question_options(id),
	UNIQUE(exam_answer_id, option_id)
);

CREATE TABLE IF NOT EXISTS user_question_stats (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	subject_id INTEGER NOT NULL,
	question_key TEXT NOT NULL,
	wrong_count INTEGER NOT NULL DEFAULT 0 CHECK (wrong_count >= 0),
	correct_count INTEGER NOT NULL DEFAULT 0 CHECK (correct_count >= 0),
	last_answered_at DATETIME,
	last_wrong_at DATETIME,
	mastery_status TEXT NOT NULL DEFAULT 'new' CHECK (mastery_status IN ('new', 'weak', 'mastered')),
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(subject_id) REFERENCES subjects(id),
	UNIQUE(user_id, subject_id, question_key)
);

CREATE TABLE IF NOT EXISTS subject_entitlements (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	subject_id INTEGER NOT NULL,
	source TEXT NOT NULL CHECK (source IN ('free', 'purchase', 'gift', 'admin')),
	starts_at DATETIME,
	ends_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(subject_id) REFERENCES subjects(id)
);

CREATE INDEX IF NOT EXISTS idx_subjects_status ON subjects(status);
CREATE INDEX IF NOT EXISTS idx_question_sets_subject_created_at ON question_sets(subject_id, created_at);
CREATE INDEX IF NOT EXISTS idx_question_sets_subject_is_active ON question_sets(subject_id, is_active);
CREATE INDEX IF NOT EXISTS idx_questions_subject_status ON questions(subject_id, status);
CREATE INDEX IF NOT EXISTS idx_questions_question_set_id ON questions(question_set_id);
CREATE INDEX IF NOT EXISTS idx_question_options_question_sort ON question_options(question_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_import_jobs_subject_created_at ON import_jobs(subject_id, created_at);
CREATE INDEX IF NOT EXISTS idx_exams_user_started_at ON exams(user_id, started_at);
CREATE INDEX IF NOT EXISTS idx_exams_subject_started_at ON exams(subject_id, started_at);
CREATE INDEX IF NOT EXISTS idx_exam_answers_exam_is_correct ON exam_answers(exam_id, is_correct);
CREATE INDEX IF NOT EXISTS idx_user_question_stats_user_mastery ON user_question_stats(user_id, mastery_status);
CREATE INDEX IF NOT EXISTS idx_subject_entitlements_user_subject ON subject_entitlements(user_id, subject_id);
`,
	},
}

func OpenSQLite(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func RunMigrations(db *sql.DB, migrations []Migration) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	name TEXT PRIMARY KEY,
	applied_at DATETIME NOT NULL
);`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, migration := range migrations {
		var exists string
		err := db.QueryRow("SELECT name FROM schema_migrations WHERE name = ?", migration.Name).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", migration.Name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.Name, err)
		}

		if _, err := tx.Exec(migration.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)",
			migration.Name,
			time.Now().UTC(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.Name, err)
		}
	}

	return nil
}
