package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"huddle-api/internal/apperror"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionNotFound = errors.New("refresh session not found")
)

// Repository defines data access operations for users and refresh sessions.
type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	CreateSession(ctx context.Context, session *RefreshSession) error
	GetSessionByID(ctx context.Context, id uuid.UUID) (*RefreshSession, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
	RotateSession(ctx context.Context, oldSessionID uuid.UUID, newSession *RefreshSession) error
}

type pgxRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

func (r *pgxRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, display_name, email, password_hash, avatar_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at;
	`
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now
	if user.Status == "" {
		user.Status = "active"
	}
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	err := r.pool.QueryRow(ctx, query,
		user.ID,
		user.DisplayName,
		user.Email,
		user.PasswordHash,
		user.AvatarURL,
		user.Status,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return apperror.Conflict("An account with this email already exists")
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

func (r *pgxRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, display_name, email, password_hash, avatar_url, status, created_at, updated_at
		FROM users
		WHERE email = $1 AND status != 'deleted';
	`
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	var u User
	err := r.pool.QueryRow(ctx, query, normalizedEmail).Scan(
		&u.ID,
		&u.DisplayName,
		&u.Email,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Status,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	return &u, nil
}

func (r *pgxRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, display_name, email, password_hash, avatar_url, status, created_at, updated_at
		FROM users
		WHERE id = $1 AND status != 'deleted';
	`
	var u User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.DisplayName,
		&u.Email,
		&u.PasswordHash,
		&u.AvatarURL,
		&u.Status,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}

	return &u, nil
}

func (r *pgxRepository) CreateSession(ctx context.Context, session *RefreshSession) error {
	query := `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6);
	`
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		session.RevokedAt,
		session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert refresh session: %w", err)
	}

	return nil
}

func (r *pgxRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*RefreshSession, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_sessions
		WHERE id = $1;
	`
	var s RefreshSession
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.UserID,
		&s.TokenHash,
		&s.ExpiresAt,
		&s.RevokedAt,
		&s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	return &s, nil
}

func (r *pgxRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE refresh_sessions
		SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL;
	`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh session: %w", err)
	}
	return nil
}

func (r *pgxRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE refresh_sessions
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL;
	`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user sessions: %w", err)
	}
	return nil
}

func (r *pgxRepository) RotateSession(ctx context.Context, oldSessionID uuid.UUID, newSession *RefreshSession) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Revoke old session
	revokeQuery := `
		UPDATE refresh_sessions
		SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL;
	`
	tag, err := tx.Exec(ctx, revokeQuery, oldSessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke old session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("old session was already revoked or not found")
	}

	// Insert new session
	insertQuery := `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6);
	`
	if newSession.ID == uuid.Nil {
		newSession.ID = uuid.New()
	}
	if newSession.CreatedAt.IsZero() {
		newSession.CreatedAt = time.Now().UTC()
	}

	_, err = tx.Exec(ctx, insertQuery,
		newSession.ID,
		newSession.UserID,
		newSession.TokenHash,
		newSession.ExpiresAt,
		newSession.RevokedAt,
		newSession.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert rotated session: %w", err)
	}

	return tx.Commit(ctx)
}

