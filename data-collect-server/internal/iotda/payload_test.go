package iotda

import (
	"testing"
	"time"
)

func TestVerifySignatureFromDocumentation(t *testing.T) {
	token := "aaaaaa"
	timestamp := "1675654743514"
	nonce := "8b9b796d388d49bba43adaa53aaf5bc4"
	signature := "2ff821fb8a976ede7d06434395ec8c25e4100bff8b3d12d8099ef7e30b58bd4c"

	if !VerifySignature(token, timestamp, nonce, signature) {
		t.Fatal("expected documented signature to verify")
	}
	if VerifySignature(token, timestamp, nonce+"x", signature) {
		t.Fatal("expected modified nonce to fail")
	}
	if VerifySignature(token+"x", timestamp, nonce, signature) {
		t.Fatal("expected modified token to fail")
	}
	if VerifySignature(token, timestamp, nonce, "") {
		t.Fatal("expected empty signature to fail")
	}
}

func TestParsePropertyReport(t *testing.T) {
	raw := []byte(`{
		"resource": "device.property",
		"event": "report",
		"event_time": "20151212T121212Z",
		"event_time_ms": "2015-12-12T12:12:12.000Z",
		"request_id": "req-1",
		"notify_data": {
			"header": {
				"device_id": "device-1",
				"node_id": "node-1",
				"product_id": "product-1",
				"app_id": "app-1",
				"gateway_id": "device-1",
				"tags": [{"tag_key": "room", "tag_value": "office"}]
			},
			"body": {
				"services": [{
					"service_id": "Environment",
					"properties": {
						"temperature": 0,
						"humidity": 0,
						"light": 62,
						"sound": 1,
						"mode": "auto",
						"nested": {"level": 2}
					},
					"event_time": "20151212T121212Z"
				}]
			}
		}
	}`)
	received := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	event, err := Parse(raw, received)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if event.Resource != ResourceDeviceProperty || event.Event != "report" {
		t.Fatalf("unexpected resource/event: %s/%s", event.Resource, event.Event)
	}
	if event.DeviceID != "device-1" || event.NodeID != "node-1" || event.ProductID != "product-1" {
		t.Fatalf("unexpected header: %+v", event)
	}
	if len(event.Tags) != 1 || event.Tags[0].Key != "room" {
		t.Fatalf("unexpected tags: %+v", event.Tags)
	}
	if event.EventTime == nil || !event.EventTime.Equal(time.Date(2015, 12, 12, 12, 12, 12, 0, time.UTC)) {
		t.Fatalf("unexpected event time: %v", event.EventTime)
	}
	if len(event.Properties) != 6 {
		t.Fatalf("expected 6 properties, got %d", len(event.Properties))
	}

	byName := map[string]PropertyRow{}
	for _, row := range event.Properties {
		byName[row.Property] = row
	}
	if row := byName["temperature"]; row.ValueType != "number" || row.ValueNum == nil || *row.ValueNum != 0 {
		t.Fatalf("unexpected temperature row: %+v", row)
	}
	if row := byName["mode"]; row.ValueType != "string" {
		t.Fatalf("unexpected mode row: %+v", row)
	}
	if row := byName["nested"]; row.ValueType != "object" {
		t.Fatalf("unexpected nested row: %+v", row)
	}
	if row := byName["sound"]; row.ValueType != "number" || row.ValueNum == nil || *row.ValueNum != 1 {
		t.Fatalf("unexpected sound row: %+v", row)
	}
}

func TestParseDeviceStatus(t *testing.T) {
	raw := []byte(`{
		"resource": "device.status",
		"event": "update",
		"event_time": "20151212T121212Z",
		"event_time_ms": "2015-12-12T12:12:12.000Z",
		"notify_data": {
			"header": {"device_id": "device-1"},
			"body": {
				"status": "ONLINE",
				"last_online_time": "20151212T121212Z",
				"status_update_time": "2015-12-12T12:12:12.815Z"
			}
		}
	}`)
	event, err := Parse(raw, time.Now())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if event.Status == nil || event.Status.Status != "ONLINE" {
		t.Fatalf("unexpected status: %+v", event.Status)
	}
	if event.Status.StatusAt.IsZero() {
		t.Fatal("expected status time")
	}
	if event.Status.LastOnlineAt == nil {
		t.Fatal("expected last online time")
	}
}

func TestParseRejectsInvalidPayloads(t *testing.T) {
	if _, err := Parse([]byte(`not json`), time.Now()); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
	if _, err := Parse([]byte(`{"event":"report"}`), time.Now()); err == nil {
		t.Fatal("expected missing resource to fail")
	}
	if _, err := Parse([]byte(`{"resource":"device.property","notify_data":{"body":{}}}`), time.Now()); err == nil {
		t.Fatal("expected missing device_id to fail")
	}
}

func TestDedupeKeyStable(t *testing.T) {
	raw := []byte(`{"resource":"device.property","notify_data":{"header":{"device_id":"d"},"body":{}}}`)
	first, err := Parse(raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Parse(raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.DedupeKey != second.DedupeKey {
		t.Fatal("dedupe key must be stable for identical payloads")
	}
	if len(first.DedupeKey) != 64 {
		t.Fatalf("unexpected dedupe key: %q", first.DedupeKey)
	}
}

func TestTimestampMillis(t *testing.T) {
	parsed, ok := TimestampMillis("1675654743514")
	if !ok {
		t.Fatal("expected millisecond timestamp to parse")
	}
	if parsed.UnixMilli() != 1675654743514 {
		t.Fatalf("unexpected timestamp: %v", parsed)
	}
	if _, ok := TimestampMillis(""); ok {
		t.Fatal("expected empty timestamp to fail")
	}
}
