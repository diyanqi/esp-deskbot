package store

import (
	"encoding/json"
	"time"
)

// Device is a device seen through an IoTDA push notification.
type Device struct {
	DeviceID        string          `json:"device_id"`
	NodeID          *string         `json:"node_id,omitempty"`
	ProductID       *string         `json:"product_id,omitempty"`
	AppID           *string         `json:"app_id,omitempty"`
	GatewayID       *string         `json:"gateway_id,omitempty"`
	Status          *string         `json:"status,omitempty"`
	StatusUpdatedAt *time.Time      `json:"status_updated_at,omitempty"`
	LastOnlineAt    *time.Time      `json:"last_online_at,omitempty"`
	FirstSeenAt     time.Time       `json:"first_seen_at"`
	LastSeenAt      time.Time       `json:"last_seen_at"`
	Tags            json.RawMessage `json:"tags,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// DeviceFilter selects devices for the list endpoint.
type DeviceFilter struct {
	Search string
	Status string
	Limit  int
	Offset int
}

// TelemetryFilter selects normalized property values.
type TelemetryFilter struct {
	DeviceID  string
	ServiceID string
	Property  string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

// TelemetryRow is one property value.
type TelemetryRow struct {
	ID         int64           `json:"id"`
	DeviceID   string          `json:"device_id"`
	ServiceID  string          `json:"service_id"`
	Property   string          `json:"property"`
	Value      json.RawMessage `json:"value"`
	ValueType  string          `json:"value_type"`
	ValueNum   *float64        `json:"value_num,omitempty"`
	EventTime  time.Time       `json:"event_time"`
	ReceivedAt time.Time       `json:"received_at"`
	RequestID  *string         `json:"request_id,omitempty"`
}

// EventFilter selects raw IoTDA events.
type EventFilter struct {
	DeviceID   string
	Resource   string
	Event      string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
	IncludeRaw bool
}

// EventRow is a stored IoTDA push notification.
type EventRow struct {
	ID          int64           `json:"id"`
	Resource    string          `json:"resource"`
	Event       *string         `json:"event,omitempty"`
	RequestID   *string         `json:"request_id,omitempty"`
	DeviceID    *string         `json:"device_id,omitempty"`
	NodeID      *string         `json:"node_id,omitempty"`
	ProductID   *string         `json:"product_id,omitempty"`
	AppID       *string         `json:"app_id,omitempty"`
	GatewayID   *string         `json:"gateway_id,omitempty"`
	EventTime   *time.Time      `json:"event_time,omitempty"`
	EventTimeMS *time.Time      `json:"event_time_ms,omitempty"`
	Tags        json.RawMessage `json:"tags,omitempty"`
	Body        json.RawMessage `json:"body"`
	RawPayload  json.RawMessage `json:"raw_payload,omitempty"`
	ReceivedAt  time.Time       `json:"received_at"`
}

// SeriesFilter selects an aggregated numeric series.
type SeriesFilter struct {
	DeviceID  string
	ServiceID string
	Property  string
	From      time.Time
	To        time.Time
	Bucket    string
}

// SeriesPoint is one aggregated bucket.
type SeriesPoint struct {
	Bucket time.Time `json:"bucket"`
	Count  int64     `json:"count"`
	Avg    *float64  `json:"avg,omitempty"`
	Min    *float64  `json:"min,omitempty"`
	Max    *float64  `json:"max,omitempty"`
}
