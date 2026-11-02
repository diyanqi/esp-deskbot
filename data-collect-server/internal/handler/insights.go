package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"data-collect-server/internal/store"
)

func (h *Handler) handleInsights(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	hours := 24
	if value := strings.TrimSpace(r.URL.Query().Get("hours")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 168 {
			writeError(w, r, http.StatusBadRequest, "invalid_argument", "hours must be between 1 and 168")
			return
		}
		hours = parsed
	}

	to := time.Now().UTC()
	from := to.Add(-time.Duration(hours) * time.Hour)
	insights, err := h.store.GetInsights(r.Context(), deviceID, store.InsightsRange{
		From: from,
		To:   to,
	})
	if err != nil {
		h.logger.Error("query insights", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to aggregate insights")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": insights})
}
