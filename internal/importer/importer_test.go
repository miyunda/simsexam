package importer

import (
	"context"
	"database/sql"
	"path/filepath"
	"simsexam/internal/database"
	"testing"
)

func TestImportDocumentCreatesSubjectVersionAndQuestions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	doc, err := ParseFile(filepath.Join("..", "..", "docs", "examples", "se-demo.md"))
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	result, err := ImportDocument(context.Background(), db, doc, ImportOptions{
		SourceType:     "markdown_import",
		SourceFilename: "se-demo.md",
		SourceChecksum: "checksum-1",
		Activate:       true,
	})
	if err != nil {
		t.Fatalf("ImportDocument returned error: %v", err)
	}

	if result.QuestionsCount != 2 {
		t.Fatalf("expected 2 questions, got %d", result.QuestionsCount)
	}
	if result.OptionsCount != 8 {
		t.Fatalf("expected 8 options, got %d", result.OptionsCount)
	}

	var currentQuestionSetID sql.NullInt64
	if err := db.QueryRow(`SELECT current_question_set_id FROM subjects WHERE id = ?`, result.SubjectID).Scan(&currentQuestionSetID); err != nil {
		t.Fatalf("query current_question_set_id: %v", err)
	}
	if !currentQuestionSetID.Valid || currentQuestionSetID.Int64 != result.QuestionSetID {
		t.Fatalf("expected current_question_set_id=%d, got %+v", result.QuestionSetID, currentQuestionSetID)
	}

	var questionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM questions WHERE question_set_id = ?`, result.QuestionSetID).Scan(&questionCount); err != nil {
		t.Fatalf("count questions: %v", err)
	}
	if questionCount != 2 {
		t.Fatalf("expected 2 persisted questions, got %d", questionCount)
	}
}

func TestImportDocumentCreatesNewVersionAndDeactivatesOldOne(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	doc, err := ParseFile(filepath.Join("..", "..", "docs", "examples", "se-demo.md"))
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	first, err := ImportDocument(context.Background(), db, doc, ImportOptions{
		SourceType:     "markdown_import",
		SourceFilename: "se-demo.v1.md",
		SourceChecksum: "checksum-1",
		Activate:       true,
	})
	if err != nil {
		t.Fatalf("first import failed: %v", err)
	}

	doc.Manifest.Title = "SE Demo Subject Updated"
	doc.Manifest.QuestionCount = 1
	doc.Manifest.Version = "2026-04-24"
	second, err := ImportDocument(context.Background(), db, doc, ImportOptions{
		SourceType:     "markdown_import",
		SourceFilename: "se-demo.v2.md",
		SourceChecksum: "checksum-2",
		Activate:       true,
	})
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}

	if first.QuestionSetID == second.QuestionSetID {
		t.Fatal("expected a new question set id on second import")
	}

	var activeCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM question_sets WHERE subject_id = ? AND is_active = 1`, second.SubjectID).Scan(&activeCount); err != nil {
		t.Fatalf("count active question sets: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active question set, got %d", activeCount)
	}

	var totalQuestionSets int
	if err := db.QueryRow(`SELECT COUNT(*) FROM question_sets WHERE subject_id = ?`, second.SubjectID).Scan(&totalQuestionSets); err != nil {
		t.Fatalf("count question sets: %v", err)
	}
	if totalQuestionSets != 2 {
		t.Fatalf("expected 2 question sets, got %d", totalQuestionSets)
	}

	var title string
	if err := db.QueryRow(`SELECT title FROM subjects WHERE id = ?`, second.SubjectID).Scan(&title); err != nil {
		t.Fatalf("query subject title: %v", err)
	}
	if title != "SE Demo Subject Updated" {
		t.Fatalf("expected updated title, got %q", title)
	}

	var totalQuestions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM questions WHERE subject_id = ?`, second.SubjectID).Scan(&totalQuestions); err != nil {
		t.Fatalf("count subject questions: %v", err)
	}
	if totalQuestions != 4 {
		t.Fatalf("expected 4 question snapshots across two versions, got %d", totalQuestions)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite returned error: %v", err)
	}
	if err := database.RunMigrations(db, database.V1Migrations); err != nil {
		db.Close()
		t.Fatalf("RunMigrations returned error: %v", err)
	}
	return db
}
