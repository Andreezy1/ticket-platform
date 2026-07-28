package main

import (
	"log/slog"
	"os"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/config"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/logger"
	"github.com/joho/godotenv"
)

func main() {
	logger.Init(slog.LevelInfo)

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", slog.Any("error", err))
		os.Exit(1)
	}
	app, err := newApp(cfg)
	if err != nil {
		slog.Error("Failed to create app", slog.Any("error", err))
		os.Exit(1)
	}

	run(app)
}
