/*
 * SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Independent procedural eye renderer: rounded-quad eye geometry with
 * expressions, gaze, blinking and subtle micro-motion. Pixels are written
 * into a caller-owned RGB565 buffer and only the dirty rectangle is touched
 * on each frame.
 */
#include "emotive_eyes.h"

#include <math.h>
#include <stdlib.h>
#include <string.h>

#include "esp_heap_caps.h"
#include "esp_random.h"
#include "src/misc/cache/instance/lv_image_cache.h"

#define EE_COVERAGE_SAMPLES 8
#define EE_MAX_POINTS 32
#define EE_TRANSITION_MS 500.0f
#define EE_BLINK_MS 260.0f
#define EE_BASE_EYE_SIZE 70.0f
#define EE_DEFAULT_EYE_SIZE 70
#define EE_DEFAULT_EYE_GAP 15
#define EE_DEFAULT_BG_COLOR 0x0000
#define EE_DEFAULT_EYE_COLOR 0xFFFF
#define EE_CLIP_MARGIN 2.0f
#define EE_PI 3.14159265358979f

typedef struct {
    float x;
    float y;
} ee_point_t;

typedef struct {
    float offset_x;
    float offset_y;
    float width;
    float height;
    float slope_top;
    float slope_bottom;
    float radius_top;
    float radius_bottom;
} ee_eye_cfg_t;

typedef struct {
    float width;
    float height;
    float offset_x;
    float offset_y;
    float slope_top;
    float slope_bottom;
    float radius_top;
    float radius_bottom;
} ee_preset_t;

typedef struct {
    float offset_x;
    float offset_y;
    float height;
    float width;
    uint32_t period_ms;
} ee_variation_t;

typedef struct {
    bool mirrored;
    float center_x;
    float center_y;

    ee_eye_cfg_t cur;
    ee_eye_cfg_t from;
    ee_eye_cfg_t to;
    ee_eye_cfg_t out;
    bool transitioning;
    uint32_t transition_start;

    float gaze_x;
    float gaze_y;
    float gaze_scale;
    float gaze_from_x;
    float gaze_from_y;
    float gaze_from_scale;
    float gaze_to_x;
    float gaze_to_y;
    float gaze_to_scale;
    bool gaze_active;
    uint32_t gaze_start;
    uint32_t gaze_ms;

    bool blink_active;
    uint32_t blink_start;

    float var_offset_x;
    float var_offset_y;
    float var_height;
    float var_width;
    uint32_t var_period;
    uint32_t var_start;
} ee_eye_t;

struct emotive_eyes {
    int width;
    int height;
    uint16_t *pixels;
    lv_image_dsc_t dsc;
    uint16_t eye_color;
    uint16_t bg_color;
    int eye_size;
    int eye_gap;
    float scale;

    ee_eye_t left;
    ee_eye_t right;

    int emotion;
    bool auto_behavior;
    bool auto_gaze;
    bool auto_blink;
    bool sleeping;

    uint32_t next_blink;
    uint32_t next_gaze;
    uint32_t next_emotion;
    float gaze_last_x;
    float gaze_last_y;

    bool dirty;
    bool full_dirty;
    bool has_last;
    emotive_eyes_rect_t last;
    emotive_eyes_rect_t dirty_rect;

    uint8_t alpha_lut[256];
};

enum {
    P_NORMAL = 0,
    P_ANGRY,
    P_GLEE,
    P_HAPPY,
    P_SAD,
    P_WORRIED,
    P_WORRIED_ALT,
    P_FOCUSED,
    P_ANNOYED,
    P_ANNOYED_ALT,
    P_SURPRISED,
    P_SKEPTIC,
    P_SKEPTIC_ALT,
    P_FRUSTRATED,
    P_UNIMPRESSED,
    P_UNIMPRESSED_ALT,
    P_SLEEPY,
    P_SLEEPY_ALT,
    P_SUSPICIOUS,
    P_SUSPICIOUS_ALT,
    P_SQUINT,
    P_SQUINT_ALT,
    P_FURIOUS,
    P_SCARED,
    P_AWE,
    P_COQUETTE,
    P_COUNT,
};

/* Values are expressed for the 70 px reference eye and scaled by the
 * configured eye size. Rectangular eyes use large corner radii to read as
 * friendly rounded shapes; slopes tilt the upper or lower edge. */
static const ee_preset_t s_presets[P_COUNT] = {
    [P_NORMAL]          = {62, 62,  0,  0,  0.00f,  0.00f, 13, 13},
    [P_ANGRY]           = {58, 32, -3,  0,  0.28f,  0.00f,  3, 17},
    [P_GLEE]            = {60, 13,  0,  0,  0.00f,  0.00f, 13,  0},
    [P_HAPPY]           = {60, 15,  0,  0,  0.00f,  0.00f, 15,  0},
    [P_SAD]             = {60, 24,  0,  0, -0.48f,  0.00f,  2, 14},
    [P_WORRIED]         = {60, 38,  0,  0, -0.12f,  0.00f, 11, 14},
    [P_WORRIED_ALT]     = {60, 52,  0,  0, -0.22f,  0.00f, 11, 14},
    [P_FOCUSED]         = {60, 23,  0,  0,  0.22f,  0.00f,  5,  2},
    [P_ANNOYED]         = {60, 19,  0,  0,  0.00f,  0.00f,  1, 14},
    [P_ANNOYED_ALT]     = {60,  8,  0,  0,  0.00f,  0.00f,  1,  6},
    [P_SURPRISED]       = {66, 66, -2,  0,  0.00f,  0.00f, 24, 24},
    [P_SKEPTIC]         = {60, 60,  0,  0,  0.00f,  0.00f, 14, 14},
    [P_SKEPTIC_ALT]     = {60, 39,  0, -8,  0.28f,  0.00f,  2, 14},
    [P_FRUSTRATED]      = {60, 19,  4, -6,  0.00f,  0.00f,  1, 14},
    [P_UNIMPRESSED]     = {60, 19,  4,  0,  0.00f,  0.00f,  2, 14},
    [P_UNIMPRESSED_ALT] = {60, 35,  4, -4,  0.00f,  0.00f,  2, 23},
    [P_SLEEPY]          = {60, 22,  0, -2, -0.48f, -0.48f,  6,  6},
    [P_SLEEPY_ALT]      = {60, 13,  0, -2, -0.48f, -0.48f,  6,  6},
    [P_SUSPICIOUS]      = {60, 35,  0,  0,  0.00f,  0.00f, 12,  6},
    [P_SUSPICIOUS_ALT]  = {60, 25,  0, -4,  0.22f,  0.00f, 11,  6},
    [P_SQUINT]          = {53, 53,-12, -4,  0.00f,  0.00f, 12, 12},
    [P_SQUINT_ALT]      = {31, 31,  6,  0,  0.00f,  0.00f,  8,  8},
    [P_FURIOUS]         = {60, 46, -2,  0,  0.38f,  0.00f,  3, 13},
    [P_SCARED]          = {60, 60, -4,  0, -0.12f,  0.00f, 18, 13},
    [P_AWE]             = {66, 52,  2,  0, -0.12f,  0.12f, 18, 18},
    [P_COQUETTE]        = {53, 25,  0, -6,  0.24f, -0.24f, 12, 10},
};

static const uint8_t s_right_preset[EMOTIVE_EYES_COUNT] = {
    P_NORMAL, P_ANGRY, P_GLEE, P_HAPPY, P_SAD, P_WORRIED, P_FOCUSED,
    P_ANNOYED, P_SURPRISED, P_SKEPTIC, P_FRUSTRATED, P_UNIMPRESSED,
    P_SLEEPY, P_SUSPICIOUS, P_SQUINT, P_FURIOUS, P_SCARED, P_AWE,
    P_COQUETTE,
};

static const uint8_t s_left_preset[EMOTIVE_EYES_COUNT] = {
    P_NORMAL, P_ANGRY, P_GLEE, P_HAPPY, P_SAD, P_WORRIED_ALT, P_FOCUSED,
    P_ANNOYED_ALT, P_SURPRISED, P_SKEPTIC_ALT, P_FRUSTRATED, P_UNIMPRESSED_ALT,
    P_SLEEPY_ALT, P_SUSPICIOUS_ALT, P_SQUINT_ALT, P_FURIOUS, P_SCARED, P_AWE,
    P_COQUETTE,
};

static const char *const s_emotion_names[EMOTIVE_EYES_COUNT] = {
    "normal", "angry", "glee", "happy", "sad", "worried", "focused",
    "annoyed", "surprised", "skeptic", "frustrated", "unimpressed",
    "sleepy", "suspicious", "squint", "furious", "scared", "awe",
    "coquette",
};

static const ee_variation_t s_variations[EMOTIVE_EYES_COUNT] = {
    [EMOTIVE_EYES_NORMAL]    = {0, 0, 2.5f, 1.5f, 1100},
    [EMOTIVE_EYES_ANGRY]     = {0, 2, 0.0f, 0.0f,  320},
    [EMOTIVE_EYES_GLEE]      = {0, 5, 0.0f, 0.0f,  320},
    [EMOTIVE_EYES_COQUETTE]  = {0, 3, 0.0f, 0.0f,  420},
    [EMOTIVE_EYES_SLEEPY]    = {0, 0, 2.0f, 0.0f, 2400},
};

static float ee_clampf(float v, float lo, float hi)
{
    if (v < lo) return lo;
    if (v > hi) return hi;
    return v;
}

static float ee_lerpf(float a, float b, float t)
{
    return a + (b - a) * t;
}

static float ee_smoothstep(float t)
{
    if (t <= 0.0f) return 0.0f;
    if (t >= 1.0f) return 1.0f;
    return t * t * (3.0f - 2.0f * t);
}

static float ee_triangle(uint32_t elapsed, uint32_t period)
{
    if (period == 0) return 0.0f;
    float t = (float)(elapsed % period) / (float)period;
    return t < 0.5f ? t * 2.0f : (1.0f - t) * 2.0f;
}

static uint32_t ee_rand_range(uint32_t lo, uint32_t hi)
{
    if (hi <= lo) return lo;
    return lo + (esp_random() % (hi - lo + 1));
}

static uint16_t ee_blend565(uint16_t fg, uint16_t bg, uint8_t alpha)
{
    if (alpha == 255) return fg;
    if (alpha == 0) return bg;

    uint32_t fr = (fg >> 11) & 0x1FU;
    uint32_t fgn = (fg >> 5) & 0x3FU;
    uint32_t fb = fg & 0x1FU;
    uint32_t br = (bg >> 11) & 0x1FU;
    uint32_t bgn = (bg >> 5) & 0x3FU;
    uint32_t bb = bg & 0x1FU;
    uint32_t inv = 255U - alpha;

    uint32_t r = (fr * alpha + br * inv + 127U) / 255U;
    uint32_t g = (fgn * alpha + bgn * inv + 127U) / 255U;
    uint32_t b = (fb * alpha + bb * inv + 127U) / 255U;
    return (uint16_t)((r << 11) | (g << 5) | b);
}

static bool ee_rect_valid(const emotive_eyes_rect_t *r)
{
    return r->x2 >= r->x1 && r->y2 >= r->y1;
}

static void ee_rect_union(emotive_eyes_rect_t *dst, const emotive_eyes_rect_t *src)
{
    if (!ee_rect_valid(src)) return;
    if (!ee_rect_valid(dst)) {
        *dst = *src;
        return;
    }
    if (src->x1 < dst->x1) dst->x1 = src->x1;
    if (src->y1 < dst->y1) dst->y1 = src->y1;
    if (src->x2 > dst->x2) dst->x2 = src->x2;
    if (src->y2 > dst->y2) dst->y2 = src->y2;
}

static void ee_rect_clip(emotive_eyes_rect_t *r, int width, int height)
{
    if (r->x1 < 0) r->x1 = 0;
    if (r->y1 < 0) r->y1 = 0;
    if (r->x2 >= width) r->x2 = width - 1;
    if (r->y2 >= height) r->y2 = height - 1;
}

static void ee_fill_rect(emotive_eyes_t *eyes, const emotive_eyes_rect_t *r, uint16_t color)
{
    if (!ee_rect_valid(r)) return;
    const int width = eyes->width;
    uint16_t *row = eyes->pixels + (size_t)r->y1 * width;
    for (int y = r->y1; y <= r->y2; y++, row += width) {
        for (int x = r->x1; x <= r->x2; x++) {
            row[x] = color;
        }
    }
}

static uint8_t ee_coverage_alpha(const emotive_eyes_t *eyes, float coverage)
{
    int index = (int)(ee_clampf(coverage, 0.0f, 1.0f) * 255.0f + 0.5f);
    return eyes->alpha_lut[index];
}

static void ee_apply_preset(emotive_eyes_t *eyes, ee_eye_t *eye, int preset_index)
{
    const ee_preset_t *p = &s_presets[preset_index];
    float scale = eyes->scale;

    eye->to.width = p->width * scale;
    eye->to.height = p->height * scale;
    eye->to.offset_x = (eye->mirrored ? -p->offset_x : p->offset_x) * scale;
    eye->to.offset_y = p->offset_y * scale;
    /* Mirroring the left eye keeps asymmetric expressions readable. */
    eye->to.slope_top = eye->mirrored ? p->slope_top : -p->slope_top;
    eye->to.slope_bottom = eye->mirrored ? p->slope_bottom : -p->slope_bottom;
    eye->to.radius_top = p->radius_top * scale;
    eye->to.radius_bottom = p->radius_bottom * scale;
}

static void ee_apply_variation(ee_eye_t *eye, int emotion, uint32_t now)
{
    const ee_variation_t *v = &s_variations[emotion];

    eye->var_offset_x = v->offset_x * 1.0f;
    eye->var_offset_y = v->offset_y * 1.0f;
    eye->var_height = v->height;
    eye->var_width = v->width;
    eye->var_period = v->period_ms;
    eye->var_start = now;
}

static void ee_eye_begin_transition(ee_eye_t *eye, uint32_t now)
{
    eye->from = eye->cur;
    eye->transition_start = now;
    eye->transitioning = true;
}

static void ee_set_emotion_internal(emotive_eyes_t *eyes, int emotion, bool animate)
{
    uint32_t now = lv_tick_get();

    eyes->emotion = emotion;
    ee_apply_preset(eyes, &eyes->left, s_left_preset[emotion]);
    ee_apply_preset(eyes, &eyes->right, s_right_preset[emotion]);
    if (animate) {
        ee_eye_begin_transition(&eyes->left, now);
        ee_eye_begin_transition(&eyes->right, now);
    } else {
        eyes->left.cur = eyes->left.to;
        eyes->left.transitioning = false;
        eyes->right.cur = eyes->right.to;
        eyes->right.transitioning = false;
    }
    ee_apply_variation(&eyes->left, emotion, now);
    ee_apply_variation(&eyes->right, emotion, now);
    eyes->dirty = true;
}

static void ee_apply_layout(emotive_eyes_t *eyes)
{
    eyes->scale = (float)eyes->eye_size / EE_BASE_EYE_SIZE;
    eyes->left.center_x = eyes->width * 0.5f - (eyes->eye_size * 0.5f + eyes->eye_gap);
    eyes->right.center_x = eyes->width * 0.5f + (eyes->eye_size * 0.5f + eyes->eye_gap);
    eyes->left.center_y = eyes->height * 0.5f;
    eyes->right.center_y = eyes->height * 0.5f;
}

static bool ee_update_eye(ee_eye_t *eye, uint32_t now)
{
    bool active = false;

    if (eye->transitioning) {
        float t = (float)(now - eye->transition_start) / EE_TRANSITION_MS;
        float eased = ee_smoothstep(t);
        eye->cur.offset_x = ee_lerpf(eye->from.offset_x, eye->to.offset_x, eased);
        eye->cur.offset_y = ee_lerpf(eye->from.offset_y, eye->to.offset_y, eased);
        eye->cur.width = ee_lerpf(eye->from.width, eye->to.width, eased);
        eye->cur.height = ee_lerpf(eye->from.height, eye->to.height, eased);
        eye->cur.slope_top = ee_lerpf(eye->from.slope_top, eye->to.slope_top, eased);
        eye->cur.slope_bottom = ee_lerpf(eye->from.slope_bottom, eye->to.slope_bottom, eased);
        eye->cur.radius_top = ee_lerpf(eye->from.radius_top, eye->to.radius_top, eased);
        eye->cur.radius_bottom = ee_lerpf(eye->from.radius_bottom, eye->to.radius_bottom, eased);
        if (t >= 1.0f) {
            eye->cur = eye->to;
            eye->transitioning = false;
        }
        active = true;
    } else {
        eye->cur = eye->to;
    }

    if (eye->gaze_active) {
        float t = (float)(now - eye->gaze_start) / (float)eye->gaze_ms;
        float eased = ee_smoothstep(t);
        eye->gaze_x = ee_lerpf(eye->gaze_from_x, eye->gaze_to_x, eased);
        eye->gaze_y = ee_lerpf(eye->gaze_from_y, eye->gaze_to_y, eased);
        eye->gaze_scale = ee_lerpf(eye->gaze_from_scale, eye->gaze_to_scale, eased);
        if (t >= 1.0f) {
            eye->gaze_active = false;
        }
        active = true;
    }

    ee_eye_cfg_t out = eye->cur;
    out.offset_x += eye->gaze_x;
    out.offset_y += eye->gaze_y;
    out.height *= eye->gaze_scale;

    if (eye->var_period > 0) {
        float v = ee_triangle(now - eye->var_start, eye->var_period);
        out.offset_x += eye->var_offset_x * v;
        out.offset_y += eye->var_offset_y * v;
        out.height += eye->var_height * v;
        out.width += eye->var_width * v;
        active = true;
    }

    if (eye->blink_active) {
        float t = (float)(now - eye->blink_start) / EE_BLINK_MS;
        float b;
        if (t >= 1.0f) {
            eye->blink_active = false;
            b = 0.0f;
        } else if (t < 0.4f) {
            b = t / 0.4f;
        } else if (t < 0.6f) {
            b = 1.0f;
        } else {
            b = (1.0f - t) / 0.4f;
        }
        /* The frame that ends the blink still changes the shape, so it must
         * be repainted even though blink_active just became false. */
        active = true;
        float base_width = out.width;
        out.width = ee_lerpf(base_width, base_width * 1.10f, b);
        out.height *= (1.0f - b);
        out.radius_top *= (1.0f - b);
        out.radius_bottom *= (1.0f - b);
    }

    eye->out = out;
    return active;
}

static int ee_build_boundary(const ee_eye_t *eye, ee_point_t *pts, int max_pts)
{
    const ee_eye_cfg_t *c = &eye->out;
    if (c->width <= 0.0f || c->height <= 0.0f || max_pts < 3) return 0;

    float half_w = c->width * 0.5f;
    float rT = ee_clampf(c->radius_top, 0.0f, half_w);
    float rB = ee_clampf(c->radius_bottom, 0.0f, half_w);
    float dT = c->height * c->slope_top * 0.5f;
    float dB = c->height * c->slope_bottom * 0.5f;

    float available = c->height - fabsf(dT - dB) - 1.0f;
    if (available < 0.0f) available = 0.0f;
    if (rT + rB > available && rT + rB > 0.0f) {
        float k = available / (rT + rB);
        rT *= k;
        rB *= k;
    }

    float left = eye->center_x + c->offset_x - half_w;
    float right = eye->center_x + c->offset_x + half_w;
    float top = eye->center_y + c->offset_y - c->height * 0.5f;
    float bottom = eye->center_y + c->offset_y + c->height * 0.5f;

    float tlx = left + rT,  tly = top + rT - dT;
    float trx = right - rT, try_ = top + rT + dT;
    float brx = right - rB, bry = bottom - rB + dB;
    float blx = left + rB,  bly = bottom - rB - dB;

    int n = 0;
#define EE_ADD(px, py) do { if (n < max_pts) { pts[n].x = (px); pts[n].y = (py); n++; } } while (0)
    if (rT > 0.5f) {
        for (int a = 180; a <= 270; a += 15) {
            float rad = (float)a * EE_PI / 180.0f;
            EE_ADD(tlx + rT * cosf(rad), tly + rT * sinf(rad));
        }
    } else {
        EE_ADD(tlx, tly);
    }
    if (rT > 0.5f) {
        for (int a = 285; a <= 360; a += 15) {
            float rad = (float)a * EE_PI / 180.0f;
            EE_ADD(trx + rT * cosf(rad), try_ + rT * sinf(rad));
        }
    } else {
        EE_ADD(trx, try_);
    }
    if (rB > 0.5f) {
        for (int a = 15; a <= 90; a += 15) {
            float rad = (float)a * EE_PI / 180.0f;
            EE_ADD(brx + rB * cosf(rad), bry + rB * sinf(rad));
        }
    } else {
        EE_ADD(brx, bry);
    }
    if (rB > 0.5f) {
        for (int a = 105; a <= 180; a += 15) {
            float rad = (float)a * EE_PI / 180.0f;
            EE_ADD(blx + rB * cosf(rad), bly + rB * sinf(rad));
        }
    } else {
        EE_ADD(blx, bly);
    }
#undef EE_ADD
    return n;
}

static bool ee_sample_span(const ee_point_t *pts, int n, float y, float *out_left, float *out_right)
{
    float lo = 1e30f;
    float hi = -1e30f;
    int hits = 0;

    for (int i = 0; i < n; i++) {
        ee_point_t a = pts[i];
        ee_point_t b = pts[(i + 1) % n];
        if (a.y == b.y) continue;
        if (a.y > b.y) {
            ee_point_t tmp = a;
            a = b;
            b = tmp;
        }
        if (y < a.y || y >= b.y) continue;
        float x = a.x + (y - a.y) * (b.x - a.x) / (b.y - a.y);
        if (x < lo) lo = x;
        if (x > hi) hi = x;
        hits++;
    }
    if (hits < 2) return false;
    *out_left = lo;
    *out_right = hi;
    return true;
}

static void ee_render_eye(emotive_eyes_t *eyes, const ee_eye_t *eye, const emotive_eyes_rect_t *clip)
{
    ee_point_t pts[EE_MAX_POINTS];
    int n = ee_build_boundary(eye, pts, EE_MAX_POINTS);
    if (n < 3) return;

    const uint16_t fg = eyes->eye_color;
    const uint16_t bg = eyes->bg_color;

    for (int y = clip->y1; y <= clip->y2; y++) {
        float lefts[EE_COVERAGE_SAMPLES];
        float rights[EE_COVERAGE_SAMPLES];
        bool ok[EE_COVERAGE_SAMPLES];
        int ok_count = 0;
        float min_left = 1e30f;
        float max_right = -1e30f;
        float max_left = -1e30f;
        float min_right = 1e30f;

        for (int s = 0; s < EE_COVERAGE_SAMPLES; s++) {
            float sy = (float)y + ((float)s + 0.5f) / (float)EE_COVERAGE_SAMPLES;
            float l, r;
            ok[s] = ee_sample_span(pts, n, sy, &l, &r);
            if (!ok[s]) continue;
            lefts[s] = l;
            rights[s] = r;
            ok_count++;
            if (l < min_left) min_left = l;
            if (r > max_right) max_right = r;
            if (l > max_left) max_left = l;
            if (r < min_right) min_right = r;
        }
        if (ok_count == 0) continue;

        int x_start = (int)floorf(min_left);
        int x_end = (int)ceilf(max_right) - 1;
        if (x_start < clip->x1) x_start = clip->x1;
        if (x_end > clip->x2) x_end = clip->x2;
        if (x_end < x_start) continue;

        int full_start = (int)ceilf(max_left);
        int full_end = (int)floorf(min_right) - 1;
        if (full_start < x_start) full_start = x_start;
        if (full_start > x_end + 1) full_start = x_end + 1;
        if (full_end > x_end) full_end = x_end;
        if (full_end < full_start - 1) full_end = full_start - 1;

        uint16_t *row = eyes->pixels + (size_t)y * eyes->width;

        for (int x = x_start; x < full_start; x++) {
            float coverage = 0.0f;
            for (int s = 0; s < EE_COVERAGE_SAMPLES; s++) {
                if (!ok[s]) continue;
                float l = lefts[s] > (float)x ? lefts[s] : (float)x;
                float r = rights[s] < (float)(x + 1) ? rights[s] : (float)(x + 1);
                if (r > l) coverage += r - l;
            }
            uint8_t alpha = ee_coverage_alpha(eyes, coverage / (float)EE_COVERAGE_SAMPLES);
            if (alpha) row[x] = ee_blend565(fg, bg, alpha);
        }

        if (full_end >= full_start) {
            for (int x = full_start; x <= full_end; x++) {
                row[x] = fg;
            }
        }

        for (int x = full_end + 1; x <= x_end; x++) {
            float coverage = 0.0f;
            for (int s = 0; s < EE_COVERAGE_SAMPLES; s++) {
                if (!ok[s]) continue;
                float l = lefts[s] > (float)x ? lefts[s] : (float)x;
                float r = rights[s] < (float)(x + 1) ? rights[s] : (float)(x + 1);
                if (r > l) coverage += r - l;
            }
            uint8_t alpha = ee_coverage_alpha(eyes, coverage / (float)EE_COVERAGE_SAMPLES);
            if (alpha) row[x] = ee_blend565(fg, bg, alpha);
        }
    }
}

static void ee_draw_disc(emotive_eyes_t *eyes, const emotive_eyes_rect_t *clip,
                         float cx, float cy, float radius, uint16_t color)
{
    if (radius <= 0.0f) return;
    int x1 = (int)floorf(cx - radius - 1.0f);
    int x2 = (int)ceilf(cx + radius + 1.0f);
    int y1 = (int)floorf(cy - radius - 1.0f);
    int y2 = (int)ceilf(cy + radius + 1.0f);
    if (x1 < clip->x1) x1 = clip->x1;
    if (y1 < clip->y1) y1 = clip->y1;
    if (x2 > clip->x2) x2 = clip->x2;
    if (y2 > clip->y2) y2 = clip->y2;

    for (int y = y1; y <= y2; y++) {
        float dy = (float)y + 0.5f - cy;
        for (int x = x1; x <= x2; x++) {
            float dx = (float)x + 0.5f - cx;
            float dist = sqrtf(dx * dx + dy * dy);
            float coverage = radius + 0.5f - dist;
            if (coverage <= 0.0f) continue;
            uint8_t alpha = ee_coverage_alpha(eyes, coverage);
            uint16_t *p = &eyes->pixels[(size_t)y * eyes->width + x];
            *p = ee_blend565(color, *p, alpha);
        }
    }
}

static void ee_draw_diamond(emotive_eyes_t *eyes, const emotive_eyes_rect_t *clip,
                            float cx, float cy, float radius, uint16_t color)
{
    if (radius <= 0.0f) return;
    int x1 = (int)floorf(cx - radius - 1.0f);
    int x2 = (int)ceilf(cx + radius + 1.0f);
    int y1 = (int)floorf(cy - radius - 1.0f);
    int y2 = (int)ceilf(cy + radius + 1.0f);
    if (x1 < clip->x1) x1 = clip->x1;
    if (y1 < clip->y1) y1 = clip->y1;
    if (x2 > clip->x2) x2 = clip->x2;
    if (y2 > clip->y2) y2 = clip->y2;

    for (int y = y1; y <= y2; y++) {
        float dy = fabsf((float)y + 0.5f - cy) / radius;
        for (int x = x1; x <= x2; x++) {
            float dx = fabsf((float)x + 0.5f - cx) / radius;
            float coverage = (1.0f - (dx + dy)) * radius;
            if (coverage <= 0.0f) continue;
            uint8_t alpha = ee_coverage_alpha(eyes, coverage);
            uint16_t *p = &eyes->pixels[(size_t)y * eyes->width + x];
            *p = ee_blend565(color, *p, alpha);
        }
    }
}

static void ee_draw_sleep_indicator(emotive_eyes_t *eyes, const emotive_eyes_rect_t *clip)
{
    float s = eyes->scale;
    float cx = (float)eyes->width - 51.0f * s;
    float cy = (float)eyes->height - 44.0f * s;
    float r = 13.0f * s;

    ee_draw_disc(eyes, clip, cx, cy, r, eyes->eye_color);
    ee_draw_disc(eyes, clip, cx + r * 0.62f, cy - r * 0.10f, r * 0.86f, eyes->bg_color);
    ee_draw_diamond(eyes, clip, cx + r * 1.28f, cy - r * 0.92f, 4.5f * s, eyes->eye_color);
    ee_draw_diamond(eyes, clip, cx + r * 1.52f, cy + r * 0.85f, 3.2f * s, eyes->eye_color);
}

static void ee_eye_bounds(const emotive_eyes_t *eyes, const ee_eye_t *eye, emotive_eyes_rect_t *r)
{
    const ee_eye_cfg_t *c = &eye->out;
    float half_w = c->width * 0.5f + EE_CLIP_MARGIN;
    float slope = fmaxf(fabsf(c->slope_top), fabsf(c->slope_bottom)) * c->height * 0.5f;
    float half_h = c->height * 0.5f + slope + EE_CLIP_MARGIN;
    float cx = eye->center_x + c->offset_x;
    float cy = eye->center_y + c->offset_y;
    r->x1 = (int)floorf(cx - half_w);
    r->x2 = (int)ceilf(cx + half_w);
    r->y1 = (int)floorf(cy - half_h);
    r->y2 = (int)ceilf(cy + half_h);
    ee_rect_clip(r, eyes->width, eyes->height);
}

static void ee_current_bounds(const emotive_eyes_t *eyes, emotive_eyes_rect_t *out)
{
    emotive_eyes_rect_t left;
    emotive_eyes_rect_t right;
    ee_eye_bounds(eyes, &eyes->left, &left);
    ee_eye_bounds(eyes, &eyes->right, &right);
    *out = left;
    ee_rect_union(out, &right);
    if (eyes->sleeping) {
        float s = eyes->scale;
        emotive_eyes_rect_t moon = {
            .x1 = (int)floorf((float)eyes->width - 70.0f * s),
            .y1 = (int)floorf((float)eyes->height - 70.0f * s),
            .x2 = (int)ceilf((float)eyes->width - 24.0f * s),
            .y2 = (int)ceilf((float)eyes->height - 22.0f * s),
        };
        ee_rect_clip(&moon, eyes->width, eyes->height);
        ee_rect_union(out, &moon);
    }
}

static void ee_auto_look(emotive_eyes_t *eyes)
{
    float x;
    float y;

    if (fabsf(eyes->gaze_last_x) > 0.28f || fabsf(eyes->gaze_last_y) > 0.28f) {
        x = 0.0f;
        y = 0.0f;
    } else {
        x = (((float)(esp_random() % 2001) - 1000.0f) / 1000.0f) * 0.55f;
        y = (((float)(esp_random() % 2001) - 1000.0f) / 1000.0f) * 0.40f;
    }
    emotive_eyes_look(eyes, x, y);
}

static void ee_random_emotion(emotive_eyes_t *eyes)
{
    static const uint8_t pool[] = {
        EMOTIVE_EYES_NORMAL, EMOTIVE_EYES_NORMAL, EMOTIVE_EYES_NORMAL,
        EMOTIVE_EYES_HAPPY, EMOTIVE_EYES_HAPPY, EMOTIVE_EYES_GLEE,
        EMOTIVE_EYES_SURPRISED, EMOTIVE_EYES_WORRIED, EMOTIVE_EYES_SAD,
        EMOTIVE_EYES_SLEEPY, EMOTIVE_EYES_SQUINT, EMOTIVE_EYES_COQUETTE,
        EMOTIVE_EYES_AWE, EMOTIVE_EYES_SKEPTIC, EMOTIVE_EYES_ANGRY,
    };
    int emotion = pool[esp_random() % (sizeof(pool) / sizeof(pool[0]))];

    if (emotion == eyes->emotion) {
        emotion = (emotion + 1) % EMOTIVE_EYES_COUNT;
    }
    ee_set_emotion_internal(eyes, emotion, true);
}

emotive_eyes_t *emotive_eyes_create(int width, int height)
{
    if (width <= 0 || height <= 0) return NULL;

    emotive_eyes_t *eyes = calloc(1, sizeof(*eyes));
    if (!eyes) return NULL;

    size_t bytes = (size_t)width * (size_t)height * sizeof(uint16_t);
    eyes->pixels = heap_caps_malloc(bytes, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT);
    if (!eyes->pixels) {
        eyes->pixels = heap_caps_malloc(bytes, MALLOC_CAP_8BIT);
    }
    if (!eyes->pixels) {
        free(eyes);
        return NULL;
    }

    eyes->width = width;
    eyes->height = height;
    eyes->eye_color = EE_DEFAULT_EYE_COLOR;
    eyes->bg_color = EE_DEFAULT_BG_COLOR;
    eyes->eye_size = EE_DEFAULT_EYE_SIZE;
    eyes->eye_gap = EE_DEFAULT_EYE_GAP;
    eyes->emotion = -1;
    eyes->auto_gaze = true;
    eyes->auto_blink = true;
    eyes->left.gaze_scale = 1.0f;
    eyes->right.gaze_scale = 1.0f;

    for (int i = 0; i < 256; i++) {
        eyes->alpha_lut[i] = (uint8_t)lroundf(255.0f * sqrtf((float)i / 255.0f));
    }

    eyes->dsc.header.magic = LV_IMAGE_HEADER_MAGIC;
    eyes->dsc.header.cf = LV_COLOR_FORMAT_RGB565;
    eyes->dsc.header.w = (uint32_t)width;
    eyes->dsc.header.h = (uint32_t)height;
    eyes->dsc.header.stride = (uint32_t)width * sizeof(uint16_t);
    eyes->dsc.data_size = (uint32_t)bytes;
    eyes->dsc.data = (const uint8_t *)eyes->pixels;

    eyes->left.mirrored = true;
    eyes->right.mirrored = false;

    ee_apply_layout(eyes);
    ee_fill_rect(eyes, &(emotive_eyes_rect_t){0, 0, width - 1, height - 1}, eyes->bg_color);
    ee_set_emotion_internal(eyes, EMOTIVE_EYES_NORMAL, false);

    uint32_t now = lv_tick_get();
    eyes->next_blink = now + ee_rand_range(2200, 5200);
    eyes->next_gaze = now + ee_rand_range(1400, 3600);
    eyes->next_emotion = now + ee_rand_range(5500, 9000);
    eyes->full_dirty = true;
    eyes->dirty = false;

    return eyes;
}

void emotive_eyes_destroy(emotive_eyes_t *eyes)
{
    if (!eyes) return;
    lv_image_cache_drop(&eyes->dsc);
    heap_caps_free(eyes->pixels);
    free(eyes);
}

lv_image_dsc_t *emotive_eyes_image_dsc(emotive_eyes_t *eyes)
{
    return eyes ? &eyes->dsc : NULL;
}

int emotive_eyes_width(const emotive_eyes_t *eyes)
{
    return eyes ? eyes->width : 0;
}

int emotive_eyes_height(const emotive_eyes_t *eyes)
{
    return eyes ? eyes->height : 0;
}

bool emotive_eyes_emotion_from_name(const char *name, int *out_emotion)
{
    if (!name || !out_emotion) return false;
    for (int i = 0; i < EMOTIVE_EYES_COUNT; i++) {
        if (strcmp(name, s_emotion_names[i]) == 0) {
            *out_emotion = i;
            return true;
        }
    }
    return false;
}

const char *emotive_eyes_emotion_name(int emotion)
{
    if (emotion < 0 || emotion >= EMOTIVE_EYES_COUNT) return NULL;
    return s_emotion_names[emotion];
}

void emotive_eyes_set_emotion(emotive_eyes_t *eyes, int emotion)
{
    if (!eyes || emotion < 0 || emotion >= EMOTIVE_EYES_COUNT) return;
    if (emotion == eyes->emotion) return;
    ee_set_emotion_internal(eyes, emotion, true);
}

int emotive_eyes_get_emotion(const emotive_eyes_t *eyes)
{
    return eyes ? eyes->emotion : -1;
}

void emotive_eyes_set_auto_behavior(emotive_eyes_t *eyes, bool enabled)
{
    if (!eyes) return;
    eyes->auto_behavior = enabled;
    eyes->next_emotion = lv_tick_get() + ee_rand_range(5500, 9000);
}

void emotive_eyes_set_auto_gaze(emotive_eyes_t *eyes, bool enabled)
{
    if (!eyes) return;
    eyes->auto_gaze = enabled;
    eyes->next_gaze = lv_tick_get() + ee_rand_range(1400, 3600);
}

void emotive_eyes_set_auto_blink(emotive_eyes_t *eyes, bool enabled)
{
    if (!eyes) return;
    eyes->auto_blink = enabled;
    eyes->next_blink = lv_tick_get() + ee_rand_range(2200, 5200);
}

void emotive_eyes_blink(emotive_eyes_t *eyes)
{
    if (!eyes) return;
    uint32_t now = lv_tick_get();
    eyes->left.blink_active = true;
    eyes->left.blink_start = now;
    eyes->right.blink_active = true;
    eyes->right.blink_start = now;
    eyes->dirty = true;
}

void emotive_eyes_look(emotive_eyes_t *eyes, float x, float y)
{
    if (!eyes) return;

    x = ee_clampf(x, -1.0f, 1.0f);
    y = ee_clampf(y, -1.0f, 1.0f);

    float shift = 13.0f * eyes->scale;
    float lift = 9.0f * eyes->scale;
    float scale_y = 1.0f - 0.05f * fabsf(x) - 0.07f * fabsf(y);
    if (scale_y < 0.90f) scale_y = 0.90f;

    uint32_t now = lv_tick_get();
    ee_eye_t *both[2] = {&eyes->left, &eyes->right};
    for (int i = 0; i < 2; i++) {
        ee_eye_t *eye = both[i];
        eye->gaze_from_x = eye->gaze_x;
        eye->gaze_from_y = eye->gaze_y;
        eye->gaze_from_scale = eye->gaze_scale;
        eye->gaze_to_x = x * shift;
        eye->gaze_to_y = y * lift;
        eye->gaze_to_scale = scale_y;
        eye->gaze_start = now;
        eye->gaze_ms = 220;
        eye->gaze_active = true;
    }
    eyes->gaze_last_x = x;
    eyes->gaze_last_y = y;
    eyes->dirty = true;
}

void emotive_eyes_set_sleeping(emotive_eyes_t *eyes, bool sleeping)
{
    if (!eyes || eyes->sleeping == sleeping) return;
    eyes->sleeping = sleeping;
    if (sleeping) {
        ee_set_emotion_internal(eyes, EMOTIVE_EYES_SLEEPY, true);
        emotive_eyes_look(eyes, 0.0f, 0.35f);
    }
    eyes->dirty = true;
}

void emotive_eyes_set_colors(emotive_eyes_t *eyes, uint16_t eye_color, uint16_t background_color)
{
    if (!eyes) return;
    eyes->eye_color = eye_color;
    eyes->bg_color = background_color;
    eyes->full_dirty = true;
    eyes->dirty = true;
}

uint16_t emotive_eyes_eye_color(const emotive_eyes_t *eyes)
{
    return eyes ? eyes->eye_color : 0xFFFF;
}

uint16_t emotive_eyes_background_color(const emotive_eyes_t *eyes)
{
    return eyes ? eyes->bg_color : 0x0000;
}

void emotive_eyes_set_layout(emotive_eyes_t *eyes, int eye_size, int eye_gap)
{
    if (!eyes || eye_size <= 0 || eye_gap < 0) return;
    if (eyes->eye_size == eye_size && eyes->eye_gap == eye_gap) return;

    eyes->eye_size = eye_size;
    eyes->eye_gap = eye_gap;
    ee_apply_layout(eyes);
    if (eyes->emotion >= 0) {
        ee_set_emotion_internal(eyes, eyes->emotion, true);
    }
    eyes->dirty = true;
}

bool emotive_eyes_update(emotive_eyes_t *eyes)
{
    if (!eyes) return false;
    uint32_t now = lv_tick_get();

    if (eyes->auto_blink && !eyes->sleeping && (int32_t)(now - eyes->next_blink) >= 0) {
        emotive_eyes_blink(eyes);
        eyes->next_blink = now + ee_rand_range(2200, 5200);
    }
    if (eyes->auto_gaze && !eyes->sleeping && (int32_t)(now - eyes->next_gaze) >= 0) {
        ee_auto_look(eyes);
        eyes->next_gaze = now + ee_rand_range(1400, 3600);
    }
    if (eyes->auto_behavior && (int32_t)(now - eyes->next_emotion) >= 0) {
        ee_random_emotion(eyes);
        eyes->next_emotion = now + ee_rand_range(5500, 9000);
    }

    bool active = false;
    active |= ee_update_eye(&eyes->left, now);
    active |= ee_update_eye(&eyes->right, now);

    emotive_eyes_rect_t current;
    ee_current_bounds(eyes, &current);

    if (!active && !eyes->dirty && !eyes->full_dirty) {
        return false;
    }

    emotive_eyes_rect_t redraw;
    if (eyes->full_dirty || !eyes->has_last) {
        redraw.x1 = 0;
        redraw.y1 = 0;
        redraw.x2 = eyes->width - 1;
        redraw.y2 = eyes->height - 1;
    } else {
        redraw = current;
        ee_rect_union(&redraw, &eyes->last);
    }
    ee_rect_clip(&redraw, eyes->width, eyes->height);
    if (!ee_rect_valid(&redraw)) {
        eyes->dirty = false;
        eyes->full_dirty = false;
        return false;
    }

    ee_fill_rect(eyes, &redraw, eyes->bg_color);
    ee_render_eye(eyes, &eyes->left, &redraw);
    ee_render_eye(eyes, &eyes->right, &redraw);
    if (eyes->sleeping) {
        ee_draw_sleep_indicator(eyes, &redraw);
    }

    eyes->dirty_rect = redraw;
    eyes->last = current;
    eyes->has_last = ee_rect_valid(&current);
    eyes->dirty = false;
    eyes->full_dirty = false;
    return true;
}

emotive_eyes_rect_t emotive_eyes_dirty_rect(const emotive_eyes_t *eyes)
{
    if (!eyes) {
        emotive_eyes_rect_t empty = {0, 0, -1, -1};
        return empty;
    }
    return eyes->dirty_rect;
}
