-- SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE IF NOT EXISTS posture_frames (
    id              bigserial PRIMARY KEY,
    device_id       text NOT NULL,
    captured_at     timestamptz NOT NULL,
    status          text,
    summary         text,
    person_detected integer NOT NULL DEFAULT 0,
    request_id      text,
    image_mime      text NOT NULL DEFAULT 'image/jpeg',
    image_bytes     bytea NOT NULL,
    metadata        jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_posture_frames_device_time
    ON posture_frames (device_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_posture_frames_created_at
    ON posture_frames (created_at DESC);
