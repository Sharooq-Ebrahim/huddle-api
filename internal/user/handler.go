package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"huddle-api/internal/apperror"
	"huddle-api/internal/middleware"
)


type Handler struct {
	service Service
}


func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}


func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	{
		users.GET("/me", h.GetMe)
		users.PATCH("/me", h.UpdateMe)
		users.GET("/search", h.SearchUsers)
		users.GET("/:id", h.GetUserProfile)
	}
}


func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	profile, err := h.service.GetMe(c.Request.Context(), userID)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, profile)
}


func (h *Handler) UpdateMe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	updated, err := h.service.UpdateMe(c.Request.Context(), userID, &req)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, updated)
}


func (h *Handler) GetUserProfile(c *gin.Context) {
	rawID := c.Param("id")
	targetID, err := uuid.Parse(rawID)
	if err != nil {
		status, resp := apperror.ToResponse(apperror.Validation("Invalid user ID", []apperror.FieldError{
			{Field: "id", Message: "Must be a valid UUID"},
		}))
		c.JSON(status, resp)
		return
	}

	profile, err := h.service.GetUserProfile(c.Request.Context(), targetID)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, profile)
}


func (h *Handler) SearchUsers(c *gin.Context) {
	currentUserID, _ := middleware.GetUserID(c)
	query := c.Query("q")

	results, err := h.service.SearchUsers(c.Request.Context(), query, currentUserID)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, results)
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
	case "url":
		return "Must be a valid URL"
	default:
		return "Failed validation rule: " + tag
	}
}

