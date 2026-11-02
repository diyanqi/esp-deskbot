package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"data-collect-server/internal/iotda"
)

func (h *Handler) handlePush(w http.ResponseWriter, r *http.Request) {
	if err := h.verifyPushSignature(r); err != nil {
		h.logger.Warn("rejected IoTDA push",
			"request_id", requestIDFrom(r.Context()),
			"remote", r.RemoteAddr,
			"error", err,
		)
		writeError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "cannot read request body")
		return
	}
	if len(body) == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_argument", "empty request body")
		return
	}

	event, err := iotda.Parse(body, time.Now().UTC())
	if err != nil {
		h.logger.Warn("invalid IoTDA push payload",
			"request_id", requestIDFrom(r.Context()),
			"error", err,
		)
		writeError(w, r, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}

	inserted, err := h.store.Ingest(r.Context(), event)
	if err != nil {
		h.logger.Error("store IoTDA push",
			"request_id", requestIDFrom(r.Context()),
			"resource", event.Resource,
			"device_id", event.DeviceID,
			"error", err,
		)
		writeError(w, r, http.StatusInternalServerError, "internal", "failed to persist push notification")
		return
	}

	h.logger.Info("accepted IoTDA push",
		"request_id", requestIDFrom(r.Context()),
		"resource", event.Resource,
		"event", event.Event,
		"device_id", event.DeviceID,
		"properties", len(event.Properties),
		"status", statusValue(event),
		"inserted", inserted,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"inserted": inserted,
	})
}

func (h *Handler) verifyPushSignature(r *http.Request) error {
	if h.pushToken == "" {
		if h.devAllowUnsigned {
			return nil
		}
		return errors.New("push authentication is not configured")
	}

	timestamp := r.Header.Get("timestamp")
	nonce := r.Header.Get("nonce")
	signature := r.Header.Get("signature")
	if timestamp == "" || nonce == "" || signature == "" {
		return errors.New("missing timestamp, nonce or signature header")
	}
	if !iotda.VerifySignature(h.pushToken, timestamp, nonce, signature) {
		return errors.New("signature mismatch")
	}
	if h.signatureMaxAge > 0 {
		receivedAt, ok := iotda.TimestampMillis(timestamp)
		if !ok {
			return errors.New("timestamp header is invalid")
		}
		delta := time.Since(receivedAt)
		if delta < 0 {
			delta = -delta
		}
		if delta > h.signatureMaxAge {
			return errors.New("timestamp is outside the accepted window")
		}
	}
	return nil
}

func statusValue(event *iotda.Event) string {
	if event.Status == nil {
		return ""
	}
	return event.Status.Status
}
