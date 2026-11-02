package conversation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"huddle-api/internal/apperror"
)

// Service defines business logic for conversation management.
type Service interface {
	CreateConversation(ctx context.Context, currentUserID uuid.UUID, req *CreateConversationRequest) (*ConversationDetails, error)
	ListConversations(ctx context.Context, currentUserID uuid.UUID, cursor *time.Time, limit int) (*ConversationListResponse, error)
	GetConversationDetails(ctx context.Context, conversationID, currentUserID uuid.UUID) (*ConversationDetails, error)
	AddMemberToGroup(ctx context.Context, conversationID, currentUserID, targetUserID uuid.UUID, role string) error
	RemoveMemberFromGroup(ctx context.Context, conversationID, currentUserID, targetUserID uuid.UUID) error
}

type conversationService struct {
	repo   Repository
	logger *slog.Logger
}

// NewService creates a new conversation service instance.
func NewService(repo Repository, logger *slog.Logger) Service {
	return &conversationService{
		repo:   repo,
		logger: logger,
	}
}

func (s *conversationService) CreateConversation(ctx context.Context, currentUserID uuid.UUID, req *CreateConversationRequest) (*ConversationDetails, error) {
	switch req.Type {
	case TypeDirect:
		if req.MemberID == nil {
			return nil, apperror.Validation("Validation failed", []apperror.FieldError{
				{Field: "member_id", Message: "member_id is required for direct conversations"},
			})
		}
		targetID := *req.MemberID
		if targetID == currentUserID {
			return nil, apperror.Validation("Validation failed", []apperror.FieldError{
				{Field: "member_id", Message: "Cannot create a direct conversation with yourself"},
			})
		}

		// Idempotency: Return existing direct conversation if one already exists
		existing, err := s.repo.FindDirectConversation(ctx, currentUserID, targetID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if existing != nil {
			members, err := s.repo.GetActiveMembers(ctx, existing.ID)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			return &ConversationDetails{
				Conversation: *existing,
				Members:      members,
			}, nil
		}

		// Create new direct conversation
		conv := &Conversation{
			ID:        uuid.New(),
			Type:      TypeDirect,
			CreatedBy: currentUserID,
		}
		members := []ConversationMember{
			{ConversationID: conv.ID, UserID: currentUserID, Role: RoleMember},
			{ConversationID: conv.ID, UserID: targetID, Role: RoleMember},
		}

		if err := s.repo.CreateConversationWithMembers(ctx, conv, members); err != nil {
			return nil, apperror.Internal(err)
		}

		activeMembers, err := s.repo.GetActiveMembers(ctx, conv.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}

		return &ConversationDetails{
			Conversation: *conv,
			Members:      activeMembers,
		}, nil

	case TypeGroup:
		if req.GroupName == nil || strings.TrimSpace(*req.GroupName) == "" {
			return nil, apperror.Validation("Validation failed", []apperror.FieldError{
				{Field: "group_name", Message: "group_name is required for group conversations"},
			})
		}
		cleanName := strings.TrimSpace(*req.GroupName)
		if len(cleanName) > 100 {
			return nil, apperror.Validation("Validation failed", []apperror.FieldError{
				{Field: "group_name", Message: "group_name must not exceed 100 characters"},
			})
		}

		conv := &Conversation{
			ID:        uuid.New(),
			Type:      TypeGroup,
			GroupName: &cleanName,
			CreatedBy: currentUserID,
		}

		// Build member set to deduplicate
		memberSet := make(map[uuid.UUID]bool)
		memberSet[currentUserID] = true
		for _, mid := range req.MemberIDs {
			if mid != uuid.Nil {
				memberSet[mid] = true
			}
		}

		members := make([]ConversationMember, 0, len(memberSet))
		members = append(members, ConversationMember{
			ConversationID: conv.ID,
			UserID:         currentUserID,
			Role:           RoleOwner,
		})

		for mid := range memberSet {
			if mid != currentUserID {
				members = append(members, ConversationMember{
					ConversationID: conv.ID,
					UserID:         mid,
					Role:           RoleMember,
				})
			}
		}

		if err := s.repo.CreateConversationWithMembers(ctx, conv, members); err != nil {
			return nil, apperror.Internal(err)
		}

		activeMembers, err := s.repo.GetActiveMembers(ctx, conv.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}

		return &ConversationDetails{
			Conversation: *conv,
			Members:      activeMembers,
		}, nil

	default:
		return nil, apperror.Validation("Invalid conversation type", nil)
	}
}

func (s *conversationService) ListConversations(ctx context.Context, currentUserID uuid.UUID, cursor *time.Time, limit int) (*ConversationListResponse, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Fetch limit + 1 to detect next page
	items, err := s.repo.ListUserConversations(ctx, currentUserID, cursor, limit+1)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	var nextCursor *string
	if len(items) > limit {
		nextCursorVal := items[limit-1].UpdatedAt.UTC().Format(time.RFC3339Nano)
		nextCursor = &nextCursorVal
		items = items[:limit]
	}

	return &ConversationListResponse{
		Conversations: items,
		NextCursor:    nextCursor,
	}, nil
}

func (s *conversationService) GetConversationDetails(ctx context.Context, conversationID, currentUserID uuid.UUID) (*ConversationDetails, error) {
	// Verify membership
	member, err := s.repo.GetMember(ctx, conversationID, currentUserID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return nil, apperror.Forbidden("You are not a member of this conversation")
		}
		return nil, apperror.Internal(err)
	}
	if member.LeftAt != nil {
		return nil, apperror.Forbidden("You have left this conversation")
	}

	conv, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			return nil, apperror.NotFound("Conversation not found")
		}
		return nil, apperror.Internal(err)
	}

	members, err := s.repo.GetActiveMembers(ctx, conversationID)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	return &ConversationDetails{
		Conversation: *conv,
		Members:      members,
	}, nil
}

func (s *conversationService) AddMemberToGroup(ctx context.Context, conversationID, currentUserID, targetUserID uuid.UUID, role string) error {
	conv, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			return apperror.NotFound("Conversation not found")
		}
		return apperror.Internal(err)
	}

	if conv.Type != TypeGroup {
		return apperror.Validation("Cannot add members to a direct conversation", nil)
	}

	// Caller must be owner or admin
	callerMember, err := s.repo.GetMember(ctx, conversationID, currentUserID)
	if err != nil || callerMember.LeftAt != nil {
		return apperror.Forbidden("You are not an active member of this group")
	}
	if callerMember.Role != RoleOwner && callerMember.Role != RoleAdmin {
		return apperror.Forbidden("Only group admins or owners can add new members")
	}

	assignedRole := RoleMember
	if role == RoleAdmin {
		assignedRole = RoleAdmin
	}

	return s.repo.AddMember(ctx, conversationID, targetUserID, assignedRole)
}

func (s *conversationService) RemoveMemberFromGroup(ctx context.Context, conversationID, currentUserID, targetUserID uuid.UUID) error {
	conv, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			return apperror.NotFound("Conversation not found")
		}
		return apperror.Internal(err)
	}

	if conv.Type != TypeGroup {
		return apperror.Validation("Cannot remove members from a direct conversation", nil)
	}

	callerMember, err := s.repo.GetMember(ctx, conversationID, currentUserID)
	if err != nil || callerMember.LeftAt != nil {
		return apperror.Forbidden("You are not an active member of this group")
	}

	if currentUserID == targetUserID {
		if callerMember.Role == RoleOwner {
			members, err := s.repo.GetActiveMembers(ctx, conversationID)
			if err == nil {
				ownerCount := 0
				for _, m := range members {
					if m.Role == RoleOwner {
						ownerCount++
					}
				}
				if ownerCount <= 1 {
					return apperror.Validation("Cannot leave group as the sole owner. Please transfer ownership or delete group.", nil)
				}
			}
		}
		return s.repo.RemoveMember(ctx, conversationID, currentUserID)
	}

	if callerMember.Role != RoleOwner && callerMember.Role != RoleAdmin {
		return apperror.Forbidden("Only group admins or owners can remove members")
	}

	return s.repo.RemoveMember(ctx, conversationID, targetUserID)
}
