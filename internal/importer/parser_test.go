package importer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseStringParsesValidDocument(t *testing.T) {
	content := `# Subject: se-demo

## Meta
- slug: se-demo
- title: SE Demo Subject
- description: Demo import file
- duration_minutes: 20
- question_count: 2
- access_level: free
- status: published
- version: 2026-04-23

---

## Question
key: demo-001
type: single

What color is the sky on a clear day?

- [x] Blue
- [ ] Green
- [ ] Red
- [ ] Yellow

### Explanation
Under ordinary daytime conditions, the sky usually appears blue.

---

## Question
key: demo-002
type: multiple

Which of the following are planets? Choose two.

- [x] Mars
- [x] Jupiter
- [ ] Moon
- [ ] Sun

### Explanation
Mars and Jupiter are planets.
`

	doc, err := ParseString(content)
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}

	if doc.Manifest.Slug != "se-demo" {
		t.Fatalf("expected slug se-demo, got %q", doc.Manifest.Slug)
	}
	if len(doc.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(doc.Questions))
	}
	if got := doc.Questions[0].Options[0].Text; got != "Blue" {
		t.Fatalf("expected first option Blue, got %q", got)
	}
}

func TestParseStringRejectsUnexpectedContentAfterOptions(t *testing.T) {
	content := `# Subject: se-demo

## Meta
- slug: se-demo
- title: SE Demo Subject
- duration_minutes: 20
- question_count: 1
- access_level: free
- status: published

---

## Question
key: demo-001
type: single

Question text.

- [x] A
- [ ] B

This line should not appear after options.
`

	_, err := ParseString(content)
	if err == nil {
		t.Fatal("expected ParseString to fail")
	}
}

func codeQuestionDocument(body string) string {
	return "# Subject: code-demo\n\n## Meta\n- slug: code-demo\n- title: Code Demo\n- duration_minutes: 20\n- question_count: 1\n- access_level: free\n- status: published\n\n---\n\n## Question\nkey: code-001\ntype: single\n\n" + body
}

func TestParseFencedCodePreservesQuestionSyntax(t *testing.T) {
	for _, fence := range []string{"```", "~~~", "````"} {
		t.Run(fence, func(t *testing.T) {
			code := fence + "text\n---\n## Question\n### Explanation\n- [x] literal checklist\n  indented code\n```nested\n" + fence + "\n"
			// An inner shorter fence cannot close a longer fence.
			if fence == "```" {
				code = strings.ReplaceAll(code, "```nested\n", "~~~\n")
			}
			body := "Read this code:\n\n" + code + "\n- [x] Yes\n- [ ] No\n\n### Explanation\n\n" + code
			doc, err := ParseString(codeQuestionDocument(body))
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Questions) != 1 || len(doc.Questions[0].Options) != 2 {
				t.Fatalf("unexpected structure: %+v", doc)
			}
			q := doc.Questions[0]
			if q.Stem != strings.TrimSpace("Read this code:\n\n"+code) || q.Explanation != strings.TrimSpace(code) {
				t.Fatalf("code was altered: %+v", q)
			}
		})
	}
}

func TestParseMultilineCodeOptions(t *testing.T) {
	for _, firstLine := range []string{"Policy:\n  ```json", "```json"} {
		t.Run(firstLine, func(t *testing.T) {
			body := "Choose the policy.\n\n- [x] " + firstLine + "\n  {\n    \"Action\": \"ec2:*\"\n  }\n  ---\n  - [ ] literal\n  ### Explanation\n  ```\n- [ ] Another policy\n\n### Explanation\nCorrect policy."
			doc, err := ParseString(codeQuestionDocument(body))
			if err != nil {
				t.Fatal(err)
			}
			q := doc.Questions[0]
			if len(q.Options) != 2 || !q.Options[0].IsCorrect || !strings.Contains(q.Options[0].Text, "\n  \"Action\": \"ec2:*\"\n") || !strings.Contains(q.Options[0].Text, "\n---\n- [ ] literal\n### Explanation\n```") {
				t.Fatalf("unexpected options: %+v", q.Options)
			}
		})
	}
}

func TestParseRejectsUnclosedCodeFence(t *testing.T) {
	for _, body := range []string{
		"```json\n{}\n- [x] Yes\n- [ ] No",
		"Question\n- [x] Yes\n- [ ] No\n### Explanation\n~~~\nunclosed",
		"Question\n- [x] ```json\n  {}\n- [ ] No",
	} {
		_, err := ParseString(codeQuestionDocument(body))
		if err == nil {
			t.Fatal("expected unterminated or unindented code error")
		}
	}
}

func TestParseFencedCodeFollowedByAnotherQuestion(t *testing.T) {
	body := "```text\n---\n```\n- [x] Yes\n- [ ] No\n\n---\n\n## Question\nkey: code-002\ntype: single\n\nSecond question.\n- [x] Yes\n- [ ] No"
	doc, err := ParseString(codeQuestionDocument(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Questions) != 2 {
		t.Fatalf("expected two questions, got %d", len(doc.Questions))
	}
}

func TestParseFenceIndentationMatchesRendering(t *testing.T) {
	for _, indent := range []string{"", " ", "  ", "   "} {
		t.Run(fmt.Sprintf("stem-%d-spaces", len(indent)), func(t *testing.T) {
			body := indent + "```text\n---\n" + indent + "```\n- [x] Yes\n- [ ] No"
			doc, err := ParseString(codeQuestionDocument(body))
			if err != nil {
				t.Fatal(err)
			}
			want := indent + "```text\n---\n" + indent + "```"
			if doc.Questions[0].Stem != want {
				t.Fatalf("indentation changed: %q", doc.Questions[0].Stem)
			}
		})
		t.Run(fmt.Sprintf("option-%d-content-spaces", len(indent)), func(t *testing.T) {
			body := "Question\n- [x] Policy:\n  " + indent + "```text\n  ---\n  " + indent + "```\n- [ ] No"
			doc, err := ParseString(codeQuestionDocument(body))
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Questions[0].Options) != 2 {
				t.Fatal("code split into options")
			}
		})
	}
	for _, prefix := range []string{"    ", "\t"} {
		t.Run(fmt.Sprintf("literal-prefix-%q", prefix), func(t *testing.T) {
			body := prefix + "```json\n" + prefix + "{}\n" + prefix + "```\n- [x] Yes\n- [ ] No"
			doc, err := ParseString(codeQuestionDocument(body))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(doc.Questions[0].Stem, prefix+"```json") {
				t.Fatalf("lost code indentation: %q", doc.Questions[0].Stem)
			}
		})
	}
	// A fence-looking indented code example must not swallow the next question.
	body := "Read this:\n\n    ```json\n    {}\n- [x] Yes\n- [ ] No\n\n---\n\n## Question\nkey: code-002\ntype: single\n\nSecond.\n- [x] Yes\n- [ ] No"
	doc, err := ParseString(codeQuestionDocument(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Questions) != 2 {
		t.Fatalf("got %d questions", len(doc.Questions))
	}
}

func TestParseUnclosedFenceReportsOpeningLine(t *testing.T) {
	for _, body := range []string{
		"```text\ncode\n    ```\n- [x] Yes\n- [ ] No",
		"Question\n- [x] ```text\n  code\n- [ ] No",
		"Question\n- [x] Policy:\n  ```text\n  code\n- [ ] No",
		"Question\n- [x] ```text\n  code\n      ```\n- [ ] No",
	} {
		content := codeQuestionDocument(body)
		_, err := ParseString(content)
		var parseErr *ParseError
		if !errors.As(err, &parseErr) || parseErr.Message != "unclosed fenced code block" {
			t.Fatalf("expected unclosed fence error, got %v", err)
		}
		openingLine := strings.Count(content[:strings.Index(content, "```text")], "\n") + 1
		if parseErr.Line != openingLine {
			t.Fatalf("expected opening line %d, got %d", openingLine, parseErr.Line)
		}
	}
}

func TestParseClosedOptionFenceReportsIndentationError(t *testing.T) {
	content := codeQuestionDocument("Question\n- [x] ```text\nnot indented\n  ```\n- [ ] No")
	_, err := ParseString(content)
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Message != "option code blocks must be indented by two spaces" {
		t.Fatalf("expected indentation error, got %v", err)
	}
	wantLine := strings.Count(content[:strings.Index(content, "not indented")], "\n") + 1
	if parseErr.Line != wantLine {
		t.Fatalf("expected line %d, got %d", wantLine, parseErr.Line)
	}
}
