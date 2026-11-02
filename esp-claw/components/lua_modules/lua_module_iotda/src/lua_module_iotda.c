/*
 * SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Huawei Cloud IoTDA device client for Lua.
 *
 * The module owns one MQTTS connection. It builds the CONNECT credentials
 * required by IoTDA (clientId + HMAC-SHA256 password), reports properties
 * and events to the platform topics, and handles the commands defined by
 * the DeskHealthAssistant product model.
 */
#include "lua_module_iotda.h"

#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "cJSON.h"
#include "cap_lua.h"
#include "esp_app_desc.h"
#include "esp_crt_bundle.h"
#include "esp_log.h"
#include "esp_system.h"
#include "esp_timer.h"
#include "esp_wifi.h"
#include "mqtt_client.h"
#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include "lauxlib.h"
#include "mbedtls/md.h"

#define IOTDA_TAG "lua_iotda"

#define IOTDA_DEVICE_ID_LEN 128
#define IOTDA_SECRET_LEN 160
#define IOTDA_HOST_LEN 192
#define IOTDA_CLIENT_ID_LEN 256
#define IOTDA_PASSWORD_LEN 72
#define IOTDA_TOPIC_LEN 320
#define IOTDA_URI_LEN 320
#define IOTDA_ERROR_LEN 160

#define IOTDA_DEFAULT_PORT 8883
#define IOTDA_DEFAULT_KEEPALIVE 120
#define IOTDA_DEFAULT_REPORT_INTERVAL 5
#define IOTDA_MIN_REPORT_INTERVAL 5
#define IOTDA_MAX_REPORT_INTERVAL 3600

#define IOTDA_MQTT_BUFFER_SIZE 2048
#define IOTDA_MQTT_OUTBOX_LIMIT 8192
#define IOTDA_MQTT_TASK_STACK 6144
#define IOTDA_MQTT_TASK_PRIORITY 5
#define IOTDA_MQTT_RECONNECT_MS 5000
#define IOTDA_MQTT_TIMEOUT_MS 10000

typedef struct {
    bool started;
    lua_State *owner;
    esp_mqtt_client_handle_t client;
    SemaphoreHandle_t mutex;

    char device_id[IOTDA_DEVICE_ID_LEN];
    char secret[IOTDA_SECRET_LEN];
    char host[IOTDA_HOST_LEN];
    int port;
    char client_id[IOTDA_CLIENT_ID_LEN];
    char password[IOTDA_PASSWORD_LEN];
    char broker_uri[IOTDA_URI_LEN];

    char topic_properties[IOTDA_TOPIC_LEN];
    char topic_commands[IOTDA_TOPIC_LEN];
    char topic_events[IOTDA_TOPIC_LEN];
    char topic_response_prefix[IOTDA_TOPIC_LEN];

    volatile bool connected;
    volatile bool reminder_pending;
    volatile uint32_t queued_reports;
    volatile uint32_t published_reports;
    volatile uint32_t publish_errors;

    int report_interval_s;
    uint32_t last_report_ms;
    char last_error[IOTDA_ERROR_LEN];
} iotda_state_t;

static iotda_state_t s_iotda;

static void iotda_mqtt_event_handler(void *args, esp_event_base_t base,
                                     int32_t event_id, void *event_data);

static void iotda_set_error(const char *fmt, ...)
{
    char text[IOTDA_ERROR_LEN];
    va_list args;

    va_start(args, fmt);
    vsnprintf(text, sizeof(text), fmt, args);
    va_end(args);

    if (s_iotda.mutex && xSemaphoreTake(s_iotda.mutex, pdMS_TO_TICKS(100)) == pdTRUE) {
        strlcpy(s_iotda.last_error, text, sizeof(s_iotda.last_error));
        xSemaphoreGive(s_iotda.mutex);
    } else {
        strlcpy(s_iotda.last_error, text, sizeof(s_iotda.last_error));
    }
    ESP_LOGW(IOTDA_TAG, "%s", text);
}

static void iotda_copy_string(char *dst, size_t size, const char *src)
{
    if (!dst || size == 0) {
        return;
    }
    strlcpy(dst, src ? src : "", size);
}

static void iotda_format_timestamp(char out[11])
{
    time_t now = time(NULL);
    struct tm tm_utc;

    if (now > 1700000000 && gmtime_r(&now, &tm_utc) != NULL) {
        char text[32];
        int written = snprintf(text, sizeof(text), "%04d%02d%02d%02d",
                               tm_utc.tm_year + 1900, tm_utc.tm_mon + 1,
                               tm_utc.tm_mday, tm_utc.tm_hour);
        if (written == 10) {
            memcpy(out, text, 11);
            return;
        }
    }
    /* IoTDA's HMACSHA256-without-timestamp-check mode accepts a fixed
     * timestamp. Keep a valid-looking value when SNTP has not run yet. */
    strlcpy(out, "2026110114", 11);
}

static void iotda_format_datetime(char *out, size_t size)
{
    time_t now = time(NULL);
    struct tm tm_utc;

    if (now > 1700000000 && gmtime_r(&now, &tm_utc) != NULL) {
        strftime(out, size, "%Y%m%dT%H%M%SZ", &tm_utc);
    } else {
        strlcpy(out, "20261101T140000Z", size);
    }
}

static void iotda_hmac_sha256_hex(const char *key, const char *message, char out[65])
{
    unsigned char digest[32];
    const mbedtls_md_info_t *info = mbedtls_md_info_from_type(MBEDTLS_MD_SHA256);

    if (!info || mbedtls_md_hmac(info,
                                 (const unsigned char *)key, strlen(key),
                                 (const unsigned char *)message, strlen(message),
                                 digest) != 0) {
        out[0] = '\0';
        return;
    }
    for (int i = 0; i < (int)sizeof(digest); i++) {
        snprintf(out + i * 2, 3, "%02x", digest[i]);
    }
    out[64] = '\0';
}

static bool iotda_timestamp_from_client_id(const char *client_id, char out[11])
{
    const char *last;

    if (!client_id || !client_id[0]) {
        return false;
    }
    last = strrchr(client_id, '_');
    if (!last || strlen(last + 1) != 10) {
        return false;
    }
    for (int i = 1; i <= 10; i++) {
        if (last[i] < '0' || last[i] > '9') {
            return false;
        }
    }
    memcpy(out, last + 1, 10);
    out[10] = '\0';
    return true;
}

static bool iotda_prepare_credentials(void)
{
    char timestamp[11];
    char generated[11];

    iotda_format_timestamp(generated);
    timestamp[0] = '\0';

    bool has_timestamp = s_iotda.client_id[0] &&
        iotda_timestamp_from_client_id(s_iotda.client_id, timestamp);
    if (!has_timestamp) {
        snprintf(s_iotda.client_id, sizeof(s_iotda.client_id),
                 "%s_0_0_%s", s_iotda.device_id, generated);
        strlcpy(timestamp, generated, sizeof(timestamp));
    }

    /* IoTDA: Password = HMACSHA256(key = timestamp, message = secret). */
    iotda_hmac_sha256_hex(timestamp, s_iotda.secret, s_iotda.password);
    if (!s_iotda.password[0]) {
        iotda_set_error("HMAC-SHA256 password generation failed");
        return false;
    }
    return true;
}

static void iotda_table_to_json(lua_State *L, int index, cJSON *object)
{
    if (!lua_istable(L, index)) {
        return;
    }
    lua_pushnil(L);
    while (lua_next(L, index) != 0) {
        const char *key = lua_type(L, -2) == LUA_TSTRING ? lua_tostring(L, -2) : NULL;
        if (key) {
            int type = lua_type(L, -1);
            if (type == LUA_TBOOLEAN) {
                cJSON_AddBoolToObject(object, key, lua_toboolean(L, -1));
            } else if (type == LUA_TNUMBER) {
                lua_Number value = lua_tonumber(L, -1);
                if (value == (lua_Number)(long long)value) {
                    cJSON_AddNumberToObject(object, key, (double)(long long)value);
                } else {
                    cJSON_AddNumberToObject(object, key, (double)value);
                }
            } else if (type == LUA_TSTRING) {
                cJSON_AddStringToObject(object, key, lua_tostring(L, -1));
            } else if (type == LUA_TTABLE) {
                cJSON *child = cJSON_AddObjectToObject(object, key);
                if (child) {
                    iotda_table_to_json(L, lua_gettop(L), child);
                }
            } else {
                cJSON_AddNullToObject(object, key);
            }
        }
        lua_pop(L, 1);
    }
}

static void iotda_append_device_status(cJSON *services)
{
    cJSON *service = cJSON_CreateObject();
    if (!service) {
        return;
    }
    cJSON_AddStringToObject(service, "service_id", "DeviceStatus");
    cJSON *properties = cJSON_AddObjectToObject(service, "properties");
    if (!properties) {
        cJSON_Delete(service);
        return;
    }

    wifi_ap_record_t ap = {0};
    bool wifi_connected = esp_wifi_sta_get_ap_info(&ap) == ESP_OK;
    uint64_t uptime_s = esp_timer_get_time() / 1000000ULL;
    uint32_t free_heap = (uint32_t)esp_get_free_heap_size();
    const char *firmware = esp_app_get_description() ? esp_app_get_description()->version : "";
    char last_report[24];

    if (uptime_s > 2147483647ULL) {
        uptime_s = 2147483647ULL;
    }
    if (free_heap > 2147483647U) {
        free_heap = 2147483647U;
    }
    iotda_format_datetime(last_report, sizeof(last_report));

    cJSON_AddNumberToObject(properties, "wifiConnected", wifi_connected ? 1 : 0);
    cJSON_AddNumberToObject(properties, "rssi", wifi_connected ? ap.rssi : 0);
    cJSON_AddNumberToObject(properties, "uptime", (double)uptime_s);
    cJSON_AddNumberToObject(properties, "freeHeap", (double)free_heap);
    cJSON_AddNumberToObject(properties, "reportInterval", s_iotda.report_interval_s);
    cJSON_AddStringToObject(properties, "firmware", firmware);
    cJSON_AddStringToObject(properties, "lastReport", last_report);
    cJSON_AddItemToArray(services, service);
}

static int iotda_enqueue(const char *topic, char *payload, bool throttle)
{
    uint32_t now_ms;
    int interval_ms;

    if (!s_iotda.started || !s_iotda.client) {
        cJSON_free(payload);
        return -1;
    }
    if (!s_iotda.connected) {
        cJSON_free(payload);
        return -1;
    }

    now_ms = (uint32_t)(esp_timer_get_time() / 1000);
    interval_ms = (s_iotda.report_interval_s > 0 ? s_iotda.report_interval_s : 1) * 1000;
    if (throttle && s_iotda.last_report_ms != 0 &&
        (int32_t)(now_ms - s_iotda.last_report_ms) < interval_ms) {
        cJSON_free(payload);
        return 0;
    }

    int msg_id = esp_mqtt_client_enqueue(s_iotda.client, topic, payload, 0, 1, 0, false);
    if (msg_id >= 0) {
        s_iotda.queued_reports++;
        if (throttle) {
            s_iotda.last_report_ms = now_ms;
        }
    } else {
        s_iotda.publish_errors++;
        iotda_set_error("MQTT enqueue failed: %d", msg_id);
    }
    cJSON_free(payload);
    return msg_id;
}

static int lua_module_iotda_start(lua_State *L)
{
    luaL_checktype(L, 1, LUA_TTABLE);
    if (s_iotda.started) {
        lua_pushboolean(L, 1);
        return 1;
    }

    lua_getfield(L, 1, "device_id");
    const char *device_id = lua_tostring(L, -1);
    lua_getfield(L, 1, "secret");
    const char *secret = lua_tostring(L, -1);
    lua_getfield(L, 1, "host");
    const char *host = lua_tostring(L, -1);
    lua_getfield(L, 1, "client_id");
    const char *client_id = lua_tostring(L, -1);

    if (!device_id || !device_id[0] || !secret || !secret[0] || !host || !host[0]) {
        lua_pop(L, 4);
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "device_id, secret and host are required");
        return 2;
    }

    int port = IOTDA_DEFAULT_PORT;
    int keepalive = IOTDA_DEFAULT_KEEPALIVE;
    int report_interval = IOTDA_DEFAULT_REPORT_INTERVAL;
    lua_getfield(L, 1, "port");
    if (lua_isinteger(L, -1)) {
        port = (int)lua_tointeger(L, -1);
    }
    lua_pop(L, 1);
    lua_getfield(L, 1, "keepalive");
    if (lua_isinteger(L, -1)) {
        keepalive = (int)lua_tointeger(L, -1);
    }
    lua_pop(L, 1);
    lua_getfield(L, 1, "report_interval");
    if (lua_isinteger(L, -1)) {
        report_interval = (int)lua_tointeger(L, -1);
    }
    lua_pop(L, 1);
    if (report_interval < IOTDA_MIN_REPORT_INTERVAL) {
        report_interval = IOTDA_MIN_REPORT_INTERVAL;
    } else if (report_interval > IOTDA_MAX_REPORT_INTERVAL) {
        report_interval = IOTDA_MAX_REPORT_INTERVAL;
    }
    if (port <= 0 || port > 65535) {
        port = IOTDA_DEFAULT_PORT;
    }
    if (keepalive < 30) {
        keepalive = 30;
    }

    iotda_copy_string(s_iotda.device_id, sizeof(s_iotda.device_id), device_id);
    iotda_copy_string(s_iotda.secret, sizeof(s_iotda.secret), secret);
    iotda_copy_string(s_iotda.host, sizeof(s_iotda.host), host);
    iotda_copy_string(s_iotda.client_id, sizeof(s_iotda.client_id), client_id);
    s_iotda.port = port;
    s_iotda.report_interval_s = report_interval;
    lua_pop(L, 4);

    if (!iotda_prepare_credentials()) {
        lua_pushboolean(L, 0);
        lua_pushstring(L, s_iotda.last_error);
        return 2;
    }

    snprintf(s_iotda.broker_uri, sizeof(s_iotda.broker_uri), "mqtts://%s:%d",
             s_iotda.host, s_iotda.port);
    snprintf(s_iotda.topic_properties, sizeof(s_iotda.topic_properties),
             "$oc/devices/%s/sys/properties/report", s_iotda.device_id);
    snprintf(s_iotda.topic_commands, sizeof(s_iotda.topic_commands),
             "$oc/devices/%s/sys/commands/#", s_iotda.device_id);
    snprintf(s_iotda.topic_events, sizeof(s_iotda.topic_events),
             "$oc/devices/%s/sys/events/up", s_iotda.device_id);
    snprintf(s_iotda.topic_response_prefix, sizeof(s_iotda.topic_response_prefix),
             "$oc/devices/%s/sys/commands/response/request_id=", s_iotda.device_id);

    esp_mqtt_client_config_t mqtt_config = {
        .broker = {
            .address = {
                .uri = s_iotda.broker_uri,
            },
            .verification = {
                .crt_bundle_attach = esp_crt_bundle_attach,
            },
        },
        .credentials = {
            .client_id = s_iotda.client_id,
            .username = s_iotda.device_id,
            .authentication = {
                .password = s_iotda.password,
            },
        },
        .session = {
            .keepalive = keepalive,
            .protocol_ver = MQTT_PROTOCOL_V_3_1_1,
        },
        .network = {
            .reconnect_timeout_ms = IOTDA_MQTT_RECONNECT_MS,
            .timeout_ms = IOTDA_MQTT_TIMEOUT_MS,
        },
        .task = {
            .priority = IOTDA_MQTT_TASK_PRIORITY,
            .stack_size = IOTDA_MQTT_TASK_STACK,
        },
        .buffer = {
            .size = IOTDA_MQTT_BUFFER_SIZE,
            .out_size = IOTDA_MQTT_BUFFER_SIZE,
        },
        .outbox = {
            .limit = IOTDA_MQTT_OUTBOX_LIMIT,
        },
    };

    s_iotda.client = esp_mqtt_client_init(&mqtt_config);
    if (!s_iotda.client) {
        iotda_set_error("esp_mqtt_client_init failed");
        lua_pushboolean(L, 0);
        lua_pushstring(L, s_iotda.last_error);
        return 2;
    }

    s_iotda.connected = false;
    s_iotda.reminder_pending = false;
    s_iotda.queued_reports = 0;
    s_iotda.published_reports = 0;
    s_iotda.publish_errors = 0;
    s_iotda.last_report_ms = 0;
    s_iotda.last_error[0] = '\0';
    s_iotda.owner = L;
    s_iotda.started = true;

    /* The MQTT handler is installed before the client starts. */
    if (esp_mqtt_client_register_event(s_iotda.client, ESP_EVENT_ANY_ID,
                                       iotda_mqtt_event_handler, &s_iotda) != ESP_OK ||
        esp_mqtt_client_start(s_iotda.client) != ESP_OK) {
        iotda_set_error("esp_mqtt_client_start failed");
        esp_mqtt_client_destroy(s_iotda.client);
        s_iotda.client = NULL;
        s_iotda.started = false;
        s_iotda.owner = NULL;
        lua_pushboolean(L, 0);
        lua_pushstring(L, s_iotda.last_error);
        return 2;
    }

    ESP_LOGI(IOTDA_TAG, "starting device=%s host=%s:%d client_id=%s interval=%ds",
             s_iotda.device_id, s_iotda.host, s_iotda.port,
             s_iotda.client_id, s_iotda.report_interval_s);
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_module_iotda_publish(lua_State *L)
{
    luaL_checktype(L, 1, LUA_TTABLE);
    if (!s_iotda.started || !s_iotda.client) {
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "iotda is not started");
        return 2;
    }

    bool force = false;
    if (!lua_isnoneornil(L, 2)) {
        if (lua_isboolean(L, 2)) {
            force = lua_toboolean(L, 2);
        } else if (lua_istable(L, 2)) {
            lua_getfield(L, 2, "force");
            force = lua_toboolean(L, -1);
            lua_pop(L, 1);
        }
    }

    cJSON *root = cJSON_CreateObject();
    if (!root) {
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "out of memory");
        return 2;
    }
    cJSON *services = cJSON_AddArrayToObject(root, "services");
    int count = (int)lua_rawlen(L, 1);

    for (int i = 1; i <= count; i++) {
        lua_rawgeti(L, 1, i);
        if (!lua_istable(L, -1)) {
            lua_pop(L, 1);
            continue;
        }
        int element = lua_gettop(L);
        lua_getfield(L, element, "service_id");
        const char *service_id = lua_tostring(L, -1);
        lua_getfield(L, element, "properties");
        if (!service_id || !service_id[0] || !lua_istable(L, -1)) {
            lua_pop(L, 3);
            continue;
        }
        cJSON *service = cJSON_CreateObject();
        if (service) {
            cJSON_AddStringToObject(service, "service_id", service_id);
            cJSON *properties = cJSON_AddObjectToObject(service, "properties");
            if (properties) {
                iotda_table_to_json(L, element + 2, properties);
            }
            cJSON_AddItemToArray(services, service);
        }
        lua_pop(L, 3);
    }

    iotda_append_device_status(services);
    char *payload = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (!payload) {
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "payload encode failed");
        return 2;
    }

    int msg_id = iotda_enqueue(s_iotda.topic_properties, payload, !force);
    lua_pushboolean(L, msg_id > 0);
    if (msg_id == 0) {
        lua_pushliteral(L, "throttled");
        return 2;
    }
    if (msg_id < 0) {
        lua_pushstring(L, s_iotda.last_error);
        return 2;
    }
    lua_pushinteger(L, msg_id);
    return 2;
}

static int lua_module_iotda_publish_event(lua_State *L)
{
    const char *service_id = luaL_checkstring(L, 1);
    const char *event_type = luaL_checkstring(L, 2);
    if (!lua_isnoneornil(L, 3) && !lua_istable(L, 3)) {
        return luaL_error(L, "iotda.publish_event paras must be a table");
    }
    if (!s_iotda.started || !s_iotda.client) {
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "iotda is not started");
        return 2;
    }

    cJSON *root = cJSON_CreateObject();
    if (!root) {
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "out of memory");
        return 2;
    }
    cJSON *services = cJSON_AddArrayToObject(root, "services");
    cJSON *service = cJSON_CreateObject();
    if (service) {
        char event_time[24];
        iotda_format_datetime(event_time, sizeof(event_time));
        cJSON_AddStringToObject(service, "service_id", service_id);
        cJSON_AddStringToObject(service, "event_type", event_type);
        cJSON_AddStringToObject(service, "event_time", event_time);
        cJSON *paras = cJSON_AddObjectToObject(service, "paras");
        if (paras && !lua_isnoneornil(L, 3)) {
            iotda_table_to_json(L, 3, paras);
        }
        cJSON_AddItemToArray(services, service);
    }
    char *payload = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (!payload) {
        lua_pushboolean(L, 0);
        lua_pushliteral(L, "payload encode failed");
        return 2;
    }
    int msg_id = iotda_enqueue(s_iotda.topic_events, payload, false);
    lua_pushboolean(L, msg_id >= 0);
    if (msg_id < 0) {
        lua_pushstring(L, s_iotda.last_error);
        return 2;
    }
    lua_pushinteger(L, msg_id);
    return 2;
}

static int lua_module_iotda_is_connected(lua_State *L)
{
    lua_pushboolean(L, s_iotda.started && s_iotda.connected);
    return 1;
}

static int lua_module_iotda_take_reminder(lua_State *L)
{
    bool pending = s_iotda.reminder_pending;
    s_iotda.reminder_pending = false;
    lua_pushboolean(L, pending);
    return 1;
}

static int lua_module_iotda_set_report_interval(lua_State *L)
{
    lua_Integer interval = luaL_checkinteger(L, 1);
    if (interval < IOTDA_MIN_REPORT_INTERVAL) {
        interval = IOTDA_MIN_REPORT_INTERVAL;
    } else if (interval > IOTDA_MAX_REPORT_INTERVAL) {
        interval = IOTDA_MAX_REPORT_INTERVAL;
    }
    s_iotda.report_interval_s = (int)interval;
    lua_pushinteger(L, interval);
    return 1;
}

static int lua_module_iotda_get_report_interval(lua_State *L)
{
    lua_pushinteger(L, s_iotda.report_interval_s);
    return 1;
}

static int lua_module_iotda_status(lua_State *L)
{
    lua_newtable(L);
    lua_pushboolean(L, s_iotda.started);
    lua_setfield(L, -2, "started");
    lua_pushboolean(L, s_iotda.connected);
    lua_setfield(L, -2, "connected");
    lua_pushinteger(L, s_iotda.report_interval_s);
    lua_setfield(L, -2, "report_interval");
    lua_pushinteger(L, (lua_Integer)s_iotda.queued_reports);
    lua_setfield(L, -2, "queued");
    lua_pushinteger(L, (lua_Integer)s_iotda.published_reports);
    lua_setfield(L, -2, "published");
    lua_pushinteger(L, (lua_Integer)s_iotda.publish_errors);
    lua_setfield(L, -2, "errors");
    lua_pushstring(L, s_iotda.device_id);
    lua_setfield(L, -2, "device_id");
    lua_pushstring(L, s_iotda.client_id);
    lua_setfield(L, -2, "client_id");
    lua_pushstring(L, s_iotda.last_error);
    lua_setfield(L, -2, "last_error");
    return 1;
}

static int lua_module_iotda_stop(lua_State *L)
{
    if (s_iotda.client) {
        esp_mqtt_client_stop(s_iotda.client);
        esp_mqtt_client_destroy(s_iotda.client);
        s_iotda.client = NULL;
    }
    s_iotda.started = false;
    s_iotda.connected = false;
    s_iotda.owner = NULL;
    lua_pushboolean(L, 1);
    return 1;
}

static void iotda_extract_request_id(const char *topic, int topic_len,
                                     char *out, size_t out_size)
{
    static const char key[] = "request_id=";
    char buffer[IOTDA_TOPIC_LEN];
    int length;

    out[0] = '\0';
    if (!topic || topic_len <= 0 || out_size == 0) {
        return;
    }
    length = topic_len < (int)sizeof(buffer) - 1 ? topic_len : (int)sizeof(buffer) - 1;
    memcpy(buffer, topic, (size_t)length);
    buffer[length] = '\0';

    const char *start = strstr(buffer, key);
    if (start) {
        strlcpy(out, start + sizeof(key) - 1, out_size);
    }
}

static void iotda_send_command_response(const char *request_id,
                                        const char *command_name, int result)
{
    char topic[IOTDA_TOPIC_LEN];
    char response_name[96];
    char *payload;
    cJSON *response;

    if (!request_id || !request_id[0] || !command_name || !command_name[0]) {
        return;
    }
    snprintf(topic, sizeof(topic), "%s%s", s_iotda.topic_response_prefix, request_id);
    snprintf(response_name, sizeof(response_name), "%s_RSP", command_name);

    response = cJSON_CreateObject();
    if (!response) {
        return;
    }
    cJSON_AddNumberToObject(response, "result_code", 0);
    cJSON_AddStringToObject(response, "response_name", response_name);
    cJSON *paras = cJSON_AddObjectToObject(response, "paras");
    if (paras) {
        cJSON_AddNumberToObject(paras, "result", result);
    }
    payload = cJSON_PrintUnformatted(response);
    cJSON_Delete(response);
    if (payload) {
        esp_mqtt_client_enqueue(s_iotda.client, topic, payload, 0, 1, 0, false);
        cJSON_free(payload);
    }
}

static void iotda_handle_command(esp_mqtt_event_handle_t event)
{
    char payload[512];
    char request_id[96];
    int length = event->data_len;
    cJSON *root;
    cJSON *paras;
    const char *service_id;
    const char *command_name;
    int result = 1;

    if (!event->data || length <= 0) {
        return;
    }
    if (event->total_data_len > event->data_len &&
        event->current_data_offset + event->data_len < event->total_data_len) {
        ESP_LOGW(IOTDA_TAG, "fragmented command ignored");
        return;
    }
    if (length >= (int)sizeof(payload)) {
        length = (int)sizeof(payload) - 1;
    }
    memcpy(payload, event->data, (size_t)length);
    payload[length] = '\0';

    iotda_extract_request_id(event->topic, event->topic_len,
                             request_id, sizeof(request_id));
    root = cJSON_Parse(payload);
    if (!root) {
        ESP_LOGW(IOTDA_TAG, "invalid command JSON");
        return;
    }

    service_id = cJSON_GetStringValue(cJSON_GetObjectItem(root, "service_id"));
    command_name = cJSON_GetStringValue(cJSON_GetObjectItem(root, "command_name"));
    paras = cJSON_GetObjectItem(root, "paras");
    if (!service_id) {
        service_id = "";
    }
    if (!command_name) {
        command_name = "";
    }

    if (strcmp(command_name, "SET_REPORT_INTERVAL") == 0) {
        cJSON *interval = cJSON_GetObjectItem(paras, "interval");
        if (cJSON_IsNumber(interval)) {
            int value = interval->valueint;
            if (value < IOTDA_MIN_REPORT_INTERVAL) {
                value = IOTDA_MIN_REPORT_INTERVAL;
            } else if (value > IOTDA_MAX_REPORT_INTERVAL) {
                value = IOTDA_MAX_REPORT_INTERVAL;
            }
            s_iotda.report_interval_s = value;
            result = 0;
            ESP_LOGI(IOTDA_TAG, "command set report interval: %d s", value);
        }
    } else if (strcmp(command_name, "TRIGGER_REMINDER") == 0) {
        s_iotda.reminder_pending = true;
        result = 0;
        ESP_LOGI(IOTDA_TAG, "command trigger reminder");
    } else {
        ESP_LOGW(IOTDA_TAG, "unknown command: %s (service=%s)", command_name, service_id);
    }

    iotda_send_command_response(request_id, command_name, result);
    cJSON_Delete(root);
}

static void iotda_mqtt_event_handler(void *args, esp_event_base_t base,
                                     int32_t event_id, void *event_data)
{
    (void)args;
    (void)base;
    esp_mqtt_event_handle_t event = (esp_mqtt_event_handle_t)event_data;

    switch ((esp_mqtt_event_id_t)event_id) {
    case MQTT_EVENT_CONNECTED:
        s_iotda.connected = true;
        ESP_LOGI(IOTDA_TAG, "connected to IoTDA");
        esp_mqtt_client_subscribe(event->client, s_iotda.topic_commands, 1);
        break;
    case MQTT_EVENT_DISCONNECTED:
        s_iotda.connected = false;
        ESP_LOGW(IOTDA_TAG, "disconnected from IoTDA");
        break;
    case MQTT_EVENT_SUBSCRIBED:
        ESP_LOGI(IOTDA_TAG, "subscribed to command topic");
        break;
    case MQTT_EVENT_PUBLISHED:
        s_iotda.published_reports++;
        if (s_iotda.published_reports == 1 ||
            (s_iotda.published_reports % 20U) == 0U) {
            ESP_LOGI(IOTDA_TAG, "reports acknowledged: %u",
                     (unsigned)s_iotda.published_reports);
        }
        break;
    case MQTT_EVENT_DATA:
        iotda_handle_command(event);
        break;
    case MQTT_EVENT_ERROR:
        if (event->error_handle) {
            if (event->error_handle->error_type == MQTT_ERROR_TYPE_TCP_TRANSPORT) {
                iotda_set_error("TLS transport error: esp_tls=0x%x stack=0x%x cert_flags=0x%x",
                                event->error_handle->esp_tls_last_esp_err,
                                event->error_handle->esp_tls_stack_err,
                                event->error_handle->esp_tls_cert_verify_flags);
            } else if (event->error_handle->error_type == MQTT_ERROR_TYPE_CONNECTION_REFUSED) {
                iotda_set_error("MQTT connection refused: 0x%x",
                                event->error_handle->connect_return_code);
            } else {
                iotda_set_error("MQTT error type %d", event->error_handle->error_type);
            }
        } else {
            iotda_set_error("MQTT error without details");
        }
        break;
    default:
        break;
    }
}

static void iotda_exit_cleanup(lua_State *L)
{
    if (s_iotda.owner != L) {
        return;
    }
    if (s_iotda.client) {
        esp_mqtt_client_stop(s_iotda.client);
        esp_mqtt_client_destroy(s_iotda.client);
        s_iotda.client = NULL;
    }
    s_iotda.started = false;
    s_iotda.connected = false;
    s_iotda.owner = NULL;
}

int luaopen_iotda(lua_State *L)
{
    lua_newtable(L);
    lua_pushcfunction(L, lua_module_iotda_start);
    lua_setfield(L, -2, "start");
    lua_pushcfunction(L, lua_module_iotda_stop);
    lua_setfield(L, -2, "stop");
    lua_pushcfunction(L, lua_module_iotda_publish);
    lua_setfield(L, -2, "publish");
    lua_pushcfunction(L, lua_module_iotda_publish_event);
    lua_setfield(L, -2, "publish_event");
    lua_pushcfunction(L, lua_module_iotda_is_connected);
    lua_setfield(L, -2, "is_connected");
    lua_pushcfunction(L, lua_module_iotda_take_reminder);
    lua_setfield(L, -2, "take_reminder");
    lua_pushcfunction(L, lua_module_iotda_set_report_interval);
    lua_setfield(L, -2, "set_report_interval");
    lua_pushcfunction(L, lua_module_iotda_get_report_interval);
    lua_setfield(L, -2, "get_report_interval");
    lua_pushcfunction(L, lua_module_iotda_status);
    lua_setfield(L, -2, "status");
    return 1;
}

esp_err_t lua_module_iotda_register(void)
{
    esp_err_t err;

    if (!s_iotda.mutex) {
        s_iotda.mutex = xSemaphoreCreateMutex();
        if (!s_iotda.mutex) {
            return ESP_ERR_NO_MEM;
        }
    }
    err = cap_lua_register_module("iotda", luaopen_iotda);
    if (err != ESP_OK) {
        return err;
    }
    return cap_lua_register_exit_cleanup(iotda_exit_cleanup);
}
