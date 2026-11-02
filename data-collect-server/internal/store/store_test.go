package store

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"data-collect-server/internal/iotda"
)

func TestStoreIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	ctx := context.Background()

	database, err := New(ctx, dsn, Options{RunMigrations: true, Logger: logger})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer database.Close()

	if _, err := database.pool.Exec(ctx,
		"TRUNCATE telemetry, iotda_events, devices RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	eventTime := now.Format("20060102T150405Z")
	raw := []byte(fmt.Sprintf(`{
		"resource": "device.property",
		"event": "report",
		"event_time": "%s",
		"request_id": "req-1",
		"notify_data": {
			"header": {
				"device_id": "device-1",
				"node_id": "node-1",
				"product_id": "product-1"
			},
			"body": {
				"services": [{
					"service_id": "Environment",
					"properties": {"temperature": 0, "humidity": 0, "light": 62, "sound": 0},
					"event_time": "%s"
				}]
			}
		}
	}`, eventTime, eventTime))

	event, err := iotda.Parse(raw, now)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	inserted, err := database.Ingest(ctx, event)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if !inserted {
		t.Fatal("expected first ingest to insert")
	}
	inserted, err = database.Ingest(ctx, event)
	if err != nil {
		t.Fatalf("duplicate ingest: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate ingest to be ignored")
	}

	devices, hasMore, total, err := database.ListDevices(ctx, DeviceFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 1 || hasMore || total != 1 || devices[0].DeviceID != "device-1" {
		t.Fatalf("unexpected devices: %+v hasMore=%t total=%d", devices, hasMore, total)
	}
	if devices[0].LastSeenAt.Location() != time.UTC {
		t.Fatalf("expected UTC device timestamp, got %v", devices[0].LastSeenAt)
	}

	latest, err := database.LatestTelemetry(ctx, "device-1", "", "")
	if err != nil {
		t.Fatalf("latest telemetry: %v", err)
	}
	if len(latest) != 4 {
		t.Fatalf("expected 4 latest values, got %d", len(latest))
	}
	var light *TelemetryRow
	for index := range latest {
		if latest[index].Property == "light" {
			light = &latest[index]
		}
	}
	if light == nil || light.ValueNum == nil || *light.ValueNum != 62 {
		t.Fatalf("unexpected light row: %+v", light)
	}
	if light.EventTime.Location() != time.UTC {
		t.Fatalf("expected UTC telemetry timestamp, got %v", light.EventTime)
	}

	rows, more, err := database.QueryTelemetry(ctx, TelemetryFilter{
		DeviceID: "device-1", Limit: 10,
	})
	if err != nil {
		t.Fatalf("query telemetry: %v", err)
	}
	if len(rows) != 4 || more {
		t.Fatalf("unexpected telemetry rows: %d hasMore=%t", len(rows), more)
	}

	points, err := database.QuerySeries(ctx, SeriesFilter{
		DeviceID:  "device-1",
		ServiceID: "Environment",
		Property:  "light",
		From:      now.Add(-time.Hour),
		To:        now.Add(time.Hour),
		Bucket:    "minute",
	})
	if err != nil {
		t.Fatalf("query series: %v", err)
	}
	if len(points) != 1 || points[0].Count != 1 || points[0].Avg == nil || *points[0].Avg != 62 {
		t.Fatalf("unexpected series: %+v", points)
	}

	events, more, err := database.QueryEvents(ctx, EventFilter{DeviceID: "device-1", Limit: 10})
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	if len(events) != 1 || more {
		t.Fatalf("unexpected events: %d hasMore=%t", len(events), more)
	}

	statusRaw := []byte(fmt.Sprintf(`{
		"resource": "device.status",
		"event": "update",
		"event_time": "%s",
		"notify_data": {
			"header": {"device_id": "device-1"},
			"body": {
				"status": "ONLINE",
				"last_online_time": "%s",
				"status_update_time": "%s"
			}
		}
	}`, eventTime, eventTime, now.Format(time.RFC3339Nano)))
	statusEvent, err := iotda.Parse(statusRaw, now)
	if err != nil {
		t.Fatalf("parse status: %v", err)
	}
	if _, err := database.Ingest(ctx, statusEvent); err != nil {
		t.Fatalf("ingest status: %v", err)
	}
	device, err := database.GetDevice(ctx, "device-1")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.Status == nil || *device.Status != "ONLINE" {
		t.Fatalf("unexpected device status: %+v", device)
	}
}
