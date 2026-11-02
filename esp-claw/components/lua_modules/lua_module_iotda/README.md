# Lua IoTDA

Huawei Cloud IoTDA MQTTS client. The module builds the IoTDA CONNECT
credentials, reports service properties and events, and handles the
`SET_REPORT_INTERVAL` / `TRIGGER_REMINDER` commands.

## Usage

```lua
local iotda = require("iotda")

local ok, err = iotda.start({
    device_id = "<device_id>",
    secret = "<device_secret>",
    host = "<instance>.st1.iotda-device.cn-north-4.myhuaweicloud.com",
    port = 8883,
    client_id = "<device_id>_0_0_<YYYYMMDDHH>", -- optional
    report_interval = 5,
    keepalive = 120,
})
if not ok then
    print("iotda start failed: " .. tostring(err))
end
```

`client_id` is optional. When omitted, the module builds
`{device_id}_0_0_{UTC YYYYMMDDHH}`. The password is always
`HMAC-SHA256(key = timestamp, message = secret)` in lowercase hex.

## Properties

```lua
iotda.publish({
    {
        service_id = "Environment",
        properties = {
            temperature = 0,
            humidity = 0,
            light = 62,
            sound = 0,
        },
    },
    {
        service_id = "Posture",
        properties = {
            personDetected = 1,
            postureStatus = "good",
            postureSummary = "坐姿良好",
            issueCount = 0,
            alert = 0,
            detecting = 0,
            lastSeq = 17,
        },
    },
}, true) -- optional second argument: force (skip report_interval throttling)
```

Every report automatically appends a `DeviceStatus` service with
`wifiConnected`, `rssi`, `uptime`, `freeHeap`, `reportInterval`, `firmware`
and `lastReport`.

The module returns `true, msg_id` when the message is queued,
`false, "throttled"` when the report interval has not elapsed, and
`false, reason` when it is not connected.

## Events

```lua
iotda.publish_event("Posture", "posture_alert", {
    status = "bad",
    summary = "请调整坐姿",
})
```

The event is sent to `$oc/devices/{device_id}/sys/events/up`.

## Commands

Subscribe is automatic after CONNECT. Commands are parsed from
`$oc/devices/{device_id}/sys/commands/#`:

| Command | Behavior |
|---|---|
| `SET_REPORT_INTERVAL` | Clamp `paras.interval` to 5..3600 s and update throttling |
| `TRIGGER_REMINDER` | Set a pending reminder consumed by `take_reminder()` |

```lua
if iotda.take_reminder() then
    -- show the local rest reminder
end
```

## Other methods

- `iotda.is_connected()` -> boolean
- `iotda.status()` -> table with `started`, `connected`, `report_interval`,
  `queued`, `published`, `errors`, `device_id`, `client_id`, `last_error`
- `iotda.set_report_interval(seconds)` -> clamped interval
- `iotda.get_report_interval()` -> seconds
- `iotda.stop()` -> true

## Configuration

The device reads connection parameters from the DATA root's
`config/iotda.json`. See `application/edge_agent/iotda/iotda.example.json`
and the product model in `application/edge_agent/iotda/` for the complete
property and command definitions.
