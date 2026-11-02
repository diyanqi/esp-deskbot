# API 契约

本文档描述三类接口：

1. **IoTDA 推送回调**：华为云 IoTDA 主动调用本服务，写入数据。
2. **只读查询 API**：业务侧查询设备、最新值、时序数据、聚合序列、原始事件、家长端统计和标注画面。
3. **演示 / AI 接口**：生成 AI 健康报告、注入演示场景（仅演示环境开启）。

Base URL 示例：`https://116.62.55.142:11294`。

---

## 1. 通用约定

### 1.1 编码与时间

- 请求/响应均为 `application/json; charset=utf-8`。
- 时间字段使用 RFC3339，服务端统一按 **UTC** 返回，例如 `2026-11-02T09:00:00Z`。
- 查询参数中的 `from`、`to` 接受：
  - RFC3339：`2026-11-02T09:00:00Z`
  - 日期：`2026-11-02`（等价于当天 00:00:00 UTC）
  - IoTDA 时间格式：`20261102T090000Z`

### 1.2 分页

列表接口使用 `limit` / `offset`：

| 参数 | 默认 | 最大 | 说明 |
|---|---:|---:|---|
| `limit` | 100 | 1000 | 返回条数 |
| `offset` | 0 | - | 跳过条数 |

响应：

```json
{
  "data": [],
  "pagination": {
    "limit": 100,
    "offset": 0,
    "has_more": false,
    "total": 0
  }
}
```

`total` 仅设备列表返回；时序/事件列表通过 `has_more` 判断是否还有下一页。

### 1.3 错误格式

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "from must be before to",
    "request_id": "8f0c1e1a3d2b4c5d"
  }
}
```

| HTTP | code | 含义 |
|---:|---|---|
| 400 | `invalid_argument` | 参数或报文不合法 |
| 401 | `unauthorized` | 签名或 API Token 错误 |
| 403 | `demo_disabled` | 演示接口未开启 |
| 404 | `not_found` | 资源不存在 |
| 405 | `method_not_allowed` | HTTP 方法不支持 |
| 413 | `payload_too_large` | 请求体超过限制 |
| 500 | `internal` | 服务端错误 |
| 503 | `unavailable` | 数据库不可用 |

响应头 `X-Request-ID` 会返回请求 ID，便于排查。

---

## 2. IoTDA 推送回调

### 2.1 请求

```http
POST /data/push HTTP/1.1
Content-Type: application/json; charset=utf-8
timestamp: 1675654743514
nonce: 8b9b796d388d49bba43adaa53aaf5bc4
signature: 2ff821fb8a976ede7d06434395ec8c25e4100bff8b3d12d8099ef7e30b58bd4c
```

| Header | 必填 | 说明 |
|---|---|---|
| `timestamp` | 是 | IoTDA 推送时间戳，毫秒或秒 |
| `nonce` | 是 | 平台随机数 |
| `signature` | 是 | 签名，见 2.2 |
| `Content-Type` | 是 | `application/json; charset=utf-8` |

### 2.2 签名

IoTDA 在控制台勾选“鉴权”并填写 Token 后，用以下规则生成 `signature`：

1. 取 `token`、`timestamp`、`nonce` 三个字符串；
2. 按字典序排序；
3. 直接拼接（无分隔符）；
4. 对拼接结果计算 SHA-256，输出小写十六进制。

```text
signature = hex(sha256(sort([token, timestamp, nonce]).join("")))
```

示例（来自华为云文档）：

```text
token     = aaaaaa
timestamp = 1675654743514
nonce     = 8b9b796d388d49bba43adaa53aaf5bc4
sorted    = 1675654743514 + 8b9b796d388d49bba43adaa53aaf5bc4 + aaaaaa
signature = 2ff821fb8a976ede7d06434395ec8c25e4100bff8b3d12d8099ef7e30b58bd4c
```

服务端使用常量时间比较；签名不正确返回 `401`。

### 2.3 时间窗

服务端默认接受 `timestamp` 与当前时间偏差不超过 `10m` 的请求（`IOTDA_SIGNATURE_MAX_AGE`）。设为 `0` 可关闭时效校验。请确保服务器时钟同步。

### 2.4 请求体

#### 设备属性上报

```json
{
  "resource": "device.property",
  "event": "report",
  "event_time": "20261102T090000Z",
  "event_time_ms": "2026-11-02T09:00:00.000Z",
  "request_id": "req-1",
  "notify_data": {
    "header": {
      "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
      "node_id": "hbot",
      "product_id": "6ae74bcd7f2e6c302fa9194e",
      "app_id": "app-1",
      "gateway_id": "6ae74bcd7f2e6c302fa9194e_hbot",
      "tags": [
        { "tag_key": "room", "tag_value": "office" }
      ]
    },
    "body": {
      "services": [
        {
          "service_id": "Environment",
          "properties": {
            "temperature": 0,
            "humidity": 0,
            "light": 62,
            "sound": 0
          },
          "event_time": "20261102T090000Z"
        },
        {
          "service_id": "Posture",
          "properties": {
            "personDetected": 1,
            "postureStatus": "good",
            "postureSummary": "坐姿良好",
            "issueCount": 0,
            "alert": 0,
            "detecting": 0,
            "lastSeq": 17
          },
          "event_time": "20261102T090000Z"
        }
      ]
    }
  }
}
```

每个 `properties` 中的键都会写入一条 `telemetry`：

- JSON number → `value_type=number`，同时写入 `value_num`
- JSON string → `value_type=string`
- JSON boolean → `value_type=boolean`
- JSON object/array → `value_type=object/array`，原样保存在 `value_json`
- JSON null → `value_type=null`

`event_time` 优先级：service 的 `event_time` → 顶层 `event_time` → `event_time_ms` → 服务端接收时间。

#### 设备状态变更

```json
{
  "resource": "device.status",
  "event": "update",
  "event_time": "20261102T090000Z",
  "notify_data": {
    "header": { "device_id": "6ae74bcd7f2e6c302fa9194e_hbot" },
    "body": {
      "status": "ONLINE",
      "last_online_time": "20261102T090000Z",
      "status_update_time": "2026-11-02T09:00:00.815Z"
    }
  }
}
```

`status` 取值：`ONLINE` / `OFFLINE` / `ABNORMAL` / `FROZEN`。服务端会更新 `devices.status`、`status_updated_at`、`last_online_at`。

#### 其他资源

`device.message`、`device.message.status`、批量任务等资源也会被原样保存到 `iotda_events`，但不会归一化为 telemetry。

### 2.5 响应

成功：

```json
{
  "status": "ok",
  "inserted": true
}
```

- `inserted=true`：新事件已写入。
- `inserted=false`：相同报文已处理过（`sha256(raw body)` 去重），不会重复写入 telemetry。

IoTDA 只把 HTTP 200 视为成功。服务端在 15 秒内返回；非 2xx 会被平台判定为推送失败并可能重试。连续失败 10 次后平台会把目标地址加入黑名单，请保证数据库与网络稳定。

---

## 3. 查询 API

### 3.1 鉴权

设置 `API_TOKEN` 后，所有 `/api/v1/*` 请求必须携带：

```http
Authorization: Bearer <API_TOKEN>
```

`/healthz`、`/readyz` 和 `/data/push` 不使用该 Token。

浏览器跨域访问时，后端根据 `CORS_ALLOW_ORIGINS`（逗号分隔的 Origin）返回 CORS 头，并处理 `OPTIONS` 预检。生产环境建议使用明确的 Origin 列表，而不是 `*`。

### 3.2 设备列表

```http
GET /api/v1/devices?limit=100&offset=0&search=hbot&status=ONLINE
```

| 参数 | 说明 |
|---|---|
| `search` | 对 `device_id`、`node_id` 做模糊匹配 |
| `status` | 精确匹配设备状态 |

响应：

```json
{
  "data": [
    {
      "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
      "node_id": "hbot",
      "product_id": "6ae74bcd7f2e6c302fa9194e",
      "app_id": "app-1",
      "gateway_id": "6ae74bcd7f2e6c302fa9194e_hbot",
      "status": "ONLINE",
      "status_updated_at": "2026-11-02T09:00:00Z",
      "last_online_at": "2026-11-02T09:00:00Z",
      "first_seen_at": "2026-11-02T08:00:00Z",
      "last_seen_at": "2026-11-02T09:00:00Z",
      "tags": [{ "tag_key": "room", "tag_value": "office" }],
      "created_at": "2026-11-02T08:00:00Z",
      "updated_at": "2026-11-02T09:00:00Z"
    }
  ],
  "pagination": { "limit": 100, "offset": 0, "has_more": false, "total": 1 }
}
```

### 3.3 设备详情

```http
GET /api/v1/devices/{device_id}
```

响应：

```json
{ "data": { "...": "同上" } }
```

不存在返回 `404 not_found`。

### 3.4 最新值

```http
GET /api/v1/devices/{device_id}/latest?service_id=Environment&property=light
```

`service_id`、`property` 均可选。返回每个 `(service_id, property)` 最新一条：

```json
{
  "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
  "data": [
    {
      "id": 22,
      "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
      "service_id": "Environment",
      "property": "light",
      "value": 62,
      "value_type": "number",
      "value_num": 62,
      "event_time": "2026-11-02T09:00:00Z",
      "received_at": "2026-11-02T09:00:01Z",
      "request_id": "req-1"
    }
  ]
}
```

### 3.5 时序数据

```http
GET /api/v1/devices/{device_id}/telemetry?service_id=Posture&property=alert&from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z&limit=100&offset=0
```

跨设备查询使用：

```http
GET /api/v1/telemetry?device_id=...&service_id=...&property=...
```

参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| `device_id` | 路径或查询参数 | 设备 ID；`/api/v1/telemetry` 通过查询参数传入 |
| `service_id` | 否 | 服务过滤 |
| `property` | 否 | 属性过滤 |
| `from` / `to` | 否 | 时间范围，左闭右开 |
| `limit` / `offset` | 否 | 分页 |

按 `event_time DESC, id DESC` 返回。

### 3.6 聚合序列

```http
GET /api/v1/devices/{device_id}/series?service_id=Environment&property=light&from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z&bucket=hour
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `service_id` | 是 | 服务 |
| `property` | 是 | 数值属性 |
| `from` / `to` | 否 | 默认最近 24 小时 |
| `bucket` | 否 | `minute` / `hour` / `day` / `week`，默认 `hour` |

仅对 `value_num` 非空的属性聚合：

```json
{
  "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
  "service_id": "Environment",
  "property": "light",
  "bucket": "hour",
  "from": "2026-11-01T00:00:00Z",
  "to": "2026-11-02T00:00:00Z",
  "data": [
    { "bucket": "2026-11-02T09:00:00Z", "count": 3, "avg": 61.3, "min": 60, "max": 62 }
  ]
}
```

### 3.7 原始事件

```http
GET /api/v1/events?device_id=...&resource=device.property&event=report&from=...&to=...&include_raw=true&limit=100
```

| 参数 | 说明 |
|---|---|
| `device_id` | 设备过滤 |
| `resource` | `device.property`、`device.status`、`device.message` 等 |
| `event` | `report`、`update` 等 |
| `from` / `to` | 按 `COALESCE(event_time, received_at)` 过滤 |
| `include_raw` | `true` 时返回完整原始报文 `raw_payload` |

```json
{
  "data": [
    {
      "id": 4,
      "resource": "device.property",
      "event": "report",
      "request_id": "req-1",
      "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
      "event_time": "2026-11-02T09:00:00Z",
      "event_time_ms": "2026-11-02T09:00:00Z",
      "body": { "services": ["..."] },
      "received_at": "2026-11-02T09:00:01Z"
    }
  ],
  "pagination": { "limit": 100, "offset": 0, "has_more": false }
}
```

### 3.8 家长端聚合 insights

```http
GET /api/v1/insights?device_id=6ae74bcd7f2e6c302fa9194e_hbot&hours=24
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `device_id` | 是 | 设备 ID |
| `hours` | 否 | 汇总窗口，1–168，默认 24 |

返回一次性聚合结果，供前端成长评分、7×24 热力图和 24h 回放使用：

```json
{
  "data": {
    "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
    "from": "2026-11-01T09:00:00Z",
    "to": "2026-11-02T09:00:00Z",
    "hours": 24,
    "has_data": true,
    "health_score": 75,
    "summary": {
      "posture_samples": 288,
      "good": 202,
      "warning": 42,
      "bad": 20,
      "unknown": 24,
      "good_rate": 0.701,
      "alert_rate": 0.215,
      "person_rate": 0.917,
      "alert_minutes": 62,
      "alert_events": 30,
      "sound_events": 23,
      "avg_light": 45.3,
      "min_light": 10,
      "max_light": 80,
      "light_comfort": 0.756,
      "avg_rssi": -56.0,
      "avg_heap_kb": 174.8,
      "longest_bad_streak_minutes": 10
    },
    "hourly": [
      { "bucket": "2026-11-02T08:00:00Z", "good_rate": 0.8, "alert_rate": 0.1, "avg_light": 55, "samples": 12 }
    ],
    "heatmap": [
      { "weekday": 1, "hour": 8, "good_rate": 0.75, "alert_rate": 0.17, "avg_light": 54, "samples": 12 }
    ],
    "timeline": [
      { "time": "2026-11-02T08:00:00Z", "status": "good", "alert": false, "light": 55, "sound": false, "person": true }
    ],
    "risk_windows": [
      { "weekday": 1, "hour": 4, "good_rate": 0.5, "alert_rate": 0.5, "avg_light": 19, "samples": 12 }
    ]
  }
}
```

说明：

- `summary` 按 `hours` 窗口统计；`heatmap` 固定聚合最近 7 天；`weekday` 为 1=周一 … 7=周日。
- `timeline` 为 5 分钟粒度，最多 288 个点（24 小时），供回放使用。
- `health_score` 由良好率、提醒频率、光照舒适度和在座时长综合计算，范围 0–100。
- `alert_events` 按时间轴中 `alert` 从 false→true 的次数统计；`longest_bad_streak_minutes` 为连续 `status=bad` 的最长分钟数。

### 3.9 AI 健康报告

```http
POST /api/v1/ai/report
Authorization: Bearer <API_TOKEN>
Content-Type: application/json

{ "device_id": "6ae74bcd7f2e6c302fa9194e_hbot", "hours": 24 }
```

服务端会重新聚合 insights，并将精简统计发送给 OpenAI 兼容接口（默认 `gpt-4o`）。返回结构化报告：

```json
{
  "data": {
    "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
    "hours": 24,
    "score": 70,
    "level": "good",
    "headline": "孩子的坐姿表现良好，但仍有提升空间。",
    "summary": "过去 24 小时……",
    "findings": [
      { "title": "警示事件频繁", "detail": "……", "severity": "warn" }
    ],
    "risk_windows": ["周一 04:00-05:00", "周二 02:00-03:00"],
    "suggestions": ["每 40 分钟提醒孩子起身活动……"],
    "encouragement": "继续保持努力！",
    "disclaimer": "本报告由数据统计和 AI 生成，仅供参考，不构成医疗建议。",
    "source": "ai",
    "model": "gpt-4o",
    "generated_at": "2026-11-02T09:00:05Z"
  }
}
```

- `source=ai` 表示模型生成；上游超时/报错或 `AI_ENABLED=false` 时返回 `source=local`，内容由本地规则生成，接口仍然可用。
- `AI_API_KEY` 为空时不发送 `Authorization` 头，适配本项目提供的演示接口。
- AI 请求最长约 45 秒（`AI_TIMEOUT`），因此 `HTTP_WRITE_TIMEOUT` 默认提高到 60s。
- 不要把生产 API Key 写进前端；前端只调用本服务。

### 3.10 标注画面帧

本地识别服务（`../server.py`）生成 `_annotated.jpg` 后自动上传，前端通过本服务读取并展示。

#### 上传

```http
POST /api/v1/frames
Authorization: Bearer <API_TOKEN>
Content-Type: multipart/form-data
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `image`（或 `file`） | 是 | JPEG/PNG 标注图，最大 10 MiB |
| `device_id` | 是 | 设备 ID |
| `captured_at` | 否 | RFC3339 拍摄时间，默认当前时间 |
| `status` | 否 | `good` / `warning` / `bad` / `unknown` |
| `summary` | 否 | 识别摘要 |
| `person_detected` | 否 | `true` / `false` / `1` / `0` |
| `request_id` | 否 | 本地文件名或请求 ID |
| `metadata` | 否 | JSON 字符串，例如 `{"issues":[],"keypoints_count":33}` |

返回 `201`：

```json
{
  "data": {
    "id": 12,
    "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
    "captured_at": "2026-11-02T09:00:00Z",
    "status": "warning",
    "summary": "轻微偏移",
    "person_detected": 1,
    "image_mime": "image/jpeg",
    "image_url": "/api/v1/frames/12/image",
    "created_at": "2026-11-02T09:00:01Z"
  }
}
```

每台设备只保留最新 80 帧，旧帧自动清理。

#### 查询

```http
GET /api/v1/frames?device_id=...&limit=12
GET /api/v1/frames/latest?device_id=...
GET /api/v1/frames/{frame_id}/image
```

- 列表接口返回最近帧元数据（不含图片二进制）。
- 图片接口返回 `image/jpeg` 等原始二进制；前端使用带 `Authorization` 的 `fetch` 取 Blob 后展示，避免把 Token 拼进 `<img>` URL。
- 帧不存在返回 `404 not_found`。

### 3.11 演示场景注入

```http
POST /api/v1/demo/scenario
Authorization: Bearer <API_TOKEN>
Content-Type: application/json

{ "device_id": "6ae74bcd7f2e6c302fa9194e_hbot", "status": "bad", "light": 35, "sound": 0 }
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `device_id` | 是 | 设备 ID |
| `status` | 是 | `good` / `warning` / `bad` / `away` |
| `light` | 否 | 光照，默认 60 |
| `sound` | 否 | 声音，默认 0 |

服务端会构造一条标准 `device.property` 报文，经过与真实 IoTDA 推送相同的 `Parse → Ingest` 链路写入 `telemetry` 和 `iotda_events`，因此前端轮询到的不是假状态，而是真实数据库变化。

- 需要 `DEMO_ENABLED=true`，否则返回 `403 demo_disabled`。
- 仅建议演示环境开启；生产环境请关闭。

### 3.12 家庭留言

家长/孩子在三个客户端发送留言，ESP 通过 HTTP 拉取并显示在屏幕上。所有接口都需要 Bearer Token。

#### 发送留言

```http
POST /api/v1/messages
Authorization: Bearer <API_TOKEN>
Content-Type: application/json

{
  "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
  "author": "parent",
  "author_name": "妈妈",
  "content": "坐直一点，妈妈在看着你哦。"
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `device_id` | 是 | 目标设备 |
| `author` | 是 | `parent` 或 `child` |
| `author_name` | 否 | 默认 `家长` / `孩子`，最多 40 字符 |
| `content` | 是 | 1–500 字符 |

返回 `201`：

```json
{
  "data": {
    "id": 12,
    "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
    "author": "parent",
    "author_name": "妈妈",
    "content": "坐直一点，妈妈在看着你哦。",
    "created_at": "2026-11-02T05:57:00Z"
  }
}
```

#### 客户端留言列表

```http
GET /api/v1/messages?device_id=...&limit=30
```

返回最近留言（按创建时间倒序）；`delivered_at` 非空表示 ESP 已拉取并确认显示。

#### ESP 拉取未送达留言

```http
GET /api/v1/device/messages?device_id=...
```

返回 `delivered_at IS NULL` 的留言（最多 20 条，按时间正序）。ESP 的 `posture_worker` 每约 10 秒调用一次，将结果写入 `messages.json`，LVGL 表情屏显示最新留言。

#### ESP 确认送达

```http
POST /api/v1/device/messages/ack
Authorization: Bearer <API_TOKEN>
Content-Type: application/json

{ "ids": [12, 13] }
```

将对应留言的 `delivered_at`/`read_at` 置为当前时间，返回：

```json
{ "updated": 2 }
```

ESP 必须先成功写入本地 `messages.json` 再 ack；写入失败时不 ack，下一轮会重新拉取。

#### ESP 配置

ESP 的 DATA 根目录下 `config/messages.json`：

```json
{
  "base_url": "http://192.168.50.162:11294",
  "token": "test-api-token",
  "device_id": "6ae74bcd7f2e6c302fa9194e_hbot"
}
```

`base_url` 必须指向 ESP 能访问到的 Go 服务地址；`token` 与后端 `API_TOKEN` 一致。

### 3.13 久坐提醒上报约定

久坐计时在 ESP 端完成，不新增 IoTDA 产品模型属性。达到 30 分钟后，ESP 把普通属性上报中的：

- `postureStatus` 置为 `warning`
- `postureSummary` 写成 `久坐提醒：已连续坐 N 分钟，起来活动一下。`
- `alert` 置为 `1`
- `issueCount` 至少为 `1`

这样 Web / HarmonyOS / 小程序都能通过现有最新值和事件流显示“久坐提醒”，无需重新导入产品模型。ESP 端使用连续 4 个有效人体样本（约 20 秒）确认在座、连续 4 个缺失样本确认离开；短暂离座（<120 秒）只暂停计时，超过 120 秒才重置，降低单帧误判的影响。

---

## 4. 限制与版本

- 推送体最大 5 MiB（`HTTP_MAX_BODY_BYTES`）。
- 标注帧最大 10 MiB；每台设备保留最新 80 帧。
- 查询 `limit` 最大 1000。
- IoTDA 推送处理需在 15 秒内返回 200；`HTTP_WRITE_TIMEOUT` 默认 60s 是为了容纳最长 45s 的 AI 报告请求，推送处理器本身仍然快速返回。
- AI 报告生成受 `AI_TIMEOUT`（默认 45s）限制；失败时自动降级为本地规则报告。
- API 路径版本前缀为 `/api/v1`；不兼容变更会引入 `/api/v2`。
