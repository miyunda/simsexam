package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dataSourceName string) error {
	var err error
	DB, err = sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	return createTables()
}

func createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS subjects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		question_limit INTEGER DEFAULT 10
	);

	CREATE TABLE IF NOT EXISTS questions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subject_id INTEGER,
		text TEXT NOT NULL,
		type TEXT DEFAULT 'single',
		explanation TEXT,
		FOREIGN KEY(subject_id) REFERENCES subjects(id)
	);

	CREATE TABLE IF NOT EXISTS options (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		question_id INTEGER,
		text TEXT NOT NULL,
		is_correct BOOLEAN,
		FOREIGN KEY(question_id) REFERENCES questions(id)
	);

	CREATE TABLE IF NOT EXISTS exam_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		subject_id INTEGER,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		score INTEGER,
		FOREIGN KEY(subject_id) REFERENCES subjects(id)
	);
	`
	_, err := DB.Exec(query)
	return err
}
