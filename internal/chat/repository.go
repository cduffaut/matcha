package chat

import (
	"database/sql"
	"fmt"
	// "time"
)

// PostgresMessageRepository implémente MessageRepository avec PostgreSQL.
type PostgresMessageRepository struct {
	db *sql.DB
}

func NewPostgresMessageRepository(db *sql.DB) MessageRepository {
	return &PostgresMessageRepository{db: db}
}

/* ============================== Messages ============================== */

func (r *PostgresMessageRepository) CreateMessage(m *Message) error {
	const q = `
		INSERT INTO messages (sender_id, recipient_id, content, is_read, created_at)
		VALUES ($1, $2, $3, $4, NOW() AT TIME ZONE 'UTC')
		RETURNING id, created_at`
	if err := r.db.QueryRow(q, m.SenderID, m.RecipientID, m.Content, m.IsRead).
		Scan(&m.ID, &m.CreatedAt); err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	return nil
}

func (r *PostgresMessageRepository) GetMessages(userID1, userID2 int, limit, offset int) ([]*Message, error) {
	const q = `
		SELECT m.id, m.sender_id, m.recipient_id, m.content, m.is_read, m.created_at,
			   u.username, CONCAT(u.first_name, ' ', u.last_name) AS sender_name
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE (m.sender_id = $1 AND m.recipient_id = $2)
		   OR (m.sender_id = $2 AND m.recipient_id = $1)
		ORDER BY m.created_at ASC
		LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(q, userID1, userID2, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	var out []*Message
	for rows.Next() {
		msg := &Message{}
		if err := rows.Scan(
			&msg.ID, &msg.SenderID, &msg.RecipientID, &msg.Content, &msg.IsRead, &msg.CreatedAt,
			&msg.SenderUsername, &msg.SenderName,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}
	return out, nil
}

/* =========================== Conversations =========================== */

func (r *PostgresMessageRepository) GetConversations(userID int) ([]*Conversation, error) {
	// Récupère les matchs, le dernier message et le compteur non-lu en un seul round-trip, triés.
	const q = `
WITH matches AS (
	SELECT DISTINCT
		CASE WHEN ul1.liker_id = $1 THEN ul1.liked_id ELSE ul1.liker_id END AS other_id
	FROM user_likes ul1
	JOIN user_likes ul2
	  ON ul1.liker_id = ul2.liked_id AND ul1.liked_id = ul2.liker_id
	WHERE $1 IN (ul1.liker_id, ul1.liked_id)
)
SELECT u.id AS other_id,
	   u.username,
	   CONCAT(u.first_name, ' ', u.last_name) AS name,
	   lm.id            AS last_id,
	   lm.content       AS last_content,
	   lm.sender_id     AS last_sender_id,
	   lm.created_at    AS last_created_at,
	   COALESCE(ur.unread_count, 0) AS unread_count
FROM (SELECT DISTINCT other_id FROM matches) m
JOIN users u ON u.id = m.other_id
LEFT JOIN LATERAL (
	SELECT id, content, sender_id, created_at
	FROM messages
	WHERE (sender_id = $1 AND recipient_id = m.other_id)
	   OR (sender_id = m.other_id AND recipient_id = $1)
	ORDER BY created_at DESC
	LIMIT 1
) lm ON TRUE
LEFT JOIN LATERAL (
	SELECT COUNT(*) AS unread_count
	FROM messages
	WHERE sender_id = m.other_id AND recipient_id = $1 AND is_read = FALSE
) ur ON TRUE
ORDER BY lm.created_at DESC NULLS LAST, u.username ASC`
	rows, err := r.db.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversations: %w", err)
	}
	defer rows.Close()

	var out []*Conversation
	for rows.Next() {
		var (
			conv          Conversation
			lastID        sql.NullInt64
			lastContent   sql.NullString
			lastSenderID  sql.NullInt64
			lastCreatedAt sql.NullTime
		)
		if err := rows.Scan(
			&conv.UserID,
			&conv.Username,
			&conv.Name,
			&lastID,
			&lastContent,
			&lastSenderID,
			&lastCreatedAt,
			&conv.UnreadCount,
		); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}

		if lastID.Valid {
			conv.LastMessage = &Message{
				ID:       int(lastID.Int64),
				Content:  lastContent.String,
				SenderID: int(lastSenderID.Int64),
			}
			if lastCreatedAt.Valid {
				conv.LastMessageTime = lastCreatedAt.Time
			}
		}
		out = append(out, &conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations: %w", err)
	}
	return out, nil
}

/* ============================== Read/Count ============================ */

func (r *PostgresMessageRepository) MarkMessagesAsRead(senderID, recipientID int) error {
	const q = `
		UPDATE messages
		SET is_read = TRUE
		WHERE sender_id = $1 AND recipient_id = $2 AND is_read = FALSE`
	if _, err := r.db.Exec(q, senderID, recipientID); err != nil {
		return fmt.Errorf("mark as read: %w", err)
	}
	return nil
}

func (r *PostgresMessageRepository) GetUnreadMessageCount(userID int) (int, error) {
	const q = `SELECT COUNT(*) FROM messages WHERE recipient_id = $1 AND is_read = FALSE`
	var count int
	if err := r.db.QueryRow(q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("unread total: %w", err)
	}
	return count, nil
}

func (r *PostgresMessageRepository) GetUnreadCountForConversation(userID, otherUserID int) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM messages
		WHERE sender_id = $1 AND recipient_id = $2 AND is_read = FALSE`
	var count int
	if err := r.db.QueryRow(q, otherUserID, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("unread per conversation: %w", err)
	}
	return count, nil
}

/* ============================== Matching ============================= */

func (r *PostgresMessageRepository) CanChat(userID1, userID2 int) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM user_likes WHERE liker_id = $1 AND liked_id = $2
		) AND EXISTS (
			SELECT 1 FROM user_likes WHERE liker_id = $2 AND liked_id = $1
		)`
	var ok bool
	if err := r.db.QueryRow(q, userID1, userID2).Scan(&ok); err != nil {
		return false, fmt.Errorf("can chat: %w", err)
	}
	return ok, nil
}
