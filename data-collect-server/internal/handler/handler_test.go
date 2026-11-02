package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sort"
	"strings"
	"testing"
	"time"

	"data-collect-server/internal/iotda"
	"data-collect-server/internal/store"
)

type fakeStore struct {
	ingested  *iotda.Event
	inserted  bool
	ingestErr error
	devices   []store.Device
	total     int64
	hasMore   bool
}

func (f *fakeStore) Ingest(ctx context.Context, event *iotda.Event) (bool, error) {
	f.ingested = event
	return f.inserted, f.ingestErr
}

func (f *fakeStore) ListDevices(ctx context.Context, filter store.DeviceFilter) ([]store.Device, bool, int64, error) {
	return f.devices, f.hasMore, f.total, nil
}

func (f *fakeStore) GetDevice(ctx context.Context, deviceID string) (*store.Device, error) {
	if len(f.devices) > 0 {
		return &f.devices[0], nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeStore) LatestTelemetry(ctx context.Context, deviceID, serviceID, property string) ([]store.TelemetryRow, error) {
	return []store.TelemetryRow{}, nil
}

func (f *fakeStore) QueryTelemetry(ctx context.Context, filter store.TelemetryFilter) ([]store.TelemetryRow, bool, error) {
	return []store.TelemetryRow{}, false, nil
}

func (f *fakeStore) QueryEvents(ctx context.Context, filter store.EventFilter) ([]store.EventRow, bool, error) {
	return []store.EventRow{}, false, nil
}

func (f *fakeStore) QuerySeries(ctx context.Context, filter store.SeriesFilter) ([]store.SeriesPoint, error) {
	return []store.SeriesPoint{}, nil
}

func (f *fakeStore) GetInsights(ctx context.Context, deviceID string, window store.InsightsRange) (*store.Insights, error) {
	return &store.Insights{DeviceID: deviceID, From: window.From, To: window.To}, nil
}

func (f *fakeStore) SaveFrame(ctx context.Context, input store.PostureFrameInput) (*store.PostureFrame, error) {
	return &store.PostureFrame{ID: 1, DeviceID: input.DeviceID, ImageMIME: input.ImageMIME}, nil
}

func (f *fakeStore) LatestFrame(ctx context.Context, deviceID string) (*store.PostureFrame, error) {
	return nil, store.ErrNotFound
}

func (f *fakeStore) ListFrames(ctx context.Context, deviceID string, limit int) ([]store.PostureFrame, error) {
	return []store.PostureFrame{}, nil
}

func (f *fakeStore) GetFrameImage(ctx context.Context, id int64) (string, []byte, error) {
	return "", nil, store.ErrNotFound
}

func (f *fakeStore) CreateMessage(ctx context.Context, input store.MessageInput) (*store.Message, error) {
	return &store.Message{
		ID:         1,
		DeviceID:   input.DeviceID,
		Author:     input.Author,
		AuthorName: input.AuthorName,
		Content:    input.Content,
	}, nil
}

func (f *fakeStore) ListMessages(ctx context.Context, deviceID string, limit int) ([]store.Message, error) {
	return []store.Message{}, nil
}

func (f *fakeStore) ListUndeliveredMessages(ctx context.Context, deviceID string, limit int) ([]store.Message, error) {
	return []store.Message{}, nil
}

func (f *fakeStore) AckMessages(ctx context.Context, ids []int64) (int64, error) {
	return int64(len(ids)), nil
}

func (f *fakeStore) Ping(ctx context.Context) error {
	return nil
}

func testHandler(t *testing.T, fake *fakeStore) *Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return New(fake, Options{
		PushToken:       "secret-token",
		SignatureMaxAge: 0,
		APIToken:        "api-token",
		MaxBodyBytes:    1 << 20,
	}, logger)
}

func TestPushAcceptsValidSignature(t *testing.T) {
	fake := &fakeStore{inserted: true}
	handler := testHandler(t, fake)
	payload := samplePropertyPayload()
	request := httptest.NewRequest(http.MethodPost, "/data/push", bytes.NewReader(payload))
	signRequest(request, "secret-token", "1675654743514", "nonce-1")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if fake.ingested == nil || fake.ingested.DeviceID != "device-1" {
		t.Fatalf("event was not ingested: %+v", fake.ingested)
	}
}

func TestPushRejectsBadSignature(t *testing.T) {
	fake := &fakeStore{inserted: true}
	handler := testHandler(t, fake)
	request := httptest.NewRequest(http.MethodPost, "/data/push", bytes.NewReader(samplePropertyPayload()))
	request.Header.Set("timestamp", "1675654743514")
	request.Header.Set("nonce", "nonce-1")
	request.Header.Set("signature", "deadbeef")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	if fake.ingested != nil {
		t.Fatal("event must not be ingested with a bad signature")
	}
}

func TestPushRejectsMissingHeaders(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(http.MethodPost, "/data/push", bytes.NewReader(samplePropertyPayload()))
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestPushRejectsInvalidPayload(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(http.MethodPost, "/data/push", strings.NewReader(`{"event":"report"}`))
	signRequest(request, "secret-token", "1675654743514", "nonce-1")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAPIAuth(t *testing.T) {
	fake := &fakeStore{
		devices: []store.Device{{
			DeviceID:    "device-1",
			FirstSeenAt: time.Now().UTC(),
			LastSeenAt:  time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}},
		total: 1,
	}
	handler := testHandler(t, fake)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	request.Header.Set("Authorization", "Bearer api-token")
	recorder = httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data       []store.Device `json:"data"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 || response.Pagination.Total != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestHealthEndpointDoesNotRequireAuth(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestSeriesValidatesBucket(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/devices/device-1/series?service_id=Environment&property=light&bucket=year", nil)
	request.Header.Set("Authorization", "Bearer api-token")
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCORSPreflight(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	handler := New(&fakeStore{}, Options{
		APIToken:         "api-token",
		CORSAllowOrigins: []string{"http://localhost:5173"},
	}, logger)

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("unexpected allow-origin: %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
		t.Fatalf("unexpected allow-headers: %q", got)
	}
}

func TestAIReportFallsBackWithoutClient(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/ai/report",
		strings.NewReader(`{"device_id":"device-1","hours":24}`),
	)
	request.Header.Set("Authorization", "Bearer api-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			DeviceID string `json:"device_id"`
			Source   string `json:"source"`
			Model    string `json:"model"`
			Level    string `json:"level"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.DeviceID != "device-1" {
		t.Fatalf("unexpected device: %+v", payload.Data)
	}
	if payload.Data.Source != "local" || payload.Data.Model != "rule-based" {
		t.Fatalf("expected local fallback, got %+v", payload.Data)
	}
	if payload.Data.Level == "" {
		t.Fatalf("level must be set: %+v", payload.Data)
	}
}

func TestUploadFrameStoresMultipartImage(t *testing.T) {
	handler := testHandler(t, &fakeStore{})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="image"; filename="annotated.jpg"`},
		"Content-Type":        {"image/jpeg"},
	})
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write([]byte{0xff, 0xd8, 0xff, 0xd9}); err != nil {
		t.Fatalf("write image: %v", err)
	}
	for key, value := range map[string]string{
		"device_id":       "device-1",
		"status":          "warning",
		"summary":         "轻微偏移",
		"person_detected": "true",
		"metadata":        `{"keypoints_count":33}`,
	} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/frames", &body)
	request.Header.Set("Authorization", "Bearer api-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data store.PostureFrame `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.DeviceID != "device-1" {
		t.Fatalf("unexpected frame: %+v", payload.Data)
	}
	if payload.Data.ImageURL != "/api/v1/frames/1/image" {
		t.Fatalf("unexpected image URL: %q", payload.Data.ImageURL)
	}
}

func TestDemoScenarioDisabled(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/demo/scenario",
		strings.NewReader(`{"device_id":"device-1","status":"bad"}`),
	)
	request.Header.Set("Authorization", "Bearer api-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateMessage(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/messages",
		strings.NewReader(`{"device_id":"device-1","author":"parent","content":"加油，坐直一点"}`),
	)
	request.Header.Set("Authorization", "Bearer api-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data store.Message `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.DeviceID != "device-1" || payload.Data.Author != "parent" {
		t.Fatalf("unexpected message: %+v", payload.Data)
	}
	if payload.Data.AuthorName != "家长" {
		t.Fatalf("expected default author name, got %q", payload.Data.AuthorName)
	}
}

func TestCreateMessageRejectsBlankContent(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/messages",
		strings.NewReader(`{"device_id":"device-1","author":"child","content":"   "}`),
	)
	request.Header.Set("Authorization", "Bearer api-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestAckMessages(t *testing.T) {
	handler := testHandler(t, &fakeStore{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/device/messages/ack",
		strings.NewReader(`{"ids":[1,2]}`),
	)
	request.Header.Set("Authorization", "Bearer api-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func samplePropertyPayload() []byte {
	return []byte(`{
		"resource": "device.property",
		"event": "report",
		"event_time": "20261102T090000Z",
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
					"properties": {"temperature": 0, "humidity": 0, "light": 62, "sound": 0}
				}]
			}
		}
	}`)
}

func signRequest(request *http.Request, token, timestamp, nonce string) {
	request.Header.Set("timestamp", timestamp)
	request.Header.Set("nonce", nonce)
	request.Header.Set("signature", sign(token, timestamp, nonce))
	request.Header.Set("Content-Type", "application/json")
}

func sign(token, timestamp, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	digest := sha256.Sum256([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(digest[:])
}
