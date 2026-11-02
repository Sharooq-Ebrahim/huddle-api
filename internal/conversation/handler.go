package conversation

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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
	convs := rg.Group("/conversations")
	{
		convs.POST("", h.CreateConversation)
		convs.GET("", h.ListConversations)
		convs.GET("/:id", h.GetConversation)
		convs.GET("/:id/members", h.GetMembers)
		convs.POST("/:id/members", h.AddMember)
		convs.DELETE("/:id/members/:userId", h.RemoveMember)
	}
}

func (h *Handler) CreateConversation(c *gin.Context) {
	currentUserID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	details, err := h.service.CreateConversation(c.Request.Context(), currentUserID, &req)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusCreated, details)
}

func (h *Handler) ListConversations(c *gin.Context) {
	currentUserID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	var cursor *time.Time
	if cursorParam := c.Query("cursor"); cursorParam != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorParam); err == nil {
			cursor = &t
		} else if t, err := time.Parse(time.RFC3339, cursorParam); err == nil {
			cursor = &t
		}
	}

	limit := 20
	if limitParam := c.Query("limit"); limitParam != "" {
		if parsedLimit, err := strconv.Atoi(limitParam); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	resp, err := h.service.ListConversations(c.Request.Context(), currentUserID, cursor, limit)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetConversation(c *gin.Context) {
	currentUserID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		status, resp := apperror.ToResponse(apperror.Validation("Invalid conversation ID", []apperror.FieldError{
			{Field: "id", Message: "Must be a valid UUID"},
		}))
		c.JSON(status, resp)
		return
	}

	details, err := h.service.GetConversationDetails(c.Request.Context(), convID, currentUserID)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, details)
}

func (h *Handler) GetMembers(c *gin.Context) {
	currentUserID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		status, resp := apperror.ToResponse(apperror.Validation("Invalid conversation ID", []apperror.FieldError{
			{Field: "id", Message: "Must be a valid UUID"},
		}))
		c.JSON(status, resp)
		return
	}

	details, err := h.service.GetConversationDetails(c.Request.Context(), convID, currentUserID)
	if err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"members": details.Members,
	})
}

func (h *Handler) AddMember(c *gin.Context) {
	currentUserID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		status, resp := apperror.ToResponse(apperror.Validation("Invalid conversation ID", []apperror.FieldError{
			{Field: "id", Message: "Must be a valid UUID"},
		}))
		c.JSON(status, resp)
		return
	}

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		status, resp := apperror.ToResponse(formatBindError(err))
		c.JSON(status, resp)
		return
	}

	role := RoleMember
	if req.Role != nil && *req.Role != "" {
		role = *req.Role
	}

	if err := h.service.AddMemberToGroup(c.Request.Context(), convID, currentUserID, req.UserID, role); err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.Status(http.StatusOK)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	currentUserID, ok := middleware.GetUserID(c)
	if !ok {
		status, resp := apperror.ToResponse(apperror.Unauthorized("Authentication required"))
		c.JSON(status, resp)
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		status, resp := apperror.ToResponse(apperror.Validation("Invalid conversation ID", []apperror.FieldError{
			{Field: "id", Message: "Must be a valid UUID"},
		}))
		c.JSON(status, resp)
		return
	}

	targetUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		status, resp := apperror.ToResponse(apperror.Validation("Invalid user ID", []apperror.FieldError{
			{Field: "userId", Message: "Must be a valid UUID"},
		}))
		c.JSON(status, resp)
		return
	}

	if err := h.service.RemoveMemberFromGroup(c.Request.Context(), convID, currentUserID, targetUserID); err != nil {
		status, resp := apperror.ToResponse(err)
		c.JSON(status, resp)
		return
	}

	c.Status(http.StatusOK)
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
	case "oneof":
		return "Must be one of: " + param
	default:
		return "Failed validation rule: " + tag
	}
}
