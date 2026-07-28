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
	"github.com/Andreezy1/ticket-platform/services/auth/internal/repository/postgres"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/service"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/token"
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
	userRepo := postgres.NewUserRepository(db)

	refreshTokenRepo := postgres.NewRefreshTokenRepository(db)

	privKey, err := token.LoadPrivateKey(cfg.AUTH_PRIVATE_KEY_PATH)
	if err != nil {
		return nil, err
	}
	pub := token.PublicKeyFromPrivate(privKey)

	tokenManager := token.NewTokenService(privKey, pub, cfg.AUTH_ISSUER, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)

	userService := service.NewUserService(userRepo, refreshTokenRepo, tokenManager, cfg.RefreshTokenTTL, cfg.RefreshReuseGrace)

	h := handler.NewHandler(userService, tokenManager, cfg.AccessTokenTTL)

	return &App{
		Server: createServer(h, cfg),
		DB:     db,
	}, nil
}

func createServer(h *handler.Handler, cfg *config.Config) *http.Server {
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
	return server
}

func run(app *App) {
	errorChan := make(chan error, 1)
	go func() {
		slog.Info("Starting server", slog.String("addr", app.Server.Addr))
		if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", slog.Any("error", err))
			errorChan <- err
		}
	}()

	waitShutdown(app, errorChan)
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

		app.DB.Close()
		slog.Info("Server gracefully stopped")
	}
}
