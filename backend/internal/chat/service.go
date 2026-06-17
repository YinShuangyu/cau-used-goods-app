package chat

import (
	"context"
	"fmt"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type CreateConversationInput struct {
	ProductID uint64
	UserID    uint64
}

type SendMessageInput struct {
	ConversationID uint64
	SenderID       uint64
	Content        string
}

func (s *Service) CreateOrGetConversation(ctx context.Context, input CreateConversationInput) (*Conversation, error) {
	if input.ProductID == 0 {
		return nil, fmt.Errorf("productId is required")
	}
	product, err := s.repo.GetProductForChat(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, fmt.Errorf("product not found")
	}
	if product.SellerID == input.UserID {
		return nil, fmt.Errorf("cannot chat with yourself")
	}
	if product.Status == "OFF_SHELF" || product.Status == "SOLD" || product.Status == "DELETED" {
		return nil, fmt.Errorf("product not available for chat")
	}

	conversation, err := s.repo.FindConversation(ctx, product.ID, input.UserID, product.SellerID)
	if err != nil {
		return nil, err
	}
	if conversation != nil {
		return conversation, nil
	}

	return s.repo.CreateConversation(ctx, product.ID, input.UserID, product.SellerID)
}

func (s *Service) ListConversations(ctx context.Context, userID uint64, page, pageSize int) ([]ConversationDetail, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.ListConversations(ctx, userID, page, pageSize)
}

func (s *Service) ListMessages(ctx context.Context, conversationID, userID uint64, page, pageSize int) ([]Message, int, error) {
	conversation, err := s.getConversationForUser(ctx, conversationID, userID)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	return s.repo.ListMessages(ctx, conversation.ID, page, pageSize)
}

func (s *Service) SendMessage(ctx context.Context, input SendMessageInput) (*Message, error) {
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" {
		return nil, fmt.Errorf("message content is required")
	}
	if len([]rune(input.Content)) > 500 {
		return nil, fmt.Errorf("message content cannot exceed 500 characters")
	}

	conversation, err := s.getConversationForUser(ctx, input.ConversationID, input.SenderID)
	if err != nil {
		return nil, err
	}
	if conversation.Status != ConversationStatusActive {
		return nil, fmt.Errorf("conversation is closed")
	}

	receiverID := conversation.SellerID
	if input.SenderID == conversation.SellerID {
		receiverID = conversation.BuyerID
	}
	return s.repo.CreateMessage(ctx, conversation, input.SenderID, receiverID, input.Content)
}

func (s *Service) MarkRead(ctx context.Context, conversationID, userID uint64) (int64, error) {
	conversation, err := s.getConversationForUser(ctx, conversationID, userID)
	if err != nil {
		return 0, err
	}
	return s.repo.MarkRead(ctx, conversation, userID)
}

func (s *Service) GetConversationProduct(ctx context.Context, conversationID, userID uint64) (*ConversationProduct, error) {
	conversation, err := s.getConversationForUser(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetConversationProduct(ctx, conversation.ProductID)
}

func (s *Service) getConversationForUser(ctx context.Context, conversationID, userID uint64) (*Conversation, error) {
	if conversationID == 0 {
		return nil, fmt.Errorf("conversation not found")
	}
	conversation, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conversation.BuyerID != userID && conversation.SellerID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	return conversation, nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
