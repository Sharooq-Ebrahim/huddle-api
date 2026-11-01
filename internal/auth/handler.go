package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"huddle-api/internal/apperror"
)

// Handler handles HTTP requests for authentication.
type Handler struct {
	service Service
}

// NewHandler constructs an auth HTTP handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts the auth endpoints onto a Gin router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.Logout)
	}
}

// Register godoc
// @Summary Register a new user
// @Accept json
// @Produce json
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	user, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": user,
	})
}

// Login godoc
// @Summary Login with email and password
// @Accept json
// @Produce json
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	authResp, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, authResp)
}

// Refresh godoc
// @Summary Rotate refresh token and obtain a new access token
// @Accept json
// @Produce json
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	tokenResp, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, tokenResp)
}

// Logout godoc
// @Summary Revoke the provided refresh session
// @Accept json
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.Status(http.StatusNoContent)
}

func formatBindError(err error) *apperror.AppError {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		details := make([]apperror.FieldError, len(ve))
		for i, fe := range ve {
			details[i] = apperror.FieldError{
				Field:   fe.Field(),
				Message: validationTagToMessage(fe.Tag(), fe.Param()),
			}
		}
		return apperror.Validation("Invalid request payload", details)
	}

	return apperror.Validation("Malformed JSON body", nil)
}

func validationTagToMessage(tag, param string) string {
	switch tag {
	case "required":
		return "This field is required"
	case "email":
		return "Must be a valid email address"
	case "min":
		return "Minimum length is " + param
	case "max":
		return "Maximum length is " + param
	default:
		return "Failed validation rule: " + tag
	}
}

