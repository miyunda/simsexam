package handlers

import (
	"math/rand"
	"path/filepath"
	"testing"

	"simsexam/internal/database"
)

func TestShuffledOptionIDsUsesProvidedRandomSource(t *testing.T) {
	original := []int{1, 2, 3, 4}
	got := shuffledOptionIDs(original, rand.New(rand.NewSource(1)))

	if len(got) != len(original) {
		t.Fatalf("expected %d shuffled options, got %d", len(original), len(got))
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 4 || got[3] != 3 {
		t.Fatalf("unexpected shuffled order: %#v", got)
	}
	if original[0] != 1 || original[1] != 2 || original[2] != 3 || original[3] != 4 {
		t.Fatalf("expected original slice to remain unchanged, got %#v", original)
	}
}

func TestShouldShuffleOptionsForQuestion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shuffle.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, database.V1Migrations); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatalf("exec failed for %q: %v", query, err)
		}
	}

	mustExec(`INSERT INTO subjects (id, slug, title, description, duration_minutes, question_count, access_level, status, shuffle_options_default)
		VALUES (1, 'subj-a', 'Subject A', '', 10, 1, 'free', 'published', 0)`)
	mustExec(`INSERT INTO subjects (id, slug, title, description, duration_minutes, question_count, access_level, status, shuffle_options_default)
		VALUES (2, 'subj-b', 'Subject B', '', 10, 1, 'free', 'published', 1)`)
	mustExec(`INSERT INTO question_sets (id, subject_id, source_type, source_name, is_active)
		VALUES (1, 1, 'manual', 'seed', 1)`)
	mustExec(`INSERT INTO question_sets (id, subject_id, source_type, source_name, is_active)
		VALUES (2, 2, 'manual', 'seed', 1)`)
	mustExec(`INSERT INTO questions (id, subject_id, question_set_id, external_key, type, stem_markdown, allow_option_shuffle)
		VALUES (1, 1, 1, 'q1', 'single', 'Question 1', NULL)`)
	mustExec(`INSERT INTO questions (id, subject_id, question_set_id, external_key, type, stem_markdown, allow_option_shuffle)
		VALUES (2, 2, 2, 'q2', 'single', 'Question 2', NULL)`)
	mustExec(`INSERT INTO questions (id, subject_id, question_set_id, external_key, type, stem_markdown, allow_option_shuffle)
		VALUES (3, 2, 2, 'q3', 'single', 'Question 3', 0)`)
	mustExec(`INSERT INTO questions (id, subject_id, question_set_id, external_key, type, stem_markdown, allow_option_shuffle)
		VALUES (4, 1, 1, 'q4', 'single', 'Question 4', 1)`)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	defer tx.Rollback()

	assertShuffle := func(questionID int, want bool) {
		t.Helper()
		got, err := shouldShuffleOptionsForQuestion(tx, questionID)
		if err != nil {
			t.Fatalf("shouldShuffleOptionsForQuestion(%d) failed: %v", questionID, err)
		}
		if got != want {
			t.Fatalf("question %d: expected shuffle=%v, got %v", questionID, want, got)
		}
	}

	assertShuffle(1, false)
	assertShuffle(2, true)
	assertShuffle(3, false)
	assertShuffle(4, true)
}

func TestPersistExamQuestionOptionOrderShufflesWhenEnabled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, database.V1Migrations); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatalf("exec failed for %q: %v", query, err)
		}
	}

	mustExec(`INSERT INTO subjects (id, slug, title, description, duration_minutes, question_count, access_level, status, shuffle_options_default)
		VALUES (1, 'subj', 'Subject', '', 10, 1, 'free', 'published', 1)`)
	mustExec(`INSERT INTO question_sets (id, subject_id, source_type, source_name, is_active)
		VALUES (1, 1, 'manual', 'seed', 1)`)
	mustExec(`INSERT INTO questions (id, subject_id, question_set_id, external_key, type, stem_markdown, allow_option_shuffle)
		VALUES (1, 1, 1, 'q1', 'single', 'Question 1', NULL)`)
	mustExec(`INSERT INTO question_options (question_id, option_key, content_markdown, sort_order, is_correct) VALUES (1, 'A', 'A', 1, 0)`)
	mustExec(`INSERT INTO question_options (question_id, option_key, content_markdown, sort_order, is_correct) VALUES (1, 'B', 'B', 2, 1)`)
	mustExec(`INSERT INTO question_options (question_id, option_key, content_markdown, sort_order, is_correct) VALUES (1, 'C', 'C', 3, 0)`)
	mustExec(`INSERT INTO question_options (question_id, option_key, content_markdown, sort_order, is_correct) VALUES (1, 'D', 'D', 4, 0)`)
	mustExec(`INSERT INTO exams (id, subject_id, question_set_id, mode, status) VALUES (1, 1, 1, 'practice', 'in_progress')`)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO exam_questions (exam_id, question_id, position) VALUES (1, 1, 1)`)
	if err != nil {
		t.Fatalf("insert exam question failed: %v", err)
	}
	examQuestionID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId failed: %v", err)
	}

	oldRandFactory := newOptionShuffleRand
	newOptionShuffleRand = func() *rand.Rand {
		return rand.New(rand.NewSource(1))
	}
	defer func() {
		newOptionShuffleRand = oldRandFactory
	}()

	if err := persistExamQuestionOptionOrder(tx, examQuestionID, 1); err != nil {
		t.Fatalf("persistExamQuestionOptionOrder failed: %v", err)
	}

	rows, err := tx.Query(`
		SELECT eqo.display_order, qo.sort_order
		FROM exam_question_options eqo
		JOIN question_options qo ON qo.id = eqo.question_option_id
		WHERE eqo.exam_question_id = ?
		ORDER BY eqo.display_order
	`, examQuestionID)
	if err != nil {
		t.Fatalf("query persisted option order failed: %v", err)
	}
	defer rows.Close()

	var got []int
	for rows.Next() {
		var displayOrder int
		var canonicalOrder int
		if err := rows.Scan(&displayOrder, &canonicalOrder); err != nil {
			t.Fatalf("scan persisted option order failed: %v", err)
		}
		got = append(got, canonicalOrder)
	}

	want := []int{1, 2, 4, 3}
	if len(got) != len(want) {
		t.Fatalf("expected %d persisted options, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected persisted canonical order at %d: got %v want %v", i, got, want)
		}
	}
}
