package chat

import (
	"database/sql"
	"fmt"
	"time"
)

// PostgresMessageRepository implémentation PostgreSQL du MessageRepository
type PostgresMessageRepository struct {
	db *sql.DB
}

// NewPostgresMessageRepository crée un nouveau repository pour les messages
func NewPostgresMessageRepository(db *sql.DB) MessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) CreateMessage(message *Message) error {
	query := `
		INSERT INTO messages (sender_id, recipient_id, content, is_read, created_at)
		VALUES ($1, $2, $3, $4, NOW() AT TIME ZONE 'UTC')
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		query,
		message.SenderID,
		message.RecipientID,
		message.Content,
		message.IsRead,
	).Scan(&message.ID, &message.CreatedAt)

	if err != nil {
		return fmt.Errorf("erreur lors de la création du message: %w", err)
	}

	return nil
}

// GetMessages récupère les messages d'une conversation
func (r *PostgresMessageRepository) GetMessages(userID1, userID2 int, limit, offset int) ([]*Message, error) {
	// ✅ QUERY SIMPLIFIÉE - la conversion timezone se fait maintenant dans MarshalJSON
	query := `
		SELECT m.id, m.sender_id, m.recipient_id, m.content, m.is_read, m.created_at,
			   u.username, CONCAT(u.first_name, ' ', u.last_name) as sender_name
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE (m.sender_id = $1 AND m.recipient_id = $2) 
		   OR (m.sender_id = $2 AND m.recipient_id = $1)
		ORDER BY m.created_at ASC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(query, userID1, userID2, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		message := &Message{}
		err := rows.Scan(
			&message.ID,
			&message.SenderID,
			&message.RecipientID,
			&message.Content,
			&message.IsRead,
			&message.CreatedAt, // ✅ Récupéré en UTC, converti dans MarshalJSON
			&message.SenderUsername,
			&message.SenderName,
		)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un message: %w", err)
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des messages: %w", err)
	}

	return messages, nil
}

// GetConversations récupère la liste des conversations d'un utilisateur
func (r *PostgresMessageRepository) GetConversations(userID int) ([]*Conversation, error) {
	// VERSION SIMPLE : D'abord récupérer tous les matchs
	matchQuery := `
		SELECT DISTINCT 
			CASE 
				WHEN ul1.liker_id = $1 THEN ul1.liked_id 
				ELSE ul1.liker_id 
			END as matched_user_id,
			u.username,
			CONCAT(u.first_name, ' ', u.last_name) as name
		FROM user_likes ul1
		JOIN user_likes ul2 ON (
			(ul1.liker_id = $1 AND ul1.liked_id = ul2.liker_id AND ul2.liked_id = $1)
			OR 
			(ul1.liked_id = $1 AND ul1.liker_id = ul2.liked_id AND ul2.liker_id = $1)
		)
		JOIN users u ON u.id = CASE 
			WHEN ul1.liker_id = $1 THEN ul1.liked_id 
			ELSE ul1.liker_id 
		END
		WHERE ul1.liker_id = $1 OR ul1.liked_id = $1
	`

	rows, err := r.db.Query(matchQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des matchs: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	seen := make(map[int]bool)

	for rows.Next() {
		var matchedUserID int
		var username, name string

		err := rows.Scan(&matchedUserID, &username, &name)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un match: %w", err)
		}

		// Éviter les doublons
		if seen[matchedUserID] {
			continue
		}
		seen[matchedUserID] = true

		conv := &Conversation{
			UserID:   matchedUserID,
			Username: username,
			Name:     name,
		}

		// Récupérer le dernier message pour cette conversation
		messageQuery := `
			SELECT id, content, sender_id, created_at
			FROM messages 
			WHERE (sender_id = $1 AND recipient_id = $2) OR (sender_id = $2 AND recipient_id = $1)
			ORDER BY created_at DESC
			LIMIT 1
		`

		var msgID int
		var msgContent string
		var msgSenderID int
		var msgCreatedAt time.Time

		err = r.db.QueryRow(messageQuery, userID, matchedUserID).Scan(&msgID, &msgContent, &msgSenderID, &msgCreatedAt)
		if err == nil {
			// Il y a un dernier message
			conv.LastMessage = &Message{
				ID:       msgID,
				Content:  msgContent,
				SenderID: msgSenderID,
			}
			conv.LastMessageTime = msgCreatedAt
		} else if err != sql.ErrNoRows {
			return nil, fmt.Errorf("erreur lors de la récupération du dernier message: %w", err)
		}
		// Si err == sql.ErrNoRows, pas de message, on continue

		// Récupérer le nombre de messages non lus
		unreadQuery := `
			SELECT COUNT(*) 
			FROM messages 
			WHERE sender_id = $1 AND recipient_id = $2 AND is_read = FALSE
		`

		err = r.db.QueryRow(unreadQuery, matchedUserID, userID).Scan(&conv.UnreadCount)
		if err != nil {
			conv.UnreadCount = 0 // En cas d'erreur, mettre 0
		}

		conversations = append(conversations, conv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des matchs: %w", err)
	}

	// Trier par dernier message (les plus récents en premier)
	for i := 0; i < len(conversations); i++ {
		for j := i + 1; j < len(conversations); j++ {
			if conversations[i].LastMessage == nil && conversations[j].LastMessage != nil {
				// Échanger : ceux avec messages en premier
				conversations[i], conversations[j] = conversations[j], conversations[i]
			} else if conversations[i].LastMessage != nil && conversations[j].LastMessage != nil {
				// Comparer les dates
				if conversations[j].LastMessageTime.After(conversations[i].LastMessageTime) {
					conversations[i], conversations[j] = conversations[j], conversations[i]
				}
			}
		}
	}

	return conversations, nil
}

// MarkMessagesAsRead marque les messages d'une conversation comme lus
func (r *PostgresMessageRepository) MarkMessagesAsRead(senderID, recipientID int) error {
	query := `
		UPDATE messages 
		SET is_read = TRUE 
		WHERE sender_id = $1 AND recipient_id = $2 AND is_read = FALSE
	`

	_, err := r.db.Exec(query, senderID, recipientID)
	if err != nil {
		return fmt.Errorf("erreur lors du marquage des messages comme lus: %w", err)
	}

	return nil
}

// GetUnreadMessageCount compte le nombre total de messages non lus pour un utilisateur
func (r *PostgresMessageRepository) GetUnreadMessageCount(userID int) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM messages 
		WHERE recipient_id = $1 AND is_read = FALSE
	`

	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des messages non lus: %w", err)
	}

	return count, nil
}

// GetUnreadCountForConversation compte les messages non lus d'une conversation spécifique
func (r *PostgresMessageRepository) GetUnreadCountForConversation(userID, otherUserID int) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM messages 
		WHERE sender_id = $1 AND recipient_id = $2 AND is_read = FALSE
	`

	var count int
	err := r.db.QueryRow(query, otherUserID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des messages non lus pour la conversation: %w", err)
	}

	return count, nil
}

// CanChat vérifie si deux utilisateurs peuvent discuter (sont matchés)
func (r *PostgresMessageRepository) CanChat(userID1, userID2 int) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_likes 
			WHERE liker_id = $1 AND liked_id = $2
		) AND EXISTS(
			SELECT 1 FROM user_likes 
			WHERE liker_id = $2 AND liked_id = $1
		)
	`

	var canChat bool
	err := r.db.QueryRow(query, userID1, userID2).Scan(&canChat)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}

	return canChat, nil
}
