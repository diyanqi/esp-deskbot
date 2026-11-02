package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"data-collect-server/internal/store"
)

const maxFrameBytes = 10 << 20

func (h *Handler) handleUploadFrame(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFrameBytes)
	if err := r.ParseMultipartForm(maxFrameBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "frame is too large")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "image file is required")
		return
	}
	defer file.Close()

	image, err := io.ReadAll(io.LimitReader(file, maxFrameBytes+1))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "cannot read image")
		return
	}
	if len(image) == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "image is empty")
		return
	}
	if len(image) > maxFrameBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "frame is too large")
		return
	}

	deviceID := strings.TrimSpace(r.FormValue("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}

	capturedAt := time.Now().UTC()
	if value := strings.TrimSpace(r.FormValue("captured_at")); value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			capturedAt = parsed.UTC()
		} else if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			capturedAt = parsed.UTC()
		} else {
			writeError(w, r, http.StatusBadRequest, "invalid_argument", "captured_at must be RFC3339")
			return
		}
	}

	personDetected := false
	if value := strings.TrimSpace(r.FormValue("person_detected")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			if value == "1" {
				parsed = true
			} else if value == "0" {
				parsed = false
			} else {
				writeError(w, r, http.StatusBadRequest, "invalid_argument", "person_detected must be a boolean")
				return
			}
		}
		personDetected = parsed
	}

	var metadata []byte
	if value := strings.TrimSpace(r.FormValue("metadata")); value != "" {
		if !json.Valid([]byte(value)) {
			writeError(w, r, http.StatusBadRequest, "invalid_argument", "metadata must be valid JSON")
			return
		}
		metadata = []byte(value)
	}

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = http.DetectContentType(image)
	}
	if !strings.HasPrefix(mime, "image/") {
		writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "image content type is required")
		return
	}

	frame, err := h.store.SaveFrame(r.Context(), store.PostureFrameInput{
		DeviceID:       deviceID,
		CapturedAt:     capturedAt,
		Status:         strings.TrimSpace(r.FormValue("status")),
		Summary:        strings.TrimSpace(r.FormValue("summary")),
		PersonDetected: personDetected,
		RequestID:      strings.TrimSpace(r.FormValue("request_id")),
		ImageMIME:      mime,
		Image:          image,
		Metadata:       metadata,
	})
	if err != nil {
		h.logger.Error("save posture frame", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to store frame")
		return
	}
	frame.ImageURL = frameImageURL(frame.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"data": frame})
}

func (h *Handler) handleLatestFrame(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	frame, err := h.store.LatestFrame(r.Context(), deviceID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "no posture frame available")
		return
	}
	if err != nil {
		h.logger.Error("latest posture frame", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to load frame")
		return
	}
	frame.ImageURL = frameImageURL(frame.ID)
	writeJSON(w, http.StatusOK, map[string]any{"data": frame})
}

func (h *Handler) handleListFrames(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	limit := 12
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, r, http.StatusBadRequest, "invalid_argument", "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	frames, err := h.store.ListFrames(r.Context(), deviceID, limit)
	if err != nil {
		h.logger.Error("list posture frames", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to list frames")
		return
	}
	for index := range frames {
		frames[index].ImageURL = frameImageURL(frames[index].ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": frames})
}

func (h *Handler) handleFrameImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("frame_id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "invalid frame id")
		return
	}
	mime, data, err := h.store.GetFrameImage(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "frame not found")
		return
	}
	if err != nil {
		h.logger.Error("get frame image", "frame_id", id, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to load frame image")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, max-age=30")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func frameImageURL(id int64) string {
	return fmt.Sprintf("/api/v1/frames/%d/image", id)
}
