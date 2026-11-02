package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Message is a parent/child note delivered to the ESP desk display.
type Message struct {
	ID          int64      `json:"id"`
	DeviceID    string     `json:"device_id"`
	Author      string     `json:"author"`
	AuthorName  string     `json:"author_name"`
	Content     string     `json:"content"`
	CreatedAt   time.Time  `json:"created_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
}

// MessageInput is the validated payload used to create a message.
type MessageInput struct {
	DeviceID   string
	Author     string
	AuthorName string
	Content    string
}

// CreateMessage stores a new family message.
func (s *Store) CreateMessage(ctx context.Context, input MessageInput) (*Message, error) {
	message := &Message{
		DeviceID:   input.DeviceID,
		Author:     input.Author,
		AuthorName: input.AuthorName,
		Content:    input.Content,
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO messages (device_id, author, author_name, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		input.DeviceID, input.Author, input.AuthorName, input.Content,
	).Scan(&message.ID, &message.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}
	message.CreatedAt = message.CreatedAt.UTC()
	return message, nil
}

// ListMessages returns recent messages for the client message board.
func (s *Store) ListMessages(ctx context.Context, deviceID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, author, author_name, content,
		       created_at, delivered_at, read_at
		FROM messages
		WHERE ($1 = '' OR device_id = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

// ListUndeliveredMessages returns messages that have not reached the device.
func (s *Store) ListUndeliveredMessages(ctx context.Context, deviceID string, limit int) ([]Message, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, author, author_name, content,
		       created_at, delivered_at, read_at
		FROM messages
		WHERE device_id = $1 AND delivered_at IS NULL
		ORDER BY created_at ASC, id ASC
		LIMIT $2`, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list undelivered messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan undelivered message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

// AckMessages marks messages as delivered/read after the device stores them.
func (s *Store) AckMessages(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE messages
		SET delivered_at = COALESCE(delivered_at, now()),
		    read_at = now()
		WHERE id = ANY($1) AND delivered_at IS NULL`, ids)
	if err != nil {
		return 0, fmt.Errorf("ack messages: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanMessage(row pgx.Row) (Message, error) {
	var message Message
	var deliveredAt, readAt pgtype.Timestamptz
	err := row.Scan(
		&message.ID, &message.DeviceID, &message.Author, &message.AuthorName,
		&message.Content, &message.CreatedAt, &deliveredAt, &readAt,
	)
	if err != nil {
		return Message{}, err
	}
	message.CreatedAt = message.CreatedAt.UTC()
	if deliveredAt.Valid {
		value := deliveredAt.Time.UTC()
		message.DeliveredAt = &value
	}
	if readAt.Valid {
		value := readAt.Time.UTC()
		message.ReadAt = &value
	}
	return message, nil
}
