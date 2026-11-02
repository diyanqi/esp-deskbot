# Lua Camera Snapshot

One-shot OV2640 JPEG snapshot backed by Espressif's `esp32-camera` driver.

This module exists for boards whose DVP camera has its own clock oscillator
(for example a self-clocked 24 MHz OV2640 module), where the V4L2/`esp_video`
path may fail to lock onto the JPEG frame stream. It initializes the sensor,
grabs one frame, writes it as JPEG, and deinitializes the driver again.

## Usage

```lua
local camera_snapshot = require("camera_snapshot")

local bytes = camera_snapshot.snapshot("/fatfs/capture.jpg", {
  width = 320,
  height = 240,
  quality = 12,
})
print("saved", bytes)
```

- `width` / `height`: `320x240` (default) or `640x480`.
  `320x240` is recommended for self-clocked OV2640 modules.
- `quality`: JPEG quality, `0..63` (default `12`).

## Board configuration

The module reads the DVP pin map and XCLK frequency from the board manager's
`camera` device configuration, and reuses the board-managed I2C bus for SCCB.
Keep the board camera device at `init_skip: true` so `esp_video` does not claim
the DVP pins first.
