package app

import (
	"context"
	"net/http"

	"simsexam/internal/bootstrap"
	"simsexam/internal/config"
	"simsexam/internal/database"
	"simsexam/internal/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type ServerApp struct {
	Config config.ServerConfig
	DB     *databaseHandle
	Router http.Handler
}

type databaseHandle struct{}

func NewServerApp(ctx context.Context, cfg config.ServerConfig) (*ServerApp, error) {
	if err := database.InitDB(cfg.DBPath); err != nil {
		return nil, err
	}

	if _, err := bootstrap.PrepareV1Database(ctx, database.DB, bootstrap.V1BootstrapOptions{}); err != nil {
		_ = database.DB.Close()
		database.DB = nil
		return nil, err
	}

	return &ServerApp{
		Config: cfg,
		DB:     &databaseHandle{},
		Router: NewRouter(),
	}, nil
}

func (a *ServerApp) Close() error {
	if database.DB == nil {
		return nil
	}
	err := database.DB.Close()
	database.DB = nil
	return err
}

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static", fileServer))

	r.Get("/", handlers.Home)
	r.Get("/admin/subjects", handlers.AdminSubjects)
	r.Get("/admin/import", handlers.AdminImportForm)
	r.Post("/admin/import", handlers.AdminImportSubmit)
	r.Get("/admin/subjects/{id}/questions", handlers.AdminSubjectQuestions)
	r.Get("/admin/questions/{id}/edit", handlers.AdminEditQuestionForm)
	r.Post("/admin/questions/{id}/edit", handlers.AdminEditQuestionSubmit)

	r.Route("/exam", func(r chi.Router) {
		r.Post("/start", handlers.StartExam)
		r.Get("/{id}/question/{qIdx}", handlers.GetQuestion)
		r.Post("/{id}/answer", handlers.SubmitAnswer)
		r.Get("/{id}/result", handlers.ExamResult)
	})

	return r
}
