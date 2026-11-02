package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const frameRetentionPerDevice = 80

// PostureFrame is an annotated camera frame produced by the local analysis
// service and uploaded for the parent dashboard.
type PostureFrame struct {
	ID             int64           `json:"id"`
	DeviceID       string          `json:"device_id"`
	CapturedAt     time.Time       `json:"captured_at"`
	Status         *string         `json:"status,omitempty"`
	Summary        *string         `json:"summary,omitempty"`
	PersonDetected int             `json:"person_detected"`
	RequestID      *string         `json:"request_id,omitempty"`
	ImageMIME      string          `json:"image_mime"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ImageURL       string          `json:"image_url,omitempty"`
}

// PostureFrameInput is the payload accepted from the analysis service.
type PostureFrameInput struct {
	DeviceID       string
	CapturedAt     time.Time
	Status         string
	Summary        string
	PersonDetected bool
	RequestID      string
	ImageMIME      string
	Image          []byte
	Metadata       json.RawMessage
}

// SaveFrame stores one annotated frame and prunes old frames for the device.
func (s *Store) SaveFrame(ctx context.Context, input PostureFrameInput) (*PostureFrame, error) {
	if input.DeviceID == "" {
		return nil, errors.New("device_id is required")
	}
	if len(input.Image) == 0 {
		return nil, errors.New("image is empty")
	}
	if input.CapturedAt.IsZero() {
		input.CapturedAt = time.Now().UTC()
	}
	mime := input.ImageMIME
	if mime == "" {
		mime = "image/jpeg"
	}
	var metadata any
	if len(input.Metadata) > 0 {
		if !json.Valid(input.Metadata) {
			return nil, errors.New("metadata is not valid JSON")
		}
		metadata = string(input.Metadata)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin frame transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	frame := &PostureFrame{
		DeviceID:       input.DeviceID,
		CapturedAt:     input.CapturedAt.UTC(),
		PersonDetected: boolToInt(input.PersonDetected),
		ImageMIME:      mime,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO devices (device_id) VALUES ($1)
		ON CONFLICT (device_id) DO UPDATE
		SET last_seen_at = now(), updated_at = now()`, frame.DeviceID); err != nil {
		return nil, fmt.Errorf("touch frame device: %w", err)
	}
	var status, summary, requestID pgtype.Text
	if input.Status != "" {
		status = pgtype.Text{String: input.Status, Valid: true}
	}
	if input.Summary != "" {
		summary = pgtype.Text{String: input.Summary, Valid: true}
	}
	if input.RequestID != "" {
		requestID = pgtype.Text{String: input.RequestID, Valid: true}
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO posture_frames (
			device_id, captured_at, status, summary, person_detected,
			request_id, image_mime, image_bytes, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		RETURNING id, created_at`,
		frame.DeviceID, frame.CapturedAt, status, summary, frame.PersonDetected,
		requestID, frame.ImageMIME, input.Image, metadata,
	).Scan(&frame.ID, &frame.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert posture frame: %w", err)
	}
	frame.Status = textPtr(status)
	frame.Summary = textPtr(summary)
	frame.RequestID = textPtr(requestID)
	if metadata != nil {
		frame.Metadata = json.RawMessage(input.Metadata)
	}
	frame.CreatedAt = frame.CreatedAt.UTC()

	if _, err := tx.Exec(ctx, `
		DELETE FROM posture_frames
		WHERE device_id = $1
		  AND id NOT IN (
			SELECT id FROM posture_frames
			WHERE device_id = $1
			ORDER BY captured_at DESC, id DESC
			LIMIT $2
		  )`, frame.DeviceID, frameRetentionPerDevice); err != nil {
		return nil, fmt.Errorf("prune posture frames: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit posture frame: %w", err)
	}
	return frame, nil
}

// LatestFrame returns the newest frame metadata for a device (or all devices
// when deviceID is empty). Image bytes are not included.
func (s *Store) LatestFrame(ctx context.Context, deviceID string) (*PostureFrame, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, device_id, captured_at, status, summary, person_detected,
		       request_id, image_mime, metadata, created_at
		FROM posture_frames
		WHERE ($1 = '' OR device_id = $1)
		ORDER BY captured_at DESC, id DESC
		LIMIT 1`, deviceID)
	frame, err := scanFrame(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest frame: %w", err)
	}
	return &frame, nil
}

// ListFrames returns recent frame metadata ordered newest first.
func (s *Store) ListFrames(ctx context.Context, deviceID string, limit int) ([]PostureFrame, error) {
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_id, captured_at, status, summary, person_detected,
		       request_id, image_mime, metadata, created_at
		FROM posture_frames
		WHERE ($1 = '' OR device_id = $1)
		ORDER BY captured_at DESC, id DESC
		LIMIT $2`, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list frames: %w", err)
	}
	defer rows.Close()

	frames := make([]PostureFrame, 0, limit)
	for rows.Next() {
		frame, err := scanFrame(rows)
		if err != nil {
			return nil, fmt.Errorf("scan frame: %w", err)
		}
		frames = append(frames, frame)
	}
	return frames, rows.Err()
}

// GetFrameImage returns the stored image bytes and MIME type.
func (s *Store) GetFrameImage(ctx context.Context, id int64) (string, []byte, error) {
	var mime string
	var data []byte
	err := s.pool.QueryRow(ctx,
		"SELECT image_mime, image_bytes FROM posture_frames WHERE id = $1", id).
		Scan(&mime, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("get frame image: %w", err)
	}
	return mime, data, nil
}

func scanFrame(row pgx.Row) (PostureFrame, error) {
	var frame PostureFrame
	var status, summary, requestID pgtype.Text
	var metadata []byte

	err := row.Scan(
		&frame.ID, &frame.DeviceID, &frame.CapturedAt, &status, &summary,
		&frame.PersonDetected, &requestID, &frame.ImageMIME, &metadata, &frame.CreatedAt,
	)
	if err != nil {
		return PostureFrame{}, err
	}
	frame.CapturedAt = frame.CapturedAt.UTC()
	frame.CreatedAt = frame.CreatedAt.UTC()
	frame.Status = textPtr(status)
	frame.Summary = textPtr(summary)
	frame.RequestID = textPtr(requestID)
	if len(metadata) > 0 {
		frame.Metadata = json.RawMessage(metadata)
	}
	return frame, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
