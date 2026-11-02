package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const deviceColumns = `
	device_id, node_id, product_id, app_id, gateway_id,
	status, status_updated_at, last_online_at,
	first_seen_at, last_seen_at, tags, created_at, updated_at`

// ListDevices returns devices ordered by last_seen_at.
func (s *Store) ListDevices(ctx context.Context, filter DeviceFilter) ([]Device, bool, int64, error) {
	where := `WHERE ($1 = '' OR device_id ILIKE '%' || $1 || '%' OR COALESCE(node_id, '') ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status = $2)`
	args := []any{filter.Search, filter.Status}

	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM devices "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, fmt.Errorf("count devices: %w", err)
	}

	query := "SELECT " + deviceColumns + " FROM devices " + where +
		" ORDER BY last_seen_at DESC LIMIT $3 OFFSET $4"
	rows, err := s.pool.Query(ctx, query,
		filter.Search, filter.Status, filter.Limit+1, filter.Offset)
	if err != nil {
		return nil, false, 0, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices, err := scanDevices(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(devices) > filter.Limit
	if hasMore {
		devices = devices[:filter.Limit]
	}
	return devices, hasMore, total, nil
}

// GetDevice returns one device by ID.
func (s *Store) GetDevice(ctx context.Context, deviceID string) (*Device, error) {
	row := s.pool.QueryRow(ctx, "SELECT "+deviceColumns+" FROM devices WHERE device_id = $1", deviceID)
	device, err := scanDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}
	return &device, nil
}

func scanDevices(rows pgx.Rows) ([]Device, error) {
	devices := make([]Device, 0, 16)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}
	return devices, nil
}

func scanDevice(row pgx.Row) (Device, error) {
	var device Device
	var nodeID, productID, appID, gatewayID, status pgtype.Text
	var statusAt, lastOnline pgtype.Timestamptz
	var tags []byte

	err := row.Scan(
		&device.DeviceID, &nodeID, &productID, &appID, &gatewayID,
		&status, &statusAt, &lastOnline,
		&device.FirstSeenAt, &device.LastSeenAt, &tags,
		&device.CreatedAt, &device.UpdatedAt,
	)
	if err != nil {
		return Device{}, err
	}
	device.NodeID = textPtr(nodeID)
	device.ProductID = textPtr(productID)
	device.AppID = textPtr(appID)
	device.GatewayID = textPtr(gatewayID)
	device.Status = textPtr(status)
	device.StatusUpdatedAt = timePtr(statusAt)
	device.LastOnlineAt = timePtr(lastOnline)
	device.FirstSeenAt = device.FirstSeenAt.UTC()
	device.LastSeenAt = device.LastSeenAt.UTC()
	device.CreatedAt = device.CreatedAt.UTC()
	device.UpdatedAt = device.UpdatedAt.UTC()
	if len(tags) > 0 {
		device.Tags = json.RawMessage(tags)
	}
	return device, nil
}

// LatestTelemetry returns the newest value for each property of a device.
func (s *Store) LatestTelemetry(ctx context.Context, deviceID, serviceID, property string) ([]TelemetryRow, error) {
	var conditions []string
	args := []any{deviceID}
	conditions = append(conditions, "device_id = $1")
	next := 2
	if serviceID != "" {
		conditions = append(conditions, fmt.Sprintf("service_id = $%d", next))
		args = append(args, serviceID)
		next++
	}
	if property != "" {
		conditions = append(conditions, fmt.Sprintf("property = $%d", next))
		args = append(args, property)
	}

	query := `SELECT DISTINCT ON (service_id, property)
		id, device_id, service_id, property, value_json, value_type, value_num,
		event_time, received_at, request_id
		FROM telemetry
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY service_id, property, event_time DESC, id DESC
		LIMIT 1000`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("latest telemetry: %w", err)
	}
	defer rows.Close()

	items := make([]TelemetryRow, 0, 16)
	for rows.Next() {
		item, err := scanTelemetry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan latest telemetry: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest telemetry: %w", err)
	}
	return items, nil
}

// QueryTelemetry returns telemetry ordered newest first.
func (s *Store) QueryTelemetry(ctx context.Context, filter TelemetryFilter) ([]TelemetryRow, bool, error) {
	conditions := make([]string, 0, 5)
	args := make([]any, 0, 7)
	next := 1

	if filter.DeviceID != "" {
		conditions = append(conditions, fmt.Sprintf("device_id = $%d", next))
		args = append(args, filter.DeviceID)
		next++
	}
	if filter.ServiceID != "" {
		conditions = append(conditions, fmt.Sprintf("service_id = $%d", next))
		args = append(args, filter.ServiceID)
		next++
	}
	if filter.Property != "" {
		conditions = append(conditions, fmt.Sprintf("property = $%d", next))
		args = append(args, filter.Property)
		next++
	}
	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("event_time >= $%d", next))
		args = append(args, *filter.From)
		next++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("event_time < $%d", next))
		args = append(args, *filter.To)
		next++
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := `SELECT
		id, device_id, service_id, property, value_json, value_type, value_num,
		event_time, received_at, request_id
		FROM telemetry ` + where + `
		ORDER BY event_time DESC, id DESC
		LIMIT $` + fmt.Sprint(next) + ` OFFSET $` + fmt.Sprint(next+1)
	args = append(args, filter.Limit+1, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query telemetry: %w", err)
	}
	defer rows.Close()

	items := make([]TelemetryRow, 0, filter.Limit)
	for rows.Next() {
		item, err := scanTelemetry(rows)
		if err != nil {
			return nil, false, fmt.Errorf("scan telemetry: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate telemetry: %w", err)
	}
	hasMore := len(items) > filter.Limit
	if hasMore {
		items = items[:filter.Limit]
	}
	return items, hasMore, nil
}

func scanTelemetry(row pgx.Row) (TelemetryRow, error) {
	var item TelemetryRow
	var value []byte
	var valueNum pgtype.Float8
	var requestID pgtype.Text

	err := row.Scan(
		&item.ID, &item.DeviceID, &item.ServiceID, &item.Property,
		&value, &item.ValueType, &valueNum,
		&item.EventTime, &item.ReceivedAt, &requestID,
	)
	if err != nil {
		return TelemetryRow{}, err
	}
	item.Value = json.RawMessage(value)
	if valueNum.Valid {
		number := valueNum.Float64
		item.ValueNum = &number
	}
	item.EventTime = item.EventTime.UTC()
	item.ReceivedAt = item.ReceivedAt.UTC()
	item.RequestID = textPtr(requestID)
	return item, nil
}

// QueryEvents returns raw IoTDA notifications.
func (s *Store) QueryEvents(ctx context.Context, filter EventFilter) ([]EventRow, bool, error) {
	conditions := make([]string, 0, 6)
	args := make([]any, 0, 8)
	next := 1

	if filter.DeviceID != "" {
		conditions = append(conditions, fmt.Sprintf("device_id = $%d", next))
		args = append(args, filter.DeviceID)
		next++
	}
	if filter.Resource != "" {
		conditions = append(conditions, fmt.Sprintf("resource = $%d", next))
		args = append(args, filter.Resource)
		next++
	}
	if filter.Event != "" {
		conditions = append(conditions, fmt.Sprintf("event = $%d", next))
		args = append(args, filter.Event)
		next++
	}
	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(event_time, received_at) >= $%d", next))
		args = append(args, *filter.From)
		next++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(event_time, received_at) < $%d", next))
		args = append(args, *filter.To)
		next++
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	rawColumn := "NULL::jsonb"
	if filter.IncludeRaw {
		rawColumn = "raw_payload"
	}
	query := `SELECT
		id, resource, event, request_id, device_id, node_id, product_id, app_id, gateway_id,
		event_time, event_time_ms, tags, body, received_at, ` + rawColumn + `
		FROM iotda_events ` + where + `
		ORDER BY COALESCE(event_time, received_at) DESC, id DESC
		LIMIT $` + fmt.Sprint(next) + ` OFFSET $` + fmt.Sprint(next+1)
	args = append(args, filter.Limit+1, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	events := make([]EventRow, 0, filter.Limit)
	for rows.Next() {
		var row EventRow
		var event, requestID, deviceID, nodeID, productID, appID, gatewayID pgtype.Text
		var eventTime, eventTimeMS pgtype.Timestamptz
		var tags, body, raw []byte

		err := rows.Scan(
			&row.ID, &row.Resource, &event, &requestID, &deviceID, &nodeID,
			&productID, &appID, &gatewayID,
			&eventTime, &eventTimeMS, &tags, &body, &row.ReceivedAt, &raw,
		)
		if err != nil {
			return nil, false, fmt.Errorf("scan event: %w", err)
		}
		row.Event = textPtr(event)
		row.RequestID = textPtr(requestID)
		row.DeviceID = textPtr(deviceID)
		row.NodeID = textPtr(nodeID)
		row.ProductID = textPtr(productID)
		row.AppID = textPtr(appID)
		row.GatewayID = textPtr(gatewayID)
		row.EventTime = timePtr(eventTime)
		row.EventTimeMS = timePtr(eventTimeMS)
		row.ReceivedAt = row.ReceivedAt.UTC()
		if len(tags) > 0 {
			row.Tags = json.RawMessage(tags)
		}
		row.Body = json.RawMessage(body)
		if len(raw) > 0 {
			row.RawPayload = json.RawMessage(raw)
		}
		events = append(events, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate events: %w", err)
	}
	hasMore := len(events) > filter.Limit
	if hasMore {
		events = events[:filter.Limit]
	}
	return events, hasMore, nil
}

// QuerySeries aggregates numeric telemetry into fixed time buckets.
func (s *Store) QuerySeries(ctx context.Context, filter SeriesFilter) ([]SeriesPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT date_trunc($1, event_time) AS bucket,
		       count(*),
		       avg(value_num),
		       min(value_num),
		       max(value_num)
		FROM telemetry
		WHERE device_id = $2
		  AND service_id = $3
		  AND property = $4
		  AND event_time >= $5
		  AND event_time < $6
		  AND value_num IS NOT NULL
		GROUP BY bucket
		ORDER BY bucket`,
		filter.Bucket, filter.DeviceID, filter.ServiceID, filter.Property,
		filter.From, filter.To)
	if err != nil {
		return nil, fmt.Errorf("query series: %w", err)
	}
	defer rows.Close()

	points := make([]SeriesPoint, 0, 64)
	for rows.Next() {
		var point SeriesPoint
		var avg, min, max pgtype.Float8
		if err := rows.Scan(&point.Bucket, &point.Count, &avg, &min, &max); err != nil {
			return nil, fmt.Errorf("scan series point: %w", err)
		}
		if avg.Valid {
			value := avg.Float64
			point.Avg = &value
		}
		if min.Valid {
			value := min.Float64
			point.Min = &value
		}
		if max.Valid {
			value := max.Float64
			point.Max = &value
		}
		point.Bucket = point.Bucket.UTC()
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate series: %w", err)
	}
	return points, nil
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}
