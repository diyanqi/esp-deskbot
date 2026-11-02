package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"data-collect-server/internal/iotda"
)

type demoScenarioRequest struct {
	DeviceID string  `json:"device_id"`
	Status   string  `json:"status"`
	Light    float64 `json:"light"`
	Sound    float64 `json:"sound"`
}

type demoScenario struct {
	postureStatus string
	summary       string
	issueCount    int
	alert         int
	person        int
}

func (h *Handler) handleDemoScenario(w http.ResponseWriter, r *http.Request) {
	if !h.demoEnabled {
		writeError(w, r, http.StatusForbidden, "demo_disabled", "demo mode is disabled on this server")
		return
	}
	var request demoScenarioRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "invalid JSON body")
		return
	}
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	if request.DeviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}

	scenario, ok := demoScenarios[strings.ToLower(strings.TrimSpace(request.Status))]
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "status must be good, warning, bad, or away")
		return
	}
	if request.Light <= 0 {
		request.Light = 60
	}
	if request.Sound < 0 {
		request.Sound = 0
	}
	now := time.Now().UTC()
	requestID := fmt.Sprintf("demo-%d", now.UnixNano())

	envelope := map[string]any{
		"resource":   iotda.ResourceDeviceProperty,
		"event":      "report",
		"event_time": now.Format("20060102T150405Z"),
		"request_id": requestID,
		"notify_data": map[string]any{
			"header": map[string]any{"device_id": request.DeviceID},
			"body": map[string]any{
				"services": []any{
					map[string]any{
						"service_id": "Environment",
						"properties": map[string]any{
							"temperature": 0,
							"humidity":    0,
							"light":       request.Light,
							"sound":       request.Sound,
						},
					},
					map[string]any{
						"service_id": "Posture",
						"properties": map[string]any{
							"personDetected": scenario.person,
							"postureStatus":  scenario.postureStatus,
							"postureSummary": scenario.summary,
							"issueCount":     scenario.issueCount,
							"alert":          scenario.alert,
							"detecting":      0,
							"lastSeq":        now.Unix(),
						},
					},
					map[string]any{
						"service_id": "DeviceStatus",
						"properties": map[string]any{
							"wifiConnected":  1,
							"rssi":           -55,
							"uptime":         0,
							"freeHeap":       180000,
							"reportInterval": 5,
							"firmware":       "demo",
							"lastReport":     now.Unix(),
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to build demo event")
		return
	}
	event, err := iotda.Parse(raw, now)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to parse demo event")
		return
	}
	inserted, err := h.store.Ingest(r.Context(), event)
	if err != nil {
		h.logger.Error("ingest demo scenario", "device_id", request.DeviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to ingest demo scenario")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"inserted":   inserted,
			"device_id":  request.DeviceID,
			"status":     scenario.postureStatus,
			"summary":    scenario.summary,
			"request_id": requestID,
			"event_time": now,
		},
	})
}

var demoScenarios = map[string]demoScenario{
	"good": {
		postureStatus: "good",
		summary:       "坐姿良好，继续保持",
		issueCount:    0,
		alert:         0,
		person:        1,
	},
	"warning": {
		postureStatus: "warning",
		summary:       "轻微偏移，注意坐直",
		issueCount:    1,
		alert:         1,
		person:        1,
	},
	"bad": {
		postureStatus: "bad",
		summary:       "坐姿告警：请调整坐姿",
		issueCount:    2,
		alert:         1,
		person:        1,
	},
	"away": {
		postureStatus: "unknown",
		summary:       "暂时离开书桌",
		issueCount:    0,
		alert:         0,
		person:        0,
	},
}
