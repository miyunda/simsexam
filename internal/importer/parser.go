package importer

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	subjectHeaderRegex = regexp.MustCompile(`^# Subject:\s+(.+)$`)
	metaItemRegex      = regexp.MustCompile(`^- ([a-z_]+):\s*(.*)$`)
	questionFieldRegex = regexp.MustCompile(`^([a-z_]+):\s*(.+)$`)
	optionRegex        = regexp.MustCompile(`^- \[( |x|X)\] (.+)$`)
)

type lineEntry struct {
	Number int
	Text   string
}

func ParseFile(path string) (Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	return ParseString(string(content))
}

func ParseString(content string) (Document, error) {
	lines := scanLines(content)
	if len(lines) == 0 {
		return Document{}, &ParseError{Message: "empty document"}
	}

	headerIndex, err := firstNonEmptyLine(lines)
	if err != nil {
		return Document{}, err
	}

	headerMatch := subjectHeaderRegex.FindStringSubmatch(lines[headerIndex].Text)
	if headerMatch == nil {
		return Document{}, &ParseError{
			Line:    lines[headerIndex].Number,
			Message: "expected '# Subject: <subject-slug>' header",
		}
	}

	doc := Document{
		HeaderSubjectSlug: strings.TrimSpace(headerMatch[1]),
	}

	metaHeadingIndex := nextNonEmptyLine(lines, headerIndex+1)
	if metaHeadingIndex == -1 || lines[metaHeadingIndex].Text != "## Meta" {
		line := 0
		if metaHeadingIndex != -1 {
			line = lines[metaHeadingIndex].Number
		}
		return Document{}, &ParseError{
			Line:    line,
			Message: "expected '## Meta' section after subject header",
		}
	}

	questionStart := -1
	for i := metaHeadingIndex + 1; i < len(lines); i++ {
		if lines[i].Text == "---" {
			questionStart = i + 1
			break
		}
		if err := applyManifestLine(&doc.Manifest, lines[i]); err != nil {
			return Document{}, err
		}
	}
	if questionStart == -1 {
		return Document{}, &ParseError{Message: "expected at least one question block separated by '---'"}
	}

	blocks := splitQuestionBlocks(lines[questionStart:])
	if len(blocks) == 0 {
		return Document{}, &ParseError{Message: "expected at least one question block"}
	}

	for _, block := range blocks {
		q, err := parseQuestionBlock(block)
		if err != nil {
			return Document{}, err
		}
		doc.Questions = append(doc.Questions, q)
	}

	return doc, nil
}

func scanLines(content string) []lineEntry {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []lineEntry
	lineNo := 1
	for scanner.Scan() {
		lines = append(lines, lineEntry{
			Number: lineNo,
			Text:   strings.TrimRight(scanner.Text(), "\r"),
		})
		lineNo++
	}
	return lines
}

func firstNonEmptyLine(lines []lineEntry) (int, error) {
	for i, line := range lines {
		if strings.TrimSpace(line.Text) != "" {
			return i, nil
		}
	}
	return -1, &ParseError{Message: "empty document"}
}

func nextNonEmptyLine(lines []lineEntry, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].Text) != "" {
			return i
		}
	}
	return -1
}

func applyManifestLine(manifest *Manifest, line lineEntry) error {
	if strings.TrimSpace(line.Text) == "" {
		return nil
	}

	match := metaItemRegex.FindStringSubmatch(line.Text)
	if match == nil {
		return &ParseError{
			Line:    line.Number,
			Message: "expected meta item in the form '- key: value'",
		}
	}

	key := match[1]
	value := strings.TrimSpace(match[2])

	switch key {
	case "slug":
		manifest.Slug = value
	case "title":
		manifest.Title = value
	case "description":
		manifest.Description = value
	case "duration_minutes":
		v, err := strconv.Atoi(value)
		if err != nil {
			return &ParseError{Line: line.Number, Message: "duration_minutes must be an integer"}
		}
		manifest.DurationMinutes = v
	case "question_count":
		v, err := strconv.Atoi(value)
		if err != nil {
			return &ParseError{Line: line.Number, Message: "question_count must be an integer"}
		}
		manifest.QuestionCount = v
	case "access_level":
		manifest.AccessLevel = value
	case "status":
		manifest.Status = value
	case "version":
		manifest.Version = value
	default:
		return &ParseError{
			Line:    line.Number,
			Message: fmt.Sprintf("unknown manifest field %q", key),
		}
	}

	return nil
}

func splitQuestionBlocks(lines []lineEntry) [][]lineEntry {
	var blocks [][]lineEntry
	var current []lineEntry

	for _, line := range lines {
		if line.Text == "---" {
			if len(current) > 0 {
				blocks = append(blocks, current)
				current = nil
			}
			continue
		}
		current = append(current, line)
	}

	if len(current) > 0 {
		blocks = append(blocks, current)
	}

	return blocks
}

func parseQuestionBlock(lines []lineEntry) (Question, error) {
	start, err := firstNonEmptyLine(lines)
	if err != nil {
		return Question{}, err
	}

	if lines[start].Text != "## Question" {
		return Question{}, &ParseError{
			Line:    lines[start].Number,
			Message: "expected '## Question' at start of question block",
		}
	}

	q := Question{Line: lines[start].Number}
	i := start + 1
	for i < len(lines) {
		text := strings.TrimSpace(lines[i].Text)
		if text == "" {
			i++
			break
		}
		match := questionFieldRegex.FindStringSubmatch(text)
		if match == nil {
			return Question{}, &ParseError{
				Line:    lines[i].Number,
				Message: "expected question field in the form 'key: value'",
			}
		}
		switch match[1] {
		case "key":
			q.Key = strings.TrimSpace(match[2])
		case "type":
			q.Type = strings.TrimSpace(match[2])
		default:
			return Question{}, &ParseError{
				Line:    lines[i].Number,
				Message: fmt.Sprintf("unknown question field %q", match[1]),
			}
		}
		i++
	}

	var stemLines []string
	var explanationLines []string
	inExplanation := false
	seenOption := false

	for ; i < len(lines); i++ {
		text := lines[i].Text
		trimmed := strings.TrimSpace(text)

		if trimmed == "### Explanation" {
			inExplanation = true
			continue
		}

		if optionMatch := optionRegex.FindStringSubmatch(trimmed); optionMatch != nil {
			if inExplanation {
				return Question{}, &ParseError{
					Line:    lines[i].Number,
					Message: "options cannot appear after explanation section",
				}
			}
			seenOption = true
			q.Options = append(q.Options, Option{
				Text:      strings.TrimSpace(optionMatch[2]),
				IsCorrect: strings.EqualFold(optionMatch[1], "x"),
				Line:      lines[i].Number,
			})
			continue
		}

		if inExplanation {
			explanationLines = append(explanationLines, text)
			continue
		}

		if seenOption {
			if trimmed == "" {
				continue
			}
			return Question{}, &ParseError{
				Line:    lines[i].Number,
				Message: "unexpected content after options; only '### Explanation' may follow options",
			}
		}

		stemLines = append(stemLines, text)
	}

	q.Stem = strings.TrimSpace(strings.Join(stemLines, "\n"))
	q.Explanation = strings.TrimSpace(strings.Join(explanationLines, "\n"))

	return q, nil
}
