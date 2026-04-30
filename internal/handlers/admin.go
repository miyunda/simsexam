package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"simsexam/internal/database"
	"simsexam/internal/importer"

	"github.com/go-chi/chi/v5"
)

type adminSubjectRow struct {
	ID                 int
	Slug               string
	Title              string
	Description        string
	Status             string
	AccessLevel        string
	DurationMinutes    int
	QuestionCount      int
	CurrentQuestionSet int
	CurrentQuestions   int
	ShuffleOptions     bool
}

type adminQuestionRow struct {
	ID          int
	Key         string
	Type        string
	Status      string
	ShuffleMode string
	Stem        string
	Explanation string
	OptionCount int
}

type adminOptionRow struct {
	ID        int
	SortOrder int
	Text      string
	IsCorrect bool
}

type adminEditQuestionData struct {
	SubjectID          int
	SubjectTitle       string
	QuestionID         int
	Key                string
	Type               string
	Stem               string
	Explanation        string
	AllowOptionShuffle string
	ChangeSummary      string
	Options            []adminOptionRow
	Errors             []string
	Warnings           []string
}

type adminEditSubjectData struct {
	SubjectID             int
	Title                 string
	Slug                  string
	ShuffleOptionsDefault bool
	Errors                []string
}

type adminQuestionRevisionRow struct {
	ID            int
	CreatedAt     string
	ChangeSummary string
	Question      adminQuestionRevisionSnapshot
}

type adminQuestionRevisionSnapshot struct {
	Key         string
	Type        string
	Status      string
	ShuffleMode string
	Stem        string
	Explanation string
	Options     []adminQuestionRevisionOption
}

type adminQuestionRevisionOption struct {
	SortOrder int
	Text      string
	IsCorrect bool
}

func AdminSubjects(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
		SELECT
			s.id,
			s.slug,
			s.title,
			COALESCE(s.description, ''),
			s.status,
			s.access_level,
			s.duration_minutes,
			s.question_count,
			s.shuffle_options_default,
			COALESCE(s.current_question_set_id, 0),
			(
				SELECT COUNT(*)
				FROM questions q
				WHERE q.subject_id = s.id
				  AND q.question_set_id = s.current_question_set_id
			) AS current_questions
		FROM subjects s
		ORDER BY s.title
	`)
	if err != nil {
		http.Error(w, "Failed to load admin subjects", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var subjects []adminSubjectRow
	for rows.Next() {
		var row adminSubjectRow
		if err := rows.Scan(
			&row.ID,
			&row.Slug,
			&row.Title,
			&row.Description,
			&row.Status,
			&row.AccessLevel,
			&row.DurationMinutes,
			&row.QuestionCount,
			&row.ShuffleOptions,
			&row.CurrentQuestionSet,
			&row.CurrentQuestions,
		); err != nil {
			http.Error(w, "Failed to load admin subjects", http.StatusInternalServerError)
			return
		}
		subjects = append(subjects, row)
	}

	renderTemplate(w, "admin_subjects.html", struct {
		Subjects []adminSubjectRow
	}{
		Subjects: subjects,
	})
}

func AdminImportForm(w http.ResponseWriter, r *http.Request) {
	renderAdminImportForm(w, http.StatusOK, "", "", nil, nil)
}

func AdminImportSubmit(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			renderAdminImportForm(w, http.StatusBadRequest, "", "", []string{"Failed to parse import form."}, nil)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			renderAdminImportForm(w, http.StatusBadRequest, "", "", []string{"Failed to parse import form."}, nil)
			return
		}
	}

	content, sourceName, rawText, err := importContentFromRequest(r)
	if err != nil {
		renderAdminImportForm(w, http.StatusBadRequest, "", rawText, []string{err.Error()}, nil)
		return
	}

	doc, err := importer.ParseString(content)
	if err != nil {
		renderAdminImportForm(w, http.StatusBadRequest, sourceName, rawText, []string{err.Error()}, nil)
		return
	}

	report := importer.ValidateDocument(doc)
	if !report.Valid() {
		renderAdminImportForm(w, http.StatusBadRequest, sourceName, rawText, validationMessages(report.Errors), validationMessages(report.Warnings))
		return
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	result, err := importer.ImportDocument(context.Background(), database.DB, doc, importer.ImportOptions{
		SourceType:     "markdown_import",
		SourceFilename: sourceName,
		SourceChecksum: checksum,
		Activate:       true,
	})
	if err != nil {
		renderAdminImportForm(w, http.StatusInternalServerError, sourceName, rawText, []string{"Import failed: " + err.Error()}, validationMessages(report.Warnings))
		return
	}

	renderTemplate(w, "admin_import_result.html", struct {
		SubjectTitle   string
		SubjectSlug    string
		SourceName     string
		QuestionSetID  int64
		ImportJobID    int64
		QuestionsCount int
		OptionsCount   int
		Warnings       []string
	}{
		SubjectTitle:   doc.Manifest.Title,
		SubjectSlug:    doc.Manifest.Slug,
		SourceName:     sourceName,
		QuestionSetID:  result.QuestionSetID,
		ImportJobID:    result.ImportJobID,
		QuestionsCount: result.QuestionsCount,
		OptionsCount:   result.OptionsCount,
		Warnings:       validationMessages(report.Warnings),
	})
}

func AdminSubjectQuestions(w http.ResponseWriter, r *http.Request) {
	subjectID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if subjectID == 0 {
		http.Error(w, "Invalid subject", http.StatusBadRequest)
		return
	}

	var subject adminSubjectRow
	err := database.DB.QueryRow(`
		SELECT id, slug, title, COALESCE(description, ''), status, access_level, duration_minutes, question_count, shuffle_options_default, COALESCE(current_question_set_id, 0)
		FROM subjects
		WHERE id = ?
	`, subjectID).Scan(
		&subject.ID,
		&subject.Slug,
		&subject.Title,
		&subject.Description,
		&subject.Status,
		&subject.AccessLevel,
		&subject.DurationMinutes,
		&subject.QuestionCount,
		&subject.ShuffleOptions,
		&subject.CurrentQuestionSet,
	)
	if err != nil {
		http.Error(w, "Subject not found", http.StatusNotFound)
		return
	}

	rows, err := database.DB.Query(`
		SELECT q.id, q.external_key, q.type, q.stem_markdown, COALESCE(q.explanation_markdown, ''),
		       q.status,
		       CASE
		         WHEN q.allow_option_shuffle IS NULL THEN 'inherit'
		         WHEN q.allow_option_shuffle = 1 THEN 'allow'
		         ELSE 'disable'
		       END AS shuffle_mode,
		       (SELECT COUNT(*) FROM question_options qo WHERE qo.question_id = q.id) AS option_count
		FROM questions q
		WHERE q.subject_id = ? AND q.question_set_id = ?
		ORDER BY q.external_key
	`, subjectID, subject.CurrentQuestionSet)
	if err != nil {
		http.Error(w, "Failed to load questions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var questions []adminQuestionRow
	for rows.Next() {
		var row adminQuestionRow
		if err := rows.Scan(&row.ID, &row.Key, &row.Type, &row.Stem, &row.Explanation, &row.Status, &row.ShuffleMode, &row.OptionCount); err != nil {
			http.Error(w, "Failed to load questions", http.StatusInternalServerError)
			return
		}
		row.Stem = cleanQuestionText(row.Stem)
		questions = append(questions, row)
	}

	renderTemplate(w, "admin_questions.html", struct {
		Subject   adminSubjectRow
		Questions []adminQuestionRow
	}{
		Subject:   subject,
		Questions: questions,
	})
}

func AdminEditSubjectForm(w http.ResponseWriter, r *http.Request) {
	data, err := loadAdminEditSubjectData(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Subject not found", http.StatusNotFound)
		return
	}
	renderTemplate(w, "admin_subject_edit.html", data)
}

func AdminEditSubjectSubmit(w http.ResponseWriter, r *http.Request) {
	data, err := loadAdminEditSubjectData(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Subject not found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		data.Errors = []string{"Failed to parse form."}
		renderTemplate(w, "admin_subject_edit.html", data)
		return
	}

	data.ShuffleOptionsDefault = r.FormValue("shuffle_options_default") == "on"
	if _, err := database.DB.Exec(`
		UPDATE subjects
		SET shuffle_options_default = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, boolToInt(data.ShuffleOptionsDefault), data.SubjectID); err != nil {
		data.Errors = []string{"Failed to save subject settings: " + err.Error()}
		renderTemplate(w, "admin_subject_edit.html", data)
		return
	}

	http.Redirect(w, r, "/admin/subjects", http.StatusSeeOther)
}

func AdminArchiveSubject(w http.ResponseWriter, r *http.Request) {
	updateAdminSubjectStatus(w, r, "archived", "Failed to archive subject")
}

func AdminRestoreSubject(w http.ResponseWriter, r *http.Request) {
	updateAdminSubjectStatus(w, r, "published", "Failed to restore subject")
}

func AdminDisableQuestion(w http.ResponseWriter, r *http.Request) {
	updateAdminQuestionStatus(w, r, "disabled", "Disabled question from admin question list", "Failed to disable question")
}

func AdminEnableQuestion(w http.ResponseWriter, r *http.Request) {
	updateAdminQuestionStatus(w, r, "active", "Re-enabled question from admin question list", "Failed to enable question")
}

func updateAdminSubjectStatus(w http.ResponseWriter, r *http.Request, nextStatus, failureMessage string) {
	subjectID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if subjectID == 0 {
		http.Error(w, "Invalid subject", http.StatusBadRequest)
		return
	}

	var currentStatus string
	err := database.DB.QueryRow(`
		SELECT status
		FROM subjects
		WHERE id = ?
	`, subjectID).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		http.Error(w, "Subject not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}
	if currentStatus == nextStatus {
		http.Redirect(w, r, "/admin/subjects", http.StatusSeeOther)
		return
	}

	if _, err := database.DB.Exec(`
		UPDATE subjects
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nextStatus, subjectID); err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/subjects", http.StatusSeeOther)
}

func updateAdminQuestionStatus(w http.ResponseWriter, r *http.Request, nextStatus, changeSummary, failureMessage string) {
	questionID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if questionID == 0 {
		http.Error(w, "Invalid question", http.StatusBadRequest)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var subjectID int
	var status string
	err = tx.QueryRow(`
		SELECT subject_id, status
		FROM questions
		WHERE id = ?
	`, questionID).Scan(&subjectID, &status)
	if err == sql.ErrNoRows {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}
	if status == nextStatus {
		http.Redirect(w, r, fmt.Sprintf("/admin/subjects/%d/questions", subjectID), http.StatusSeeOther)
		return
	}

	snapshot, err := questionSnapshot(tx, questionID)
	if err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}

	if _, err := tx.Exec(`
		UPDATE questions
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nextStatus, questionID); err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}

	if _, err := tx.Exec(`
		INSERT INTO question_revisions (question_id, change_summary, snapshot_json)
		VALUES (?, ?, ?)
	`, questionID, changeSummary, snapshot); err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, failureMessage, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/subjects/%d/questions", subjectID), http.StatusSeeOther)
}

func AdminQuestionHistory(w http.ResponseWriter, r *http.Request) {
	data, revisions, err := loadAdminQuestionHistoryData(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}
	renderTemplate(w, "admin_question_history.html", struct {
		Question  adminEditQuestionData
		Revisions []adminQuestionRevisionRow
	}{
		Question:  data,
		Revisions: revisions,
	})
}

func AdminEditQuestionForm(w http.ResponseWriter, r *http.Request) {
	data, err := loadAdminEditQuestionData(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}
	renderTemplate(w, "admin_question_edit.html", data)
}

func AdminEditQuestionSubmit(w http.ResponseWriter, r *http.Request) {
	data, err := loadAdminEditQuestionData(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}
	original := cloneAdminEditQuestionData(data)

	if err := r.ParseForm(); err != nil {
		data.Errors = []string{"Failed to parse form."}
		renderTemplate(w, "admin_question_edit.html", data)
		return
	}

	data.Type = strings.TrimSpace(r.FormValue("type"))
	data.Stem = strings.TrimSpace(r.FormValue("stem"))
	data.Explanation = strings.TrimSpace(r.FormValue("explanation"))
	data.AllowOptionShuffle = strings.TrimSpace(r.FormValue("allow_option_shuffle"))
	data.ChangeSummary = strings.TrimSpace(r.FormValue("change_summary"))

	optionTexts := r.Form["option_text"]
	optionIDs := r.Form["option_id"]
	correctIndexes := make(map[int]bool)
	for _, raw := range r.Form["correct_index"] {
		idx, _ := strconv.Atoi(raw)
		correctIndexes[idx] = true
	}

	if len(optionTexts) != len(data.Options) || len(optionIDs) != len(data.Options) {
		data.Errors = []string{"Option payload is inconsistent."}
		renderTemplate(w, "admin_question_edit.html", data)
		return
	}

	for i := range data.Options {
		id, _ := strconv.Atoi(optionIDs[i])
		data.Options[i].ID = id
		data.Options[i].Text = strings.TrimSpace(optionTexts[i])
		data.Options[i].IsCorrect = correctIndexes[i]
	}

	data.Errors = validateAdminEditQuestion(data)
	if len(data.Errors) > 0 {
		renderTemplate(w, "admin_question_edit.html", data)
		return
	}

	if data.ChangeSummary == "" {
		data.ChangeSummary = automaticQuestionChangeSummary(original, data)
	}

	if err := persistAdminQuestionEdit(data); err != nil {
		data.Errors = []string{"Failed to save question: " + err.Error()}
		renderTemplate(w, "admin_question_edit.html", data)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/subjects/%d/questions", data.SubjectID), http.StatusSeeOther)
}

func renderAdminImportForm(w http.ResponseWriter, status int, sourceName, markdownText string, errors, warnings []string) {
	w.WriteHeader(status)
	renderTemplate(w, "admin_import.html", struct {
		SourceName   string
		MarkdownText string
		Errors       []string
		Warnings     []string
	}{
		SourceName:   sourceName,
		MarkdownText: markdownText,
		Errors:       errors,
		Warnings:     warnings,
	})
}

func importContentFromRequest(r *http.Request) (content, sourceName, rawText string, err error) {
	rawText = strings.TrimSpace(r.FormValue("markdown_text"))
	if rawText != "" {
		return rawText, "pasted.md", rawText, nil
	}

	file, header, fileErr := r.FormFile("markdown_file")
	if fileErr != nil {
		return "", "", "", fmt.Errorf("Please upload a Markdown file or paste Markdown text.")
	}
	defer file.Close()

	data, readErr := io.ReadAll(file)
	if readErr != nil {
		return "", "", "", fmt.Errorf("Failed to read uploaded file.")
	}
	if len(data) == 0 {
		return "", "", "", fmt.Errorf("Uploaded file is empty.")
	}

	return string(data), filepath.Base(header.Filename), "", nil
}

func validationMessages(messages []importer.ValidationMessage) []string {
	result := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Line > 0 {
			result = append(result, fmt.Sprintf("Line %d [%s] %s", msg.Line, msg.Field, msg.Message))
			continue
		}
		result = append(result, fmt.Sprintf("[%s] %s", msg.Field, msg.Message))
	}
	return result
}

func loadAdminEditQuestionData(rawID string) (adminEditQuestionData, error) {
	questionID, _ := strconv.Atoi(rawID)
	if questionID == 0 {
		return adminEditQuestionData{}, sql.ErrNoRows
	}

	var data adminEditQuestionData
	err := database.DB.QueryRow(`
		SELECT q.id, q.external_key, q.type, q.stem_markdown, COALESCE(q.explanation_markdown, ''), s.id, s.title,
		       CASE
		         WHEN q.allow_option_shuffle IS NULL THEN 'inherit'
		         WHEN q.allow_option_shuffle = 1 THEN 'allow'
		         ELSE 'disable'
		       END AS allow_option_shuffle
		FROM questions q
		JOIN subjects s ON s.id = q.subject_id
		WHERE q.id = ?
	`, questionID).Scan(&data.QuestionID, &data.Key, &data.Type, &data.Stem, &data.Explanation, &data.SubjectID, &data.SubjectTitle, &data.AllowOptionShuffle)
	if err != nil {
		return adminEditQuestionData{}, err
	}

	rows, err := database.DB.Query(`
		SELECT id, sort_order, content_markdown, is_correct
		FROM question_options
		WHERE question_id = ?
		ORDER BY sort_order
	`, questionID)
	if err != nil {
		return adminEditQuestionData{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var opt adminOptionRow
		var isCorrect int
		if err := rows.Scan(&opt.ID, &opt.SortOrder, &opt.Text, &isCorrect); err != nil {
			return adminEditQuestionData{}, err
		}
		opt.IsCorrect = isCorrect == 1
		data.Options = append(data.Options, opt)
	}

	return data, nil
}

func loadAdminEditSubjectData(rawID string) (adminEditSubjectData, error) {
	subjectID, _ := strconv.Atoi(rawID)
	if subjectID == 0 {
		return adminEditSubjectData{}, sql.ErrNoRows
	}

	var data adminEditSubjectData
	err := database.DB.QueryRow(`
		SELECT id, title, slug, shuffle_options_default
		FROM subjects
		WHERE id = ?
	`, subjectID).Scan(&data.SubjectID, &data.Title, &data.Slug, &data.ShuffleOptionsDefault)
	if err != nil {
		return adminEditSubjectData{}, err
	}
	return data, nil
}

func loadAdminQuestionHistoryData(rawID string) (adminEditQuestionData, []adminQuestionRevisionRow, error) {
	data, err := loadAdminEditQuestionData(rawID)
	if err != nil {
		return adminEditQuestionData{}, nil, err
	}

	rows, err := database.DB.Query(`
		SELECT id, COALESCE(change_summary, ''), created_at, snapshot_json
		FROM question_revisions
		WHERE question_id = ?
		ORDER BY created_at DESC, id DESC
	`, data.QuestionID)
	if err != nil {
		return adminEditQuestionData{}, nil, err
	}
	defer rows.Close()

	type revisionSnapshot struct {
		Key                string `json:"key"`
		Type               string `json:"type"`
		Status             string `json:"status"`
		AllowOptionShuffle string `json:"allow_option_shuffle"`
		Stem               string `json:"stem"`
		Explanation        string `json:"explanation"`
		Options            []struct {
			SortOrder int    `json:"sort_order"`
			Text      string `json:"text"`
			IsCorrect bool   `json:"is_correct"`
		} `json:"options"`
	}

	var revisions []adminQuestionRevisionRow
	for rows.Next() {
		var (
			row          adminQuestionRevisionRow
			snapshotJSON string
			snapshot     revisionSnapshot
		)
		if err := rows.Scan(&row.ID, &row.ChangeSummary, &row.CreatedAt, &snapshotJSON); err != nil {
			return adminEditQuestionData{}, nil, err
		}
		if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
			return adminEditQuestionData{}, nil, err
		}

		row.Question = adminQuestionRevisionSnapshot{
			Key:         snapshot.Key,
			Type:        snapshot.Type,
			Status:      snapshot.Status,
			ShuffleMode: adminOptionShuffleSummaryLabel(snapshot.AllowOptionShuffle),
			Stem:        snapshot.Stem,
			Explanation: snapshot.Explanation,
		}
		for _, opt := range snapshot.Options {
			row.Question.Options = append(row.Question.Options, adminQuestionRevisionOption{
				SortOrder: opt.SortOrder,
				Text:      opt.Text,
				IsCorrect: opt.IsCorrect,
			})
		}
		revisions = append(revisions, row)
	}

	return data, revisions, nil
}

func validateAdminEditQuestion(data adminEditQuestionData) []string {
	var errors []string

	if data.Type != "single" && data.Type != "multiple" {
		errors = append(errors, "Question type must be single or multiple.")
	}
	if data.AllowOptionShuffle != "inherit" && data.AllowOptionShuffle != "allow" && data.AllowOptionShuffle != "disable" {
		errors = append(errors, "Option shuffling mode must be inherit, allow, or disable.")
	}
	if data.Stem == "" {
		errors = append(errors, "Question stem is required.")
	}
	if len(data.Options) < 2 {
		errors = append(errors, "At least two options are required.")
	}

	correctCount := 0
	for idx, opt := range data.Options {
		if opt.Text == "" {
			errors = append(errors, fmt.Sprintf("Option %d cannot be empty.", idx+1))
		}
		if opt.IsCorrect {
			correctCount++
		}
	}

	if data.Type == "single" && correctCount != 1 {
		errors = append(errors, "Single-choice questions must have exactly one correct option.")
	}
	if data.Type == "multiple" && correctCount < 2 {
		errors = append(errors, "Multiple-choice questions must have at least two correct options.")
	}

	return errors
}

func cloneAdminEditQuestionData(data adminEditQuestionData) adminEditQuestionData {
	clone := data
	clone.Options = append([]adminOptionRow(nil), data.Options...)
	return clone
}

func automaticQuestionChangeSummary(before, after adminEditQuestionData) string {
	var changes []string

	if before.Type != after.Type {
		changes = append(changes, fmt.Sprintf("Changed question type from %s to %s", before.Type, after.Type))
	}
	if before.Stem != after.Stem {
		changes = append(changes, "Updated question stem")
	}
	if before.Explanation != after.Explanation {
		changes = append(changes, "Updated explanation")
	}
	if before.AllowOptionShuffle != after.AllowOptionShuffle {
		changes = append(changes, fmt.Sprintf(
			"Changed option shuffling from %s to %s",
			adminOptionShuffleSummaryLabel(before.AllowOptionShuffle),
			adminOptionShuffleSummaryLabel(after.AllowOptionShuffle),
		))
	}
	if adminOptionTextsChanged(before.Options, after.Options) {
		changes = append(changes, "Updated option text")
	}
	if adminCorrectOptionsChanged(before.Options, after.Options) {
		changes = append(changes, "Updated correct options")
	}

	if len(changes) == 0 {
		return "Saved question without content changes"
	}
	return strings.Join(changes, "; ")
}

func adminOptionShuffleSummaryLabel(mode string) string {
	switch mode {
	case "allow":
		return "always allow"
	case "disable":
		return "never shuffle"
	default:
		return "inherit subject default"
	}
}

func adminOptionTextsChanged(before, after []adminOptionRow) bool {
	if len(before) != len(after) {
		return true
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Text != after[i].Text {
			return true
		}
	}
	return false
}

func adminCorrectOptionsChanged(before, after []adminOptionRow) bool {
	if len(before) != len(after) {
		return true
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].IsCorrect != after[i].IsCorrect {
			return true
		}
	}
	return false
}

func persistAdminQuestionEdit(data adminEditQuestionData) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	snapshot, err := questionSnapshot(tx, data.QuestionID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE questions
		SET type = ?, stem_markdown = ?, explanation_markdown = ?, allow_option_shuffle = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, data.Type, data.Stem, emptyStringToNil(data.Explanation), adminAllowOptionShuffleValue(data.AllowOptionShuffle), data.QuestionID); err != nil {
		return err
	}

	for _, opt := range data.Options {
		if _, err := tx.Exec(`
			UPDATE question_options
			SET content_markdown = ?, is_correct = ?
			WHERE id = ?
		`, opt.Text, boolToInt(opt.IsCorrect), opt.ID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO question_revisions (question_id, change_summary, snapshot_json)
		VALUES (?, ?, ?)
	`, data.QuestionID, data.ChangeSummary, snapshot); err != nil {
		return err
	}

	return tx.Commit()
}

func questionSnapshot(tx *sql.Tx, questionID int) (string, error) {
	type snapshotOption struct {
		ID        int    `json:"id"`
		SortOrder int    `json:"sort_order"`
		Text      string `json:"text"`
		IsCorrect bool   `json:"is_correct"`
	}
	type snapshotQuestion struct {
		ID                 int              `json:"id"`
		Key                string           `json:"key"`
		Type               string           `json:"type"`
		Status             string           `json:"status"`
		AllowOptionShuffle string           `json:"allow_option_shuffle"`
		Stem               string           `json:"stem"`
		Explanation        string           `json:"explanation"`
		Options            []snapshotOption `json:"options"`
	}

	var snap snapshotQuestion
	if err := tx.QueryRow(`
		SELECT id, external_key, type, status,
		       CASE
		         WHEN allow_option_shuffle IS NULL THEN 'inherit'
		         WHEN allow_option_shuffle = 1 THEN 'allow'
		         ELSE 'disable'
		       END AS allow_option_shuffle,
		       stem_markdown,
		       COALESCE(explanation_markdown, '')
		FROM questions
		WHERE id = ?
	`, questionID).Scan(
		&snap.ID,
		&snap.Key,
		&snap.Type,
		&snap.Status,
		&snap.AllowOptionShuffle,
		&snap.Stem,
		&snap.Explanation,
	); err != nil {
		return "", err
	}

	rows, err := tx.Query(`
		SELECT id, sort_order, content_markdown, is_correct
		FROM question_options
		WHERE question_id = ?
		ORDER BY sort_order
	`, questionID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var opt snapshotOption
		var isCorrect int
		if err := rows.Scan(&opt.ID, &opt.SortOrder, &opt.Text, &isCorrect); err != nil {
			return "", err
		}
		opt.IsCorrect = isCorrect == 1
		snap.Options = append(snap.Options, opt)
	}

	payload, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func emptyStringToNil(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func adminAllowOptionShuffleValue(mode string) any {
	switch mode {
	case "allow":
		return 1
	case "disable":
		return 0
	default:
		return nil
	}
}
