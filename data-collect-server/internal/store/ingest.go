package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"data-collect-server/internal/iotda"

	"github.com/jackc/pgx/v5"
)

// Ingest stores one IoTDA push notification and its normalized telemetry.
// It returns false when the payload was already processed (same dedupe key).
func (s *Store) Ingest(ctx context.Context, event *iotda.Event) (bool, error) {
	if event == nil {
		return false, errors.New("nil event")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tags any
	if len(event.Tags) > 0 {
		encoded, err := json.Marshal(event.Tags)
		if err != nil {
			return false, fmt.Errorf("encode tags: %w", err)
		}
		tags = string(encoded)
	}

	var eventID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO iotda_events (
			dedupe_key, resource, event, event_time, event_time_ms, request_id,
			device_id, node_id, product_id, app_id, gateway_id,
			tags, body, raw_payload, received_at
		) VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, ''),
			NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''),
			$12::jsonb, $13::jsonb, $14::jsonb, $15
		)
		ON CONFLICT (dedupe_key) DO NOTHING
		RETURNING id`,
		event.DedupeKey, event.Resource, event.Event, event.EventTime, event.EventTimeM,
		event.RequestID, event.DeviceID, event.NodeID, event.ProductID, event.AppID,
		event.GatewayID, tags, string(event.Body), string(event.RawPayload), event.ReceivedAt,
	).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit duplicate event: %w", err)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert event: %w", err)
	}

	if event.DeviceID != "" {
		var status any
		var statusAt any
		var lastOnline any
		if event.Status != nil {
			status = event.Status.Status
			statusAt = event.Status.StatusAt
			if event.Status.LastOnlineAt != nil {
				lastOnline = *event.Status.LastOnlineAt
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO devices (
				device_id, node_id, product_id, app_id, gateway_id,
				status, status_updated_at, last_online_at,
				first_seen_at, last_seen_at, tags, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8,
				$9, $9, $10::jsonb, now(), now()
			)
			ON CONFLICT (device_id) DO UPDATE SET
				node_id = COALESCE(NULLIF(EXCLUDED.node_id, ''), devices.node_id),
				product_id = COALESCE(NULLIF(EXCLUDED.product_id, ''), devices.product_id),
				app_id = COALESCE(NULLIF(EXCLUDED.app_id, ''), devices.app_id),
				gateway_id = COALESCE(NULLIF(EXCLUDED.gateway_id, ''), devices.gateway_id),
				status = COALESCE(EXCLUDED.status, devices.status),
				status_updated_at = GREATEST(devices.status_updated_at, EXCLUDED.status_updated_at),
				last_online_at = GREATEST(devices.last_online_at, EXCLUDED.last_online_at),
				last_seen_at = GREATEST(devices.last_seen_at, EXCLUDED.last_seen_at),
				tags = COALESCE(EXCLUDED.tags, devices.tags),
				updated_at = now()`,
			event.DeviceID, event.NodeID, event.ProductID, event.AppID, event.GatewayID,
			status, statusAt, lastOnline, event.ReceivedAt, tags,
		); err != nil {
			return false, fmt.Errorf("upsert device %s: %w", event.DeviceID, err)
		}
	}

	for _, row := range event.Properties {
		if row.DeviceID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO telemetry (
				device_id, service_id, property, value_json, value_type,
				value_num, event_time, received_at, request_id, raw_event_id
			) VALUES (
				$1, $2, $3, $4::jsonb, $5,
				$6, $7, $8, NULLIF($9, ''), $10
			)`,
			row.DeviceID, row.ServiceID, row.Property, string(row.Value), row.ValueType,
			row.ValueNum, row.EventTime, event.ReceivedAt, event.RequestID, eventID,
		); err != nil {
			return false, fmt.Errorf("insert telemetry %s.%s: %w", row.ServiceID, row.Property, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit event: %w", err)
	}
	return true, nil
}
