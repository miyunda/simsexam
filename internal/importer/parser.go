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
	var fence codeFence
	seenOption, inExplanation := false, false

	for _, line := range lines {
		content := line.Text
		// Match the two-space option continuation handling in parseQuestionBlock.
		if seenOption && !inExplanation && strings.HasPrefix(content, "  ") {
			content = strings.TrimPrefix(content, "  ")
		} else if fence.marker == 0 {
			if strings.TrimSpace(content) == "### Explanation" {
				inExplanation = true
			} else if !inExplanation {
				if match := optionRegex.FindStringSubmatch(strings.TrimSpace(content)); match != nil {
					seenOption = true
					content = match[2]
				}
			}
		}
		if !fence.consume(content) && line.Text == "---" {
			seenOption, inExplanation = false, false
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
	var fence codeFence
	fenceLine := 0
	var optionIndentError *ParseError

	for ; i < len(lines); i++ {
		text := lines[i].Text
		trimmed := strings.TrimSpace(text)

		// Option continuation lines use two spaces of structural indentation.
		// Strip only that prefix so code indentation is preserved verbatim.
		optionContinuation := seenOption && !inExplanation && strings.HasPrefix(text, "  ")
		content := text
		if optionContinuation {
			content = strings.TrimPrefix(text, "  ")
		}
		wasInFence := fence.marker != 0
		if fence.consume(content) {
			if !wasInFence {
				fenceLine = lines[i].Number
			}
			switch {
			case inExplanation:
				explanationLines = append(explanationLines, content)
			case seenOption:
				if !optionContinuation && trimmed != "" && optionIndentError == nil {
					// Defer this error until closure is known, so a missing closing fence
					// is reported at its opener rather than as misleading indentation.
					optionIndentError = &ParseError{Line: lines[i].Number, Message: "option code blocks must be indented by two spaces"}
				}
				q.Options[len(q.Options)-1].Text += "\n" + content
			default:
				stemLines = append(stemLines, content)
			}
			continue
		}

		if optionContinuation {
			q.Options[len(q.Options)-1].Text += "\n" + content
			continue
		}

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
			if fence.consume(optionMatch[2]) {
				fenceLine = lines[i].Number
			}
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

	if fence.marker != 0 {
		return Question{}, &ParseError{Line: fenceLine, Message: "unclosed fenced code block"}
	}

	if optionIndentError != nil {
		return Question{}, optionIndentError
	}
	q.Stem = trimMarkdownLines(stemLines)
	q.Explanation = trimMarkdownLines(explanationLines)

	return q, nil
}

// codeFence protects question delimiters and option syntax inside fenced code.
// A closing fence must match the opening marker and length, as required by
// the Markdown renderer used by the learner pages.
type codeFence struct {
	marker byte
	length int
}

func (f *codeFence) consume(line string) bool {
	// Four spaces introduce indented code, not a fence. Within a fence they
	// remain literal content and cannot close it. Tabs are not fence prefixes.
	text := strings.TrimLeft(line, " ")
	if len(line)-len(text) > 3 {
		return f.marker != 0
	}
	if f.marker != 0 {
		count := 0
		for count < len(text) && text[count] == f.marker {
			count++
		}
		if count == f.length && strings.Trim(text[count:], " ") == "" {
			f.marker, f.length = 0, 0
		}
		return true
	}
	if len(text) < 3 || (text[0] != '`' && text[0] != '~') {
		return false
	}
	count := 0
	for count < len(text) && text[count] == text[0] {
		count++
	}
	if count < 3 || (text[0] == '`' && strings.Contains(text[count:], "`")) {
		return false
	}
	f.marker, f.length = text[0], count
	return true
}

// Trim blank boundary lines without stripping Markdown-significant indentation.
func trimMarkdownLines(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
