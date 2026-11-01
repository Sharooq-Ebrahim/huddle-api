package config

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	SERVER_PORT string
	ENVIRONMENT string

	DATABASE_URL           string
	DB_MAX_CONNS           int32
	DB_MIN_CONNS           int32
	DB_MAX_CONN_LIFETIME   time.Duration
	DB_MAX_CONN_IDLE_TIME  time.Duration
	DB_HEALTH_CHECK_PERIOD time.Duration

	JWT_SECRET         string
	JWT_ACCESS_EXPIRY  time.Duration
	JWT_REFRESH_EXPIRY time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		SERVER_PORT:            os.Getenv("SERVER_PORT"),
		ENVIRONMENT:            os.Getenv("ENVIRONMENT"),
		DATABASE_URL:           os.Getenv("DATABASE_URL"),
		DB_MAX_CONNS:           25,
		DB_MIN_CONNS:           15,
		DB_MAX_CONN_LIFETIME:   30 * time.Minute,
		DB_MAX_CONN_IDLE_TIME:  5 * time.Minute,
		DB_HEALTH_CHECK_PERIOD: 10 * time.Second,
		JWT_SECRET:             os.Getenv("JWT_SECRET"),
		JWT_ACCESS_EXPIRY:      15 * time.Minute,
		JWT_REFRESH_EXPIRY:     24 * time.Hour,
	}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment configuration: %w", err)
	}

	return cfg, nil
}
