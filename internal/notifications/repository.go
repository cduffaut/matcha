package notifications

import (
	"database/sql"
	"fmt"
)

// PostgresNotificationRepository implémentation PostgreSQL du repository
type PostgresNotificationRepository struct {
	db *sql.DB
}

// NewPostgresNotificationRepository crée un nouveau repository
func NewPostgresNotificationRepository(db *sql.DB) NotificationRepository {
	return &PostgresNotificationRepository{db: db}
}

// Create crée une nouvelle notification
func (r *PostgresNotificationRepository) Create(notification *Notification) error {
	query := `
		INSERT INTO notifications (user_id, from_id, type, message, is_read)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		query,
		notification.UserID,
		notification.FromID,
		notification.Type,
		notification.Message,
		notification.IsRead,
	).Scan(&notification.ID, &notification.CreatedAt)

	if err != nil {
		return fmt.Errorf("erreur lors de la création de la notification: %w", err)
	}

	return nil
}

// GetByUserID récupère les notifications d'un utilisateur
func (r *PostgresNotificationRepository) GetByUserID(userID int, limit int) ([]*Notification, error) {
	query := `
		SELECT n.id, n.user_id, n.from_id, n.type, n.message, n.is_read, n.created_at,
			   u.username, CONCAT(u.first_name, ' ', u.last_name) as full_name
		FROM notifications n
		JOIN users u ON n.from_id = u.id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		notification := &Notification{
			FromUser: &UserInfo{},
		}

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.FromID,
			&notification.Type,
			&notification.Message,
			&notification.IsRead,
			&notification.CreatedAt,
			&notification.FromUser.Username,
			&notification.FromUser.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'une notification: %w", err)
		}

		notification.FromUser.ID = notification.FromID
		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des notifications: %w", err)
	}

	return notifications, nil
}

// MarkAsRead marque une notification comme lue
func (r *PostgresNotificationRepository) MarkAsRead(notificationID int) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE id = $1`

	_, err := r.db.Exec(query, notificationID)
	if err != nil {
		return fmt.Errorf("erreur lors du marquage de la notification comme lue: %w", err)
	}

	return nil
}

// MarkAllAsRead marque toutes les notifications d'un utilisateur comme lues
func (r *PostgresNotificationRepository) MarkAllAsRead(userID int) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`

	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("erreur lors du marquage de toutes les notifications comme lues: %w", err)
	}

	return nil
}

// GetUnreadCount récupère le nombre de notifications non lues
func (r *PostgresNotificationRepository) GetUnreadCount(userID int) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`

	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des notifications non lues: %w", err)
	}

	return count, nil
}

// Delete supprime une notification
func (r *PostgresNotificationRepository) Delete(notificationID int) error {
	query := `DELETE FROM notifications WHERE id = $1`

	_, err := r.db.Exec(query, notificationID)
	if err != nil {
		return fmt.Errorf("erreur lors de la suppression de la notification: %w", err)
	}

	return nil
}
