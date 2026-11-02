/*
 * SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
 *
 * SPDX-License-Identifier: Apache-2.0
 */
#include "lua_module_camera_snapshot.h"

#include <errno.h>
#include <stdio.h>
#include <string.h>

#include "cap_lua.h"
#include "driver/gpio.h"
#include "esp_camera.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "lauxlib.h"
#include "lua.h"
#include "sensor.h"

#if defined(CONFIG_ESP_BOARD_DEV_DISPLAY_LCD_SUPPORT) && CONFIG_ESP_BOARD_DEV_DISPLAY_LCD_SUPPORT
#include "esp_lv_adapter.h"
#define CAMERA_SNAPSHOT_HAS_DISPLAY 1
#endif

static const char *TAG = "lua_cam_snapshot";

#define CAMERA_SNAPSHOT_DEFAULT_FREQ_HZ 24000000
#define CAMERA_SNAPSHOT_DEFAULT_WIDTH   320
#define CAMERA_SNAPSHOT_DEFAULT_HEIGHT  240
#define CAMERA_SNAPSHOT_DEFAULT_QUALITY 12

#define CAMERA_SNAPSHOT_FLASH_GPIO 10

static bool camera_snapshot_config_from_board(camera_config_t *out)
{
    /* Fixed DVP wiring for this breadboard build. The board-level camera
     * device is intentionally not registered; esp32-camera owns SCCB 8/9. */
    memset(out, 0, sizeof(*out));
    out->pin_pwdn = 42;
    out->pin_reset = 47;
    out->pin_xclk = -1;
    out->pin_sccb_sda = 8;
    out->pin_sccb_scl = 9;
    out->sccb_i2c_port = -1;
    out->pin_d0 = 14;
    out->pin_d1 = 16;
    out->pin_d2 = 17;
    out->pin_d3 = 18;
    out->pin_d4 = 21;
    out->pin_d5 = 39;
    out->pin_d6 = 40;
    out->pin_d7 = 41;
    out->pin_vsync = 12;
    out->pin_href = 13;
    out->pin_pclk = 11;
    out->xclk_freq_hz = CAMERA_SNAPSHOT_DEFAULT_FREQ_HZ;
    out->ledc_timer = LEDC_TIMER_0;
    out->ledc_channel = LEDC_CHANNEL_0;
    out->pixel_format = PIXFORMAT_JPEG;
    out->frame_size = FRAMESIZE_QVGA;
    out->jpeg_quality = CAMERA_SNAPSHOT_DEFAULT_QUALITY;
    out->fb_count = 2;
    out->fb_location = CAMERA_FB_IN_DRAM;
    out->grab_mode = CAMERA_GRAB_WHEN_EMPTY;
    out->jpeg_buffer_size = 0;

    ESP_LOGI(TAG,
             "camera: pwdn=%d reset=%d xclk=%d(%d Hz) sccb_port=%d "
             "pclk=%d vsync=%d href=%d d0..d7=%d,%d,%d,%d,%d,%d,%d,%d",
             out->pin_pwdn, out->pin_reset, out->pin_xclk, out->xclk_freq_hz, out->sccb_i2c_port,
             out->pin_pclk, out->pin_vsync, out->pin_href,
             out->pin_d0, out->pin_d1, out->pin_d2, out->pin_d3,
             out->pin_d4, out->pin_d5, out->pin_d6, out->pin_d7);
    return true;
}

static void camera_snapshot_flash_off(void)
{
    gpio_config_t io = {
        .pin_bit_mask = 1ULL << CAMERA_SNAPSHOT_FLASH_GPIO,
        .mode = GPIO_MODE_OUTPUT,
        .pull_down_en = GPIO_PULLDOWN_DISABLE,
        .pull_up_en = GPIO_PULLUP_DISABLE,
        .intr_type = GPIO_INTR_DISABLE,
    };
    if (gpio_config(&io) == ESP_OK) {
        gpio_set_level(CAMERA_SNAPSHOT_FLASH_GPIO, 0);
    }
}

static void camera_snapshot_read_opts(lua_State *L, int index, int *width, int *height, int *quality)
{
    index = lua_absindex(L, index);
    if (!lua_istable(L, index)) {
        return;
    }

    lua_getfield(L, index, "width");
    if (lua_isinteger(L, -1)) {
        *width = (int)lua_tointeger(L, -1);
    }
    lua_pop(L, 1);

    lua_getfield(L, index, "height");
    if (lua_isinteger(L, -1)) {
        *height = (int)lua_tointeger(L, -1);
    }
    lua_pop(L, 1);

    lua_getfield(L, index, "quality");
    if (lua_isinteger(L, -1)) {
        *quality = (int)lua_tointeger(L, -1);
    }
    lua_pop(L, 1);
}

static void camera_snapshot_pause_display(bool *paused)
{
#if defined(CAMERA_SNAPSHOT_HAS_DISPLAY)
    if (esp_lv_adapter_is_initialized()) {
        if (esp_lv_adapter_pause(1000) == ESP_OK) {
            *paused = true;
        }
        /* Even when the Lua LVGL session does not acknowledge the pause,
         * wait one display frame so the SPI/DMA pipeline settles. */
        vTaskDelay(pdMS_TO_TICKS(80));
    }
#else
    (void)paused;
#endif
}

static void camera_snapshot_resume_display(bool *paused)
{
#if defined(CAMERA_SNAPSHOT_HAS_DISPLAY)
    if (*paused) {
        esp_lv_adapter_resume();
        *paused = false;
    }
#else
    (void)paused;
#endif
}

static int lua_module_camera_snapshot(lua_State *L)
{
    const char *path = luaL_checkstring(L, 1);
    int width = CAMERA_SNAPSHOT_DEFAULT_WIDTH;
    int height = CAMERA_SNAPSHOT_DEFAULT_HEIGHT;
    int quality = CAMERA_SNAPSHOT_DEFAULT_QUALITY;

    camera_snapshot_read_opts(L, 2, &width, &height, &quality);
    if (quality < 0 || quality > 63) {
        return luaL_error(L, "quality must be in [0, 63]");
    }

    camera_config_t config;
    if (!camera_snapshot_config_from_board(&config)) {
        return luaL_error(L, "board camera config unavailable");
    }
    if (width == 640 && height == 480) {
        config.frame_size = FRAMESIZE_VGA;
    } else if (width == 320 && height == 240) {
        config.frame_size = FRAMESIZE_QVGA;
    } else {
        return luaL_error(L, "only 320x240 and 640x480 are supported (got %dx%d)", width, height);
    }
    config.jpeg_quality = quality;

    bool display_paused = false;
    camera_snapshot_pause_display(&display_paused);
    camera_snapshot_flash_off();

    camera_fb_t *fb = NULL;
    esp_err_t err = ESP_OK;
    for (int attempt = 0; attempt < 10 && fb == NULL; attempt++) {
        err = esp_camera_init(&config);
        if (err != ESP_OK) {
            camera_snapshot_resume_display(&display_paused);
            return luaL_error(L, "camera init failed: %s", esp_err_to_name(err));
        }
        /* The device is mounted upside down: flip both axes (a 180 degree
         * rotation, not a mirror) so the uploaded image is upright. */
        sensor_t *sensor = esp_camera_sensor_get();
        if (sensor) {
            sensor->set_vflip(sensor, 1);
            sensor->set_hmirror(sensor, 1);
        }
        /* The sensor streams JPEG continuously; the first frames after a
         * fresh init are the ones that still align with the SOI marker. */
        fb = esp_camera_fb_get();
        if (fb == NULL) {
            esp_camera_deinit();
            vTaskDelay(pdMS_TO_TICKS(200));
        }
    }
    if (fb == NULL) {
        camera_snapshot_resume_display(&display_paused);
        return luaL_error(L, "camera capture failed");
    }

    FILE *file = fopen(path, "wb");
    if (file == NULL) {
        esp_camera_fb_return(fb);
        esp_camera_deinit();
        camera_snapshot_resume_display(&display_paused);
        return luaL_error(L, "cannot open %s: %s", path, strerror(errno));
    }

    size_t len = fb->len;
    size_t written = fwrite(fb->buf, 1, len, file);
    int close_rc = fclose(file);
    esp_camera_fb_return(fb);
    esp_camera_deinit();

    camera_snapshot_resume_display(&display_paused);

    if (written != len || close_rc != 0) {
        return luaL_error(L, "short write to %s", path);
    }

    ESP_LOGI(TAG, "saved %s (%u bytes, %dx%d)", path, (unsigned)len, width, height);
    lua_pushinteger(L, (lua_Integer)len);
    return 1;
}

static int lua_module_camera_snapshot_open(lua_State *L)
{
    lua_newtable(L);
    lua_pushcfunction(L, lua_module_camera_snapshot);
    lua_setfield(L, -2, "snapshot");
    return 1;
}

esp_err_t lua_module_camera_snapshot_register(void)
{
    return cap_lua_register_module("camera_snapshot", lua_module_camera_snapshot_open);
}
