package importer

import "regexp"

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type ValidationMessage struct {
	Line    int
	Field   string
	Message string
}

type ValidationReport struct {
	Errors   []ValidationMessage
	Warnings []ValidationMessage
}

func (r ValidationReport) Valid() bool {
	return len(r.Errors) == 0
}

func ValidateDocument(doc Document) ValidationReport {
	var report ValidationReport

	if doc.HeaderSubjectSlug == "" {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "header",
			Message: "subject header slug is required",
		})
	}
	if !slugRegex.MatchString(doc.HeaderSubjectSlug) {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "header.slug",
			Message: "subject header slug must contain only lowercase letters, digits, and hyphens",
		})
	}

	validateManifest(doc, &report)

	if len(doc.Questions) == 0 {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "questions",
			Message: "at least one question is required",
		})
	}

	seenKeys := make(map[string]int)
	for _, q := range doc.Questions {
		validateQuestion(q, &report)

		if q.Key != "" {
			if firstLine, exists := seenKeys[q.Key]; exists {
				report.Errors = append(report.Errors, ValidationMessage{
					Line:    q.Line,
					Field:   "question.key",
					Message: "duplicate question key; first declared at line " + itoa(firstLine),
				})
			} else {
				seenKeys[q.Key] = q.Line
			}
		}
	}

	if doc.Manifest.QuestionCount > 0 && doc.Manifest.QuestionCount > len(doc.Questions) {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.question_count",
			Message: "question_count cannot exceed the number of questions in the file",
		})
	}

	return report
}

func validateManifest(doc Document, report *ValidationReport) {
	m := doc.Manifest

	if m.Slug == "" {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.slug",
			Message: "slug is required",
		})
	}
	if m.Slug != "" && !slugRegex.MatchString(m.Slug) {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.slug",
			Message: "slug must contain only lowercase letters, digits, and hyphens",
		})
	}
	if doc.HeaderSubjectSlug != "" && m.Slug != "" && doc.HeaderSubjectSlug != m.Slug {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.slug",
			Message: "manifest slug must match subject header slug",
		})
	}
	if m.Title == "" {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.title",
			Message: "title is required",
		})
	}
	if len(m.Title) > 120 {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.title",
			Message: "title must be 120 characters or fewer",
		})
	}
	if len(m.Description) > 1000 {
		report.Warnings = append(report.Warnings, ValidationMessage{
			Field:   "manifest.description",
			Message: "description should be 1000 characters or fewer",
		})
	}
	if m.DurationMinutes <= 0 {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.duration_minutes",
			Message: "duration_minutes must be a positive integer",
		})
	}
	if m.DurationMinutes > 600 {
		report.Warnings = append(report.Warnings, ValidationMessage{
			Field:   "manifest.duration_minutes",
			Message: "duration_minutes should usually be 600 or less",
		})
	}
	if m.QuestionCount <= 0 {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.question_count",
			Message: "question_count must be a positive integer",
		})
	}
	if !oneOf(m.AccessLevel, "free", "paid", "private") {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.access_level",
			Message: "access_level must be one of: free, paid, private",
		})
	}
	if !oneOf(m.Status, "draft", "published", "archived") {
		report.Errors = append(report.Errors, ValidationMessage{
			Field:   "manifest.status",
			Message: "status must be one of: draft, published, archived",
		})
	}
}

func validateQuestion(q Question, report *ValidationReport) {
	if q.Key == "" {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.key",
			Message: "key is required",
		})
	}
	if q.Type == "" {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.type",
			Message: "type is required",
		})
	}
	if !oneOf(q.Type, "single", "multiple") {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.type",
			Message: "type must be one of: single, multiple",
		})
	}
	if q.Stem == "" {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.stem",
			Message: "question stem is required",
		})
	}
	if len(q.Options) < 2 {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.options",
			Message: "at least two options are required",
		})
	}
	if len(q.Options) != 4 {
		report.Warnings = append(report.Warnings, ValidationMessage{
			Line:    q.Line,
			Field:   "question.options",
			Message: "four options are recommended",
		})
	}
	if q.Explanation == "" {
		report.Warnings = append(report.Warnings, ValidationMessage{
			Line:    q.Line,
			Field:   "question.explanation",
			Message: "explanation is recommended",
		})
	}

	correct := 0
	for _, opt := range q.Options {
		if opt.Text == "" {
			report.Errors = append(report.Errors, ValidationMessage{
				Line:    opt.Line,
				Field:   "question.options",
				Message: "option text cannot be empty",
			})
		}
		if opt.IsCorrect {
			correct++
		}
	}

	if q.Type == "single" && correct != 1 {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.options",
			Message: "single-choice questions must have exactly one correct option",
		})
	}
	if q.Type == "multiple" && correct < 2 {
		report.Errors = append(report.Errors, ValidationMessage{
			Line:    q.Line,
			Field:   "question.options",
			Message: "multiple-choice questions must have at least two correct options",
		})
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
