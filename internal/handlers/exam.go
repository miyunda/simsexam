package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"simsexam/internal/database"
	"simsexam/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/gomarkdown/markdown"
)

var questionNumRegex = regexp.MustCompile(`(?i)^(?:question\s+)?\d+[:.]\s+`)
var newOptionShuffleRand = func() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

func cleanQuestionText(text string) string {
	return questionNumRegex.ReplaceAllString(text, "")
}

func StartExam(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse form error", http.StatusBadRequest)
		return
	}

	subjectID, _ := strconv.Atoi(r.FormValue("subject_id"))
	if subjectID == 0 {
		http.Error(w, "Invalid subject", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to start exam", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var questionSetID int
	var questionCount int
	err = tx.QueryRow(`
		SELECT current_question_set_id, question_count
		FROM subjects
		WHERE id = ? AND status = 'published' AND current_question_set_id IS NOT NULL
	`, subjectID).Scan(&questionSetID, &questionCount)
	if err != nil {
		http.Error(w, "Subject not available", http.StatusNotFound)
		return
	}

	res, err := tx.Exec(`
		INSERT INTO exams (subject_id, question_set_id, mode, status)
		VALUES (?, ?, 'practice', 'in_progress')
	`, subjectID, questionSetID)
	if err != nil {
		http.Error(w, "Failed to start exam", http.StatusInternalServerError)
		return
	}
	examID, _ := res.LastInsertId()

	rows, err := tx.Query(`
		SELECT id
		FROM questions
		WHERE subject_id = ? AND question_set_id = ? AND status = 'active'
		ORDER BY RANDOM()
		LIMIT ?
	`, subjectID, questionSetID, questionCount)
	if err != nil {
		http.Error(w, "Failed to load questions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var questionIDs []int
	for rows.Next() {
		var questionID int
		if err := rows.Scan(&questionID); err != nil {
			http.Error(w, "Failed to load questions", http.StatusInternalServerError)
			return
		}
		questionIDs = append(questionIDs, questionID)
	}
	if len(questionIDs) == 0 {
		http.Error(w, "No active questions available for this subject", http.StatusBadRequest)
		return
	}

	for idx, questionID := range questionIDs {
		res, err := tx.Exec(`
			INSERT INTO exam_questions (exam_id, question_id, position)
			VALUES (?, ?, ?)
		`, examID, questionID, idx+1)
		if err != nil {
			http.Error(w, "Failed to persist exam questions", http.StatusInternalServerError)
			return
		}
		examQuestionID, err := res.LastInsertId()
		if err != nil {
			http.Error(w, "Failed to persist exam questions", http.StatusInternalServerError)
			return
		}
		if err := persistExamQuestionOptionOrder(tx, examQuestionID, questionID); err != nil {
			http.Error(w, "Failed to persist exam question option order", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to start exam", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/exam/%d/question/1", examID), http.StatusSeeOther)
}

func GetQuestion(w http.ResponseWriter, r *http.Request) {
	examID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	qIdx, _ := strconv.Atoi(chi.URLParam(r, "qIdx"))
	if examID == 0 || qIdx < 1 {
		http.Error(w, "Invalid session or question index", http.StatusBadRequest)
		return
	}

	total, err := examQuestionCount(database.DB, examID)
	if err != nil || total == 0 || qIdx > total {
		http.Error(w, "Invalid session or question index", http.StatusBadRequest)
		return
	}

	var (
		q              models.Question
		examQuestionID int
	)
	err = database.DB.QueryRow(`
		SELECT eq.id, q.id, q.subject_id, q.stem_markdown, q.type, COALESCE(q.explanation_markdown, '')
		FROM exam_questions eq
		JOIN questions q ON q.id = eq.question_id
		WHERE eq.exam_id = ? AND eq.position = ?
	`, examID, qIdx).Scan(&examQuestionID, &q.ID, &q.SubjectID, &q.Text, &q.Type, &q.Explanation)
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}
	q.Text = cleanQuestionText(q.Text)

	rows, err := database.DB.Query(`
		SELECT qo.id, qo.question_id, qo.content_markdown
		FROM exam_question_options eqo
		JOIN question_options qo ON qo.id = eqo.question_option_id
		WHERE eqo.exam_question_id = ?
		ORDER BY eqo.display_order
	`, examQuestionID)
	if err != nil {
		http.Error(w, "Options error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var o models.Option
		if err := rows.Scan(&o.ID, &o.QuestionID, &o.Text); err != nil {
			http.Error(w, "Options error", http.StatusInternalServerError)
			return
		}
		q.Options = append(q.Options, o)
	}

	data := struct {
		SessionID    int
		Question     models.Question
		CurrentIndex int
		Total        int
		NextIndex    int
		PrevIndex    int
	}{
		SessionID:    examID,
		Question:     q,
		CurrentIndex: qIdx,
		Total:        total,
		NextIndex:    qIdx + 1,
		PrevIndex:    qIdx - 1,
	}

	renderTemplate(w, "exam.html", data)
}

func SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	examID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse form error", http.StatusBadRequest)
		return
	}

	questionID, _ := strconv.Atoi(r.FormValue("question_id"))
	qIdx, _ := strconv.Atoi(r.FormValue("current_index"))
	if examID == 0 || questionID == 0 || qIdx == 0 {
		http.Error(w, "Invalid answer submission", http.StatusBadRequest)
		return
	}

	var selected []int
	for _, raw := range r.Form["option_id"] {
		optionID, _ := strconv.Atoi(raw)
		if optionID > 0 {
			selected = append(selected, optionID)
		}
	}
	if len(selected) == 0 {
		http.Error(w, "At least one option must be selected", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to save answer", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	totalQuestions, err := examQuestionCountTx(tx, examID)
	if err != nil || totalQuestions == 0 || qIdx > totalQuestions {
		http.Error(w, "Invalid exam state", http.StatusBadRequest)
		return
	}

	isCorrect, err := validateAndScoreAnswer(tx, questionID, selected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := upsertExamAnswer(tx, examID, questionID, selected, isCorrect); err != nil {
		http.Error(w, "Failed to save answer", http.StatusInternalServerError)
		return
	}

	if qIdx == totalQuestions {
		if _, err := finalizeExam(tx, examID); err != nil {
			http.Error(w, "Failed to finish exam", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to save answer", http.StatusInternalServerError)
		return
	}

	if qIdx < totalQuestions {
		http.Redirect(w, r, fmt.Sprintf("/exam/%d/question/%d", examID, qIdx+1), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/exam/%d/result", examID), http.StatusSeeOther)
}

func ExamResult(w http.ResponseWriter, r *http.Request) {
	examID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if examID == 0 {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to load result", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	score, err := finalizeExam(tx, examID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	type ReviewItem struct {
		Number        int
		QuestionID    int
		Question      string
		UserAnswer    string
		CorrectAnswer string
		Explanation   template.HTML
		IsCorrect     bool
	}
	var reviews []ReviewItem

	rows, err := tx.Query(`
		SELECT eq.position, q.id, q.stem_markdown, COALESCE(q.explanation_markdown, ''), COALESCE(ea.is_correct, 0)
		FROM exam_questions eq
		JOIN questions q ON q.id = eq.question_id
		LEFT JOIN exam_answers ea ON ea.exam_id = eq.exam_id AND ea.question_id = eq.question_id
		WHERE eq.exam_id = ?
		ORDER BY eq.position
	`, examID)
	if err != nil {
		http.Error(w, "Failed to load result", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			position    int
			questionID  int
			qText       string
			explanation string
			isCorrect   int
		)
		if err := rows.Scan(&position, &questionID, &qText, &explanation, &isCorrect); err != nil {
			http.Error(w, "Failed to load result", http.StatusInternalServerError)
			return
		}
		if isCorrect == 1 {
			continue
		}

		userAnswers, correctAnswers, err := loadReviewAnswers(tx, examID, questionID)
		if err != nil {
			http.Error(w, "Failed to load result", http.StatusInternalServerError)
			return
		}
		htmlBytes := markdown.ToHTML([]byte(explanation), nil, nil)
		reviews = append(reviews, ReviewItem{
			Number:        position,
			QuestionID:    questionID,
			Question:      cleanQuestionText(qText),
			UserAnswer:    joinAnswerTexts(userAnswers),
			CorrectAnswer: joinAnswerTexts(correctAnswers),
			Explanation:   template.HTML(htmlBytes),
			IsCorrect:     false,
		})
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to load result", http.StatusInternalServerError)
		return
	}

	data := struct {
		ExamID            int
		Score             int
		Reviews           []ReviewItem
		FeedbackSubmitted bool
	}{
		ExamID:            examID,
		Score:             score,
		Reviews:           reviews,
		FeedbackSubmitted: r.URL.Query().Get("feedback") == "submitted",
	}

	renderTemplate(w, "result.html", data)
}

func examQuestionCount(db *sql.DB, examID int) (int, error) {
	var total int
	err := db.QueryRow(`SELECT COUNT(*) FROM exam_questions WHERE exam_id = ?`, examID).Scan(&total)
	return total, err
}

func examQuestionCountTx(tx *sql.Tx, examID int) (int, error) {
	var total int
	err := tx.QueryRow(`SELECT COUNT(*) FROM exam_questions WHERE exam_id = ?`, examID).Scan(&total)
	return total, err
}

func validateAndScoreAnswer(tx *sql.Tx, questionID int, selected []int) (bool, error) {
	rows, err := tx.Query(`
		SELECT id, is_correct
		FROM question_options
		WHERE question_id = ?
	`, questionID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	validOptions := make(map[int]bool)
	var correctIDs []int
	for rows.Next() {
		var optionID int
		var isCorrect int
		if err := rows.Scan(&optionID, &isCorrect); err != nil {
			return false, err
		}
		validOptions[optionID] = true
		if isCorrect == 1 {
			correctIDs = append(correctIDs, optionID)
		}
	}
	if len(validOptions) == 0 {
		return false, fmt.Errorf("question options not found")
	}

	seen := make(map[int]bool)
	var normalized []int
	for _, optionID := range selected {
		if !validOptions[optionID] {
			return false, fmt.Errorf("selected option does not belong to question")
		}
		if seen[optionID] {
			continue
		}
		seen[optionID] = true
		normalized = append(normalized, optionID)
	}

	sort.Ints(normalized)
	sort.Ints(correctIDs)
	if len(normalized) != len(correctIDs) {
		return false, nil
	}
	for idx := range normalized {
		if normalized[idx] != correctIDs[idx] {
			return false, nil
		}
	}
	return true, nil
}

func upsertExamAnswer(tx *sql.Tx, examID, questionID int, selected []int, isCorrect bool) error {
	var answerID int64
	err := tx.QueryRow(`
		SELECT id
		FROM exam_answers
		WHERE exam_id = ? AND question_id = ?
	`, examID, questionID).Scan(&answerID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		res, err := tx.Exec(`
			INSERT INTO exam_answers (exam_id, question_id, is_correct)
			VALUES (?, ?, ?)
		`, examID, questionID, boolToInt(isCorrect))
		if err != nil {
			return err
		}
		answerID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`
			UPDATE exam_answers
			SET answered_at = CURRENT_TIMESTAMP, is_correct = ?
			WHERE id = ?
		`, boolToInt(isCorrect), answerID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM exam_answer_options WHERE exam_answer_id = ?`, answerID); err != nil {
			return err
		}
	}

	for _, optionID := range selected {
		if _, err := tx.Exec(`
			INSERT INTO exam_answer_options (exam_answer_id, option_id)
			VALUES (?, ?)
		`, answerID, optionID); err != nil {
			return err
		}
	}
	return nil
}

func finalizeExam(tx *sql.Tx, examID int) (int, error) {
	var totalQuestions int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM exam_questions
		WHERE exam_id = ?
	`, examID).Scan(&totalQuestions); err != nil {
		return 0, err
	}
	if totalQuestions == 0 {
		return 0, sql.ErrNoRows
	}

	var correctCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM exam_answers
		WHERE exam_id = ? AND is_correct = 1
	`, examID).Scan(&correctCount); err != nil {
		return 0, err
	}

	score := (correctCount * 100) / totalQuestions
	if _, err := tx.Exec(`
		UPDATE exams
		SET score = ?, status = 'submitted', submitted_at = COALESCE(submitted_at, CURRENT_TIMESTAMP)
		WHERE id = ?
	`, score, examID); err != nil {
		return 0, err
	}
	return score, nil
}

func loadReviewAnswers(tx *sql.Tx, examID, questionID int) ([]string, []string, error) {
	userRows, err := tx.Query(`
		SELECT qo.content_markdown
		FROM exam_answers ea
		JOIN exam_answer_options eao ON eao.exam_answer_id = ea.id
		JOIN question_options qo ON qo.id = eao.option_id
		JOIN exam_questions eq ON eq.exam_id = ea.exam_id AND eq.question_id = ea.question_id
		JOIN exam_question_options eqo ON eqo.exam_question_id = eq.id AND eqo.question_option_id = qo.id
		WHERE ea.exam_id = ? AND ea.question_id = ?
		ORDER BY eqo.display_order
	`, examID, questionID)
	if err != nil {
		return nil, nil, err
	}
	defer userRows.Close()

	var userAnswers []string
	for userRows.Next() {
		var text string
		if err := userRows.Scan(&text); err != nil {
			return nil, nil, err
		}
		userAnswers = append(userAnswers, text)
	}

	correctRows, err := tx.Query(`
		SELECT qo.content_markdown
		FROM exam_questions eq
		JOIN exam_question_options eqo ON eqo.exam_question_id = eq.id
		JOIN question_options qo ON qo.id = eqo.question_option_id
		WHERE eq.exam_id = ? AND eq.question_id = ? AND qo.is_correct = 1
		ORDER BY eqo.display_order
	`, examID, questionID)
	if err != nil {
		return nil, nil, err
	}
	defer correctRows.Close()

	var correctAnswers []string
	for correctRows.Next() {
		var text string
		if err := correctRows.Scan(&text); err != nil {
			return nil, nil, err
		}
		correctAnswers = append(correctAnswers, text)
	}

	return userAnswers, correctAnswers, nil
}

func joinAnswerTexts(values []string) string {
	if len(values) == 0 {
		return "(no answer)"
	}
	return strings.Join(values, ", ")
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func persistExamQuestionOptionOrder(tx *sql.Tx, examQuestionID int64, questionID int) error {
	shouldShuffle, err := shouldShuffleOptionsForQuestion(tx, questionID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		SELECT id
		FROM question_options
		WHERE question_id = ?
		ORDER BY sort_order
	`, questionID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var optionIDs []int
	for rows.Next() {
		var optionID int
		if err := rows.Scan(&optionID); err != nil {
			return err
		}
		optionIDs = append(optionIDs, optionID)
	}
	if len(optionIDs) == 0 {
		return fmt.Errorf("question options not found")
	}
	if shouldShuffle {
		optionIDs = shuffledOptionIDs(optionIDs, newOptionShuffleRand())
	}

	displayOrder := 1
	for _, optionID := range optionIDs {
		if _, err := tx.Exec(`
			INSERT INTO exam_question_options (exam_question_id, question_option_id, display_order)
			VALUES (?, ?, ?)
		`, examQuestionID, optionID, displayOrder); err != nil {
			return err
		}
		displayOrder++
	}
	return nil
}

func shouldShuffleOptionsForQuestion(tx *sql.Tx, questionID int) (bool, error) {
	var (
		subjectDefault int
		override       sql.NullInt64
	)
	err := tx.QueryRow(`
		SELECT s.shuffle_options_default, q.allow_option_shuffle
		FROM questions q
		JOIN subjects s ON s.id = q.subject_id
		WHERE q.id = ?
	`, questionID).Scan(&subjectDefault, &override)
	if err != nil {
		return false, err
	}

	if override.Valid {
		return override.Int64 == 1, nil
	}
	return subjectDefault == 1, nil
}

func shuffledOptionIDs(optionIDs []int, rng *rand.Rand) []int {
	shuffled := append([]int(nil), optionIDs...)
	if len(shuffled) <= 1 || rng == nil {
		return shuffled
	}
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}
