package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"huddle-api/internal/apperror"
	"huddle-api/internal/auth"
)

const (
	AuthUserIDGinKey            = "auth_user_id"
	AuthUserIDContextKey contextKey = "auth_user_id"
)

// AuthRequired verifies the incoming JWT in the Authorization: Bearer header.
func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			status, resp := apperror.ToResponse(apperror.Unauthorized("Authorization header is required"))
			c.AbortWithStatusJSON(status, resp)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			status, resp := apperror.ToResponse(apperror.Unauthorized("Authorization header format must be: Bearer <token>"))
			c.AbortWithStatusJSON(status, resp)
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		userID, err := auth.ValidateAccessToken(tokenStr, jwtSecret)
		if err != nil {
			status, resp := apperror.ToResponse(apperror.Unauthorized("Invalid or expired access token"))
			c.AbortWithStatusJSON(status, resp)
			return
		}

		// Store in Gin context and standard Context
		c.Set(AuthUserIDGinKey, userID)
		ctx := context.WithValue(c.Request.Context(), AuthUserIDContextKey, userID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetUserID extracts the authenticated user ID from the Gin context.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(AuthUserIDGinKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

