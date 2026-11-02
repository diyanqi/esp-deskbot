-- SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE IF NOT EXISTS devices (
    device_id          text PRIMARY KEY,
    node_id            text,
    product_id         text,
    app_id             text,
    gateway_id         text,
    status             text,
    status_updated_at  timestamptz,
    last_online_at     timestamptz,
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at       timestamptz NOT NULL DEFAULT now(),
    tags               jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices (last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_devices_product_id ON devices (product_id);
CREATE INDEX IF NOT EXISTS idx_devices_status ON devices (status);

CREATE TABLE IF NOT EXISTS iotda_events (
    id            bigserial PRIMARY KEY,
    dedupe_key    text NOT NULL UNIQUE,
    resource      text NOT NULL,
    event         text,
    event_time    timestamptz,
    event_time_ms timestamptz,
    request_id    text,
    device_id     text,
    node_id       text,
    product_id    text,
    app_id        text,
    gateway_id    text,
    tags          jsonb,
    body          jsonb NOT NULL,
    raw_payload   jsonb NOT NULL,
    received_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_iotda_events_device_time
    ON iotda_events (device_id, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_iotda_events_resource_time
    ON iotda_events (resource, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_iotda_events_received_at
    ON iotda_events (received_at DESC);

CREATE TABLE IF NOT EXISTS telemetry (
    id           bigserial PRIMARY KEY,
    device_id    text NOT NULL REFERENCES devices (device_id) ON DELETE CASCADE,
    service_id   text NOT NULL,
    property     text NOT NULL,
    value_json   jsonb NOT NULL,
    value_type   text NOT NULL CHECK (value_type IN
                     ('number', 'string', 'boolean', 'object', 'array', 'null', 'unknown')),
    value_num    double precision,
    event_time   timestamptz NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now(),
    request_id   text,
    raw_event_id bigint REFERENCES iotda_events (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_telemetry_device_prop_time
    ON telemetry (device_id, service_id, property, event_time DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_event_time
    ON telemetry (event_time DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_raw_event_id
    ON telemetry (raw_event_id);
