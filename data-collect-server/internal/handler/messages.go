package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"data-collect-server/internal/store"
)

type createMessageRequest struct {
	DeviceID   string `json:"device_id"`
	Author     string `json:"author"`
	AuthorName string `json:"author_name"`
	Content    string `json:"content"`
}

type ackMessagesRequest struct {
	IDs []int64 `json:"ids"`
}

func (h *Handler) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	var request createMessageRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "invalid JSON body")
		return
	}
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	request.Author = strings.TrimSpace(request.Author)
	request.AuthorName = strings.TrimSpace(request.AuthorName)
	request.Content = strings.TrimSpace(request.Content)

	if request.DeviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	if request.Author != "parent" && request.Author != "child" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "author must be parent or child")
		return
	}
	if request.Content == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "content is required")
		return
	}
	if utf8.RuneCountInString(request.Content) > 500 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "content must be at most 500 characters")
		return
	}
	if request.AuthorName == "" {
		if request.Author == "parent" {
			request.AuthorName = "家长"
		} else {
			request.AuthorName = "孩子"
		}
	}
	if utf8.RuneCountInString(request.AuthorName) > 40 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "author_name is too long")
		return
	}

	message, err := h.store.CreateMessage(r.Context(), store.MessageInput{
		DeviceID:   request.DeviceID,
		Author:     request.Author,
		AuthorName: request.AuthorName,
		Content:    request.Content,
	})
	if err != nil {
		h.logger.Error("create message", "device_id", request.DeviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to create message")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": message})
}

func (h *Handler) handleListMessages(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	limit := 50
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, r, http.StatusBadRequest, "invalid_argument", "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	messages, err := h.store.ListMessages(r.Context(), deviceID, limit)
	if err != nil {
		h.logger.Error("list messages", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to list messages")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": messages})
}

func (h *Handler) handleDeviceMessages(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "device_id is required")
		return
	}
	limit := 20
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, r, http.StatusBadRequest, "invalid_argument", "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	messages, err := h.store.ListUndeliveredMessages(r.Context(), deviceID, limit)
	if err != nil {
		h.logger.Error("list device messages", "device_id", deviceID, "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to list device messages")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": messages})
}

func (h *Handler) handleAckMessages(w http.ResponseWriter, r *http.Request) {
	var request ackMessagesRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "invalid JSON body")
		return
	}
	if len(request.IDs) == 0 || len(request.IDs) > 50 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "ids must contain 1-50 message ids")
		return
	}
	ids := make([]int64, 0, len(request.IDs))
	for _, id := range request.IDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "ids must contain positive message ids")
		return
	}
	updated, err := h.store.AckMessages(r.Context(), ids)
	if err != nil {
		h.logger.Error("ack messages", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to acknowledge messages")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated})
}
