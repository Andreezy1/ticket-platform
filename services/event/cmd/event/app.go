package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Andreezy1/ticket-platform/services/event/internal/config"
	"github.com/Andreezy1/ticket-platform/services/event/internal/handler"
	"github.com/Andreezy1/ticket-platform/services/event/internal/logger"
	"github.com/Andreezy1/ticket-platform/services/event/internal/repository/postgres"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Server *http.Server
	DB     *pgxpool.Pool
}

func newApp(cfg *config.Config) (*App, error) {
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()
	db, err := postgres.NewPostgresConnect(dbCtx, cfg)
	if err != nil {
		return nil, err
	}

	h := handler.NewHandler()

	return &App{
		Server: createServer(h, cfg),
		DB:     db,
	}, nil
}

func createServer(h *handler.Handler, cfg *config.Config) *http.Server {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(logger.WithRequestID)
	router.Use(middleware.Recoverer)
	h.RegisterRoutes(router)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server
}

func run(app *App) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer app.DB.Close()
	errorChan := make(chan error, 1)
	go func() {
		slog.Info("Starting server", slog.String("addr", app.Server.Addr))
		if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errorChan <- err
		}
	}()

	return waitShutdown(app, ctx, errorChan)
}

func waitShutdown(app *App, ctx context.Context, errorchan chan error) error {
	var shutdownErr error
	select {
	case err := <-errorchan:
		shutdownErr = err
	case <-ctx.Done():
		slog.Info("shutdown signal received, exiting...")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
		if err := app.Server.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	return shutdownErr
}
