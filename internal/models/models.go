package models

import (
	"time"
)

type Subject struct {
	ID              int    `json:"id"`
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	DurationMinutes int    `json:"duration_minutes"`
	QuestionCount   int    `json:"question_count"`
	AccessLevel     string `json:"access_level"`
}

type Question struct {
	ID          int      `json:"id"`
	SubjectID   int      `json:"subject_id"`
	Text        string   `json:"text"`
	Type        string   `json:"type"`
	Explanation string   `json:"explanation"`
	Options     []Option `json:"options"`
}

type Option struct {
	ID         int    `json:"id"`
	QuestionID int    `json:"question_id"`
	Text       string `json:"text"`
	IsCorrect  bool   `json:"is_correct"`
}

type Exam struct {
	ID          int       `json:"id"`
	UserID      *int      `json:"user_id"`
	SubjectID   int       `json:"subject_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Score       int       `json:"score"`
}
