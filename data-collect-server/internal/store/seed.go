package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// MockDeviceSpec describes one synthetic device for the demo seeder.
type MockDeviceSpec struct {
	DeviceID       string
	NodeID         string
	ProductID      string
	AppID          string
	GatewayID      string
	Firmware       string
	Seed           int64
	LightBase      float64
	LightAmplitude float64
	PostureProfile string // normal, restless, sleepy
}

// MockSeedOptions controls synthetic data generation.
type MockSeedOptions struct {
	Devices  []MockDeviceSpec
	From     time.Time
	To       time.Time
	Interval time.Duration
	Reset    bool
}

// MockSeedResult reports how many rows were generated.
type MockSeedResult struct {
	Devices   int
	Telemetry int64
	Events    int64
}

// DefaultMockDevices returns the two devices used by the demo dashboard.
func DefaultMockDevices() []MockDeviceSpec {
	return []MockDeviceSpec{
		{
			DeviceID:       "6ae74bcd7f2e6c302fa9194e_hbot",
			NodeID:         "hbot",
			ProductID:      "6ae74bcd7f2e6c302fa9194e",
			AppID:          "desk-health-demo",
			GatewayID:      "6ae74bcd7f2e6c302fa9194e_hbot",
			Firmware:       "0.1.0",
			Seed:           42,
			LightBase:      55,
			LightAmplitude: 35,
			PostureProfile: "normal",
		},
		{
			DeviceID:       "demo-desk-02",
			NodeID:         "desk-02",
			ProductID:      "desk-health-demo",
			AppID:          "desk-health-demo",
			GatewayID:      "demo-desk-02",
			Firmware:       "0.1.0",
			Seed:           84,
			LightBase:      38,
			LightAmplitude: 24,
			PostureProfile: "restless",
		},
	}
}

// SeedMockData replaces the demo devices' data with synthetic history.
func (s *Store) SeedMockData(ctx context.Context, opts MockSeedOptions) (MockSeedResult, error) {
	var result MockSeedResult
	if len(opts.Devices) == 0 {
		return result, errors.New("at least one device is required")
	}
	if opts.Interval <= 0 {
		opts.Interval = time.Minute
	}
	if opts.To.IsZero() {
		opts.To = time.Now().UTC().Truncate(time.Minute)
	}
	if opts.From.IsZero() {
		opts.From = opts.To.Add(-24 * time.Hour)
	}
	if !opts.From.Before(opts.To) {
		return result, errors.New("from must be before to")
	}

	deviceIDs := make([]string, 0, len(opts.Devices))
	for _, device := range opts.Devices {
		if device.DeviceID == "" {
			return result, errors.New("device_id is required")
		}
		deviceIDs = append(deviceIDs, device.DeviceID)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if opts.Reset {
		if _, err := tx.Exec(ctx, "DELETE FROM telemetry WHERE device_id = ANY($1)", deviceIDs); err != nil {
			return result, fmt.Errorf("reset telemetry: %w", err)
		}
		if _, err := tx.Exec(ctx, "DELETE FROM iotda_events WHERE device_id = ANY($1)", deviceIDs); err != nil {
			return result, fmt.Errorf("reset events: %w", err)
		}
		if _, err := tx.Exec(ctx, "DELETE FROM devices WHERE device_id = ANY($1)", deviceIDs); err != nil {
			return result, fmt.Errorf("reset devices: %w", err)
		}
		if _, err := tx.Exec(ctx, "DELETE FROM messages WHERE device_id = ANY($1)", deviceIDs); err != nil {
			return result, fmt.Errorf("reset messages: %w", err)
		}
	}

	tags := `[{"tag_key":"room","tag_value":"office"},{"tag_key":"demo","tag_value":"true"}]`
	for _, device := range opts.Devices {
		if _, err := tx.Exec(ctx, `
			INSERT INTO devices (
				device_id, node_id, product_id, app_id, gateway_id,
				status, status_updated_at, last_online_at,
				first_seen_at, last_seen_at, tags, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, 'ONLINE', $6, $6, $7, $6, $8::jsonb, now(), now())
			ON CONFLICT (device_id) DO UPDATE SET
				node_id = EXCLUDED.node_id,
				product_id = EXCLUDED.product_id,
				app_id = EXCLUDED.app_id,
				gateway_id = EXCLUDED.gateway_id,
				status = 'ONLINE',
				status_updated_at = EXCLUDED.status_updated_at,
				last_online_at = EXCLUDED.last_online_at,
				first_seen_at = LEAST(devices.first_seen_at, EXCLUDED.first_seen_at),
				last_seen_at = GREATEST(devices.last_seen_at, EXCLUDED.last_seen_at),
				tags = EXCLUDED.tags,
				updated_at = now()`,
			device.DeviceID, device.NodeID, device.ProductID, device.AppID, device.GatewayID,
			opts.To, opts.From, tags,
		); err != nil {
			return result, fmt.Errorf("upsert demo device %s: %w", device.DeviceID, err)
		}
	}

	var telemetryRows [][]any
	var eventRows [][]any
	for _, device := range opts.Devices {
		telemetry, events := generateMockDevice(device, opts.From, opts.To, opts.Interval, tags)
		telemetryRows = append(telemetryRows, telemetry...)
		eventRows = append(eventRows, events...)
		result.Devices++
	}

	if opts.Reset {
		samples := []struct {
			author string
			name   string
			text   string
		}{
			{"parent", "妈妈", "坐直一点，妈妈在看着你哦。"},
			{"child", "小宇", "知道啦，我会保持的！"},
			{"parent", "妈妈", "学习 40 分钟记得起来活动一下。"},
		}
		for _, device := range opts.Devices {
			for _, sample := range samples {
				if _, err := tx.Exec(ctx, `
					INSERT INTO messages (device_id, author, author_name, content, created_at)
					VALUES ($1, $2, $3, $4, $5)`,
					device.DeviceID, sample.author, sample.name, sample.text, opts.To,
				); err != nil {
					return result, fmt.Errorf("seed message for %s: %w", device.DeviceID, err)
				}
			}
		}
	}

	if len(telemetryRows) > 0 {
		count, err := tx.CopyFrom(ctx, pgx.Identifier{"telemetry"}, []string{
			"device_id", "service_id", "property", "value_json", "value_type",
			"value_num", "event_time", "received_at", "request_id", "raw_event_id",
		}, pgx.CopyFromRows(telemetryRows))
		if err != nil {
			return result, fmt.Errorf("copy telemetry: %w", err)
		}
		result.Telemetry = count
	}
	if len(eventRows) > 0 {
		count, err := tx.CopyFrom(ctx, pgx.Identifier{"iotda_events"}, []string{
			"dedupe_key", "resource", "event", "event_time", "event_time_ms",
			"request_id", "device_id", "node_id", "product_id", "app_id", "gateway_id",
			"tags", "body", "raw_payload", "received_at",
		}, pgx.CopyFromRows(eventRows))
		if err != nil {
			return result, fmt.Errorf("copy events: %w", err)
		}
		result.Events = count
	}

	if _, err := tx.Exec(ctx, `
		UPDATE devices
		SET last_seen_at = $2, status = 'ONLINE', status_updated_at = $2,
		    last_online_at = $2, updated_at = now()
		WHERE device_id = ANY($1)`, deviceIDs, opts.To); err != nil {
		return result, fmt.Errorf("update demo devices: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit seed: %w", err)
	}
	return result, nil
}

type mockSnapshot struct {
	light        int
	sound        int
	person       int
	status       string
	summary      string
	issueCount   int
	alert        int
	detecting    int
	seq          int
	rssi         int
	uptime       int
	freeHeap     int
	reportPeriod int
	lastReport   string
}

func generateMockDevice(spec MockDeviceSpec, from, to time.Time, interval time.Duration,
	tags string) ([][]any, [][]any) {
	rng := rand.New(rand.NewSource(spec.Seed))
	bootTime := from.Add(-72 * time.Hour)
	seq := 0
	index := 0
	telemetry := make([][]any, 0, int(to.Sub(from)/interval)+1)
	events := make([][]any, 0, 512)

	for ts := from; !ts.After(to); ts = ts.Add(interval) {
		index++
		seq++
		snapshot := buildMockSnapshot(spec, rng, ts, bootTime, seq)
		requestID := fmt.Sprintf("mock-%s-%d", spec.DeviceID, index)

		telemetry = append(telemetry,
			mockTelemetryRow(spec.DeviceID, "Environment", "temperature", 0, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Environment", "humidity", 0, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Environment", "light", snapshot.light, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Environment", "sound", snapshot.sound, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "personDetected", snapshot.person, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "postureStatus", snapshot.status, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "postureSummary", snapshot.summary, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "issueCount", snapshot.issueCount, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "alert", snapshot.alert, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "detecting", snapshot.detecting, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "Posture", "lastSeq", snapshot.seq, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "wifiConnected", 1, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "rssi", snapshot.rssi, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "uptime", snapshot.uptime, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "freeHeap", snapshot.freeHeap, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "reportInterval", snapshot.reportPeriod, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "firmware", spec.Firmware, ts, requestID),
			mockTelemetryRow(spec.DeviceID, "DeviceStatus", "lastReport", snapshot.lastReport, ts, requestID),
		)

		// One raw event every five samples keeps the events panel populated
		// without storing the entire minute-by-minute history twice.
		if index%5 == 0 {
			events = append(events, mockPropertyEvent(spec, snapshot, ts, requestID, tags))
		}
	}

	// Add status events at the beginning and end of the demo window.
	events = append(events,
		mockStatusEvent(spec, from.Add(3*time.Second), "ONLINE", tags),
		mockStatusEvent(spec, to.Add(-3*time.Second), "ONLINE", tags),
	)
	return telemetry, events
}

func buildMockSnapshot(spec MockDeviceSpec, rng *rand.Rand, ts, bootTime time.Time, seq int) mockSnapshot {
	hourOfDay := float64(ts.Hour()) + float64(ts.Minute())/60
	light := int(math.Round(spec.LightBase +
		spec.LightAmplitude*math.Sin(2*math.Pi*(hourOfDay-9)/24) +
		rng.NormFloat64()*3.5))
	if light < 3 {
		light = 3
	} else if light > 100 {
		light = 100
	}
	sound := 0
	if rng.Intn(100) < 7 {
		sound = 1
	}

	state := mockPostureState(spec, ts)
	status := "good"
	summary := "坐姿良好"
	person := 1
	issueCount := 0
	alert := 0
	switch state {
	case "warning":
		status = "warning"
		summary = "坐姿偏移，注意肩颈"
		issueCount = 1
		alert = 1
	case "bad":
		status = "bad"
		summary = "含胸驼背，请调整坐姿"
		issueCount = 2 + rng.Intn(2)
		alert = 1
	case "unknown":
		status = "unknown"
		summary = "暂时离开"
		person = 0
	}

	rssi := -45 - int(18*math.Abs(math.Sin(float64(ts.Unix())/1800))) + rng.Intn(5) - 2
	if rssi < -95 {
		rssi = -95
	} else if rssi > -30 {
		rssi = -30
	}
	freeHeap := 180000 + int(25000*math.Sin(float64(ts.Unix())/3600)) + rng.Intn(4000) - 2000
	if freeHeap < 120000 {
		freeHeap = 120000
	} else if freeHeap > 240000 {
		freeHeap = 240000
	}
	uptime := int(ts.Sub(bootTime).Seconds())
	if uptime < 0 {
		uptime = 0
	}

	return mockSnapshot{
		light:        light,
		sound:        sound,
		person:       person,
		status:       status,
		summary:      summary,
		issueCount:   issueCount,
		alert:        alert,
		detecting:    0,
		seq:          seq,
		rssi:         rssi,
		uptime:       uptime,
		freeHeap:     freeHeap,
		reportPeriod: 5,
		lastReport:   ts.Format("20060102T150405Z"),
	}
}

func mockPostureState(spec MockDeviceSpec, ts time.Time) string {
	block := ts.Unix() / 600 // ten-minute blocks
	hash := uint32(spec.Seed)*2654435761 + uint32(block)*2246822519
	roll := int(hash % 100)

	goodCut, warningCut, badCut := 70, 85, 93
	switch spec.PostureProfile {
	case "restless":
		goodCut, warningCut, badCut = 46, 70, 90
	case "sleepy":
		goodCut, warningCut, badCut = 55, 80, 93
	}
	switch {
	case roll < goodCut:
		return "good"
	case roll < warningCut:
		return "warning"
	case roll < badCut:
		return "bad"
	default:
		return "unknown"
	}
}

func mockTelemetryRow(deviceID, serviceID, property string, value any,
	eventTime time.Time, requestID string) []any {
	valueJSON, valueType, valueNum := encodeMockValue(value)
	return []any{
		deviceID, serviceID, property, valueJSON, valueType,
		valueNum, eventTime, eventTime.Add(2 * time.Second), requestID, nil,
	}
}

func encodeMockValue(value any) (string, string, any) {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed), "number", float64(typed)
	case int64:
		return strconv.FormatInt(typed, 10), "number", float64(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), "number", typed
	case bool:
		if typed {
			return "true", "boolean", nil
		}
		return "false", "boolean", nil
	case string:
		encoded, _ := json.Marshal(typed)
		return string(encoded), "string", nil
	case nil:
		return "null", "null", nil
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded), "unknown", nil
	}
}

func mockPropertyEvent(spec MockDeviceSpec, snapshot mockSnapshot, ts time.Time,
	requestID string, tags string) []any {
	services := []any{
		map[string]any{
			"service_id": "Environment",
			"properties": map[string]any{
				"temperature": 0,
				"humidity":    0,
				"light":       snapshot.light,
				"sound":       snapshot.sound,
			},
			"event_time": ts.Format("20060102T150405Z"),
		},
		map[string]any{
			"service_id": "Posture",
			"properties": map[string]any{
				"personDetected": snapshot.person,
				"postureStatus":  snapshot.status,
				"postureSummary": snapshot.summary,
				"issueCount":     snapshot.issueCount,
				"alert":          snapshot.alert,
				"detecting":      snapshot.detecting,
				"lastSeq":        snapshot.seq,
			},
			"event_time": ts.Format("20060102T150405Z"),
		},
		map[string]any{
			"service_id": "DeviceStatus",
			"properties": map[string]any{
				"wifiConnected":  1,
				"rssi":           snapshot.rssi,
				"uptime":         snapshot.uptime,
				"freeHeap":       snapshot.freeHeap,
				"reportInterval": snapshot.reportPeriod,
				"firmware":       spec.Firmware,
				"lastReport":     snapshot.lastReport,
			},
			"event_time": ts.Format("20060102T150405Z"),
		},
	}
	body := map[string]any{"services": services}
	envelope := map[string]any{
		"resource":      "device.property",
		"event":         "report",
		"event_time":    ts.Format("20060102T150405Z"),
		"event_time_ms": ts.Format("2006-01-02T15:04:05.000Z"),
		"request_id":    requestID,
		"notify_data": map[string]any{
			"header": map[string]any{
				"device_id":  spec.DeviceID,
				"node_id":    spec.NodeID,
				"product_id": spec.ProductID,
				"app_id":     spec.AppID,
				"gateway_id": spec.GatewayID,
				"tags": []any{
					map[string]any{"tag_key": "room", "tag_value": "office"},
					map[string]any{"tag_key": "demo", "tag_value": "true"},
				},
			},
			"body": body,
		},
	}
	raw, _ := json.Marshal(envelope)
	bodyJSON, _ := json.Marshal(body)
	return mockEventRow(spec, raw, bodyJSON, "device.property", "report", ts, requestID, tags)
}

func mockStatusEvent(spec MockDeviceSpec, ts time.Time, status string, tags string) []any {
	body := map[string]any{
		"status":             status,
		"last_online_time":   ts.Format("20060102T150405Z"),
		"status_update_time": ts.Format("2006-01-02T15:04:05.815Z"),
	}
	envelope := map[string]any{
		"resource":   "device.status",
		"event":      "update",
		"event_time": ts.Format("20060102T150405Z"),
		"notify_data": map[string]any{
			"header": map[string]any{
				"device_id":  spec.DeviceID,
				"node_id":    spec.NodeID,
				"product_id": spec.ProductID,
				"app_id":     spec.AppID,
				"gateway_id": spec.GatewayID,
			},
			"body": body,
		},
	}
	raw, _ := json.Marshal(envelope)
	bodyJSON, _ := json.Marshal(body)
	return mockEventRow(spec, raw, bodyJSON, "device.status", "update", ts,
		fmt.Sprintf("mock-status-%s-%d", spec.DeviceID, ts.Unix()), tags)
}

func mockEventRow(spec MockDeviceSpec, raw, body []byte, resource, event string,
	ts time.Time, requestID, tags string) []any {
	digest := sha256.Sum256(raw)
	return []any{
		hex.EncodeToString(digest[:]), resource, event, ts, ts,
		requestID, spec.DeviceID, spec.NodeID, spec.ProductID, spec.AppID, spec.GatewayID,
		tags, string(body), string(raw), ts.Add(2 * time.Second),
	}
}
