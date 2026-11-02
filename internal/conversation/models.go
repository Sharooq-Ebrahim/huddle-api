package conversation

import (
	"time"

	"github.com/google/uuid"
)

const (
	TypeDirect = "direct"
	TypeGroup  = "group"

	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type Conversation struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	GroupName *string   `json:"group_name,omitempty"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationMember struct {
	ConversationID uuid.UUID  `json:"conversation_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Role           string     `json:"role"`
	JoinedAt       time.Time  `json:"joined_at"`
	LeftAt         *time.Time `json:"left_at,omitempty"`
}

type MemberInfo struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

type ConversationDetails struct {
	Conversation
	Members []MemberInfo `json:"members"`
}
type ConversationListItem struct {
	ID        uuid.UUID    `json:"id"`
	Type      string       `json:"type"`
	GroupName *string      `json:"group_name,omitempty"`
	UpdatedAt time.Time    `json:"updated_at"`
	Members   []MemberInfo `json:"members"`
}

type CreateConversationRequest struct {
	Type      string      `json:"type" binding:"required,oneof=direct group"`
	MemberID  *uuid.UUID  `json:"member_id,omitempty"`
	GroupName *string     `json:"group_name,omitempty"`
	MemberIDs []uuid.UUID `json:"member_ids,omitempty"`
}

type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Role   *string   `json:"role,omitempty"`
}

type ConversationListResponse struct {
	Conversations []ConversationListItem `json:"conversations"`
	NextCursor    *string                `json:"next_cursor,omitempty"`
}
