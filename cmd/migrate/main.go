package main

import (
	"flag"
	"fmt"
	"log"
	"simsexam/internal/config"
	"simsexam/internal/database"
)

func main() {
	cfg := config.LoadRuntimeConfig()
	dsn := flag.String("dsn", cfg.DBPath, "SQLite database path")
	flag.Parse()

	db, err := database.OpenSQLite(*dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, database.V1Migrations); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	fmt.Printf("Applied %d v1 migrations to %s\n", len(database.V1Migrations), *dsn)
}
