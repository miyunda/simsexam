package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"simsexam/internal/database"

	"github.com/go-chi/chi/v5"
)

type questionFeedbackQuestionSnapshot struct {
	Stem        string                           `json:"stem"`
	Explanation string                           `json:"explanation"`
	Options     []questionFeedbackSnapshotOption `json:"options"`
}

type questionFeedbackSnapshotOption struct {
	OptionID     int    `json:"option_id"`
	DisplayOrder int    `json:"display_order"`
	Text         string `json:"text"`
	IsCorrect    bool   `json:"is_correct"`
}

type questionFeedbackAnswerSnapshot struct {
	IsCorrect       bool                             `json:"is_correct"`
	SelectedOptions []questionFeedbackSnapshotOption `json:"selected_options"`
}

type adminFeedbackRow struct {
	ID                  int
	SubjectID           int
	SubjectTitle        string
	QuestionID          int
	QuestionKey         string
	QuestionStem        string
	FeedbackType        string
	Status              string
	CreatedAt           string
	QuestionReportCount int
}

type adminFeedbackDetailData struct {
	ID               int
	SubjectID        int
	SubjectTitle     string
	QuestionID       int
	QuestionKey      string
	QuestionStem     string
	QuestionSetID    int
	ExamID           sql.NullInt64
	FeedbackType     string
	Comment          string
	Status           string
	ResolutionNote   string
	CreatedAt        string
	ResolvedAt       sql.NullString
	QuestionSnapshot questionFeedbackQuestionSnapshot
	AnswerSnapshot   questionFeedbackAnswerSnapshot
}

func SubmitQuestionFeedback(w http.ResponseWriter, r *http.Request) {
	examID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if examID == 0 {
		http.Error(w, "Invalid exam", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse form error", http.StatusBadRequest)
		return
	}

	questionID, _ := strconv.Atoi(r.FormValue("question_id"))
	feedbackType := strings.TrimSpace(r.FormValue("feedback_type"))
	comment := strings.TrimSpace(r.FormValue("comment"))
	if questionID == 0 || !isValidFeedbackType(feedbackType) {
		http.Error(w, "Invalid feedback payload", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to submit feedback", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var (
		subjectID          int
		questionSetID      int
		examQuestionID     int
		anonymousSessionID sql.NullInt64
	)
	err = tx.QueryRow(`
		SELECT e.subject_id, e.question_set_id, eq.id, e.anonymous_session_id
		FROM exams e
		JOIN exam_questions eq ON eq.exam_id = e.id
		WHERE e.id = ? AND eq.question_id = ?
	`, examID, questionID).Scan(&subjectID, &questionSetID, &examQuestionID, &anonymousSessionID)
	if err != nil {
		http.Error(w, "Question not found in exam", http.StatusBadRequest)
		return
	}

	questionSnapshotJSON, answerSnapshotJSON, err := buildQuestionFeedbackSnapshots(tx, examID, examQuestionID, questionID)
	if err != nil {
		http.Error(w, "Failed to capture feedback context", http.StatusInternalServerError)
		return
	}

	if _, err := tx.Exec(`
		INSERT INTO question_feedback (
			subject_id, question_id, question_set_id, exam_id, exam_question_id,
			anonymous_session_id, feedback_type, comment, status, question_snapshot_json, answer_snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'open', ?, ?)
	`, subjectID, questionID, questionSetID, examID, examQuestionID, anonymousSessionID, feedbackType, emptyStringToNil(comment), questionSnapshotJSON, answerSnapshotJSON); err != nil {
		http.Error(w, "Failed to submit feedback", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to submit feedback", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/exam/%d/result?feedback=submitted", examID), http.StatusSeeOther)
}

func AdminFeedbackList(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "open"
	}
	subjectFilter := strings.TrimSpace(r.URL.Query().Get("subject"))
	typeFilter := strings.TrimSpace(r.URL.Query().Get("feedback_type"))

	query := `
		SELECT
			f.id,
			f.subject_id,
			s.title,
			f.question_id,
			q.external_key,
			q.stem_markdown,
			f.feedback_type,
			f.status,
			f.created_at,
			(SELECT COUNT(*) FROM question_feedback f2 WHERE f2.question_id = f.question_id) AS question_report_count
		FROM question_feedback f
		JOIN subjects s ON s.id = f.subject_id
		JOIN questions q ON q.id = f.question_id
		WHERE 1 = 1
	`
	var args []any
	if status != "" {
		query += ` AND f.status = ?`
		args = append(args, status)
	}
	if subjectFilter != "" {
		query += ` AND f.subject_id = ?`
		args = append(args, subjectFilter)
	}
	if typeFilter != "" {
		query += ` AND f.feedback_type = ?`
		args = append(args, typeFilter)
	}
	query += ` ORDER BY f.created_at DESC, f.id DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to load feedback", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var feedback []adminFeedbackRow
	for rows.Next() {
		var row adminFeedbackRow
		if err := rows.Scan(&row.ID, &row.SubjectID, &row.SubjectTitle, &row.QuestionID, &row.QuestionKey, &row.QuestionStem, &row.FeedbackType, &row.Status, &row.CreatedAt, &row.QuestionReportCount); err != nil {
			http.Error(w, "Failed to load feedback", http.StatusInternalServerError)
			return
		}
		row.QuestionStem = cleanQuestionText(row.QuestionStem)
		feedback = append(feedback, row)
	}

	subjectRows, err := database.DB.Query(`SELECT id, title FROM subjects ORDER BY title`)
	if err != nil {
		http.Error(w, "Failed to load feedback filters", http.StatusInternalServerError)
		return
	}
	defer subjectRows.Close()

	var subjects []struct {
		ID    int
		Title string
	}
	for subjectRows.Next() {
		var row struct {
			ID    int
			Title string
		}
		if err := subjectRows.Scan(&row.ID, &row.Title); err != nil {
			http.Error(w, "Failed to load feedback filters", http.StatusInternalServerError)
			return
		}
		subjects = append(subjects, row)
	}

	renderTemplate(w, "admin_feedback_list.html", struct {
		Feedback []adminFeedbackRow
		Subjects []struct {
			ID    int
			Title string
		}
		StatusFilter  string
		SubjectFilter string
		TypeFilter    string
	}{
		Feedback:      feedback,
		Subjects:      subjects,
		StatusFilter:  status,
		SubjectFilter: subjectFilter,
		TypeFilter:    typeFilter,
	})
}

func AdminFeedbackDetail(w http.ResponseWriter, r *http.Request) {
	data, err := loadAdminFeedbackDetail(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Feedback not found", http.StatusNotFound)
		return
	}
	renderTemplate(w, "admin_feedback_detail.html", data)
}

func AdminResolveFeedback(w http.ResponseWriter, r *http.Request) {
	updateFeedbackStatus(w, r, "resolved")
}

func AdminDismissFeedback(w http.ResponseWriter, r *http.Request) {
	updateFeedbackStatus(w, r, "dismissed")
}

func updateFeedbackStatus(w http.ResponseWriter, r *http.Request, status string) {
	feedbackID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if feedbackID == 0 {
		http.Error(w, "Invalid feedback", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse form error", http.StatusBadRequest)
		return
	}
	resolutionNote := strings.TrimSpace(r.FormValue("resolution_note"))
	result, err := database.DB.Exec(`
		UPDATE question_feedback
		SET status = ?, resolution_note = ?, resolved_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, emptyStringToNil(resolutionNote), feedbackID)
	if err != nil {
		http.Error(w, "Failed to update feedback", http.StatusInternalServerError)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		http.Error(w, "Feedback not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/feedback/%d", feedbackID), http.StatusSeeOther)
}

func loadAdminFeedbackDetail(rawID string) (adminFeedbackDetailData, error) {
	feedbackID, _ := strconv.Atoi(rawID)
	if feedbackID == 0 {
		return adminFeedbackDetailData{}, sql.ErrNoRows
	}

	var data adminFeedbackDetailData
	var questionSnapshotJSON string
	var answerSnapshotJSON string
	err := database.DB.QueryRow(`
		SELECT
			f.id,
			f.subject_id,
			s.title,
			f.question_id,
			q.external_key,
			q.stem_markdown,
			f.question_set_id,
			f.exam_id,
			f.feedback_type,
			COALESCE(f.comment, ''),
			f.status,
			COALESCE(f.resolution_note, ''),
			f.created_at,
			f.resolved_at,
			f.question_snapshot_json,
			f.answer_snapshot_json
		FROM question_feedback f
		JOIN subjects s ON s.id = f.subject_id
		JOIN questions q ON q.id = f.question_id
		WHERE f.id = ?
	`, feedbackID).Scan(
		&data.ID,
		&data.SubjectID,
		&data.SubjectTitle,
		&data.QuestionID,
		&data.QuestionKey,
		&data.QuestionStem,
		&data.QuestionSetID,
		&data.ExamID,
		&data.FeedbackType,
		&data.Comment,
		&data.Status,
		&data.ResolutionNote,
		&data.CreatedAt,
		&data.ResolvedAt,
		&questionSnapshotJSON,
		&answerSnapshotJSON,
	)
	if err != nil {
		return adminFeedbackDetailData{}, err
	}
	data.QuestionStem = cleanQuestionText(data.QuestionStem)
	if err := json.Unmarshal([]byte(questionSnapshotJSON), &data.QuestionSnapshot); err != nil {
		return adminFeedbackDetailData{}, err
	}
	if err := json.Unmarshal([]byte(answerSnapshotJSON), &data.AnswerSnapshot); err != nil {
		return adminFeedbackDetailData{}, err
	}
	return data, nil
}

func buildQuestionFeedbackSnapshots(tx *sql.Tx, examID, examQuestionID, questionID int) (string, string, error) {
	var questionSnapshot questionFeedbackQuestionSnapshot
	if err := tx.QueryRow(`
		SELECT q.stem_markdown, COALESCE(q.explanation_markdown, '')
		FROM questions q
		WHERE q.id = ?
	`, questionID).Scan(&questionSnapshot.Stem, &questionSnapshot.Explanation); err != nil {
		return "", "", err
	}

	rows, err := tx.Query(`
		SELECT qo.id, eqo.display_order, qo.content_markdown, qo.is_correct
		FROM exam_question_options eqo
		JOIN question_options qo ON qo.id = eqo.question_option_id
		WHERE eqo.exam_question_id = ?
		ORDER BY eqo.display_order
	`, examQuestionID)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	var answerSnapshot questionFeedbackAnswerSnapshot
	var answerCorrect int
	answerRow, err := tx.Query(`
		SELECT qo.id, qo.content_markdown, eqo.display_order, ea.is_correct
		FROM exam_answers ea
		JOIN exam_answer_options eao ON eao.exam_answer_id = ea.id
		JOIN question_options qo ON qo.id = eao.option_id
		JOIN exam_questions eq ON eq.exam_id = ea.exam_id AND eq.question_id = ea.question_id
		JOIN exam_question_options eqo ON eqo.exam_question_id = eq.id AND eqo.question_option_id = qo.id
		WHERE ea.exam_id = ? AND ea.question_id = ?
		ORDER BY eqo.display_order
	`, examID, questionID)
	if err != nil {
		return "", "", err
	}
	defer answerRow.Close()
	for answerRow.Next() {
		var opt questionFeedbackSnapshotOption
		if err := answerRow.Scan(&opt.OptionID, &opt.Text, &opt.DisplayOrder, &answerCorrect); err != nil {
			return "", "", err
		}
		answerSnapshot.SelectedOptions = append(answerSnapshot.SelectedOptions, opt)
	}
	answerSnapshot.IsCorrect = answerCorrect == 1

	for rows.Next() {
		var opt questionFeedbackSnapshotOption
		var isCorrect int
		if err := rows.Scan(&opt.OptionID, &opt.DisplayOrder, &opt.Text, &isCorrect); err != nil {
			return "", "", err
		}
		opt.IsCorrect = isCorrect == 1
		questionSnapshot.Options = append(questionSnapshot.Options, opt)
	}

	questionSnapshotJSON, err := json.Marshal(questionSnapshot)
	if err != nil {
		return "", "", err
	}
	answerSnapshotJSON, err := json.Marshal(answerSnapshot)
	if err != nil {
		return "", "", err
	}
	return string(questionSnapshotJSON), string(answerSnapshotJSON), nil
}

func isValidFeedbackType(v string) bool {
	switch v {
	case "incorrect_answer", "ambiguous_wording", "outdated_content", "typo_or_formatting", "other":
		return true
	default:
		return false
	}
}
