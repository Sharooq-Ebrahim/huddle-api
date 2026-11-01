package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	DisplayName  string     `json:"display_name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ProfileResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}


type PublicProfileResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Status      string    `json:"status"`
}


type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name" binding:"omitempty,min=1,max=100"`
	AvatarURL   *string `json:"avatar_url" binding:"omitempty,max=2048"`
}


type SearchUsersResponse struct {
	Users []PublicProfileResponse `json:"users"`
}

func ToProfileResponse(u *User) ProfileResponse {
	return ProfileResponse{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		AvatarURL:   u.AvatarURL,
		Status:      u.Status,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func ToPublicProfileResponse(u *User) PublicProfileResponse {
	return PublicProfileResponse{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		Status:      u.Status,
	}
}

