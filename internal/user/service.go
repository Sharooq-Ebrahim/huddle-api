package user

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"huddle-api/internal/apperror"
)

const (
	MaxSearchResults = 20
	MinSearchQueryLen = 2
)

// Service defines user profile business logic operations.
type Service interface {
	GetMe(ctx context.Context, userID uuid.UUID) (*ProfileResponse, error)
	UpdateMe(ctx context.Context, userID uuid.UUID, req *UpdateProfileRequest) (*ProfileResponse, error)
	GetUserProfile(ctx context.Context, targetID uuid.UUID) (*PublicProfileResponse, error)
	SearchUsers(ctx context.Context, query string, currentUserID uuid.UUID) (*SearchUsersResponse, error)
}

type userService struct {
	repo   Repository
	logger *slog.Logger
}

// NewService constructs a new user service.
func NewService(repo Repository, logger *slog.Logger) Service {
	return &userService{
		repo:   repo,
		logger: logger,
	}
}

func (s *userService) GetMe(ctx context.Context, userID uuid.UUID) (*ProfileResponse, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, apperror.NotFound("User profile not found")
		}
		return nil, apperror.Internal(err)
	}

	resp := ToProfileResponse(u)
	return &resp, nil
}

func (s *userService) UpdateMe(ctx context.Context, userID uuid.UUID, req *UpdateProfileRequest) (*ProfileResponse, error) {
	if req.DisplayName == nil && req.AvatarURL == nil {
		// Nothing to update, return existing profile
		return s.GetMe(ctx, userID)
	}

	var trimmedName *string
	if req.DisplayName != nil {
		clean := strings.TrimSpace(*req.DisplayName)
		if len(clean) < 1 || len(clean) > 100 {
			return nil, apperror.Validation("Validation failed", []apperror.FieldError{
				{Field: "display_name", Message: "Display name must be between 1 and 100 characters"},
			})
		}
		trimmedName = &clean
	}

	u, err := s.repo.UpdateUser(ctx, userID, trimmedName, req.AvatarURL)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, apperror.NotFound("User not found")
		}
		return nil, apperror.Internal(err)
	}

	resp := ToProfileResponse(u)
	return &resp, nil
}

func (s *userService) GetUserProfile(ctx context.Context, targetID uuid.UUID) (*PublicProfileResponse, error) {
	u, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, apperror.NotFound("User not found")
		}
		return nil, apperror.Internal(err)
	}

	resp := ToPublicProfileResponse(u)
	return &resp, nil
}

func (s *userService) SearchUsers(ctx context.Context, query string, currentUserID uuid.UUID) (*SearchUsersResponse, error) {
	cleanQuery := strings.TrimSpace(query)
	if len(cleanQuery) < MinSearchQueryLen {
		return nil, apperror.Validation("Validation failed", []apperror.FieldError{
			{Field: "q", Message: "Search query must be at least 2 characters"},
		})
	}

	users, err := s.repo.SearchUsers(ctx, cleanQuery, currentUserID, MaxSearchResults)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	publicProfiles := make([]PublicProfileResponse, len(users))
	for i, u := range users {
		publicProfiles[i] = ToPublicProfileResponse(u)
	}

	return &SearchUsersResponse{
		Users: publicProfiles,
	}, nil
}

