package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"simsexam/internal/app"
	"simsexam/internal/bootstrap"
	"simsexam/internal/buildinfo"
	"simsexam/internal/config"
	"simsexam/internal/database"
	"simsexam/internal/handlers"
)

func TestHomeRendersPublishedSubject(t *testing.T) {
	setupHandlerTestEnv(t)
	oldVersion, oldCommit, oldBuildTime := buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime
	t.Cleanup(func() {
		buildinfo.Version = oldVersion
		buildinfo.Commit = oldCommit
		buildinfo.BuildTime = oldBuildTime
	})
	buildinfo.Version = "v9.9.9"
	buildinfo.Commit = "abcdef123456"

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handlers.Home(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SE Demo Subject") {
		t.Fatalf("expected home page to contain seeded subject title, got body: %s", body)
	}
	if !strings.Contains(body, "Version v9.9.9 · abcdef1") {
		t.Fatalf("expected home page footer to contain version summary, got body: %s", body)
	}
}

func TestExamFlowPerfectScore(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()

	examID := startExam(t, router, 1)
	totalQuestions := examQuestionCountForTest(t, examID)
	if totalQuestions == 0 {
		t.Fatal("expected seeded exam to have questions")
	}

	for position := 1; position <= totalQuestions; position++ {
		questionID, correctOptionIDs := examQuestionAndCorrectOptions(t, examID, position)
		postAnswer(t, router, examID, questionID, position, correctOptionIDs, position == totalQuestions)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/exam/%d/result", examID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on result page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "100") {
		t.Fatalf("expected perfect score in result page, got body: %s", body)
	}
	if !strings.Contains(body, "Perfect Score!") {
		t.Fatalf("expected perfect score message, got body: %s", body)
	}
}

func TestExamFlowShowsIncorrectReview(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()

	examID := startExam(t, router, 1)
	totalQuestions := examQuestionCountForTest(t, examID)
	if totalQuestions == 0 {
		t.Fatal("expected seeded exam to have questions")
	}

	for position := 1; position <= totalQuestions; position++ {
		questionID, correctOptionIDs := examQuestionAndCorrectOptions(t, examID, position)
		selected := correctOptionIDs
		if position == 1 {
			selected = wrongOptionIDs(t, questionID, correctOptionIDs)
		}
		postAnswer(t, router, examID, questionID, position, selected, position == totalQuestions)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/exam/%d/result", examID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on result page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Review Incorrect Answers") {
		t.Fatalf("expected incorrect review section, got body: %s", body)
	}
	if !strings.Contains(body, "Correct Answer:") {
		t.Fatalf("expected correct answer text in review, got body: %s", body)
	}
}

func TestExamStartPersistsOptionDisplayOrder(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()

	examID := startExam(t, router, 1)

	rows, err := database.DB.Query(`
		SELECT eq.id, eq.question_id
		FROM exam_questions eq
		WHERE eq.exam_id = ?
		ORDER BY eq.position
	`, examID)
	if err != nil {
		t.Fatalf("query exam questions failed: %v", err)
	}
	defer rows.Close()

	var seen int
	for rows.Next() {
		var examQuestionID int
		var questionID int
		if err := rows.Scan(&examQuestionID, &questionID); err != nil {
			t.Fatalf("scan exam question failed: %v", err)
		}
		seen++

		var persistedCount int
		if err := database.DB.QueryRow(`
			SELECT COUNT(*)
			FROM exam_question_options
			WHERE exam_question_id = ?
		`, examQuestionID).Scan(&persistedCount); err != nil {
			t.Fatalf("count exam question options failed: %v", err)
		}

		var canonicalCount int
		if err := database.DB.QueryRow(`
			SELECT COUNT(*)
			FROM question_options
			WHERE question_id = ?
		`, questionID).Scan(&canonicalCount); err != nil {
			t.Fatalf("count canonical options failed: %v", err)
		}

		if persistedCount != canonicalCount {
			t.Fatalf("expected %d persisted option rows for exam_question %d, got %d", canonicalCount, examQuestionID, persistedCount)
		}
	}
	if seen == 0 {
		t.Fatal("expected at least one persisted exam question")
	}

	firstReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/exam/%d/question/1", examID), nil)
	firstRec := httptest.NewRecorder()
	router.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on first question render, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/exam/%d/question/1", examID), nil)
	secondRec := httptest.NewRecorder()
	router.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on repeated question render, got %d", secondRec.Code)
	}

	if firstRec.Body.String() != secondRec.Body.String() {
		t.Fatalf("expected repeated question renders to preserve option order")
	}
}

func TestAdminSubjectsAndQuestionsPages(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	req := httptest.NewRequest(http.MethodGet, "/admin/subjects", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SE Demo Subject") {
		t.Fatalf("expected admin subjects page to contain seeded subject, got body: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "shuffle off") {
		t.Fatalf("expected admin subjects page to show subject shuffle status, got body: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/subjects/1/questions", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "demo-001") || !strings.Contains(body, "demo-002") {
		t.Fatalf("expected admin questions page to contain seeded keys, got body: %s", body)
	}
	if !strings.Contains(body, "active") {
		t.Fatalf("expected admin questions page to show question status, got body: %s", body)
	}
	if !strings.Contains(body, "inherit") {
		t.Fatalf("expected admin questions page to show question shuffle mode, got body: %s", body)
	}
}

func TestAdminEditSubjectUpdatesShuffleDefault(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	req := httptest.NewRequest(http.MethodGet, "/admin/subjects/1/edit", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on subject settings form, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Subject Settings") {
		t.Fatalf("expected subject settings page, got body: %s", rec.Body.String())
	}

	form := url.Values{}
	form.Set("shuffle_options_default", "on")

	req = httptest.NewRequest(http.MethodPost, "/admin/subjects/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from subject settings submit, got %d with body %s", rec.Code, rec.Body.String())
	}

	var shuffleDefault int
	if err := database.DB.QueryRow(`SELECT shuffle_options_default FROM subjects WHERE id = 1`).Scan(&shuffleDefault); err != nil {
		t.Fatalf("query updated subject shuffle default failed: %v", err)
	}
	if shuffleDefault != 1 {
		t.Fatalf("expected subject shuffle default 1, got %d", shuffleDefault)
	}
}

func TestAdminImportMarkdownText(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	form := url.Values{}
	form.Set("markdown_text", `# Subject: admin-demo

## Meta
- slug: admin-demo
- title: Admin Demo Subject
- description: Imported from admin test
- duration_minutes: 15
- question_count: 1
- access_level: free
- status: published
- version: 2026-04-23

---

## Question
key: admin-001
type: single

What is 2 + 2?

- [x] 4
- [ ] 3
- [ ] 5
- [ ] 6

### Explanation
2 + 2 equals 4.
`)

	body := bytes.NewBufferString(form.Encode())
	req := httptest.NewRequest(http.MethodPost, "/admin/import", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Import Completed") {
		t.Fatalf("expected import completed page, got body: %s", rec.Body.String())
	}

	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM subjects WHERE slug = 'admin-demo'`).Scan(&count); err != nil {
		t.Fatalf("count imported subject failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected imported subject count 1, got %d", count)
	}
}

func TestAdminImportMarkdownFile(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("markdown_file", "upload-demo.md")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte(`# Subject: upload-demo

## Meta
- slug: upload-demo
- title: Upload Demo Subject
- description: Imported from uploaded file
- duration_minutes: 10
- question_count: 1
- access_level: free
- status: published
- version: 2026-04-23

---

## Question
key: upload-001
type: single

Which letter comes first?

- [x] A
- [ ] B
- [ ] C
- [ ] D

### Explanation
A comes before B, C, and D.
`)); err != nil {
		t.Fatalf("write upload content failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Upload Demo Subject") {
		t.Fatalf("expected import result to mention uploaded subject, got body: %s", rec.Body.String())
	}
}

func TestAdminEditQuestionUpdatesQuestionAndCreatesRevision(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	req := httptest.NewRequest(http.MethodGet, "/admin/questions/1/edit", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on edit form, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Edit Question") {
		t.Fatalf("expected edit question page, got body: %s", rec.Body.String())
	}

	form := url.Values{}
	form.Set("type", "single")
	form.Set("stem", "What color is the daytime sky under clear weather?")
	form.Set("explanation", "The sky usually appears blue because of Rayleigh scattering.")
	form.Set("allow_option_shuffle", "disable")
	form.Set("change_summary", "Clarified wording and explanation")
	form.Add("option_id", "1")
	form.Add("option_id", "2")
	form.Add("option_id", "3")
	form.Add("option_id", "4")
	form.Add("option_text", "Blue")
	form.Add("option_text", "Green")
	form.Add("option_text", "Orange")
	form.Add("option_text", "Purple")
	form.Add("correct_index", "0")

	req = httptest.NewRequest(http.MethodPost, "/admin/questions/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from edit submit, got %d with body %s", rec.Code, rec.Body.String())
	}

	var stem string
	var explanation string
	var allowOptionShuffle sql.NullInt64
	if err := database.DB.QueryRow(`
		SELECT stem_markdown, COALESCE(explanation_markdown, ''), allow_option_shuffle
		FROM questions
		WHERE id = 1
	`).Scan(&stem, &explanation, &allowOptionShuffle); err != nil {
		t.Fatalf("query updated question failed: %v", err)
	}
	if stem != "What color is the daytime sky under clear weather?" {
		t.Fatalf("unexpected updated stem: %q", stem)
	}
	if !strings.Contains(explanation, "Rayleigh scattering") {
		t.Fatalf("unexpected updated explanation: %q", explanation)
	}
	if !allowOptionShuffle.Valid || allowOptionShuffle.Int64 != 0 {
		t.Fatalf("expected allow_option_shuffle to be 0 after edit, got %+v", allowOptionShuffle)
	}

	var revisionCount int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM question_revisions WHERE question_id = 1`).Scan(&revisionCount); err != nil {
		t.Fatalf("count question revisions failed: %v", err)
	}
	if revisionCount != 1 {
		t.Fatalf("expected 1 question revision, got %d", revisionCount)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/questions/1/history", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on question history page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Clarified wording and explanation") {
		t.Fatalf("expected history page to contain change summary, got body: %s", body)
	}
	if !strings.Contains(body, "What color is the sky on a clear day?") {
		t.Fatalf("expected history page to contain previous stem snapshot, got body: %s", body)
	}
}

func TestAdminArchiveSubjectRemovesItFromHome(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	req := httptest.NewRequest(http.MethodPost, "/admin/subjects/1/archive", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from archive submit, got %d with body %s", rec.Code, rec.Body.String())
	}

	var status string
	if err := database.DB.QueryRow(`SELECT status FROM subjects WHERE id = 1`).Scan(&status); err != nil {
		t.Fatalf("query archived subject failed: %v", err)
	}
	if status != "archived" {
		t.Fatalf("expected subject status archived, got %q", status)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on home page, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "SE Demo Subject") {
		t.Fatalf("expected archived subject to disappear from home page, got body: %s", rec.Body.String())
	}
}

func TestAdminDisableQuestionUpdatesStatusAndCreatesRevision(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	req := httptest.NewRequest(http.MethodPost, "/admin/questions/1/disable", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from disable submit, got %d with body %s", rec.Code, rec.Body.String())
	}

	var status string
	if err := database.DB.QueryRow(`SELECT status FROM questions WHERE id = 1`).Scan(&status); err != nil {
		t.Fatalf("query disabled question failed: %v", err)
	}
	if status != "disabled" {
		t.Fatalf("expected question status disabled, got %q", status)
	}

	var revisionCount int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM question_revisions WHERE question_id = 1`).Scan(&revisionCount); err != nil {
		t.Fatalf("count question revisions after disable failed: %v", err)
	}
	if revisionCount != 1 {
		t.Fatalf("expected 1 question revision after disable, got %d", revisionCount)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/subjects/1/questions", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on admin questions page, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("expected disabled status on admin questions page, got body: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/questions/1/history", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on question history page after disable, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Question disabled from admin list") {
		t.Fatalf("expected disable summary in history page, got body: %s", rec.Body.String())
	}
}

func TestResultPageRendersMarkdownExplanationAsHTML(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()

	if _, err := database.DB.Exec(`
		UPDATE questions
		SET explanation_markdown = '**Bold note** with ` + "`code`" + `.'
	`); err != nil {
		t.Fatalf("update explanation_markdown failed: %v", err)
	}

	examID := startExam(t, router, 1)
	totalQuestions := examQuestionCountForTest(t, examID)
	if totalQuestions == 0 {
		t.Fatal("expected seeded exam to have questions")
	}

	firstQuestionID, correctOptionIDs := examQuestionAndCorrectOptions(t, examID, 1)
	postAnswer(t, router, examID, firstQuestionID, 1, wrongOptionIDs(t, firstQuestionID, correctOptionIDs), totalQuestions == 1)

	for position := 2; position <= totalQuestions; position++ {
		questionID, correctIDs := examQuestionAndCorrectOptions(t, examID, position)
		postAnswer(t, router, examID, questionID, position, correctIDs, position == totalQuestions)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/exam/%d/result", examID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on result page, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<strong>Bold note</strong>`) {
		t.Fatalf("expected rendered bold markdown in result page, got body: %s", body)
	}
	if !strings.Contains(body, `<code>code</code>`) {
		t.Fatalf("expected rendered code markdown in result page, got body: %s", body)
	}
	if strings.Contains(body, `<p><p>`) {
		t.Fatalf("expected explanation HTML not to be wrapped in nested paragraphs, got body: %s", body)
	}
}

func TestQuestionFeedbackSubmissionAndAdminReview(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()
	adminCookie := adminSessionCookie(t, router)

	examID := startExam(t, router, 1)
	totalQuestions := examQuestionCountForTest(t, examID)
	if totalQuestions == 0 {
		t.Fatal("expected seeded exam to have questions")
	}

	firstQuestionID, correctOptionIDs := examQuestionAndCorrectOptions(t, examID, 1)
	postAnswer(t, router, examID, firstQuestionID, 1, wrongOptionIDs(t, firstQuestionID, correctOptionIDs), totalQuestions == 1)

	for position := 2; position <= totalQuestions; position++ {
		questionID, correctIDs := examQuestionAndCorrectOptions(t, examID, position)
		postAnswer(t, router, examID, questionID, position, correctIDs, position == totalQuestions)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/exam/%d/result", examID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on result page, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Report a question issue") {
		t.Fatalf("expected result page to contain question feedback toggle, got body: %s", rec.Body.String())
	}

	form := url.Values{}
	form.Set("question_id", strconv.Itoa(firstQuestionID))
	form.Set("feedback_type", "incorrect_answer")
	form.Set("comment", "The answer key looks wrong for this question.")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/exam/%d/feedback", examID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from feedback submit, got %d with body %s", rec.Code, rec.Body.String())
	}
	expectedLocation := fmt.Sprintf("/exam/%d/result?feedback=submitted", examID)
	if rec.Header().Get("Location") != expectedLocation {
		t.Fatalf("expected feedback redirect %q, got %q", expectedLocation, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, expectedLocation, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on result page after feedback, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Feedback submitted") {
		t.Fatalf("expected feedback success banner, got body: %s", rec.Body.String())
	}

	var feedbackID int
	var status string
	var comment string
	if err := database.DB.QueryRow(`
		SELECT id, status, COALESCE(comment, '')
		FROM question_feedback
		WHERE exam_id = ? AND question_id = ?
	`, examID, firstQuestionID).Scan(&feedbackID, &status, &comment); err != nil {
		t.Fatalf("query feedback row failed: %v", err)
	}
	if status != "open" {
		t.Fatalf("expected feedback status open, got %q", status)
	}
	if !strings.Contains(comment, "answer key") {
		t.Fatalf("unexpected feedback comment: %q", comment)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/feedback", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on admin feedback list, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "incorrect_answer") || !strings.Contains(body, "View") {
		t.Fatalf("expected feedback list entry, got body: %s", body)
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/feedback/%d", feedbackID), nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on admin feedback detail, got %d", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, "The answer key looks wrong for this question.") {
		t.Fatalf("expected admin feedback detail to contain learner comment, got body: %s", body)
	}
	if !strings.Contains(body, "Question Snapshot") || !strings.Contains(body, "Answer Snapshot") {
		t.Fatalf("expected admin feedback detail to contain snapshots, got body: %s", body)
	}

	resolveForm := url.Values{}
	resolveForm.Set("resolution_note", "Reviewed and corrected the answer key.")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/feedback/%d/resolve", feedbackID), strings.NewReader(resolveForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from feedback resolve, got %d with body %s", rec.Code, rec.Body.String())
	}

	var resolutionNote string
	if err := database.DB.QueryRow(`
		SELECT status, COALESCE(resolution_note, '')
		FROM question_feedback
		WHERE id = ?
	`, feedbackID).Scan(&status, &resolutionNote); err != nil {
		t.Fatalf("query resolved feedback failed: %v", err)
	}
	if status != "resolved" {
		t.Fatalf("expected feedback status resolved, got %q", status)
	}
	if !strings.Contains(resolutionNote, "corrected") {
		t.Fatalf("unexpected resolution note: %q", resolutionNote)
	}
}

func TestAdminRoutesRedirectWithoutSession(t *testing.T) {
	setupHandlerTestEnv(t)
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/admin/subjects", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for unauthenticated admin request, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("expected redirect to /admin/login, got %q", rec.Header().Get("Location"))
	}
}

func TestAdminRoutesFailClosedWithoutConfiguration(t *testing.T) {
	setupHandlerTestEnv(t)
	t.Setenv(config.EnvAdminPassword, "")
	t.Setenv(config.EnvAdminSessionKey, "")
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/admin/subjects", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when admin access is not configured, got %d", rec.Code)
	}
}

func setupHandlerTestEnv(t *testing.T) {
	t.Helper()

	changeToRepoRoot(t)
	t.Setenv(config.EnvAdminPassword, "admin-pass")
	t.Setenv(config.EnvAdminSessionKey, "admin-session-secret")

	dbPath := filepath.Join(t.TempDir(), "handlers.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		if database.DB != nil {
			_ = database.DB.Close()
			database.DB = nil
		}
	})

	seedPath := filepath.Join("docs", "examples", "se-demo.md")
	if _, err := bootstrap.PrepareV1Database(context.Background(), database.DB, bootstrap.V1BootstrapOptions{
		SeedFiles: []string{seedPath},
	}); err != nil {
		t.Fatalf("PrepareV1Database failed: %v", err)
	}
}

func changeToRepoRoot(t *testing.T) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(original, "..", ".."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir to repo root failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
}

func newTestRouter() http.Handler {
	return app.NewRouter(config.LoadServerConfig())
}

func adminSessionCookie(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()

	form := url.Values{}
	form.Set("password", "admin-pass")

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from admin login, got %d with body %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "simsexam_admin_session" {
			return cookie
		}
	}
	t.Fatal("expected admin session cookie after login")
	return nil
}

func startExam(t *testing.T, router http.Handler, subjectID int) int {
	t.Helper()

	form := url.Values{}
	form.Set("subject_id", strconv.Itoa(subjectID))

	req := httptest.NewRequest(http.MethodPost, "/exam/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from start exam, got %d with body %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	re := regexp.MustCompile(`/exam/(\d+)/question/1`)
	match := re.FindStringSubmatch(location)
	if match == nil {
		t.Fatalf("unexpected redirect location: %s", location)
	}
	examID, _ := strconv.Atoi(match[1])
	return examID
}

func examQuestionCountForTest(t *testing.T, examID int) int {
	t.Helper()

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM exam_questions WHERE exam_id = ?`, examID).Scan(&total); err != nil {
		t.Fatalf("count exam questions failed: %v", err)
	}
	return total
}

func examQuestionAndCorrectOptions(t *testing.T, examID, position int) (int, []int) {
	t.Helper()

	var questionID int
	if err := database.DB.QueryRow(`
		SELECT question_id
		FROM exam_questions
		WHERE exam_id = ? AND position = ?
	`, examID, position).Scan(&questionID); err != nil {
		t.Fatalf("query exam question failed: %v", err)
	}

	rows, err := database.DB.Query(`
		SELECT id
		FROM question_options
		WHERE question_id = ? AND is_correct = 1
		ORDER BY sort_order
	`, questionID)
	if err != nil {
		t.Fatalf("query correct options failed: %v", err)
	}
	defer rows.Close()

	var optionIDs []int
	for rows.Next() {
		var optionID int
		if err := rows.Scan(&optionID); err != nil {
			t.Fatalf("scan correct option failed: %v", err)
		}
		optionIDs = append(optionIDs, optionID)
	}
	return questionID, optionIDs
}

func wrongOptionIDs(t *testing.T, questionID int, correctOptionIDs []int) []int {
	t.Helper()

	correctSet := make(map[int]bool, len(correctOptionIDs))
	for _, optionID := range correctOptionIDs {
		correctSet[optionID] = true
	}

	rows, err := database.DB.Query(`
		SELECT id
		FROM question_options
		WHERE question_id = ? AND is_correct = 0
		ORDER BY sort_order
	`, questionID)
	if err != nil {
		t.Fatalf("query wrong options failed: %v", err)
	}
	defer rows.Close()

	var wrong []int
	for rows.Next() {
		var optionID int
		if err := rows.Scan(&optionID); err != nil {
			t.Fatalf("scan wrong option failed: %v", err)
		}
		wrong = append(wrong, optionID)
	}
	if len(correctOptionIDs) == 1 {
		return wrong[:1]
	}
	return wrong[:len(correctOptionIDs)]
}

func postAnswer(t *testing.T, router http.Handler, examID, questionID, position int, optionIDs []int, final bool) {
	t.Helper()

	form := url.Values{}
	form.Set("question_id", strconv.Itoa(questionID))
	form.Set("current_index", strconv.Itoa(position))
	for _, optionID := range optionIDs {
		form.Add("option_id", strconv.Itoa(optionID))
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/exam/%d/answer", examID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 from submit answer, got %d with body %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if final {
		expected := fmt.Sprintf("/exam/%d/result", examID)
		if location != expected {
			t.Fatalf("expected final redirect %s, got %s", expected, location)
		}
		return
	}

	expected := fmt.Sprintf("/exam/%d/question/%d", examID, position+1)
	if location != expected {
		t.Fatalf("expected next question redirect %s, got %s", expected, location)
	}
}
