package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/config"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type App struct {
	Server *http.Server
}

func newApp(cfg *config.Config) (*App, error) {
	h := handler.NewHandler()

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
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

	return &App{
		Server: server,
	}, nil
}

func run(app *App) {
	errorchan := make(chan error, 1)
	go func() {
		slog.Info("Starting server", slog.String("addr", app.Server.Addr))
		if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", slog.Any("error", err))
			errorchan <- err
		}
	}()
	waitShutdown(app, errorchan)
}

func waitShutdown(app *App, errorchan chan error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errorchan:
		slog.Error("Server error", slog.Any("error", err))
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("shutdown signal received, exiting...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := app.Server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server shutdown error", slog.Any("error", err))
		}
		slog.Info("Server gracefully stopped")
	}
}
