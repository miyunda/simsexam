package handlers

import (
	"net/http"
	"simsexam/internal/database"
	"simsexam/internal/models"
)

func Home(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
		SELECT id, slug, title, description, duration_minutes, question_count, access_level
		FROM subjects
		WHERE status = 'published' AND current_question_set_id IS NOT NULL
		ORDER BY title
	`)
	if err != nil {
		http.Error(w, "Failed to fetch subjects", 500)
		return
	}
	defer rows.Close()

	var subjects []models.Subject
	for rows.Next() {
		var s models.Subject
		if err := rows.Scan(&s.ID, &s.Slug, &s.Title, &s.Description, &s.DurationMinutes, &s.QuestionCount, &s.AccessLevel); err != nil {
			continue
		}
		subjects = append(subjects, s)
	}

	data := struct {
		Subjects []models.Subject
	}{
		Subjects: subjects,
	}

	renderTemplate(w, "home.html", data)
}
