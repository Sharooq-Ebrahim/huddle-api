package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"huddle-api/internal/apperror"
	"huddle-api/internal/auth"
	"huddle-api/internal/config"
	"huddle-api/internal/database"
	"huddle-api/internal/middleware"
	"huddle-api/internal/user"
)

// Server encapsulates the HTTP router, server lifecycle, and dependencies.
type Server struct {
	cfg         *config.Config
	db          *database.DB
	logger      *slog.Logger
	engine      *gin.Engine
	httpServer  *http.Server
	authHandler *auth.Handler
	userHandler *user.Handler
}

// New constructs and configures a new Server instance.
func New(cfg *config.Config, db *database.DB, logger *slog.Logger) *Server {
	if cfg.ENVIRONMENT == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()

	var authHandler *auth.Handler
	var userHandler *user.Handler
	if db != nil && db.Pool != nil {
		authRepo := auth.NewRepository(db.Pool)
		authService := auth.NewService(authRepo, cfg, logger)
		authHandler = auth.NewHandler(authService)

		userRepo := user.NewRepository(db.Pool)
		userService := user.NewService(userRepo, logger)
		userHandler = user.NewHandler(userService)
	}

	s := &Server{
		cfg:         cfg,
		db:          db,
		logger:      logger,
		engine:      engine,
		authHandler: authHandler,
		userHandler: userHandler,
	}

	s.setupMiddleware()
	s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         ":" + cfg.SERVER_PORT,
		Handler:      engine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// SetAuthHandler allows injecting a mock or configured auth handler (useful for unit testing).
func (s *Server) SetAuthHandler(h *auth.Handler) {
	s.authHandler = h
}

// SetUserHandler allows injecting a mock or configured user handler (useful for unit testing).
func (s *Server) SetUserHandler(h *user.Handler) {
	s.userHandler = h
}

func (s *Server) setupMiddleware() {
	// Custom panic recovery that uses structured logging and returns standard JSON error
	s.engine.Use(func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}
				s.logger.Error("panic recovered",
					slog.String("error", err.Error()),
					slog.String("path", c.Request.URL.Path),
				)
				status, resp := apperror.ToResponse(apperror.Internal(err))
				c.AbortWithStatusJSON(status, resp)
			}
		}()
		c.Next()
	})

	s.engine.Use(middleware.RequestID())
	s.engine.Use(middleware.Logging(s.logger))
}

func (s *Server) setupRoutes() {
	// Liveness check: Is the HTTP process running?
	s.engine.GET("/healthz", s.handleHealthz)

	// Readiness check: Are all dependencies (PostgreSQL) ready to accept traffic?
	s.engine.GET("/readyz", s.handleReadyz)

	// API v1 route group
	v1 := s.engine.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		// Mount unauthenticated auth routes
		if s.authHandler != nil {
			s.authHandler.RegisterRoutes(v1)
		}

		// Protected routes group requiring valid Bearer JWT
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(s.cfg.JWT_SECRET))
		{
			// Mount protected user profile routes
			if s.userHandler != nil {
				s.userHandler.RegisterRoutes(protected)
			}
		}
	}
}

func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReadyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		s.logger.Warn("readiness check failed", slog.String("error", err.Error()))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unhealthy",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ready",
		"database": "connected",
	})
}

// Engine returns the Gin engine for testing and route inspection.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Start runs the HTTP server and waits for context cancellation for graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	serverErrChan := make(chan error, 1)

	go func() {
		s.logger.Info("starting HTTP server",
			slog.String("port", s.cfg.SERVER_PORT),
			slog.String("environment", s.cfg.ENVIRONMENT),
		)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- err
		}
		close(serverErrChan)
	}()

	select {
	case err := <-serverErrChan:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.logger.Info("shutdown signal received; draining in-flight requests")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("graceful server shutdown failed", slog.String("error", err.Error()))
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		s.logger.Info("HTTP server stopped gracefully")
		return nil
	}
}
