// Package iotda implements the Huawei Cloud IoTDA HTTP push contract.
package iotda

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Resource names carried by IoTDA data forwarding pushes.
const (
	ResourceDeviceProperty = "device.property"
	ResourceDeviceStatus   = "device.status"
	ResourceDeviceMessage  = "device.message"
)

// Envelope is the outer payload pushed by IoTDA.
type Envelope struct {
	Resource    string          `json:"resource"`
	Event       string          `json:"event"`
	EventTime   string          `json:"event_time"`
	EventTimeMS string          `json:"event_time_ms"`
	RequestID   string          `json:"request_id"`
	NotifyData  NotifyData      `json:"notify_data"`
	Raw         json.RawMessage `json:"-"`
}

// NotifyData carries the notification header and body.
type NotifyData struct {
	Header Header          `json:"header"`
	Body   json.RawMessage `json:"body"`
}

// Header identifies the device that produced the notification.
type Header struct {
	AppID     string `json:"app_id"`
	DeviceID  string `json:"device_id"`
	NodeID    string `json:"node_id"`
	ProductID string `json:"product_id"`
	GatewayID string `json:"gateway_id"`
	Tags      []Tag  `json:"tags"`
}

// Tag is a device tag attached by IoTDA.
type Tag struct {
	Key   string `json:"tag_key"`
	Value string `json:"tag_value"`
}

// PropertyReport is the body of a device.property notification.
type PropertyReport struct {
	Services []PropertyService `json:"services"`
}

// PropertyService is one service block inside a property report.
type PropertyService struct {
	ServiceID  string                     `json:"service_id"`
	Properties map[string]json.RawMessage `json:"properties"`
	EventTime  string                     `json:"event_time"`
}

// DeviceStatus is the body of a device.status notification.
type DeviceStatus struct {
	Status           string `json:"status"`
	LastOnlineTime   string `json:"last_online_time"`
	StatusUpdateTime string `json:"status_update_time"`
}

// PropertyRow is one normalized telemetry point.
type PropertyRow struct {
	DeviceID  string
	ServiceID string
	Property  string
	Value     json.RawMessage
	ValueType string
	ValueNum  *float64
	EventTime time.Time
}

// StatusChange is one normalized device status change.
type StatusChange struct {
	Status       string
	StatusAt     time.Time
	LastOnlineAt *time.Time
}

// Event is the normalized representation stored by the service.
type Event struct {
	DedupeKey  string
	Resource   string
	Event      string
	RequestID  string
	EventTime  *time.Time
	EventTimeM *time.Time
	DeviceID   string
	NodeID     string
	ProductID  string
	AppID      string
	GatewayID  string
	Tags       []Tag
	Body       json.RawMessage
	RawPayload json.RawMessage
	ReceivedAt time.Time
	Properties []PropertyRow
	Status     *StatusChange
}

// Parse decodes and validates the outer IoTDA envelope.
func Parse(raw []byte, receivedAt time.Time) (*Event, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode push payload: %w", err)
	}
	if envelope.Resource == "" {
		return nil, errors.New("resource is required")
	}
	if (envelope.Resource == ResourceDeviceProperty || envelope.Resource == ResourceDeviceStatus) &&
		envelope.NotifyData.Header.DeviceID == "" {
		return nil, errors.New("notify_data.header.device_id is required")
	}
	if len(envelope.NotifyData.Body) == 0 || bytes.Equal(envelope.NotifyData.Body, []byte("null")) {
		envelope.NotifyData.Body = json.RawMessage(`{}`)
	}
	if !json.Valid(envelope.NotifyData.Body) {
		return nil, errors.New("notify_data.body is not valid JSON")
	}

	digest := sha256.Sum256(raw)
	event := &Event{
		DedupeKey:  hex.EncodeToString(digest[:]),
		Resource:   envelope.Resource,
		Event:      envelope.Event,
		RequestID:  envelope.RequestID,
		DeviceID:   envelope.NotifyData.Header.DeviceID,
		NodeID:     envelope.NotifyData.Header.NodeID,
		ProductID:  envelope.NotifyData.Header.ProductID,
		AppID:      envelope.NotifyData.Header.AppID,
		GatewayID:  envelope.NotifyData.Header.GatewayID,
		Tags:       envelope.NotifyData.Header.Tags,
		Body:       envelope.NotifyData.Body,
		RawPayload: append(json.RawMessage(nil), raw...),
		ReceivedAt: receivedAt.UTC(),
	}
	if parsed, ok := ParseTime(envelope.EventTime); ok {
		event.EventTime = &parsed
	}
	if parsed, ok := ParseTime(envelope.EventTimeMS); ok {
		event.EventTimeM = &parsed
	}

	switch envelope.Resource {
	case ResourceDeviceProperty:
		properties, err := parseProperties(event)
		if err != nil {
			return nil, err
		}
		event.Properties = properties
	case ResourceDeviceStatus:
		status, err := parseDeviceStatus(event)
		if err != nil {
			return nil, err
		}
		event.Status = status
	}
	return event, nil
}

func parseProperties(event *Event) ([]PropertyRow, error) {
	var report PropertyReport
	if err := json.Unmarshal(event.Body, &report); err != nil {
		return nil, fmt.Errorf("decode property body: %w", err)
	}
	if len(report.Services) == 0 {
		return nil, nil
	}

	rows := make([]PropertyRow, 0, len(report.Services))
	for _, service := range report.Services {
		if service.ServiceID == "" {
			continue
		}
		eventTime := event.timeFor(service.EventTime)
		for property, raw := range service.Properties {
			if property == "" || len(raw) == 0 {
				continue
			}
			valueType, valueNum, err := classifyValue(raw)
			if err != nil {
				// Keep the raw event even if one value is malformed; skip
				// only the corresponding telemetry row.
				continue
			}
			rows = append(rows, PropertyRow{
				DeviceID:  event.DeviceID,
				ServiceID: service.ServiceID,
				Property:  property,
				Value:     append(json.RawMessage(nil), raw...),
				ValueType: valueType,
				ValueNum:  valueNum,
				EventTime: eventTime,
			})
		}
	}
	return rows, nil
}

func parseDeviceStatus(event *Event) (*StatusChange, error) {
	var status DeviceStatus
	if err := json.Unmarshal(event.Body, &status); err != nil {
		return nil, fmt.Errorf("decode device status body: %w", err)
	}
	if status.Status == "" {
		return nil, nil
	}
	change := &StatusChange{
		Status: status.Status,
	}
	if parsed, ok := ParseTime(status.StatusUpdateTime); ok {
		change.StatusAt = parsed
	} else if event.EventTime != nil {
		change.StatusAt = *event.EventTime
	} else if event.EventTimeM != nil {
		change.StatusAt = *event.EventTimeM
	} else {
		change.StatusAt = event.ReceivedAt
	}
	if parsed, ok := ParseTime(status.LastOnlineTime); ok {
		change.LastOnlineAt = &parsed
	}
	return change, nil
}

func (e *Event) timeFor(serviceTime string) time.Time {
	if parsed, ok := ParseTime(serviceTime); ok {
		return parsed
	}
	if e.EventTime != nil {
		return *e.EventTime
	}
	if e.EventTimeM != nil {
		return *e.EventTimeM
	}
	return e.ReceivedAt
}

// ParseTime accepts the timestamp formats used by IoTDA.
func ParseTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"20060102T150405Z",
		"2006-01-02T15:04:05.000Z",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// VerifySignature validates the IoTDA token signature.
//
// IoTDA sends three headers: token, timestamp and nonce. The signature is
// sha256 of the token, timestamp and nonce sorted lexicographically and
// concatenated.
func VerifySignature(token, timestamp, nonce, signature string) bool {
	if token == "" {
		return false
	}
	parts := make([]string, 0, 3)
	parts = append(parts, token)
	if timestamp != "" {
		parts = append(parts, timestamp)
	}
	if nonce != "" {
		parts = append(parts, nonce)
	}
	sortStrings(parts)
	digest := sha256.Sum256([]byte(joinStrings(parts)))
	expected := hex.EncodeToString(digest[:])
	provided := strings.ToLower(strings.TrimSpace(signature))
	return constantTimeEqual(expected, provided)
}

// TimestampMillis parses IoTDA's millisecond timestamp header.
func TimestampMillis(value string) (time.Time, bool) {
	var millis int64
	if _, err := fmt.Sscanf(value, "%d", &millis); err != nil || millis <= 0 {
		return time.Time{}, false
	}
	// The platform sends milliseconds, but accept seconds for compatibility.
	if millis < 1_000_000_000_000 {
		return time.Unix(millis, 0).UTC(), true
	}
	return time.UnixMilli(millis).UTC(), true
}

func classifyValue(raw json.RawMessage) (string, *float64, error) {
	if !json.Valid(raw) {
		return "", nil, errors.New("invalid JSON value")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", nil, err
	}
	switch typed := value.(type) {
	case nil:
		return "null", nil, nil
	case bool:
		return "boolean", nil, nil
	case string:
		return "string", nil, nil
	case json.Number:
		number, err := typed.Float64()
		if err != nil {
			return "number", nil, nil
		}
		return "number", &number, nil
	case []any:
		return "array", nil, nil
	case map[string]any:
		return "object", nil, nil
	default:
		return "unknown", nil, nil
	}
}
