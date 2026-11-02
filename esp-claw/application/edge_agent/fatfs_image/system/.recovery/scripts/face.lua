-- Desk health assistant face.
--
-- The default screen is intentionally only the procedural eyes. Text and
-- posture artwork are interrupt views: confirmed bad posture, family
-- messages, sedentary/cloud rest reminders. Everything else is communicated
-- by eye expression alone.
local delay = require("delay")
local gpio = require("gpio")
local json = require("json")
local lvgl = require("lvgl")
local storage = require("storage")

local ok_iotda, iotda = pcall(require, "iotda")
if not ok_iotda then iotda = nil end

local ROOT = storage.get_root_dir()
local STATE = storage.join_path(ROOT, "posture_state.json")
local MESSAGES = storage.join_path(ROOT, "messages.json")
local PIN = 48

local W, H = 320, 240
local TICK_MS = 25
local TICKS_PER_SECOND = 40
local REST_SHOW_TICKS = 10 * TICKS_PER_SECOND
local MESSAGE_SHOW_TICKS = 15 * TICKS_PER_SECOND
local ART_SHOW_TICKS = 12 * TICKS_PER_SECOND
local ART_COOLDOWN_TICKS = 40 * TICKS_PER_SECOND

local beep_ok = pcall(gpio.set_direction, PIN, "output")
if beep_ok then pcall(gpio.set_level, PIN, 0) end

local function beep()
  if not beep_ok then return end
  pcall(gpio.set_level, PIN, 1)
  delay.delay_ms(150)
  pcall(gpio.set_level, PIN, 0)
end

lvgl.init({ buffer_lines = 40, tick_ms = 5, task_period_ms = 10, rotation = 180 })
local font = lvgl.font_load(storage.join_path(ROOT, "fonts/face.ttf"), { size = 18, cache_size = 128 })
if font then lvgl.set_default_font(font) end

local bigfont = nil
if font then
  local ok, loaded = pcall(lvgl.font_load,
    storage.join_path(ROOT, "fonts/face.ttf"), { size = 28, cache_size = 64 })
  if ok then bigfont = loaded end
end

local scr = lvgl.create_screen()
scr:set_style({ bg_color = "#000000" })

-- Default view: full-screen eyes only.
local EYES = lvgl.emotive_eyes(scr, {
  x = 0,
  y = 0,
  w = W,
  h = H,
  eye_size = 70,
  eye_gap = 16,
  eye_color = "#ffffff",
  bg_color = "#000000",
  auto_behavior = false,
  auto_gaze = true,
  auto_blink = true,
})

-- Interrupt views. All are hidden until a real interruption happens.
local art = lvgl.image(scr, { x = 0, y = 0, w = W, h = H })
local art_hint = lvgl.label(scr, { text = "", align = "center", y = 0 })
local msg_title = lvgl.label(scr, { text = "", align = "center", y = -58 })
local msg_body = lvgl.label(scr, { text = "", align = "center", y = 12 })
local rest_title = lvgl.label(scr, { text = "", align = "center", y = -34 })
local rest_body = lvgl.label(scr, { text = "", align = "center", y = 38 })

pcall(art_hint.set_size, art_hint, 300, 80)
pcall(msg_body.set_size, msg_body, 290, 150)
pcall(rest_body.set_size, rest_body, 300, 90)

art_hint:set_style({ text_color = "#FFFFFF" })
msg_title:set_style({ text_color = "#FFD166" })
msg_body:set_style({ text_color = "#FFFFFF" })
rest_title:set_style({ text_color = "#FFC46B" })
rest_body:set_style({ text_color = "#FFFFFF" })

if bigfont then
  local ok_a = pcall(art_hint.set_style, art_hint, { font = bigfont })
  local ok_b = pcall(msg_title.set_style, msg_title, { font = bigfont })
  local ok_c = pcall(rest_title.set_style, rest_title, { font = bigfont })
  if not (ok_a and ok_b and ok_c) then bigfont = nil end
end

local function hide_obj(obj)
  pcall(obj.add_flag, obj, "HIDDEN")
end

local function show_obj(obj)
  pcall(obj.clear_flag, obj, "HIDDEN")
end

local ALL_OVERLAYS = { art, art_hint, msg_title, msg_body, rest_title, rest_body }
for i = 1, #ALL_OVERLAYS do
  hide_obj(ALL_OVERLAYS[i])
end

local cache = {}
local function set_label(obj, key, text, color)
  if cache[key] ~= text then
    local ok = pcall(obj.set_text, obj, text)
    if ok then cache[key] = text end
  end
  if color and cache[key .. "c"] ~= color then
    local ok = pcall(obj.set_style, obj, { text_color = color })
    if ok then cache[key .. "c"] = color end
  end
end

local ART_DIR = storage.join_path(ROOT, "posture_art")
local ART_CODES = {
  forward_head = true,
  head_drooped = true,
  torso_forward = true,
  torso_side = true,
  shoulder_tilt = true,
  head_tilt = true,
  hand_on_head = true,
}
local ART_HINTS = {
  forward_head = "头抬起来",
  head_drooped = "挺直背",
  torso_forward = "坐直一点",
  torso_side = "身体回正",
  shoulder_tilt = "肩膀放平",
  head_tilt = "头摆正",
  hand_on_head = "手放下来",
}

local art_loaded = false
local art_code = nil
local function load_art(code)
  if art_loaded and art_code == code then
    return true
  end
  local path = storage.join_path(ART_DIR, code .. ".rgb565")
  local ok, data = pcall(storage.read_file, path)
  if not ok or type(data) ~= "string" or #data ~= W * H * 2 then
    art_loaded = false
    return false
  end
  local ok2 = pcall(art.set_raw_rgb565, art, W, H, data)
  art_loaded = ok2
  if ok2 then art_code = code end
  return ok2
end

local tick = 0
local current_view = nil
local message_active = false
local message_until = 0
local resting = false
local rest_left = 0
local art_until = 0
local art_cooldown_until = 0

local function set_view(view)
  if current_view == view then
    return
  end
  current_view = view
  hide_obj(EYES)
  for i = 1, #ALL_OVERLAYS do
    hide_obj(ALL_OVERLAYS[i])
  end
  if view == "eyes" then
    show_obj(EYES)
  elseif view == "art" then
    if art_loaded then
      show_obj(art)
    else
      show_obj(art_hint)
    end
  elseif view == "message" then
    show_obj(msg_title)
    show_obj(msg_body)
  elseif view == "rest" then
    show_obj(rest_title)
    show_obj(rest_body)
  end
end

local function sync_view()
  if message_active then
    set_view("message")
  elseif resting then
    set_view("rest")
  elseif art_until > tick then
    set_view("art")
  else
    set_view("eyes")
  end
end

local last_emotion = nil
local function eye_mood(emotion, sleeping)
  if sleeping then
    EYES:set_sleeping(true)
    last_emotion = "sleepy"
    return
  end
  if last_emotion == "sleepy" then
    EYES:set_sleeping(false)
  end
  if last_emotion ~= emotion then
    EYES:set_emotion(emotion)
    last_emotion = emotion
  end
end

pcall(math.randomseed, os.time())
local GOOD_MOODS = { "happy", "focused", "normal", "glee", "coquette" }
local next_good_at = 0
local function pick_good_mood()
  if tick < next_good_at then
    return nil
  end
  next_good_at = tick + 8 * TICKS_PER_SECOND + math.random(0, 8 * TICKS_PER_SECOND)
  return GOOD_MOODS[math.random(#GOOD_MOODS)]
end

local function bad_art_info(d)
  local p = d.posture or {}
  local issue = (p.issues or {})[1]
  local code = issue and tostring(issue.code or "") or ""
  if code == "" or not ART_CODES[code] then
    code = "forward_head"
  end
  return code, ART_HINTS[code] or "坐直一点"
end

local function trigger_art(code, hint)
  art_until = tick + ART_SHOW_TICKS
  art_cooldown_until = tick + ART_COOLDOWN_TICKS
  set_label(art_hint, "art_hint", hint or "坐直一点", "#FFFFFF")
  load_art(code)
  -- Force a view refresh even when art was already showing and a new image
  -- failed to load; otherwise the old frame would stay on screen.
  current_view = nil
  beep()
  sync_view()
end

local function trigger_rest(text)
  rest_left = REST_SHOW_TICKS
  resting = true
  set_label(rest_title, "rest_title", "休息一下", "#FFC46B")
  set_label(rest_body, "rest_body", text or "起来活动一下", "#FFFFFF")
  beep()
  sync_view()
end

local function show_message(author, content)
  message_active = true
  message_until = tick + MESSAGE_SHOW_TICKS
  local text = tostring(content or "")
  if #text > 180 then
    text = string.sub(text, 1, 180) .. "..."
  end
  set_label(msg_title, "msg_title", tostring(author or "家长") .. "留言", "#FFD166")
  set_label(msg_body, "msg_body", text, "#FFFFFF")
  beep()
  sync_view()
end

local last_message_id = 0
local function read_messages()
  local ok, text = pcall(storage.read_file, MESSAGES)
  if not ok or type(text) ~= "string" or text == "" then return end
  local ok2, snapshot = pcall(json.decode, text)
  if not ok2 or type(snapshot) ~= "table" then return end
  local list = snapshot.messages
  if type(list) ~= "table" then return end

  local newest = nil
  for i = 1, #list do
    local item = list[i]
    if type(item) == "table" then
      local id = tonumber(item.id) or 0
      if id > last_message_id then
        if newest == nil or id > (tonumber(newest.id) or 0) then
          newest = item
        end
      end
    end
  end
  if newest == nil then return end
  last_message_id = tonumber(newest.id) or last_message_id
  local author = newest.author_name
  if author == nil or author == "" then
    author = newest.author == "child" and "孩子" or "家长"
  end
  local content = tostring(newest.content or "")
  if content == "" then return end
  show_message(tostring(author), content)
end

local sedentary_shown = false
local bad_count = 0
local alerted = false

local function reset_bad_art()
  bad_count = 0
  alerted = false
  art_until = 0
  art_cooldown_until = 0
end

local function handle_state(x)
  local sit = x.sitting
  if type(sit) ~= "table" then sit = nil end
  if sit and not sit.sedentary then
    sedentary_shown = false
  end

  if resting then return end

  if sit and sit.sedentary and not sedentary_shown then
    sedentary_shown = true
    local minutes = tonumber(sit.minutes) or math.floor((tonumber(sit.seconds) or 0) / 60)
    trigger_rest(string.format("已经坐了 %d 分钟\n站起来活动一下", minutes))
    return
  end

  if not x.ok then
    reset_bad_art()
    sync_view()
    eye_mood("worried")
    return
  end

  local d = x.data or {}
  if d.person_detected ~= true then
    reset_bad_art()
    sync_view()
    eye_mood("sleepy", true)
    return
  end

  local p = d.posture or {}
  local q = p.status or "u"
  if q == "bad" or q == "warning" then
    bad_count = bad_count + 1
    if bad_count >= 3 then
      local code, hint = bad_art_info(d)
      if not alerted then
        alerted = true
        trigger_art(code, hint)
      elseif tick >= art_cooldown_until and art_until <= tick then
        trigger_art(code, hint)
      else
        sync_view()
      end
      eye_mood("annoyed")
    else
      sync_view()
      eye_mood("worried")
    end
    return
  end

  if q == "good" then
    reset_bad_art()
    sync_view()
    local mood = pick_good_mood()
    if mood then
      eye_mood(mood)
    end
    return
  end

  reset_bad_art()
  sync_view()
  eye_mood("normal")
end

local last_sampling = -1
local last_done = -1
local function read_state()
  local ok, text = pcall(storage.read_file, STATE)
  if not ok or type(text) ~= "string" or text == "" then return end
  local ok2, s = pcall(json.decode, text)
  if not ok2 or type(s) ~= "table" then return end
  local seq = tonumber(s.seq) or 0
  if s.phase == "sampling" then
    if seq ~= last_sampling then
      last_sampling = seq
      if not resting and not message_active then
        eye_mood("focused")
        sync_view()
      end
    end
  elseif s.phase == "done" and seq ~= last_done then
    last_done = seq
    handle_state(s)
  end
end

-- The first painted frame is eyes only. All text/art views stay hidden.
sync_view()
eye_mood("normal")
scr:load()
lvgl.process_events(30)

local next_read = 1
local next_message_read = 1
while true do
  tick = tick + 1

  if tick >= next_read then
    next_read = tick + math.floor(0.1 * TICKS_PER_SECOND)
    read_state()
  end

  if tick >= next_message_read then
    next_message_read = tick + TICKS_PER_SECOND
    read_messages()
  end

  if message_active and tick >= message_until then
    message_active = false
    sync_view()
  end

  if rest_left > 0 then
    rest_left = rest_left - 1
    if rest_left == 0 then
      resting = false
      sync_view()
    end
  end

  if art_until > 0 and tick >= art_until then
    art_until = 0
    sync_view()
  end

  if iotda then
    local ok, pending = pcall(iotda.take_reminder)
    if ok and pending then
      trigger_rest("起来活动一下，喝口水吧")
    end
  end

  sync_view()
  EYES:update()
  lvgl.process_events(0)
  delay.delay_ms(TICK_MS)
end
