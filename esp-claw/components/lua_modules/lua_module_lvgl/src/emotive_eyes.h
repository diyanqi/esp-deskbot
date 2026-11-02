/*
 * SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Procedural animated eyes rendered into a mutable RGB565 image buffer.
 * The visual behaviour is inspired by the public "Emotive Eyes" concept
 * (procedural rounded-quad eyes with expressions, gaze and blinking), but
 * this is an independent implementation written for LVGL.
 */
#pragma once

#include <stdbool.h>
#include <stdint.h>

#include "lvgl.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef enum {
    EMOTIVE_EYES_NORMAL = 0,
    EMOTIVE_EYES_ANGRY,
    EMOTIVE_EYES_GLEE,
    EMOTIVE_EYES_HAPPY,
    EMOTIVE_EYES_SAD,
    EMOTIVE_EYES_WORRIED,
    EMOTIVE_EYES_FOCUSED,
    EMOTIVE_EYES_ANNOYED,
    EMOTIVE_EYES_SURPRISED,
    EMOTIVE_EYES_SKEPTIC,
    EMOTIVE_EYES_FRUSTRATED,
    EMOTIVE_EYES_UNIMPRESSED,
    EMOTIVE_EYES_SLEEPY,
    EMOTIVE_EYES_SUSPICIOUS,
    EMOTIVE_EYES_SQUINT,
    EMOTIVE_EYES_FURIOUS,
    EMOTIVE_EYES_SCARED,
    EMOTIVE_EYES_AWE,
    EMOTIVE_EYES_COQUETTE,
    EMOTIVE_EYES_COUNT,
} emotive_eyes_emotion_t;

typedef struct emotive_eyes emotive_eyes_t;

typedef struct {
    int x1;
    int y1;
    int x2;
    int y2;
} emotive_eyes_rect_t;

emotive_eyes_t *emotive_eyes_create(int width, int height);
void emotive_eyes_destroy(emotive_eyes_t *eyes);

lv_image_dsc_t *emotive_eyes_image_dsc(emotive_eyes_t *eyes);
int emotive_eyes_width(const emotive_eyes_t *eyes);
int emotive_eyes_height(const emotive_eyes_t *eyes);

bool emotive_eyes_emotion_from_name(const char *name, int *out_emotion);
const char *emotive_eyes_emotion_name(int emotion);

void emotive_eyes_set_emotion(emotive_eyes_t *eyes, int emotion);
int emotive_eyes_get_emotion(const emotive_eyes_t *eyes);
void emotive_eyes_set_auto_behavior(emotive_eyes_t *eyes, bool enabled);
void emotive_eyes_set_auto_gaze(emotive_eyes_t *eyes, bool enabled);
void emotive_eyes_set_auto_blink(emotive_eyes_t *eyes, bool enabled);
void emotive_eyes_blink(emotive_eyes_t *eyes);
void emotive_eyes_look(emotive_eyes_t *eyes, float x, float y);
void emotive_eyes_set_sleeping(emotive_eyes_t *eyes, bool sleeping);
void emotive_eyes_set_colors(emotive_eyes_t *eyes, uint16_t eye_color, uint16_t background_color);
uint16_t emotive_eyes_eye_color(const emotive_eyes_t *eyes);
uint16_t emotive_eyes_background_color(const emotive_eyes_t *eyes);
void emotive_eyes_set_layout(emotive_eyes_t *eyes, int eye_size, int eye_gap);

/* Redraw whichever parts changed since the previous call. Returns true when
 * the buffer was modified; the changed area can be read with
 * emotive_eyes_dirty_rect(). */
bool emotive_eyes_update(emotive_eyes_t *eyes);
emotive_eyes_rect_t emotive_eyes_dirty_rect(const emotive_eyes_t *eyes);

#ifdef __cplusplus
}
#endif
