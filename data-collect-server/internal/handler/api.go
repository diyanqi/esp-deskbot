package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"data-collect-server/internal/store"
)

type pagination struct {
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"has_more"`
	Total   *int64 `json:"total,omitempty"`
}

type listResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination pagination `json:"pagination"`
}

func (h *Handler) handleListDevices(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r, 100, 1000)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	offset, err := parseOffset(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}

	devices, hasMore, total, err := h.store.ListDevices(r.Context(), store.DeviceFilter{
		Search: strings.TrimSpace(r.URL.Query().Get("search")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.logger.Error("list devices", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to list devices")
		return
	}
	if devices == nil {
		devices = []store.Device{}
	}
	writeJSON(w, http.StatusOK, listResponse[store.Device]{
		Data: devices,
		Pagination: pagination{
			Limit:   limit,
			Offset:  offset,
			HasMore: hasMore,
			Total:   &total,
		},
	})
}

func (h *Handler) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.PathValue("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	device, err := h.store.GetDevice(r.Context(), deviceID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "device not found")
		return
	}
	if err != nil {
		h.logger.Error("get device", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to load device")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": device})
}

func (h *Handler) handleLatestTelemetry(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.PathValue("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	serviceID := strings.TrimSpace(r.URL.Query().Get("service_id"))
	property := strings.TrimSpace(r.URL.Query().Get("property"))

	items, err := h.store.LatestTelemetry(r.Context(), deviceID, serviceID, property)
	if err != nil {
		h.logger.Error("latest telemetry", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to load latest telemetry")
		return
	}
	if items == nil {
		items = []store.TelemetryRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id": deviceID,
		"data":      items,
	})
}

func (h *Handler) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.PathValue("device_id"))
	if deviceID == "" {
		deviceID = strings.TrimSpace(r.URL.Query().Get("device_id"))
	}
	limit, err := parseLimit(r, 100, 1000)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	offset, err := parseOffset(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}

	items, hasMore, err := h.store.QueryTelemetry(r.Context(), store.TelemetryFilter{
		DeviceID:  deviceID,
		ServiceID: strings.TrimSpace(r.URL.Query().Get("service_id")),
		Property:  strings.TrimSpace(r.URL.Query().Get("property")),
		From:      from,
		To:        to,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		h.logger.Error("query telemetry", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to query telemetry")
		return
	}
	if items == nil {
		items = []store.TelemetryRow{}
	}
	writeJSON(w, http.StatusOK, listResponse[store.TelemetryRow]{
		Data:       items,
		Pagination: pagination{Limit: limit, Offset: offset, HasMore: hasMore},
	})
}

func (h *Handler) handleSeries(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.PathValue("device_id"))
	serviceID := strings.TrimSpace(r.URL.Query().Get("service_id"))
	property := strings.TrimSpace(r.URL.Query().Get("property"))
	if deviceID == "" || serviceID == "" || property == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id, service_id and property are required")
		return
	}

	bucket := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("bucket")))
	if bucket == "" {
		bucket = "hour"
	}
	switch bucket {
	case "minute", "hour", "day", "week":
	default:
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "bucket must be minute, hour, day or week")
		return
	}

	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	if parsed, ok := parseQueryTime(r.URL.Query().Get("to")); ok {
		to = parsed
	} else if value := r.URL.Query().Get("to"); value != "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "to must be RFC3339 or YYYY-MM-DD")
		return
	}
	if parsed, ok := parseQueryTime(r.URL.Query().Get("from")); ok {
		from = parsed
	} else if value := r.URL.Query().Get("from"); value != "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "from must be RFC3339 or YYYY-MM-DD")
		return
	}
	if !from.Before(to) {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "from must be before to")
		return
	}

	points, err := h.store.QuerySeries(r.Context(), store.SeriesFilter{
		DeviceID:  deviceID,
		ServiceID: serviceID,
		Property:  property,
		From:      from,
		To:        to,
		Bucket:    bucket,
	})
	if err != nil {
		h.logger.Error("query series", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to query series")
		return
	}
	if points == nil {
		points = []store.SeriesPoint{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":  deviceID,
		"service_id": serviceID,
		"property":   property,
		"bucket":     bucket,
		"from":       from,
		"to":         to,
		"data":       points,
	})
}

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r, 100, 1000)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	offset, err := parseOffset(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	includeRaw, err := parseBoolQuery(r.URL.Query().Get("include_raw"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "include_raw must be true or false")
		return
	}

	events, hasMore, err := h.store.QueryEvents(r.Context(), store.EventFilter{
		DeviceID:   strings.TrimSpace(r.URL.Query().Get("device_id")),
		Resource:   strings.TrimSpace(r.URL.Query().Get("resource")),
		Event:      strings.TrimSpace(r.URL.Query().Get("event")),
		From:       from,
		To:         to,
		Limit:      limit,
		Offset:     offset,
		IncludeRaw: includeRaw,
	})
	if err != nil {
		h.logger.Error("query events", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to query events")
		return
	}
	if events == nil {
		events = []store.EventRow{}
	}
	writeJSON(w, http.StatusOK, listResponse[store.EventRow]{
		Data:       events,
		Pagination: pagination{Limit: limit, Offset: offset, HasMore: hasMore},
	})
}

func parseBoolQuery(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}
