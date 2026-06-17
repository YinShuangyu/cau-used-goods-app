package chat

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetProductForChat(ctx context.Context, productID uint64) (*ProductForChat, error) {
	query := `
		SELECT id, seller_id, title, status
		FROM products
		WHERE id = ? AND is_deleted = 0
	`
	var item ProductForChat
	if err := r.db.QueryRowContext(ctx, query, productID).Scan(&item.ID, &item.SellerID, &item.Title, &item.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get product for chat: %w", err)
	}
	return &item, nil
}

func (r *Repository) FindConversation(ctx context.Context, productID, buyerID, sellerID uint64) (*Conversation, error) {
	query := `
		SELECT id, product_id, buyer_id, seller_id, last_message_id, last_message_content,
			DATE_FORMAT(last_message_time, '%Y-%m-%d %H:%i:%s'), buyer_unread_count,
			seller_unread_count, status, DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'),
			DATE_FORMAT(update_time, '%Y-%m-%d %H:%i:%s')
		FROM chat_conversations
		WHERE product_id = ? AND buyer_id = ? AND seller_id = ?
	`
	return scanConversation(r.db.QueryRowContext(ctx, query, productID, buyerID, sellerID))
}

func (r *Repository) GetConversationByID(ctx context.Context, conversationID uint64) (*Conversation, error) {
	query := `
		SELECT id, product_id, buyer_id, seller_id, last_message_id, last_message_content,
			DATE_FORMAT(last_message_time, '%Y-%m-%d %H:%i:%s'), buyer_unread_count,
			seller_unread_count, status, DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'),
			DATE_FORMAT(update_time, '%Y-%m-%d %H:%i:%s')
		FROM chat_conversations
		WHERE id = ?
	`
	return scanConversation(r.db.QueryRowContext(ctx, query, conversationID))
}

func (r *Repository) CreateConversation(ctx context.Context, productID, buyerID, sellerID uint64) (*Conversation, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO chat_conversations (product_id, buyer_id, seller_id, status)
		VALUES (?, ?, ?, ?)
	`, productID, buyerID, sellerID, ConversationStatusActive)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get created conversation id: %w", err)
	}
	return r.GetConversationByID(ctx, uint64(id))
}

func (r *Repository) ListConversations(ctx context.Context, userID uint64, page, pageSize int) ([]ConversationDetail, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM chat_conversations
		WHERE buyer_id = ? OR seller_id = ?
	`, userID, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	query := `
		SELECT c.id, c.product_id, c.buyer_id, c.seller_id, c.last_message_id,
			c.last_message_content, DATE_FORMAT(c.last_message_time, '%Y-%m-%d %H:%i:%s'),
			c.buyer_unread_count, c.seller_unread_count, c.status,
			DATE_FORMAT(c.create_time, '%Y-%m-%d %H:%i:%s'),
			DATE_FORMAT(c.update_time, '%Y-%m-%d %H:%i:%s'),
			p.title, pi.image_url, u.id, u.nickname,
			CASE WHEN c.buyer_id = ? THEN c.buyer_unread_count ELSE c.seller_unread_count END
		FROM chat_conversations c
		INNER JOIN products p ON p.id = c.product_id
		LEFT JOIN product_images pi ON pi.product_id = c.product_id AND pi.sort_order = 0
		LEFT JOIN users u ON u.id = CASE WHEN c.buyer_id = ? THEN c.seller_id ELSE c.buyer_id END
		WHERE c.buyer_id = ? OR c.seller_id = ?
		ORDER BY COALESCE(c.last_message_time, c.update_time) DESC, c.id DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, userID, userID, userID, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	items := make([]ConversationDetail, 0)
	for rows.Next() {
		var item ConversationDetail
		var lastMessageID sql.NullInt64
		var lastMessageContent, lastMessageTime sql.NullString
		var productImage, targetNickname sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ProductID, &item.BuyerID, &item.SellerID, &lastMessageID,
			&lastMessageContent, &lastMessageTime, &item.BuyerUnreadCount, &item.SellerUnreadCount,
			&item.Status, &item.CreateTime, &item.UpdateTime, &item.ProductTitle,
			&productImage, &item.TargetUserID, &targetNickname, &item.UnreadCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan conversation detail: %w", err)
		}
		fillConversationNullableFields(&item.Conversation, lastMessageID, lastMessageContent, lastMessageTime)
		if productImage.Valid {
			item.ProductImage = &productImage.String
		}
		if targetNickname.Valid {
			item.TargetNickname = &targetNickname.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate conversations: %w", err)
	}
	return items, total, nil
}

func (r *Repository) ListMessages(ctx context.Context, conversationID uint64, page, pageSize int) ([]Message, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM chat_messages
		WHERE conversation_id = ?
	`, conversationID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count chat messages: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, conversation_id, sender_id, receiver_id, content, message_type,
			read_status, DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s')
		FROM chat_messages
		WHERE conversation_id = ?
		ORDER BY create_time ASC, id ASC
		LIMIT ? OFFSET ?
	`, conversationID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()

	items := make([]Message, 0)
	for rows.Next() {
		var item Message
		if err := rows.Scan(
			&item.ID, &item.ConversationID, &item.SenderID, &item.ReceiverID,
			&item.Content, &item.MessageType, &item.ReadStatus, &item.CreateTime,
		); err != nil {
			return nil, 0, fmt.Errorf("scan chat message: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate chat messages: %w", err)
	}
	return items, total, nil
}

func (r *Repository) CreateMessage(ctx context.Context, conversation *Conversation, senderID, receiverID uint64, content string) (*Message, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin chat message tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO chat_messages (conversation_id, sender_id, receiver_id, content, message_type, read_status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, conversation.ID, senderID, receiverID, content, MessageTypeText, ReadStatusUnread)
	if err != nil {
		return nil, fmt.Errorf("create chat message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get created chat message id: %w", err)
	}
	messageID := uint64(id)

	unreadColumn := "seller_unread_count"
	if receiverID == conversation.BuyerID {
		unreadColumn = "buyer_unread_count"
	}
	updateQuery := fmt.Sprintf(`
		UPDATE chat_conversations
		SET last_message_id = ?, last_message_content = ?, last_message_time = NOW(),
			%s = %s + 1, update_time = CURRENT_TIMESTAMP
		WHERE id = ?
	`, unreadColumn, unreadColumn)
	if _, err := tx.ExecContext(ctx, updateQuery, messageID, content, conversation.ID); err != nil {
		return nil, fmt.Errorf("update conversation after message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit chat message tx: %w", err)
	}

	return r.GetMessageByID(ctx, messageID)
}

func (r *Repository) GetMessageByID(ctx context.Context, messageID uint64) (*Message, error) {
	var item Message
	if err := r.db.QueryRowContext(ctx, `
		SELECT id, conversation_id, sender_id, receiver_id, content, message_type,
			read_status, DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s')
		FROM chat_messages
		WHERE id = ?
	`, messageID).Scan(
		&item.ID, &item.ConversationID, &item.SenderID, &item.ReceiverID,
		&item.Content, &item.MessageType, &item.ReadStatus, &item.CreateTime,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat message: %w", err)
	}
	return &item, nil
}

func (r *Repository) MarkRead(ctx context.Context, conversation *Conversation, userID uint64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin mark read tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE chat_messages
		SET read_status = ?
		WHERE conversation_id = ? AND receiver_id = ? AND read_status = ?
	`, ReadStatusRead, conversation.ID, userID, ReadStatusUnread)
	if err != nil {
		return 0, fmt.Errorf("mark chat messages read: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check marked chat messages: %w", err)
	}

	unreadColumn := "seller_unread_count"
	if userID == conversation.BuyerID {
		unreadColumn = "buyer_unread_count"
	}
	updateQuery := fmt.Sprintf(`
		UPDATE chat_conversations
		SET %s = 0, update_time = CURRENT_TIMESTAMP
		WHERE id = ?
	`, unreadColumn)
	if _, err := tx.ExecContext(ctx, updateQuery, conversation.ID); err != nil {
		return 0, fmt.Errorf("clear conversation unread count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit mark read tx: %w", err)
	}
	return affected, nil
}

func (r *Repository) GetConversationProduct(ctx context.Context, productID uint64) (*ConversationProduct, error) {
	query := `
		SELECT p.id, p.title, p.price, p.status, p.seller_id
		FROM products p
		WHERE p.id = ? AND p.is_deleted = 0
	`
	var item ConversationProduct
	if err := r.db.QueryRowContext(ctx, query, productID).Scan(
		&item.ID, &item.Title, &item.Price, &item.Status, &item.SellerID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation product: %w", err)
	}

	imageQuery := `
		SELECT image_url FROM product_images
		WHERE product_id = ?
		ORDER BY sort_order ASC
	`
	rows, err := r.db.QueryContext(ctx, imageQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("get product images: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("scan product image: %w", err)
		}
		item.Images = append(item.Images, url)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate product images: %w", err)
	}

	return &item, nil
}

func scanConversation(row *sql.Row) (*Conversation, error) {
	var item Conversation
	var lastMessageID sql.NullInt64
	var lastMessageContent, lastMessageTime sql.NullString
	if err := row.Scan(
		&item.ID, &item.ProductID, &item.BuyerID, &item.SellerID, &lastMessageID,
		&lastMessageContent, &lastMessageTime, &item.BuyerUnreadCount,
		&item.SellerUnreadCount, &item.Status, &item.CreateTime, &item.UpdateTime,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan conversation: %w", err)
	}
	fillConversationNullableFields(&item, lastMessageID, lastMessageContent, lastMessageTime)
	return &item, nil
}

func fillConversationNullableFields(item *Conversation, lastMessageID sql.NullInt64, lastMessageContent, lastMessageTime sql.NullString) {
	if lastMessageID.Valid {
		value := uint64(lastMessageID.Int64)
		item.LastMessageID = &value
	}
	if lastMessageContent.Valid {
		item.LastMessageContent = &lastMessageContent.String
	}
	if lastMessageTime.Valid {
		item.LastMessageTime = &lastMessageTime.String
	}
}
