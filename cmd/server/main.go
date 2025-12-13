package main

import (
	"fmt"
	"log"
	"net/http"
	"simsexam/internal/database"
	"simsexam/internal/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// 1. Initialize DB
	if err := database.InitDB("./simsexam.db"); err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer database.DB.Close()

	// 2. Seed Data
	database.SeedInitialData()

	// 3. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static Files
	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static", fileServer))

	// Routes
	r.Get("/", handlers.Home)

	r.Route("/exam", func(r chi.Router) {
		r.Post("/start", handlers.StartExam)
		r.Get("/{id}/question/{qIdx}", handlers.GetQuestion)
		r.Post("/{id}/answer", handlers.SubmitAnswer)
		r.Get("/{id}/result", handlers.ExamResult)
	})

	// 4. Start Server
	port := ":6080"
	fmt.Printf("Server starting on http://localhost%s\n", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
