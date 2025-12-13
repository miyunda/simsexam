package models

import (
	"time"
)

type Subject struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Question struct {
	ID          int      `json:"id"`
	SubjectID   int      `json:"subject_id"`
	Text        string   `json:"text"`
	Type        string   `json:"type"` // "single", "multiple"
	Explanation string   `json:"explanation"`
	Options     []Option `json:"options"`
}

type Option struct {
	ID         int    `json:"id"`
	QuestionID int    `json:"question_id"`
	Text       string `json:"text"`
	IsCorrect  bool   `json:"is_correct"`
}

type ExamSession struct {
	ID          int       `json:"id"`
	UserID      *int      `json:"user_id"` // Nullable for anonymous
	SubjectID   int       `json:"subject_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Score       int       `json:"score"` // 0-100
}
