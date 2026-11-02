/*
 * SPDX-FileCopyrightText: 2026 Espressif Systems (Shanghai) CO LTD
 *
 * SPDX-License-Identifier: Apache-2.0
 */
#include "lua_lvgl_private.h"

#include "emotive_eyes.h"

static emotive_eyes_t *lua_lvgl_emotive_eyes_get(lua_State *L, lv_obj_t **out_obj)
{
    lua_lvgl_obj_ud_t *ud = lua_lvgl_check_ud(L, 1);

    *out_obj = lua_lvgl_lock_widget(L, ud, LUA_LVGL_OBJ_EMOTIVE_EYES);
    if (!ud->record || !ud->record->data) {
        lua_lvgl_unlock();
        luaL_error(L, "lvgl emotive_eyes object is not initialized");
    }
    return (emotive_eyes_t *)ud->record->data;
}

static bool lua_lvgl_parse_color_field(lua_State *L, int opts_index, const char *field,
                                       uint16_t *out_color)
{
    bool present = false;

    lua_getfield(L, opts_index, field);
    if (!lua_isnil(L, -1)) {
        lv_color_t color;
        if (lua_lvgl_parse_color(L, -1, &color) != ESP_OK) {
            lua_pop(L, 1);
            luaL_error(L, "lvgl emotive_eyes option '%s' must be a color", field);
        }
        *out_color = lv_color_to_u16(color);
        present = true;
    }
    lua_pop(L, 1);
    return present;
}

int lua_lvgl_emotive_eyes_new(lua_State *L)
{
    lua_lvgl_obj_ud_t *parent_ud = lua_lvgl_check_ud(L, 1);
    lua_lvgl_opts_t opts;
    bool has_opts = false;
    int width;
    int height;
    int eye_size = 70;
    int eye_gap = 15;
    bool auto_behavior = false;
    bool auto_gaze = true;
    bool auto_blink = true;
    bool sleeping = false;
    int emotion = EMOTIVE_EYES_NORMAL;
    uint16_t eye_color = 0xFFFF;
    uint16_t bg_color = 0x0000;
    const char *emotion_name = NULL;
    esp_err_t err;
    lv_obj_t *parent;
    lv_obj_t *obj;
    lv_align_t align;
    lua_lvgl_obj_ud_t *created_ud;
    const char *obj_error = NULL;
    emotive_eyes_t *eyes;

    lua_lvgl_parse_opts(L, 2, &opts);
    has_opts = opts.has_opts;
    width = has_opts ? lua_lvgl_get_opt_int_field(L, 2, "w", s_lvgl.width) : s_lvgl.width;
    height = has_opts ? lua_lvgl_get_opt_int_field(L, 2, "h", s_lvgl.height) : s_lvgl.height;
    if (width <= 0) width = 320;
    if (height <= 0) height = 240;

    if (has_opts) {
        lua_Integer value;

        lua_getfield(L, 2, "eye_size");
        if (!lua_isnil(L, -1)) {
            value = luaL_checkinteger(L, -1);
            if (value > 0) eye_size = (int)value;
        }
        lua_pop(L, 1);

        lua_getfield(L, 2, "eye_gap");
        if (!lua_isnil(L, -1)) {
            value = luaL_checkinteger(L, -1);
            if (value >= 0) eye_gap = (int)value;
        }
        lua_pop(L, 1);

        auto_behavior = lua_lvgl_get_opt_bool_field(L, 2, "auto_behavior", auto_behavior);
        auto_gaze = lua_lvgl_get_opt_bool_field(L, 2, "auto_gaze", auto_gaze);
        auto_blink = lua_lvgl_get_opt_bool_field(L, 2, "auto_blink", auto_blink);
        sleeping = lua_lvgl_get_opt_bool_field(L, 2, "sleeping", sleeping);
        lua_lvgl_parse_color_field(L, 2, "eye_color", &eye_color);
        lua_lvgl_parse_color_field(L, 2, "bg_color", &bg_color);

        lua_getfield(L, 2, "emotion");
        if (!lua_isnil(L, -1)) {
            emotion_name = luaL_checkstring(L, -1);
        }
        lua_pop(L, 1);
    }
    if (emotion_name && !emotive_eyes_emotion_from_name(emotion_name, &emotion)) {
        return luaL_error(L, "lvgl emotive_eyes unknown emotion '%s'", emotion_name);
    }
    if (opts.align_value && lua_lvgl_parse_align(L, opts.align_value, &align) != ESP_OK) {
        return luaL_error(L, "lvgl emotive_eyes invalid align value");
    }

    err = lua_lvgl_lock();
    if (err != ESP_OK) {
        return lua_lvgl_error_esp(L, "lock", err);
    }
    parent = lua_lvgl_validate_ud_locked(parent_ud, NULL, &obj_error);
    if (!parent) {
        lua_lvgl_unlock();
        return luaL_error(L, "%s", obj_error);
    }

    obj = lv_image_create(parent);
    if (!obj) {
        lua_lvgl_unlock();
        return luaL_error(L, "lvgl emotive_eyes image create failed");
    }
    lv_obj_remove_flag(obj, LV_OBJ_FLAG_SCROLLABLE);
    lv_obj_set_style_pad_all(obj, 0, 0);
    lv_obj_set_size(obj, width, height);
    if (opts.align_value) {
        lv_obj_align(obj, align, opts.x, opts.y);
    } else {
        lv_obj_set_pos(obj, opts.x, opts.y);
    }

    eyes = emotive_eyes_create(width, height);
    if (!eyes) {
        lv_obj_delete(obj);
        lua_lvgl_unlock();
        return luaL_error(L, "lvgl emotive_eyes buffer allocation failed");
    }
    emotive_eyes_set_layout(eyes, eye_size, eye_gap);
    emotive_eyes_set_colors(eyes, eye_color, bg_color);
    emotive_eyes_set_auto_behavior(eyes, auto_behavior);
    emotive_eyes_set_auto_gaze(eyes, auto_gaze);
    emotive_eyes_set_auto_blink(eyes, auto_blink);
    emotive_eyes_set_emotion(eyes, emotion);
    if (sleeping) {
        emotive_eyes_set_sleeping(eyes, true);
    }
    lv_image_set_src(obj, emotive_eyes_image_dsc(eyes));

    created_ud = lua_lvgl_push_obj(L, obj, LUA_LVGL_OBJ_EMOTIVE_EYES);
    if (!created_ud) {
        lv_obj_delete(obj);
        emotive_eyes_destroy(eyes);
        lua_lvgl_unlock();
        return luaL_error(L, "lvgl object record allocation failed");
    }
    created_ud->record->data = eyes;
    (void)emotive_eyes_update(eyes);
    lua_lvgl_unlock();
    return 1;
}

static int lua_lvgl_emotive_eyes_set_emotion(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);
    const char *name = luaL_checkstring(L, 2);
    int emotion;

    (void)obj;
    if (!emotive_eyes_emotion_from_name(name, &emotion)) {
        lua_lvgl_unlock();
        return luaL_error(L, "lvgl emotive_eyes unknown emotion '%s'", name);
    }
    emotive_eyes_set_emotion(eyes, emotion);
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_get_emotion(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);
    const char *name = emotive_eyes_emotion_name(emotive_eyes_get_emotion(eyes));

    (void)obj;
    lua_lvgl_unlock();
    lua_pushstring(L, name ? name : "");
    return 1;
}

static int lua_lvgl_emotive_eyes_blink(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);

    (void)obj;
    emotive_eyes_blink(eyes);
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_look(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);
    lua_Number x = luaL_optnumber(L, 2, 0);
    lua_Number y = luaL_optnumber(L, 3, 0);

    (void)obj;
    emotive_eyes_look(eyes, (float)x, (float)y);
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_set_auto(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);

    if (!lua_isnoneornil(L, 2)) {
        emotive_eyes_set_auto_behavior(eyes, lua_toboolean(L, 2));
    }
    if (!lua_isnoneornil(L, 3)) {
        emotive_eyes_set_auto_gaze(eyes, lua_toboolean(L, 3));
    }
    if (!lua_isnoneornil(L, 4)) {
        emotive_eyes_set_auto_blink(eyes, lua_toboolean(L, 4));
    }
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_set_sleeping(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);

    (void)obj;
    emotive_eyes_set_sleeping(eyes, lua_toboolean(L, 2));
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_set_colors(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);
    lv_color_t color;
    uint16_t eye_color = emotive_eyes_eye_color(eyes);
    uint16_t bg_color = emotive_eyes_background_color(eyes);

    (void)obj;
    if (!lua_isnoneornil(L, 2)) {
        if (lua_lvgl_parse_color(L, 2, &color) != ESP_OK) {
            lua_lvgl_unlock();
            return luaL_error(L, "lvgl emotive_eyes eye color must be a color");
        }
        eye_color = lv_color_to_u16(color);
    }
    if (!lua_isnoneornil(L, 3)) {
        if (lua_lvgl_parse_color(L, 3, &color) != ESP_OK) {
            lua_lvgl_unlock();
            return luaL_error(L, "lvgl emotive_eyes background color must be a color");
        }
        bg_color = lv_color_to_u16(color);
    }
    emotive_eyes_set_colors(eyes, eye_color, bg_color);
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_set_layout(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);
    lua_Integer eye_size = luaL_optinteger(L, 2, 70);
    lua_Integer eye_gap = luaL_optinteger(L, 3, 15);

    (void)obj;
    emotive_eyes_set_layout(eyes, (int)eye_size, (int)eye_gap);
    lua_lvgl_unlock();
    lua_pushboolean(L, 1);
    return 1;
}

static int lua_lvgl_emotive_eyes_update(lua_State *L)
{
    lv_obj_t *obj;
    emotive_eyes_t *eyes = lua_lvgl_emotive_eyes_get(L, &obj);

    if (emotive_eyes_update(eyes)) {
        emotive_eyes_rect_t r = emotive_eyes_dirty_rect(eyes);
        lv_area_t area = {.x1 = r.x1, .y1 = r.y1, .x2 = r.x2, .y2 = r.y2};

        lv_obj_invalidate_area(obj, &area);
        lua_lvgl_unlock();
        lua_pushboolean(L, 1);
    } else {
        lua_lvgl_unlock();
        lua_pushboolean(L, 0);
    }
    return 1;
}

const luaL_Reg lua_lvgl_emotive_eyes_methods[] = {
    {"set_emotion", lua_lvgl_emotive_eyes_set_emotion},
    {"get_emotion", lua_lvgl_emotive_eyes_get_emotion},
    {"blink", lua_lvgl_emotive_eyes_blink},
    {"look", lua_lvgl_emotive_eyes_look},
    {"set_auto", lua_lvgl_emotive_eyes_set_auto},
    {"set_sleeping", lua_lvgl_emotive_eyes_set_sleeping},
    {"set_colors", lua_lvgl_emotive_eyes_set_colors},
    {"set_eye_layout", lua_lvgl_emotive_eyes_set_layout},
    {"update", lua_lvgl_emotive_eyes_update},
    {NULL, NULL},
};
