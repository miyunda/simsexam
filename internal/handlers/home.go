package handlers

import (
	"net/http"
	"simsexam/internal/database"
	"simsexam/internal/models"
)

func Home(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, description FROM subjects")
	if err != nil {
		http.Error(w, "Failed to fetch subjects", 500)
		return
	}
	defer rows.Close()

	var subjects []models.Subject
	for rows.Next() {
		var s models.Subject
		if err := rows.Scan(&s.ID, &s.Name, &s.Description); err != nil {
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
