package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"simsexam/internal/database"
	"simsexam/internal/models"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gomarkdown/markdown"
)

// In-memory store for active sessions (for simplicity/prototype)
// In production, use Redis or DB for session state
var activeSessions = struct {
	sync.RWMutex
	m map[int]*SessionData
}{m: make(map[int]*SessionData)}

type SessionData struct {
	ExamSessionID int
	QuestionIDs   []int
	CurrentIndex  int
	Answers       map[int][]int // QuestionID -> []OptionID
}

var questionNumRegex = regexp.MustCompile(`(?i)^(?:question\s+)?\d+[:.]\s+`)

func cleanQuestionText(text string) string {
	return questionNumRegex.ReplaceAllString(text, "")
}

func StartExam(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse form error", 400)
		return
	}
	subjectIDInt, _ := strconv.Atoi(r.FormValue("subject_id"))
	if subjectIDInt == 0 {
		http.Error(w, "Invalid subject", 400)
		return
	}

	// Create Exam Session in DB
	res, err := database.DB.Exec("INSERT INTO exam_sessions (subject_id) VALUES (?)", subjectIDInt)
	if err != nil {
		http.Error(w, "Failed to start exam", 500)
		return
	}
	sessionID, _ := res.LastInsertId()

	// 3. Get Questions (shuffled)
	var limit int
	// Get limit for this specific subject
	err = database.DB.QueryRow("SELECT question_limit FROM subjects WHERE id = ?", subjectIDInt).Scan(&limit)
	if err != nil {
		log.Printf("Error getting question limit for subject %d: %v", subjectIDInt, err)
		limit = 10 // Fallback
	}

	rows, err := database.DB.Query("SELECT id FROM questions WHERE subject_id = ? ORDER BY RANDOM() LIMIT ?", subjectIDInt, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var qIDs []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		qIDs = append(qIDs, id)
	}

	// Store session
	activeSessions.Lock()
	activeSessions.m[int(sessionID)] = &SessionData{
		ExamSessionID: int(sessionID),
		QuestionIDs:   qIDs,
		CurrentIndex:  0,
		Answers:       make(map[int][]int),
	}
	activeSessions.Unlock()

	http.Redirect(w, r, fmt.Sprintf("/exam/%d/question/1", sessionID), http.StatusSeeOther)
}

func GetQuestion(w http.ResponseWriter, r *http.Request) {
	sessionID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	qIdx, _ := strconv.Atoi(chi.URLParam(r, "qIdx"))

	activeSessions.RLock()
	session, exists := activeSessions.m[sessionID]
	activeSessions.RUnlock()

	if !exists || qIdx < 1 || qIdx > len(session.QuestionIDs) {
		http.Error(w, "Invalid session or question index", 400)
		return
	}

	qID := session.QuestionIDs[qIdx-1]

	// Fetch Question and Options
	var q models.Question
	err := database.DB.QueryRow("SELECT id, text, type FROM questions WHERE id = ?", qID).Scan(&q.ID, &q.Text, &q.Type)
	if err != nil {
		http.Error(w, "Question not found", 404)
		return
	}
	q.Text = cleanQuestionText(q.Text)

	rows, err := database.DB.Query("SELECT id, text FROM options WHERE question_id = ?", qID)
	if err != nil {
		http.Error(w, "Options error", 500)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var o models.Option
		rows.Scan(&o.ID, &o.Text)
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
		SessionID:    sessionID,
		Question:     q,
		CurrentIndex: qIdx,
		Total:        len(session.QuestionIDs),
		NextIndex:    qIdx + 1,
		PrevIndex:    qIdx - 1,
	}

	renderTemplate(w, "exam.html", data)
}

func SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	sessionID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Parse form error", 400)
		return
	}

	qID, _ := strconv.Atoi(r.FormValue("question_id"))
	qIdx, _ := strconv.Atoi(r.FormValue("current_index"))

	// multiple options
	var selectedOpts []int
	for _, v := range r.Form["option_id"] {
		id, _ := strconv.Atoi(v)
		selectedOpts = append(selectedOpts, id)
	}

	var totalQuestions int
	activeSessions.Lock()
	if session, exists := activeSessions.m[sessionID]; exists {
		session.Answers[qID] = selectedOpts
		session.CurrentIndex = qIdx // update progress
		totalQuestions = len(session.QuestionIDs)
	}
	activeSessions.Unlock()

	// Navigate
	if qIdx < totalQuestions {
		http.Redirect(w, r, fmt.Sprintf("/exam/%d/question/%d", sessionID, qIdx+1), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/exam/%d/result", sessionID), http.StatusSeeOther)
	}
}

func ExamResult(w http.ResponseWriter, r *http.Request) {
	sessionID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	activeSessions.RLock()
	session, exists := activeSessions.m[sessionID]
	activeSessions.RUnlock()

	if !exists {
		http.Error(w, "Session not found", 404)
		return
	}

	// Calculate Score and Gather Review Data
	correctCount := 0
	type ReviewItem struct {
		Number        int
		Question      string
		UserAnswer    string
		CorrectAnswer string
		Explanation   template.HTML
		IsCorrect     bool
	}
	var reviews []ReviewItem

	for i, qID := range session.QuestionIDs {
		var qText, explanation string
		err := database.DB.QueryRow("SELECT text, explanation FROM questions WHERE id = ?", qID).Scan(&qText, &explanation)
		if err != nil {
			continue
		}
		qText = cleanQuestionText(qText)

		// Get Correct Options
		var correctOptTexts []string
		var correctOptIDs []int
		rows, _ := database.DB.Query("SELECT id, text FROM options WHERE question_id = ? AND is_correct = 1", qID)
		for rows.Next() {
			var id int
			var txt string
			rows.Scan(&id, &txt)
			correctOptIDs = append(correctOptIDs, id)
			correctOptTexts = append(correctOptTexts, txt)
		}
		rows.Close()

		userOptIDs := session.Answers[qID]
		var userOptTexts []string
		if len(userOptIDs) > 0 {
			// Build query for user answers (placeholder generation)
			// SQLite doesn't support array parameters smoothly, loop or construct query
			// Simple loop for now
			for _, id := range userOptIDs {
				var txt string
				database.DB.QueryRow("SELECT text FROM options WHERE id = ?", id).Scan(&txt)
				userOptTexts = append(userOptTexts, txt)
			}
		}

		// Check correctness
		// Sets match?
		isCorrect := false
		if len(userOptIDs) == len(correctOptIDs) {
			matchCount := 0
			for _, uID := range userOptIDs {
				for _, cID := range correctOptIDs {
					if uID == cID {
						matchCount++
						break
					}
				}
			}
			if matchCount == len(correctOptIDs) {
				isCorrect = true
			}
		}

		if isCorrect {
			correctCount++
		}

		// Only show review for wrong answers? User said "review of each wrong answer".
		// But usually users want to see all or just wrong. Let's show wrong ones primarily or mark them.
		// Detailed requirement: "review of each wrong answer".
		if !isCorrect {
			// Join texts for display
			md := []byte(explanation)
			htmlBytes := markdown.ToHTML(md, nil, nil)

			reviews = append(reviews, ReviewItem{
				Number:        i + 1,
				Question:      qText,
				UserAnswer:    fmt.Sprintf("%v", userOptTexts), // Simple formatting
				CorrectAnswer: fmt.Sprintf("%v", correctOptTexts),
				Explanation:   template.HTML(htmlBytes),
				IsCorrect:     false,
			})
		}
	}

	totalQuestions := len(session.QuestionIDs)
	if totalQuestions == 0 {
		totalQuestions = 1 // Prevent divide by zero
	}
	score := (correctCount * 100) / totalQuestions

	// Update DB
	database.DB.Exec("UPDATE exam_sessions SET score = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?", score, sessionID)

	data := struct {
		Score   int
		Reviews []ReviewItem
	}{
		Score:   score,
		Reviews: reviews,
	}

	renderTemplate(w, "result.html", data)
}
