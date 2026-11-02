-- Cycle through every emotive-eyes expression for visual verification.
local delay = require("delay")
local lvgl = require("lvgl")

lvgl.init({ buffer_lines = 40, tick_ms = 5, task_period_ms = 10 })
local scr = lvgl.create_screen()
scr:set_style({ bg_color = "#000000" })

local eyes = lvgl.emotive_eyes(scr, {
  x = 0,
  y = 0,
  w = 320,
  h = 240,
  auto_gaze = true,
  auto_blink = true,
})
scr:load()

local names = {
  "normal", "angry", "glee", "happy", "sad", "worried", "focused",
  "annoyed", "surprised", "skeptic", "frustrated", "unimpressed",
  "sleepy", "suspicious", "squint", "furious", "scared", "awe", "coquette",
}

local index = 1
local ticks = 0
while true do
  eyes:update()
  lvgl.process_events(0)
  delay.delay_ms(25)
  ticks = ticks + 1
  if ticks >= 60 then
    ticks = 0
    eyes:set_emotion(names[index])
    index = index % #names + 1
  end
end
