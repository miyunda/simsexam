package importer

import "testing"

func TestValidateDocumentValid(t *testing.T) {
	doc := Document{
		HeaderSubjectSlug: "se-demo",
		Manifest: Manifest{
			Slug:            "se-demo",
			Title:           "SE Demo Subject",
			DurationMinutes: 20,
			QuestionCount:   1,
			AccessLevel:     "free",
			Status:          "published",
		},
		Questions: []Question{
			{
				Key:  "demo-001",
				Type: "single",
				Stem: "Question text",
				Options: []Option{
					{Text: "A", IsCorrect: true},
					{Text: "B", IsCorrect: false},
					{Text: "C", IsCorrect: false},
					{Text: "D", IsCorrect: false},
				},
				Explanation: "Explanation text",
			},
		},
	}

	report := ValidateDocument(doc)
	if !report.Valid() {
		t.Fatalf("expected report to be valid, got errors: %+v", report.Errors)
	}
}

func TestValidateDocumentRejectsDuplicateQuestionKeys(t *testing.T) {
	doc := Document{
		HeaderSubjectSlug: "se-demo",
		Manifest: Manifest{
			Slug:            "se-demo",
			Title:           "SE Demo Subject",
			DurationMinutes: 20,
			QuestionCount:   2,
			AccessLevel:     "free",
			Status:          "published",
		},
		Questions: []Question{
			{
				Key:  "demo-001",
				Type: "single",
				Stem: "Question 1",
				Options: []Option{
					{Text: "A", IsCorrect: true},
					{Text: "B", IsCorrect: false},
				},
				Line: 10,
			},
			{
				Key:  "demo-001",
				Type: "single",
				Stem: "Question 2",
				Options: []Option{
					{Text: "A", IsCorrect: true},
					{Text: "B", IsCorrect: false},
				},
				Line: 20,
			},
		},
	}

	report := ValidateDocument(doc)
	if report.Valid() {
		t.Fatal("expected duplicate key validation error")
	}
}
