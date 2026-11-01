package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"huddle-api/internal/config"
	"huddle-api/internal/database"
	"huddle-api/internal/server"
)

func main() {
	// Setup root context that cancels on interrupt / termination signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load()

	slog.Info("config loaded", slog.Any("config", cfg))
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize structured logger
	var logHandler slog.Handler
	if cfg.ENVIRONMENT == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	logger.Info("initializing huddle-api service",
		slog.String("env", cfg.ENVIRONMENT),
		slog.String("port", cfg.SERVER_PORT),
	)

	// Connect to PostgreSQL database pool
	db, err := database.Connect(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to connect to postgresql", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// Run pending database migrations
	if err := database.RunMigrations(cfg.DATABASE_URL, "migrations", logger); err != nil {
		logger.Error("failed to apply database migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize and run HTTP server with graceful shutdown
	srv := server.New(cfg, db, logger)
	if err := srv.Start(ctx); err != nil {
		logger.Error("server fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("huddle-api service shutdown complete")
}
