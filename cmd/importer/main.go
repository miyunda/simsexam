package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"simsexam/internal/database"
	"strings"
)

func main() {
	subjectName := flag.String("subject", "", "Subject name")
	filePath := flag.String("file", "", "Path to markdown file")
	flag.Parse()

	if *subjectName == "" || *filePath == "" {
		log.Fatal("Usage: go run ./cmd/importer -subject <name> -file <path>")
	}

	// Init DB
	if err := database.InitDB("./simsexam.db"); err != nil {
		log.Fatal(err)
	}

	// Get Subject ID
	var subjectID int
	err := database.DB.QueryRow("SELECT id FROM subjects WHERE name = ?", *subjectName).Scan(&subjectID)
	if err != nil {
		log.Fatalf("Subject '%s' not found: %v", *subjectName, err)
	}

	file, err := os.Open(*filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var contentBuilder strings.Builder
	for scanner.Scan() {
		contentBuilder.WriteString(scanner.Text() + "\n")
	}

	content := contentBuilder.String()
	blocks := strings.Split(content, "---")

	count := 0
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		processBlock(subjectID, block)
		count++
	}

	fmt.Printf("Imported %d questions for subject '%s'.\n", count, *subjectName)
}

func processBlock(subjectID int, block string) {
	lines := strings.Split(strings.TrimSpace(block), "\n")
	if len(lines) == 0 {
		return
	}

	// 1. Identify Question Text (First paragraph usually)
	// Simple assumption: Everything up until first Option (A.) is the question text.
	// OR use regex to find start of options.

	optionRegex := regexp.MustCompile(`^(\*{0,2}[A-E]\*{0,2}\.?) `)
	explanationRegex := regexp.MustCompile(`^> \*\*Explanation:\*\*`)

	var qTextBuilder strings.Builder
	var options []string
	var explanationBuilder strings.Builder

	inOptions := false
	inExplanation := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if explanationRegex.MatchString(line) {
			inExplanation = true
			inOptions = false
			// Remove "> **Explanation:** " prefix
			line = strings.TrimPrefix(line, "> **Explanation:** ")
			line = strings.TrimPrefix(line, "**Explanation:** ") // Just in case
			explanationBuilder.WriteString(line + "\n")
			continue
		}

		if inExplanation {
			if strings.HasPrefix(line, ">") {
				line = strings.TrimPrefix(line, ">")
			}
			explanationBuilder.WriteString(strings.TrimSpace(line) + "\n")
			continue
		}

		if optionRegex.MatchString(line) {
			inOptions = true
			options = append(options, line)
			continue
		}

		if !inOptions && !inExplanation {
			qTextBuilder.WriteString(line + "\n")
		}
	}

	qText := strings.TrimSpace(qTextBuilder.String())
	explanation := strings.TrimSpace(explanationBuilder.String())

	if qText == "" || len(options) == 0 {
		log.Printf("Skipping invalid block (no text or options). qText len: %d, options len: %d\n", len(qText), len(options))
		return // Skip malformed
	}

	// Detect Type
	qType := "single"
	if strings.Contains(strings.ToLower(qText), "choose two") || strings.Contains(strings.ToLower(qText), "select two") {
		qType = "multiple"
	}

	// Insert Question
	res, err := database.DB.Exec("INSERT INTO questions (subject_id, text, type, explanation) VALUES (?, ?, ?, ?)", subjectID, qText, qType, explanation)
	if err != nil {
		log.Printf("Error inserting question: %v", err)
		return
	}
	qID, _ := res.LastInsertId()

	// Process Options
	for _, optLine := range options {
		isCorrect := false
		if strings.HasPrefix(optLine, "**") {
			isCorrect = true
			optLine = strings.TrimPrefix(optLine, "**")
			optLine = strings.TrimSuffix(optLine, "**")
		}

		// Clean up "A. " or "A."
		// Regex to remove leading letter markers
		cleanRegex := regexp.MustCompile(`^[A-E]\.?\s*`)
		optText := cleanRegex.ReplaceAllString(optLine, "")

		_, err = database.DB.Exec("INSERT INTO options (question_id, text, is_correct) VALUES (?, ?, ?)", qID, optText, isCorrect)
		if err != nil {
			log.Printf("Error inserting option: %v", err)
		}
	}
}
