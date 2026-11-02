// Package handler exposes the IoTDA push callback and read-only query API.
package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"data-collect-server/internal/ai"
	"data-collect-server/internal/iotda"
	"data-collect-server/internal/store"
)

// Store is the persistence interface used by the HTTP layer.
type Store interface {
	Ingest(ctx context.Context, event *iotda.Event) (bool, error)
	ListDevices(ctx context.Context, filter store.DeviceFilter) ([]store.Device, bool, int64, error)
	GetDevice(ctx context.Context, deviceID string) (*store.Device, error)
	LatestTelemetry(ctx context.Context, deviceID, serviceID, property string) ([]store.TelemetryRow, error)
	QueryTelemetry(ctx context.Context, filter store.TelemetryFilter) ([]store.TelemetryRow, bool, error)
	QueryEvents(ctx context.Context, filter store.EventFilter) ([]store.EventRow, bool, error)
	QuerySeries(ctx context.Context, filter store.SeriesFilter) ([]store.SeriesPoint, error)
	GetInsights(ctx context.Context, deviceID string, window store.InsightsRange) (*store.Insights, error)
	SaveFrame(ctx context.Context, input store.PostureFrameInput) (*store.PostureFrame, error)
	LatestFrame(ctx context.Context, deviceID string) (*store.PostureFrame, error)
	ListFrames(ctx context.Context, deviceID string, limit int) ([]store.PostureFrame, error)
	GetFrameImage(ctx context.Context, id int64) (string, []byte, error)
	CreateMessage(ctx context.Context, input store.MessageInput) (*store.Message, error)
	ListMessages(ctx context.Context, deviceID string, limit int) ([]store.Message, error)
	ListUndeliveredMessages(ctx context.Context, deviceID string, limit int) ([]store.Message, error)
	AckMessages(ctx context.Context, ids []int64) (int64, error)
	Ping(ctx context.Context) error
}

// Options configures the HTTP handler.
type Options struct {
	PushToken        string
	SignatureMaxAge  time.Duration
	DevAllowUnsigned bool
	APIToken         string
	CORSAllowOrigins []string
	MaxBodyBytes     int64
	AIClient         *ai.Client
	DemoEnabled      bool
}

// Handler implements all HTTP endpoints.
type Handler struct {
	store            Store
	logger           *slog.Logger
	pushToken        string
	signatureMaxAge  time.Duration
	devAllowUnsigned bool
	apiToken         string
	corsOrigins      []string
	maxBodyBytes     int64
	aiClient         *ai.Client
	demoEnabled      bool
}

// New creates a Handler.
func New(store Store, options Options, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if options.MaxBodyBytes <= 0 {
		options.MaxBodyBytes = 5 << 20
	}
	return &Handler{
		store:            store,
		logger:           logger,
		pushToken:        options.PushToken,
		signatureMaxAge:  options.SignatureMaxAge,
		devAllowUnsigned: options.DevAllowUnsigned,
		apiToken:         options.APIToken,
		corsOrigins:      options.CORSAllowOrigins,
		maxBodyBytes:     options.MaxBodyBytes,
		aiClient:         options.AIClient,
		demoEnabled:      options.DemoEnabled,
	}
}

// Routes builds the complete HTTP router.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /data/push", http.HandlerFunc(h.handlePush))
	mux.Handle("GET /healthz", http.HandlerFunc(h.handleHealth))
	mux.Handle("GET /readyz", http.HandlerFunc(h.handleReady))

	mux.Handle("GET /api/v1/devices", h.apiAuth(http.HandlerFunc(h.handleListDevices)))
	mux.Handle("GET /api/v1/devices/{device_id}", h.apiAuth(http.HandlerFunc(h.handleGetDevice)))
	mux.Handle("GET /api/v1/devices/{device_id}/latest", h.apiAuth(http.HandlerFunc(h.handleLatestTelemetry)))
	mux.Handle("GET /api/v1/devices/{device_id}/telemetry", h.apiAuth(http.HandlerFunc(h.handleTelemetry)))
	mux.Handle("GET /api/v1/devices/{device_id}/series", h.apiAuth(http.HandlerFunc(h.handleSeries)))
	mux.Handle("GET /api/v1/telemetry", h.apiAuth(http.HandlerFunc(h.handleTelemetry)))
	mux.Handle("GET /api/v1/events", h.apiAuth(http.HandlerFunc(h.handleEvents)))
	mux.Handle("GET /api/v1/insights", h.apiAuth(http.HandlerFunc(h.handleInsights)))
	mux.Handle("POST /api/v1/frames", h.apiAuth(http.HandlerFunc(h.handleUploadFrame)))
	mux.Handle("GET /api/v1/frames", h.apiAuth(http.HandlerFunc(h.handleListFrames)))
	mux.Handle("GET /api/v1/frames/latest", h.apiAuth(http.HandlerFunc(h.handleLatestFrame)))
	mux.Handle("GET /api/v1/frames/{frame_id}/image", h.apiAuth(http.HandlerFunc(h.handleFrameImage)))
	mux.Handle("POST /api/v1/ai/report", h.apiAuth(http.HandlerFunc(h.handleAIReport)))
	mux.Handle("POST /api/v1/demo/scenario", h.apiAuth(http.HandlerFunc(h.handleDemoScenario)))
	mux.Handle("POST /api/v1/messages", h.apiAuth(http.HandlerFunc(h.handleCreateMessage)))
	mux.Handle("GET /api/v1/messages", h.apiAuth(http.HandlerFunc(h.handleListMessages)))
	mux.Handle("GET /api/v1/device/messages", h.apiAuth(http.HandlerFunc(h.handleDeviceMessages)))
	mux.Handle("POST /api/v1/device/messages/ack", h.apiAuth(http.HandlerFunc(h.handleAckMessages)))

	return h.recoverPanics(h.requestID(h.logRequests(h.cors(mux))))
}

func (h *Handler) cors(next http.Handler) http.Handler {
	if len(h.corsOrigins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		wildcard := false
		for _, candidate := range h.corsOrigins {
			if candidate == "*" {
				wildcard = true
				allowed = true
				break
			}
			if strings.EqualFold(candidate, origin) {
				allowed = true
				break
			}
		}
		if origin != "" && allowed {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers",
				"Authorization, Content-Type, X-Request-ID, timestamp, nonce, signature")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) apiAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.apiToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) ||
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, prefix)), []byte(h.apiToken)) != 1 {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing or invalid API token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.store.Ping(ctx); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "unavailable", "database is not reachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{
		Code:      code,
		Message:   message,
		RequestID: requestIDFrom(r.Context()),
	}})
}

type requestIDKey struct{}

func requestIDFrom(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDKey{}).(string); ok {
		return value
	}
	return ""
}

func (h *Handler) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buffer[:])
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(payload []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(payload)
	w.bytes += int64(written)
	return written, err
}

func (h *Handler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		if writer.status == 0 {
			writer.status = http.StatusOK
		}
		level := slog.LevelInfo
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			level = slog.LevelDebug
		}
		h.logger.Log(r.Context(), level, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", writer.status,
			"bytes", writer.bytes,
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", requestIDFrom(r.Context()),
			"remote", r.RemoteAddr,
		)
	})
}

func (h *Handler) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				h.logger.Error("panic serving request",
					"path", r.URL.Path,
					"request_id", requestIDFrom(r.Context()),
					"error", recovered,
				)
				writeError(w, r, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func parseLimit(r *http.Request, fallback, max int) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if parsed > max {
		parsed = max
	}
	return parsed, nil
}

func parseOffset(r *http.Request) (int, error) {
	value := r.URL.Query().Get("offset")
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, errors.New("offset must be a non-negative integer")
	}
	return parsed, nil
}

func parseTimeRange(r *http.Request) (from, to *time.Time, err error) {
	values := map[string]**time.Time{"from": &from, "to": &to}
	for name, target := range values {
		value := r.URL.Query().Get(name)
		if value == "" {
			continue
		}
		parsed, ok := parseQueryTime(value)
		if !ok {
			return nil, nil, errors.New(name + " must be RFC3339 or YYYY-MM-DD")
		}
		*target = &parsed
	}
	if from != nil && to != nil && !from.Before(*to) {
		return nil, nil, errors.New("from must be before to")
	}
	return from, to, nil
}

func parseQueryTime(value string) (time.Time, bool) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC(), true
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.UTC(), true
	}
	return iotda.ParseTime(value)
}
