package conversation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMemberNotFound       = errors.New("member not found")
)

type Repository interface {
	FindDirectConversation(ctx context.Context, userA, userB uuid.UUID) (*Conversation, error)
	CreateConversationWithMembers(ctx context.Context, conv *Conversation, members []ConversationMember) error
	GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error)
	GetActiveMembers(ctx context.Context, conversationID uuid.UUID) ([]MemberInfo, error)
	GetMember(ctx context.Context, conversationID, userID uuid.UUID) (*ConversationMember, error)
	ListUserConversations(ctx context.Context, userID uuid.UUID, cursor *time.Time, limit int) ([]ConversationListItem, error)
	AddMember(ctx context.Context, conversationID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, conversationID, userID uuid.UUID) error
}

type pgxRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

func (r *pgxRepository) FindDirectConversation(ctx context.Context, userA, userB uuid.UUID) (*Conversation, error) {
	query := `
		SELECT c.id, c.type, c.group_name, c.created_by, c.created_at, c.updated_at
		FROM conversations c
		JOIN conversation_members m1 ON c.id = m1.conversation_id AND m1.user_id = $1 AND m1.left_at IS NULL
		JOIN conversation_members m2 ON c.id = m2.conversation_id AND m2.user_id = $2 AND m2.left_at IS NULL
		WHERE c.type = 'direct'
		LIMIT 1;
	`
	var c Conversation
	err := r.pool.QueryRow(ctx, query, userA, userB).Scan(
		&c.ID,
		&c.Type,
		&c.GroupName,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query direct conversation: %w", err)
	}
	return &c, nil
}

func (r *pgxRepository) CreateConversationWithMembers(ctx context.Context, conv *Conversation, members []ConversationMember) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if conv.ID == uuid.Nil {
		conv.ID = uuid.New()
	}
	now := time.Now().UTC()
	conv.CreatedAt = now
	conv.UpdatedAt = now

	insertConvSQL := `
		INSERT INTO conversations (id, type, group_name, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6);
	`
	_, err = tx.Exec(ctx, insertConvSQL,
		conv.ID,
		conv.Type,
		conv.GroupName,
		conv.CreatedBy,
		conv.CreatedAt,
		conv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert conversation: %w", err)
	}

	insertMemberSQL := `
		INSERT INTO conversation_members (conversation_id, user_id, role, joined_at, left_at)
		VALUES ($1, $2, $3, $4, $5);
	`
	for _, m := range members {
		joinedAt := m.JoinedAt
		if joinedAt.IsZero() {
			joinedAt = now
		}
		_, err = tx.Exec(ctx, insertMemberSQL,
			conv.ID,
			m.UserID,
			m.Role,
			joinedAt,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to insert conversation member: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *pgxRepository) GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	query := `
		SELECT id, type, group_name, created_by, created_at, updated_at
		FROM conversations
		WHERE id = $1;
	`
	var c Conversation
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.Type,
		&c.GroupName,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	return &c, nil
}

func (r *pgxRepository) GetActiveMembers(ctx context.Context, conversationID uuid.UUID) ([]MemberInfo, error) {
	query := `
		SELECT cm.user_id, u.display_name, u.avatar_url, cm.role, cm.joined_at
		FROM conversation_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.conversation_id = $1 AND cm.left_at IS NULL
		ORDER BY cm.joined_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query members: %w", err)
	}
	defer rows.Close()

	members := make([]MemberInfo, 0)
	for rows.Next() {
		var m MemberInfo
		if err := rows.Scan(
			&m.UserID,
			&m.DisplayName,
			&m.AvatarURL,
			&m.Role,
			&m.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan member row: %w", err)
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *pgxRepository) GetMember(ctx context.Context, conversationID, userID uuid.UUID) (*ConversationMember, error) {
	query := `
		SELECT conversation_id, user_id, role, joined_at, left_at
		FROM conversation_members
		WHERE conversation_id = $1 AND user_id = $2;
	`
	var m ConversationMember
	err := r.pool.QueryRow(ctx, query, conversationID, userID).Scan(
		&m.ConversationID,
		&m.UserID,
		&m.Role,
		&m.JoinedAt,
		&m.LeftAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to get member: %w", err)
	}
	return &m, nil
}

func (r *pgxRepository) ListUserConversations(ctx context.Context, userID uuid.UUID, cursor *time.Time, limit int) ([]ConversationListItem, error) {
	query := `
		SELECT c.id, c.type, c.group_name, c.updated_at
		FROM conversations c
		JOIN conversation_members cm ON c.id = cm.conversation_id
		WHERE cm.user_id = $1 AND cm.left_at IS NULL
		  AND ($2::timestamptz IS NULL OR c.updated_at < $2)
		ORDER BY c.updated_at DESC
		LIMIT $3;
	`
	rows, err := r.pool.Query(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query user conversations: %w", err)
	}
	defer rows.Close()

	items := make([]ConversationListItem, 0)
	for rows.Next() {
		var item ConversationListItem
		if err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.GroupName,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversation list item: %w", err)
		}
		items = append(items, item)
	}

	// Populate active members for each conversation item
	for i := range items {
		members, err := r.GetActiveMembers(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Members = members
	}

	return items, nil
}

func (r *pgxRepository) AddMember(ctx context.Context, conversationID, userID uuid.UUID, role string) error {
	query := `
		INSERT INTO conversation_members (conversation_id, user_id, role, joined_at, left_at)
		VALUES ($1, $2, $3, now(), NULL)
		ON CONFLICT (conversation_id, user_id) DO UPDATE
		SET role = EXCLUDED.role, joined_at = now(), left_at = NULL;
	`
	_, err := r.pool.Exec(ctx, query, conversationID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to add or re-join member: %w", err)
	}
	return nil
}

func (r *pgxRepository) RemoveMember(ctx context.Context, conversationID, userID uuid.UUID) error {
	query := `
		UPDATE conversation_members
		SET left_at = now()
		WHERE conversation_id = $1 AND user_id = $2 AND left_at IS NULL;
	`
	tag, err := r.pool.Exec(ctx, query, conversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}
