package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"huddle-api/internal/config"
)


type DB struct {
	Pool *pgxpool.Pool
}


func Connect(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DATABASE_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = cfg.DB_MAX_CONNS
	poolConfig.MinConns = cfg.DB_MIN_CONNS
	poolConfig.MaxConnLifetime = cfg.DB_MAX_CONN_LIFETIME
	poolConfig.MaxConnIdleTime = cfg.DB_MAX_CONN_IDLE_TIME
	poolConfig.HealthCheckPeriod = cfg.DB_HEALTH_CHECK_PERIOD

	logger.Info("connecting to postgresql pool",
		slog.Int("max_conns", int(cfg.DB_MAX_CONNS)),
		slog.Int("min_conns", int(cfg.DB_MIN_CONNS)),
	)


	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}


	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping check failed: %w", err)
	}

	logger.Info("postgresql connection pool established successfully")
	return &DB{Pool: pool}, nil
}


func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.Pool == nil {
		return fmt.Errorf("database connection pool is not initialized")
	}
	return db.Pool.Ping(ctx)
}


func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}
