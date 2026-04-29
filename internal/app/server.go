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
		Router: NewRouter(cfg),
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

func NewRouter(cfg config.ServerConfig) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static", fileServer))

	r.Get("/", handlers.Home)
	r.Get("/admin/login", handlers.AdminLoginForm(cfg))
	r.Post("/admin/login", handlers.AdminLoginSubmit(cfg))
	r.Post("/admin/logout", handlers.AdminLogout)

	r.Route("/admin", func(r chi.Router) {
		r.Use(handlers.AdminAuthMiddleware(cfg))
		r.Get("/subjects", handlers.AdminSubjects)
		r.Get("/feedback", handlers.AdminFeedbackList)
		r.Get("/feedback/{id}", handlers.AdminFeedbackDetail)
		r.Post("/feedback/{id}/resolve", handlers.AdminResolveFeedback)
		r.Post("/feedback/{id}/dismiss", handlers.AdminDismissFeedback)
		r.Get("/subjects/{id}/edit", handlers.AdminEditSubjectForm)
		r.Post("/subjects/{id}/edit", handlers.AdminEditSubjectSubmit)
		r.Get("/import", handlers.AdminImportForm)
		r.Post("/import", handlers.AdminImportSubmit)
		r.Get("/subjects/{id}/questions", handlers.AdminSubjectQuestions)
		r.Post("/subjects/{id}/archive", handlers.AdminArchiveSubject)
		r.Post("/questions/{id}/disable", handlers.AdminDisableQuestion)
		r.Get("/questions/{id}/history", handlers.AdminQuestionHistory)
		r.Get("/questions/{id}/edit", handlers.AdminEditQuestionForm)
		r.Post("/questions/{id}/edit", handlers.AdminEditQuestionSubmit)
	})

	r.Route("/exam", func(r chi.Router) {
		r.Post("/start", handlers.StartExam)
		r.Get("/{id}/question/{qIdx}", handlers.GetQuestion)
		r.Post("/{id}/answer", handlers.SubmitAnswer)
		r.Post("/{id}/feedback", handlers.SubmitQuestionFeedback)
		r.Get("/{id}/result", handlers.ExamResult)
	})

	return r
}
