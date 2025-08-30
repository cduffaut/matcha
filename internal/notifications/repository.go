package notifications

import (
	"database/sql"
	"fmt"
)

type PostgresNotificationRepository struct{ db *sql.DB }

func NewPostgresNotificationRepository(db *sql.DB) NotificationRepository {
	return &PostgresNotificationRepository{db: db}
}

func (r *PostgresNotificationRepository) Create(n *Notification) error {
	const q = `
		INSERT INTO notifications (user_id, from_id, type, message, is_read)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	if err := r.db.QueryRow(q, n.UserID, n.FromID, n.Type, n.Message, n.IsRead).
		Scan(&n.ID, &n.CreatedAt); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *PostgresNotificationRepository) GetByUserID(userID, limit int) ([]*Notification, error) {
	const q = `
		SELECT n.id, n.user_id, n.from_id, n.type, n.message, n.is_read, n.created_at,
		       u.username, COALESCE(u.first_name,'') || ' ' || COALESCE(u.last_name,'') AS full_name
		FROM notifications n
		JOIN users u ON u.id = n.from_id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2`
	rows, err := r.db.Query(q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get notifications: %w", err)
	}
	defer rows.Close()

	ns := make([]*Notification, 0, limit)
	for rows.Next() {
		n := &Notification{FromUser: &UserInfo{}}
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.FromID, &n.Type, &n.Message, &n.IsRead, &n.CreatedAt,
			&n.FromUser.Username, &n.FromUser.Name,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		n.FromUser.ID = n.FromID
		ns = append(ns, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return ns, nil
}

func (r *PostgresNotificationRepository) MarkAsRead(id int) error {
	const q = `UPDATE notifications SET is_read = TRUE WHERE id = $1`
	if _, err := r.db.Exec(q, id); err != nil {
		return fmt.Errorf("mark as read: %w", err)
	}
	return nil
}

func (r *PostgresNotificationRepository) MarkAllAsRead(userID int) error {
	const q = `UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`
	if _, err := r.db.Exec(q, userID); err != nil {
		return fmt.Errorf("mark all as read: %w", err)
	}
	return nil
}

func (r *PostgresNotificationRepository) GetUnreadCount(userID int) (int, error) {
	const q = `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`
	var c int
	if err := r.db.QueryRow(q, userID).Scan(&c); err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return c, nil
}

func (r *PostgresNotificationRepository) Delete(id int) error {
	const q = `DELETE FROM notifications WHERE id = $1`
	if _, err := r.db.Exec(q, id); err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}
