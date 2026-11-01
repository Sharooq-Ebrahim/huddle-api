package database

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(databaseURL, migrationsPath string, logger *slog.Logger) error {

	migURL, err := formatMigrateURL(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to format database URL for migrations: %w", err)
	}

	sourceURL := migrationsPath
	if !strings.HasPrefix(sourceURL, "file://") {
		sourceURL = "file://" + sourceURL
	}

	logger.Info("running database migrations",
		slog.String("source", sourceURL),
	)

	m, err := migrate.New(sourceURL, migURL)
	if err != nil {
		return fmt.Errorf("failed to initialize migration runner: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Warn("error closing migration source", slog.String("err", srcErr.Error()))
		}
		if dbErr != nil {
			logger.Warn("error closing migration db connection", slog.String("err", dbErr.Error()))
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("database schema is up to date (no new migrations)")
			return nil
		}
		return fmt.Errorf("migration run failed: %w", err)
	}

	logger.Info("database migrations applied successfully")
	return nil
}

func formatMigrateURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if u.Scheme == "postgres" || u.Scheme == "postgresql" {
		u.Scheme = "pgx5"
	}

	return u.String(), nil
}

