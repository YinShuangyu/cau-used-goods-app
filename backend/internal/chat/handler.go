package chat

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"cau-used-goods-app/backend/internal/middleware"
	"cau-used-goods-app/backend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createConversationRequest struct {
	ProductID uint64 `json:"productId" binding:"required"`
}

type sendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func (h *Handler) CreateOrGetConversation(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	conversation, err := h.service.CreateOrGetConversation(c.Request.Context(), CreateConversationInput{
		ProductID: req.ProductID,
		UserID:    userID,
	})
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, conversation)
}

func (h *Handler) ListConversations(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.service.ListConversations(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) ListMessages(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	conversationID, err := parseConversationID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid conversation id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))

	items, total, err := h.service.ListMessages(c.Request.Context(), conversationID, userID, page, pageSize)
	if err != nil {
		writeChatError(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) SendMessage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	conversationID, err := parseConversationID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid conversation id")
		return
	}

	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	message, err := h.service.SendMessage(c.Request.Context(), SendMessageInput{
		ConversationID: conversationID,
		SenderID:       userID,
		Content:        req.Content,
	})
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, message)
}

func (h *Handler) MarkRead(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	conversationID, err := parseConversationID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid conversation id")
		return
	}

	count, err := h.service.MarkRead(c.Request.Context(), conversationID, userID)
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, gin.H{
		"read":  true,
		"count": count,
	})
}

func (h *Handler) GetConversationProduct(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	conversationID, err := parseConversationID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid conversation id")
		return
	}

	product, err := h.service.GetConversationProduct(c.Request.Context(), conversationID, userID)
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, product)
}

func parseConversationID(c *gin.Context) (uint64, error) {
	return strconv.ParseUint(c.Param("id"), 10, 64)
}

func writeChatError(c *gin.Context, err error) {
	message := err.Error()
	switch message {
	case "conversation not found", "product not found":
		response.Error(c, http.StatusNotFound, response.CodeNotFound, message)
	case "permission denied":
		response.Error(c, http.StatusForbidden, response.CodeForbidden, message)
	case "productId is required", "message content is required", "message content cannot exceed 500 characters",
		"cannot chat with yourself", "product not available for chat", "conversation is closed":
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, message)
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, message)
	}
}
