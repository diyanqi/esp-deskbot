-- Background posture worker: capture, upload, parse, publish posture and
-- family messages, and maintain a debounced sitting timer.
--
-- The sitting timer is intentionally conservative:
--   * a sample only counts when a person is detected AND the posture model
--     returned a real status (good/warning/bad), so isolated false positives
--     from a chair or a background object do not start the timer;
--   * four consecutive valid samples (~20s) are required to become present;
--   * four consecutive misses (~20s) mark the child as away, but a short
--     absence (<120s) pauses rather than resets the timer;
--   * two minutes away resets the sitting session;
--   * 30 minutes of accumulated sitting triggers one reminder. The reminder is
--     included in postureSummary/alert so clients can display it without a
--     new IoTDA product-model property.

local camera_snapshot = require("camera_snapshot")
local capability = require("capability")
local delay = require("delay")
local json = require("json")
local storage = require("storage")
local sensors = require("sensors")

local UPLOAD_URL = "http://192.168.50.162:8000/upload"
local INTERVAL_MS = 5000
local ROOT = storage.get_root_dir()
local PHOTO = storage.join_path(ROOT, "posture.jpg")
local STATE = storage.join_path(ROOT, "posture_state.json")
local IOTDA_CONFIG = storage.join_path(ROOT, "config/iotda.json")
local MESSAGES_CONFIG = storage.join_path(ROOT, "config/messages.json")
local MESSAGES_STATE = storage.join_path(ROOT, "messages.json")

local PRESENCE_ON_SAMPLES = 4
local PRESENCE_OFF_SAMPLES = 4
local ABSENCE_RESET_SECONDS = 120
local SEDENTARY_SECONDS = 30 * 60

local function write_state(tbl)
    local ok, text = pcall(json.encode, tbl)
    if ok and type(text) == "string" then
        pcall(storage.write_file, STATE, text)
    else
        print("[worker] state encode failed: " .. tostring(text))
    end
end

local function parse_response(response)
    if type(response) ~= "string" then
        return nil, "no response"
    end
    local code_text, body = string.match(response, "^HTTP (%d+)[^\n]*\n(.*)$")
    if not code_text then
        return nil, response
    end
    local code = tonumber(code_text)
    if code ~= 200 then
        return nil, "HTTP " .. tostring(code) .. ": " .. tostring(body)
    end
    if not body or body == "" then
        return nil, "empty body"
    end
    local ok, data = pcall(json.decode, body)
    if not ok or type(data) ~= "table" then
        return nil, "bad json"
    end
    return data, nil
end

local function capture_and_upload()
    local ok, bytes = pcall(camera_snapshot.snapshot, PHOTO, {
        width = 320,
        height = 240,
        quality = 14,
    })
    if not ok then
        pcall(storage.remove, PHOTO)
        return nil, "拍照失败: " .. tostring(bytes)
    end

    local last_err = "上传失败"
    -- Two short attempts keep the loop responsive when the LAN analysis
    -- server is offline, so IoTDA still receives regular reports.
    for attempt = 1, 2 do
        local call_ok, response, call_err = capability.call("http_request", {
            url = UPLOAD_URL,
            method = "POST",
            headers = { ["Content-Type"] = "image/jpeg" },
            body_file = PHOTO,
            timeout_ms = 5000,
            max_body_bytes = 16384,
        })
        if call_ok then
            local data, err = parse_response(response)
            if data then
                pcall(storage.remove, PHOTO)
                return data, nil
            end
            last_err = tostring(err)
        else
            last_err = tostring(call_err or response or "上传失败")
        end
        print(string.format("[worker] upload attempt %d failed: %s", attempt, last_err))
        delay.delay_ms(300)
    end
    pcall(storage.remove, PHOTO)
    return nil, last_err
end

-- ---------------------------------------------------------------------------
-- Family messages: poll the Go server, persist for the face UI, then ack.
-- ---------------------------------------------------------------------------
local function read_message_config()
    local ok, text = pcall(storage.read_file, MESSAGES_CONFIG)
    if not ok or type(text) ~= "string" or text == "" then
        return nil
    end
    local decode_ok, config = pcall(json.decode, text)
    if not decode_ok or type(config) ~= "table" then
        return nil
    end
    if type(config.base_url) ~= "string" or config.base_url == "" then
        return nil
    end
    if type(config.device_id) ~= "string" or config.device_id == "" then
        local iotda_ok, iotda_text = pcall(storage.read_file, IOTDA_CONFIG)
        if iotda_ok and type(iotda_text) == "string" then
            local iotda_decode_ok, iotda_config = pcall(json.decode, iotda_text)
            if iotda_decode_ok and type(iotda_config) == "table" then
                config.device_id = iotda_config.device_id
            end
        end
    end
    if type(config.device_id) ~= "string" or config.device_id == "" then
        return nil
    end
    return config
end

local function poll_messages(config)
    if not config then
        return
    end
    local base_url = tostring(config.base_url):gsub("/+$", "")
    local token = type(config.token) == "string" and config.token or ""
    local device_id = tostring(config.device_id)
    local url = base_url .. "/api/v1/device/messages?device_id=" .. device_id
    local call_ok, response, call_err = capability.call("http_request", {
        url = url,
        method = "GET",
        headers = {
            ["Authorization"] = "Bearer " .. token,
            ["Accept"] = "application/json",
        },
        timeout_ms = 3000,
        max_body_bytes = 8192,
    })
    if not call_ok then
        print("[messages] poll failed: " .. tostring(call_err or response))
        return
    end
    local data, err = parse_response(response)
    if not data or type(data.data) ~= "table" then
        if err then
            print("[messages] poll parse failed: " .. tostring(err))
        end
        return
    end
    local messages = data.data
    if #messages == 0 then
        return
    end

    local encoded_ok, encoded = pcall(json.encode, {
        updated_at = os.time and os.time() or 0,
        messages = messages,
    })
    if encoded_ok then
        local write_ok, write_err = pcall(storage.write_file, MESSAGES_STATE, encoded)
        if not write_ok then
            print("[messages] persist failed: " .. tostring(write_err))
            return
        end
    else
        print("[messages] encode failed: " .. tostring(encoded))
        return
    end

    local ids = {}
    for i = 1, #messages do
        local id = tonumber(messages[i].id)
        if id then
            ids[#ids + 1] = id
        end
    end
    if #ids == 0 then
        return
    end
    local ack_body = json.encode({ ids = ids })
    local ack_ok, ack_response, ack_err = capability.call("http_request", {
        url = base_url .. "/api/v1/device/messages/ack",
        method = "POST",
        headers = {
            ["Authorization"] = "Bearer " .. token,
            ["Content-Type"] = "application/json",
            ["Accept"] = "application/json",
        },
        body = ack_body,
        timeout_ms = 3000,
        max_body_bytes = 4096,
    })
    if not ack_ok then
        print("[messages] ack failed: " .. tostring(ack_err or ack_response))
    end
end

-- ---------------------------------------------------------------------------
-- Huawei Cloud IoTDA
-- ---------------------------------------------------------------------------
local ok_iotda, iotda_or_err = pcall(require, "iotda")
local iotda = ok_iotda and iotda_or_err or nil
if not ok_iotda then
    print("[iotda] module unavailable: " .. tostring(iotda_or_err))
end

local iotda_active = false
if iotda then
    local read_ok, text = pcall(storage.read_file, IOTDA_CONFIG)
    if read_ok and type(text) == "string" and text ~= "" then
        local decode_ok, config = pcall(json.decode, text)
        if decode_ok and type(config) == "table" then
            local start_ok, started, start_err = pcall(iotda.start, config)
            iotda_active = start_ok and started == true
            if not iotda_active then
                print("[iotda] start failed: " .. tostring(start_err or started))
            end
        else
            print("[iotda] invalid config JSON")
        end
    else
        print("[iotda] config not found: " .. IOTDA_CONFIG)
    end
end

local last_env = nil

-- ---------------------------------------------------------------------------
-- Debounced sitting timer
-- ---------------------------------------------------------------------------
local sitting = {
    seconds = 0,
    absent_seconds = 0,
    present = false,
    alerted = false,
}
local presence_on = 0
local presence_off = 0
local last_sample_time = nil

local function sitting_snapshot()
    return {
        seconds = math.floor(sitting.seconds),
        minutes = math.floor(sitting.seconds / 60),
        present = sitting.present,
        sedentary = sitting.alerted,
        absent_seconds = math.floor(sitting.absent_seconds),
    }
end

local function number_or(value, fallback)
    local number = tonumber(value)
    if number == nil then
        return fallback
    end
    return number
end

local function update_sitting(data)
    local now = nil
    if os.time then
        local candidate = os.time()
        if type(candidate) == "number" and candidate > 1700000000 then
            now = candidate
        end
    end
    local delta = INTERVAL_MS / 1000
    if now and last_sample_time and now > last_sample_time then
        delta = math.min(15, math.max(0, now - last_sample_time))
    end
    if now then
        last_sample_time = now
    end

    local person = data ~= nil and data.person_detected == true
    local posture = (data and data.posture) or {}
    local status = posture.status or "unknown"
    local valid = person and (status == "good" or status == "warning" or status == "bad")

    if valid then
        presence_on = presence_on + 1
        presence_off = 0
        if presence_on >= PRESENCE_ON_SAMPLES then
            sitting.present = true
        end
    else
        presence_on = 0
        presence_off = presence_off + 1
        if presence_off >= PRESENCE_OFF_SAMPLES then
            sitting.present = false
        end
    end

    if sitting.present then
        sitting.absent_seconds = 0
        sitting.seconds = sitting.seconds + delta
        if sitting.seconds >= SEDENTARY_SECONDS and not sitting.alerted then
            sitting.alerted = true
        end
    else
        sitting.absent_seconds = sitting.absent_seconds + delta
        if sitting.absent_seconds >= ABSENCE_RESET_SECONDS then
            sitting.seconds = 0
            sitting.alerted = false
            sitting.absent_seconds = 0
            presence_on = 0
            presence_off = 0
        end
    end
    return sitting_snapshot()
end

local function publish_state(phase, seq, env, data, err, sitting_state)
    if not iotda_active then
        return
    end

    local e = env or last_env or {}
    last_env = e
    local posture = (data and data.posture) or {}
    local status = posture.status or "unknown"
    if status ~= "good" and status ~= "warning" and status ~= "bad" then
        status = "unknown"
    end
    local summary = posture.summary
    if (summary == nil or summary == "") and err ~= nil then
        summary = tostring(err)
    end
    local issues = (posture.issues and #posture.issues) or 0
    local person = data ~= nil and data.person_detected == true
    local alert = status == "bad" or status == "warning"

    if sitting_state and sitting_state.sedentary then
        status = "warning"
        summary = string.format("久坐提醒：已连续坐 %d 分钟，起来活动一下。", sitting_state.minutes or 0)
        alert = true
        if issues < 1 then
            issues = 1
        end
    end

    local services = {
        {
            service_id = "Environment",
            properties = {
                -- The DHT sensor is currently unavailable; report 0 as requested.
                temperature = 0,
                humidity = 0,
                light = number_or(e.light_pct, 0),
                sound = number_or(e.sound, 0),
            },
        },
        {
            service_id = "Posture",
            properties = {
                personDetected = person and 1 or 0,
                postureStatus = status,
                postureSummary = summary or "",
                issueCount = issues,
                alert = alert and 1 or 0,
                detecting = (phase == "sampling") and 1 or 0,
                lastSeq = seq,
            },
        },
    }

    local ok, result, publish_err = pcall(iotda.publish, services, alert or phase == "done")
    if not ok then
        print("[iotda] publish error: " .. tostring(result))
    elseif result ~= true and publish_err ~= "throttled" then
        print("[iotda] publish skipped: " .. tostring(publish_err))
    end
end

-- Give the face script time to initialise the display before the first capture.
delay.delay_ms(3000)

local message_config = read_message_config()
if message_config then
    print("[messages] polling " .. tostring(message_config.base_url) ..
        " for device " .. tostring(message_config.device_id))
else
    print("[messages] config not found or incomplete; message polling disabled")
end

local ticker = delay.periodic(INTERVAL_MS)
local seq = 0

while true do
    seq = seq + 1
    write_state({ seq = seq, phase = "sampling" })
    publish_state("sampling", seq, nil, nil, nil, nil)
    -- Let the face show the "thinking" expression before the camera pauses LVGL.
    delay.delay_ms(600)

    local env = sensors.read()
    local data, err = capture_and_upload()
    local sitting_state = update_sitting(data)
    if data then
        write_state({
            seq = seq,
            phase = "done",
            ok = true,
            data = data,
            env = env,
            sitting = sitting_state,
        })
        publish_state("done", seq, env, data, nil, sitting_state)
    else
        write_state({
            seq = seq,
            phase = "done",
            ok = false,
            error = tostring(err),
            env = env,
            sitting = sitting_state,
        })
        publish_state("done", seq, env, nil, err, sitting_state)
    end

    -- Poll every other working cycle. The request has a short timeout and
    -- never blocks posture reporting for long.
    if seq % 2 == 0 then
        pcall(poll_messages, message_config)
    end

    collectgarbage("collect")
    ticker:wait()
end
