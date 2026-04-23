package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"simsexam/internal/config"
	"simsexam/internal/database"
	"simsexam/internal/importer"
)

func main() {
	cfg := config.LoadImportConfig()
	filePath := flag.String("file", "", "Path to markdown file")
	dsn := flag.String("dsn", cfg.DBPath, "SQLite database path")
	apply := flag.Bool("apply", false, "Persist the validated document into the v1 schema")
	sourceType := flag.String("source-type", cfg.SourceType, "Import source type")
	printDoc := flag.Bool("print-doc", false, "Print parsed document summary")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("Usage: go run ./cmd/importer -file <path> [-print-doc]")
	}

	doc, err := importer.ParseFile(*filePath)
	if err != nil {
		log.Fatalf("parse failed: %v", err)
	}

	report := importer.ValidateDocument(doc)
	if *printDoc {
		fmt.Printf("Subject: %s\n", doc.Manifest.Title)
		fmt.Printf("Slug: %s\n", doc.Manifest.Slug)
		fmt.Printf("Version: %s\n", doc.Manifest.Version)
		fmt.Printf("Questions: %d\n", len(doc.Questions))
	}

	fmt.Printf("Validation summary for %s\n", *filePath)
	fmt.Printf("- subject slug: %s\n", doc.Manifest.Slug)
	fmt.Printf("- title: %s\n", doc.Manifest.Title)
	fmt.Printf("- questions: %d\n", len(doc.Questions))
	fmt.Printf("- errors: %d\n", len(report.Errors))
	fmt.Printf("- warnings: %d\n", len(report.Warnings))

	for _, msg := range report.Errors {
		if msg.Line > 0 {
			fmt.Printf("ERROR line %d [%s] %s\n", msg.Line, msg.Field, msg.Message)
			continue
		}
		fmt.Printf("ERROR [%s] %s\n", msg.Field, msg.Message)
	}
	for _, msg := range report.Warnings {
		if msg.Line > 0 {
			fmt.Printf("WARN  line %d [%s] %s\n", msg.Line, msg.Field, msg.Message)
			continue
		}
		fmt.Printf("WARN  [%s] %s\n", msg.Field, msg.Message)
	}

	if !report.Valid() {
		log.Fatal("validation failed")
	}

	fmt.Println("Validation passed.")

	if !*apply {
		return
	}

	db, err := database.OpenSQLite(*dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, database.V1Migrations); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	checksum, err := fileChecksum(*filePath)
	if err != nil {
		log.Fatalf("checksum file: %v", err)
	}

	result, err := importer.ImportDocument(context.Background(), db, doc, importer.ImportOptions{
		SourceType:     *sourceType,
		SourceFilename: filepath.Base(*filePath),
		SourceChecksum: checksum,
		Activate:       true,
	})
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}

	fmt.Printf("Imported into %s\n", *dsn)
	fmt.Printf("- subject_id: %d\n", result.SubjectID)
	fmt.Printf("- question_set_id: %d\n", result.QuestionSetID)
	fmt.Printf("- import_job_id: %d\n", result.ImportJobID)
	fmt.Printf("- inserted questions: %d\n", result.QuestionsCount)
	fmt.Printf("- inserted options: %d\n", result.OptionsCount)
}

func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}
