package importer

import "testing"

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
