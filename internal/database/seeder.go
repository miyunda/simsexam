package database

import (
	"log"
)

func SeedInitialData() {
	log.Println("Seeding database (idempotent check)...")

	// 1. General Knowledge (Common Sense)
	var s1 int
	DB.QueryRow("SELECT id FROM subjects WHERE name = ?", "General Knowledge").Scan(&s1)
	if s1 == 0 {
		res, err := DB.Exec("INSERT INTO subjects (name, description, question_limit) VALUES (?, ?, ?)", "General Knowledge", "Common sense questions for everyone.", 7)
		if err != nil {
			log.Fatal(err)
		}
		id, _ := res.LastInsertId()
		seedCommonSenseQuestions(id)
	} else {
		// Ensure limit is correct if it exists
		DB.Exec("UPDATE subjects SET question_limit = 7 WHERE name = ?", "General Knowledge")
	}

	// 2. CLF-02 (Empty for now, waiting for import)
	var s2 int
	DB.QueryRow("SELECT id FROM subjects WHERE name = ?", "AWS CLF-02").Scan(&s2)
	if s2 == 0 {
		_, err := DB.Exec("INSERT INTO subjects (name, description, question_limit) VALUES (?, ?, ?)", "AWS CLF-02", "AWS Certified Cloud Practitioner.", 50)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// Ensure limit is correct if it exists
		DB.Exec("UPDATE subjects SET question_limit = 50 WHERE name = ?", "AWS CLF-02")
	}

	log.Println("Seeding check completed.")
}

func seedCommonSenseQuestions(subjectID int64) {
	questions := []struct {
		Text           string
		Type           string
		Options        []string
		CorrectIndices []int // 0-based indices of correct options
	}{
		{
			Text:           "What is the color of the sky on a clear day?",
			Type:           "single",
			Options:        []string{"Blue", "Green", "Red", "Yellow"},
			CorrectIndices: []int{0},
		},
		{
			Text:           "Which of the following are primary colors? (Choose two)",
			Type:           "multiple",
			Options:        []string{"Red", "Blue", "Purple", "Orange"},
			CorrectIndices: []int{0, 1},
		},
		{
			Text:           "How many legs does a spider have?",
			Type:           "single",
			Options:        []string{"Eight", "Six", "Four", "Ten"},
			CorrectIndices: []int{0},
		},
		{
			Text:           "Select the gas giants in our solar system. (Choose two)",
			Type:           "multiple",
			Options:        []string{"Jupiter", "Saturn", "Mars", "Venus"},
			CorrectIndices: []int{0, 1},
		},
		{
			Text:           "What is H2O commonly known as?",
			Type:           "single",
			Options:        []string{"Water", "Salt", "Sugar", "Air"},
			CorrectIndices: []int{0},
		},
		{
			Text:           "Which countries are in North America? (Choose two)",
			Type:           "multiple",
			Options:        []string{"Canada", "USA", "France", "Brazil"},
			CorrectIndices: []int{0, 1},
		},
		{
			Text:           "Which country has the largest territory?",
			Type:           "single",
			Options:        []string{"Russia", "Canada", "China", "USA"},
			CorrectIndices: []int{0},
		},
	}

	for _, q := range questions {
		res, err := DB.Exec("INSERT INTO questions (subject_id, text, type, explanation) VALUES (?, ?, ?, ?)", subjectID, q.Text, q.Type, "Common knowledge.")
		if err != nil {
			log.Fatal(err)
		}
		qID, _ := res.LastInsertId()

		for i, optText := range q.Options {
			isCorrect := false
			for _, correctIdx := range q.CorrectIndices {
				if i == correctIdx {
					isCorrect = true
					break
				}
			}
			DB.Exec("INSERT INTO options (question_id, text, is_correct) VALUES (?, ?, ?)", qID, optText, isCorrect)
		}
	}
}
