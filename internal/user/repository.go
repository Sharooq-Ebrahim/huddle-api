package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type Repository interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, displayName *string, avatarURL *string) (*User, error)
	SearchUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]*User, error)
}

type pgxRepository struct {
	pool *pgxpool.Pool
}


func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
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
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &u, nil
}

func (r *pgxRepository) UpdateUser(ctx context.Context, id uuid.UUID, displayName *string, avatarURL *string) (*User, error) {
	setClauses := []string{"updated_at = now()"}
	args := []any{id}
	argIdx := 2

	if displayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *displayName)
		argIdx++
	}

	if avatarURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("avatar_url = $%d", argIdx))
		args = append(args, *avatarURL)
		argIdx++
	}

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $1 AND status != 'deleted'
		RETURNING id, display_name, email, password_hash, avatar_url, status, created_at, updated_at;
	`, strings.Join(setClauses, ", "))

	var u User
	err := r.pool.QueryRow(ctx, query, args...).Scan(
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
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &u, nil
}

func (r *pgxRepository) SearchUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]*User, error) {
	sqlQuery := `
		SELECT id, display_name, email, password_hash, avatar_url, status, created_at, updated_at
		FROM users
		WHERE status = 'active'
		  AND id != $1
		  AND (
		    LOWER(display_name) LIKE '%' || LOWER($2) || '%'
		    OR LOWER(email) LIKE '%' || LOWER($2) || '%'
		  )
		ORDER BY display_name ASC
		LIMIT $3;
	`
	cleanQuery := strings.TrimSpace(query)
	rows, err := r.pool.Query(ctx, sqlQuery, excludeUserID, cleanQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	users := make([]*User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID,
			&u.DisplayName,
			&u.Email,
			&u.PasswordHash,
			&u.AvatarURL,
			&u.Status,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, &u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading user rows: %w", err)
	}

	return users, nil
}
