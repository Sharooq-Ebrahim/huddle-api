package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"huddle-api/internal/apperror"
	"huddle-api/internal/config"
)

type Service interface {
	Register(ctx context.Context, req *RegisterRequest) (*UserResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*TokenResponse, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*UserResponse, error)
}

type authService struct {
	repo   Repository
	cfg    *config.Config
	logger *slog.Logger
}

func NewService(repo Repository, cfg *config.Config, logger *slog.Logger) Service {
	return &authService{
		repo:   repo,
		cfg:    cfg,
		logger: logger,
	}
}

func (s *authService) Register(ctx context.Context, req *RegisterRequest) (*UserResponse, error) {
	displayName := strings.TrimSpace(req.DisplayName)
	if len(displayName) < 1 || len(displayName) > 100 {
		return nil, apperror.Validation("Validation failed", []apperror.FieldError{
			{Field: "display_name", Message: "Display name must be between 1 and 100 characters"},
		})
	}

	if len(req.Password) < 8 {
		return nil, apperror.Validation("Validation failed", []apperror.FieldError{
			{Field: "password", Message: "Password must be at least 8 characters"},
		})
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to hash password: %w", err))
	}

	user := &User{
		DisplayName:  displayName,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Status:       "active",
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	resp := ToUserResponse(user)
	return &resp, nil
}

func (s *authService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, apperror.Unauthorized("Invalid email or password")
		}
		return nil, apperror.Internal(err)
	}

	if !CheckPassword(user.PasswordHash, req.Password) {
		return nil, apperror.Unauthorized("Invalid email or password")
	}

	if user.Status != "active" {
		return nil, apperror.Forbidden("Account is not active")
	}


	accessToken, err := GenerateAccessToken(user.ID, s.cfg.JWT_SECRET, s.cfg.JWT_ACCESS_EXPIRY)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to generate access token: %w", err))
	}


	sessionID := uuid.New()
	rawRefreshToken, tokenHash, err := GenerateRefreshToken(sessionID)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to generate refresh token: %w", err))
	}

	session := &RefreshSession{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(s.cfg.JWT_REFRESH_EXPIRY),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to persist session: %w", err))
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		User:         ToUserResponse(user),
	}, nil
}

func (s *authService) Refresh(ctx context.Context, rawRefreshToken string) (*TokenResponse, error) {
	sessionID, secretHex, err := ParseRefreshToken(rawRefreshToken)
	if err != nil {
		return nil, apperror.Unauthorized("Invalid refresh token")
	}

	session, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, apperror.Unauthorized("Session not found or expired")
		}
		return nil, apperror.Internal(err)
	}

	if session.RevokedAt != nil {
		s.logger.Warn("security alert: refresh token reuse detected",
			slog.String("user_id", session.UserID.String()),
			slog.String("session_id", session.ID.String()),
		)
		_ = s.repo.RevokeAllUserSessions(ctx, session.UserID)
		return nil, apperror.Unauthorized("Refresh token reuse detected; all sessions revoked for security")
	}


	if time.Now().After(session.ExpiresAt) {
		return nil, apperror.Unauthorized("Refresh token has expired")
	}

	if !VerifyTokenHash(session.TokenHash, secretHex) {
		return nil, apperror.Unauthorized("Invalid refresh token")
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil || user.Status != "active" {
		return nil, apperror.Unauthorized("User account is inactive or not found")
	}

	newAccessToken, err := GenerateAccessToken(user.ID, s.cfg.JWT_SECRET, s.cfg.JWT_ACCESS_EXPIRY)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to generate access token: %w", err))
	}

	newSessionID := uuid.New()
	newRawRefreshToken, newTokenHash, err := GenerateRefreshToken(newSessionID)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to generate new refresh token: %w", err))
	}

	newSession := &RefreshSession{
		ID:        newSessionID,
		UserID:    user.ID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().UTC().Add(s.cfg.JWT_REFRESH_EXPIRY),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.RotateSession(ctx, session.ID, newSession); err != nil {
		return nil, apperror.Internal(fmt.Errorf("failed to rotate session: %w", err))
	}

	return &TokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRawRefreshToken,
	}, nil
}

func (s *authService) Logout(ctx context.Context, rawRefreshToken string) error {
	sessionID, _, err := ParseRefreshToken(rawRefreshToken)
	if err != nil {
		return nil
	}

	if err := s.repo.RevokeSession(ctx, sessionID); err != nil {
		return apperror.Internal(fmt.Errorf("failed to revoke session: %w", err))
	}

	return nil
}

func (s *authService) GetUserByID(ctx context.Context, id uuid.UUID) (*UserResponse, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, apperror.NotFound("User not found")
		}
		return nil, apperror.Internal(err)
	}

	resp := ToUserResponse(user)
	return &resp, nil
}

