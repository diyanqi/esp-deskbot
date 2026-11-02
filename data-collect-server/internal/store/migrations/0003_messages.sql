-- SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE IF NOT EXISTS messages (
    id           bigserial PRIMARY KEY,
    device_id    text NOT NULL,
    author       text NOT NULL CHECK (author IN ('parent', 'child')),
    author_name  text NOT NULL,
    content      text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    read_at      timestamptz
);

CREATE INDEX IF NOT EXISTS idx_messages_device_created
    ON messages (device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_device_undelivered
    ON messages (device_id, created_at ASC)
    WHERE delivered_at IS NULL;
