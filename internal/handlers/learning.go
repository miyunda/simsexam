package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"simsexam/internal/config"
	"simsexam/internal/database"

	"github.com/go-chi/chi/v5"
	"github.com/gomarkdown/markdown"
)

type mistakeNotebookRow struct {
	SubjectID       int
	SubjectTitle    string
	QuestionKey     string
	QuestionSummary string
	WrongCount      int
	CorrectCount    int
	LastWrongAt     string
	MasteryStatus   string
}

type mistakeReviewData struct {
	SubjectTitle   string
	QuestionKey    string
	Question       string
	QuestionHTML   template.HTML
	Explanation    template.HTML
	Status         string
	WrongCount     int
	CorrectCount   int
	LastAnsweredAt string
	LastWrongAt    string
	MasteryStatus  string
	Options        []mistakeReviewOption
}

type mistakeReviewOption struct {
	Text      string
	IsCorrect bool
}

func MistakeNotebook(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r, cfg)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		rows, err := database.DB.Query(`
			SELECT
				s.id,
				s.title,
				uqs.question_key,
				COALESCE(q.stem_markdown, uqs.question_key) AS question_summary,
				uqs.wrong_count,
				uqs.correct_count,
				COALESCE(uqs.last_wrong_at, ''),
				uqs.mastery_status
			FROM user_question_stats uqs
			JOIN subjects s ON s.id = uqs.subject_id
			LEFT JOIN questions q ON q.id = (
				SELECT q2.id
				FROM questions q2
				WHERE q2.subject_id = uqs.subject_id
					AND q2.external_key = uqs.question_key
				ORDER BY
					CASE WHEN q2.status = 'active' THEN 0 ELSE 1 END,
					q2.updated_at DESC,
					q2.id DESC
				LIMIT 1
			)
			WHERE uqs.user_id = ? AND uqs.wrong_count > 0
			ORDER BY uqs.last_wrong_at DESC, uqs.id DESC
		`, userID)
		if err != nil {
			http.Error(w, "Failed to load mistakes", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var mistakes []mistakeNotebookRow
		for rows.Next() {
			var row mistakeNotebookRow
			if err := rows.Scan(
				&row.SubjectID,
				&row.SubjectTitle,
				&row.QuestionKey,
				&row.QuestionSummary,
				&row.WrongCount,
				&row.CorrectCount,
				&row.LastWrongAt,
				&row.MasteryStatus,
			); err != nil {
				http.Error(w, "Failed to load mistakes", http.StatusInternalServerError)
				return
			}
			row.QuestionSummary = truncateText(cleanQuestionText(row.QuestionSummary), 160)
			mistakes = append(mistakes, row)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Failed to load mistakes", http.StatusInternalServerError)
			return
		}

		renderTemplate(w, "mistakes.html", struct {
			Mistakes []mistakeNotebookRow
		}{
			Mistakes: mistakes,
		})
	}
}

func MistakeReview(cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r, cfg)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		subjectID, _ := strconv.Atoi(chi.URLParam(r, "subjectID"))
		questionKey := chi.URLParam(r, "questionKey")
		if subjectID == 0 || questionKey == "" {
			http.Error(w, "Mistake not found", http.StatusNotFound)
			return
		}

		var data mistakeReviewData
		var questionID int
		var questionMarkdown string
		var explanationMarkdown string
		err := database.DB.QueryRow(`
			SELECT
				s.title,
				uqs.question_key,
				uqs.wrong_count,
				uqs.correct_count,
				COALESCE(uqs.last_answered_at, ''),
				COALESCE(uqs.last_wrong_at, ''),
				uqs.mastery_status,
				q.id,
				q.stem_markdown,
				COALESCE(q.explanation_markdown, ''),
				q.status
			FROM user_question_stats uqs
			JOIN subjects s ON s.id = uqs.subject_id
			JOIN questions q ON q.id = (
				SELECT q2.id
				FROM questions q2
				WHERE q2.subject_id = uqs.subject_id
					AND q2.external_key = uqs.question_key
				ORDER BY
					CASE WHEN q2.status = 'active' THEN 0 ELSE 1 END,
					q2.updated_at DESC,
					q2.id DESC
				LIMIT 1
			)
			WHERE uqs.user_id = ?
				AND uqs.subject_id = ?
				AND uqs.question_key = ?
				AND uqs.wrong_count > 0
		`, userID, subjectID, questionKey).Scan(
			&data.SubjectTitle,
			&data.QuestionKey,
			&data.WrongCount,
			&data.CorrectCount,
			&data.LastAnsweredAt,
			&data.LastWrongAt,
			&data.MasteryStatus,
			&questionID,
			&questionMarkdown,
			&explanationMarkdown,
			&data.Status,
		)
		if err == sql.ErrNoRows {
			http.Error(w, "Mistake not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to load mistake", http.StatusInternalServerError)
			return
		}

		data.Question = cleanQuestionText(questionMarkdown)
		data.QuestionHTML = template.HTML(markdown.ToHTML([]byte(data.Question), nil, nil))
		data.Explanation = template.HTML(markdown.ToHTML([]byte(explanationMarkdown), nil, nil))

		optionRows, err := database.DB.Query(`
			SELECT content_markdown, is_correct
			FROM question_options
			WHERE question_id = ?
			ORDER BY sort_order
		`, questionID)
		if err != nil {
			http.Error(w, "Failed to load mistake", http.StatusInternalServerError)
			return
		}
		defer optionRows.Close()
		for optionRows.Next() {
			var option mistakeReviewOption
			var isCorrect int
			if err := optionRows.Scan(&option.Text, &isCorrect); err != nil {
				http.Error(w, "Failed to load mistake", http.StatusInternalServerError)
				return
			}
			option.IsCorrect = isCorrect == 1
			data.Options = append(data.Options, option)
		}

		renderTemplate(w, "mistake_review.html", data)
	}
}

func rebuildUserQuestionStatsTx(tx *sql.Tx, userID int) error {
	if _, err := tx.Exec(`DELETE FROM user_question_stats WHERE user_id = ?`, userID); err != nil {
		return err
	}
	_, err := tx.Exec(`
		INSERT INTO user_question_stats (
			user_id,
			subject_id,
			question_key,
			wrong_count,
			correct_count,
			last_answered_at,
			last_wrong_at,
			mastery_status
		)
		SELECT
			e.user_id,
			q.subject_id,
			q.external_key,
			SUM(CASE WHEN ea.is_correct = 0 THEN 1 ELSE 0 END) AS wrong_count,
			SUM(CASE WHEN ea.is_correct = 1 THEN 1 ELSE 0 END) AS correct_count,
			MAX(ea.answered_at) AS last_answered_at,
			MAX(CASE WHEN ea.is_correct = 0 THEN ea.answered_at ELSE NULL END) AS last_wrong_at,
			CASE
				WHEN SUM(CASE WHEN ea.is_correct = 0 THEN 1 ELSE 0 END) > 0
					AND SUM(CASE WHEN ea.is_correct = 1 THEN 1 ELSE 0 END) > 0 THEN 'mastered'
				WHEN SUM(CASE WHEN ea.is_correct = 0 THEN 1 ELSE 0 END) > 0 THEN 'weak'
				ELSE 'mastered'
			END AS mastery_status
		FROM exams e
		JOIN exam_answers ea ON ea.exam_id = e.id
		JOIN questions q ON q.id = ea.question_id
		WHERE e.user_id = ?
		GROUP BY e.user_id, q.subject_id, q.external_key
	`, userID)
	return err
}

func truncateText(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return fmt.Sprintf("%s...", string(runes[:maxRunes]))
}
