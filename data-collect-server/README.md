# IoTDA Data Collect Server

Go 后端服务，用于接收华为云 IoTDA 的 HTTP/HTTPS 数据转发，将设备属性、设备状态等事件写入外部 PostgreSQL，并提供只读查询 API、家长端统计（insights）、AI 健康报告、标注画面存储、家庭留言和演示场景注入。

- 接收端：`POST /data/push`
- 查询端：`GET /api/v1/...`
- 家长端聚合：`GET /api/v1/insights`
- AI 健康报告：`POST /api/v1/ai/report`（OpenAI 兼容接口，默认 `gpt-4o`）
- 摄像头标注帧：`POST/GET /api/v1/frames...`
- 家庭留言：`POST/GET /api/v1/messages` + ESP 设备拉取/确认接口
- 演示注入：`POST /api/v1/demo/scenario`（需 `DEMO_ENABLED=true`）
- 存储：PostgreSQL（`pgx/v5` + `pgxpool`）
- TLS：支持直接在 `:11294` 上提供 HTTPS，也支持由 Nginx/Caddy 终止 TLS
- Go：`1.27.3`（最新版本；构建流程包含 `govulncheck`）

完整接口契约见 [`docs/api.md`](docs/api.md) 和 [`docs/openapi.yaml`](docs/openapi.yaml)。

## 架构

```text
IoTDA 数据转发规则
        │  HTTPS POST /data/push
        │  Headers: timestamp / nonce / signature
        ▼
data-collect-server ──► PostgreSQL
        │                 ├─ iotda_events   原始推送（可审计/去重）
        │                 ├─ devices        设备与在线状态
        │                 ├─ telemetry      归一化属性值
        │                 ├─ posture_frames 本地识别的标注图（最新 80 帧/设备）
        │                 └─ messages       家长/孩子留言（ESP 拉取后确认送达）
        │
        ├──► GET  /api/v1/insights      24h 汇总 + 7×24 热力图 + 5 分钟回放
        ├──► POST /api/v1/ai/report     gpt-4o 家长健康报告（失败时本地规则兜底）
        └──► GET  /api/v1/frames/...    前端展示标注画面
```

ESP 的 posture_worker 会每 10 秒左右轮询 `/api/v1/device/messages`，把留言写到本地
`messages.json` 供 LVGL 表情屏显示，然后调用 `/api/v1/device/messages/ack` 标记送达。
久坐提醒由 ESP 本地防抖计时后通过现有 `postureSummary/alert` 上报，不需要新增 IoTDA
产品模型属性。

本地识别服务 `server.py` 在生成 `_annotated.jpg` 后，会以 multipart 将图片、状态、摘要和关键点数量上传到 `/api/v1/frames`；该上传失败不会阻塞 ESP 拿识别结果。

## 快速开始

### 1. 依赖

- Go **1.27.3** 或更高
- PostgreSQL 14+（推荐 16+）
- 可选：Docker / Docker Compose

### 2. 创建数据库

```sql
CREATE DATABASE data_collect;
CREATE USER data_collect WITH PASSWORD 'change-me';
GRANT ALL PRIVILEGES ON DATABASE data_collect TO data_collect;
```

或使用本地临时 PostgreSQL：

```bash
createdb -h 127.0.0.1 -p 5432 -U postgres data_collect
```

### 3. 配置环境变量

本目录已经创建了本地 `.env`（已包含当前 IoTDA Token，已被 `.gitignore` 忽略）。
如果从模板重建：

```bash
cp .env.example .env
# 编辑 .env，至少填写 DATABASE_URL、IOTDA_PUSH_TOKEN、API_TOKEN
```

服务启动时默认会自动执行 `internal/store/migrations/` 中的迁移。

### 4. 运行

```bash
make run
# 或
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:11294/healthz
curl http://127.0.0.1:11294/readyz
```

### 5. 配置 IoTDA 数据转发

在 IoTDA 控制台进入 **规则 > 数据转发 > 创建规则**：

| 配置项 | 值 |
|---|---|
| 数据来源 | 设备属性（如需设备状态/消息，可再添加规则） |
| 触发事件 | 设备属性上报 |
| 转发目标 | 第三方应用服务（HTTP 推送） |
| 推送 URL | `https://<服务器公网 IP 或域名>:11294/data/push` |
| 鉴权 | 勾选 |
| Token | 与 `.env` 中的 `IOTDA_PUSH_TOKEN` 完全一致（3–32 位英文/数字） |

HTTPS 推送时，IoTDA 要求先在 **设备 > 设备证书 > CA 证书** 上传应用服务器 CA 证书。可以使用 `scripts/gen-dev-cert.sh` 生成自签名 CA 与服务器证书（调试用），生产环境建议使用商用证书或内部受信 CA。

### 6. 发送一条测试推送

服务本地运行时：

```bash
IOTDA_PUSH_TOKEN=你的Token BASE_URL=http://127.0.0.1:11294 ./scripts/send-test-push.sh
```

脚本会按 IoTDA 规则计算签名：`sha256(sort(token, timestamp, nonce).join(""))`。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `:11294` | HTTP/HTTPS 监听地址 |
| `TLS_CERT_FILE` | 空 | 服务器证书路径；与 `TLS_KEY_FILE` 同时设置时启用 HTTPS |
| `TLS_KEY_FILE` | 空 | 服务器私钥路径 |
| `IOTDA_PUSH_TOKEN` | 空 | IoTDA 转发 Token；为空时需设置 `DEV_ALLOW_UNSIGNED_PUSH=1`（仅开发） |
| `IOTDA_SIGNATURE_MAX_AGE` | `10m` | 签名时间戳允许的偏差；`0` 表示不校验时效 |
| `DEV_ALLOW_UNSIGNED_PUSH` | `false` | 允许无签名推送，仅本地调试 |
| `API_TOKEN` | 空 | 查询 API 的 Bearer Token；为空则不鉴权（会打印警告） |
| `CORS_ALLOW_ORIGINS` | 空 | 允许跨域访问查询 API 的浏览器 Origin，逗号分隔；`*` 仅建议本地演示 |
| `DATABASE_URL` | 必填 | PostgreSQL DSN，例如 `postgres://user:pass@host:5432/db?sslmode=require` |
| `DB_MAX_CONNS` | `10` | 连接池最大连接数 |
| `DB_MIN_CONNS` | `1` | 连接池最小连接数 |
| `DB_MAX_CONN_LIFETIME` | `30m` | 连接最大生命周期 |
| `DB_CONNECT_TIMEOUT` | `10s` | 建立连接超时 |
| `RUN_MIGRATIONS` | `true` | 启动时自动执行迁移 |
| `HTTP_READ_TIMEOUT` | `15s` | 读取请求超时 |
| `HTTP_WRITE_TIMEOUT` | `60s` | 写响应超时；AI 报告最长约 45s，需高于 `AI_TIMEOUT` |
| `HTTP_IDLE_TIMEOUT` | `60s` | keep-alive 空闲超时 |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | 优雅退出超时 |
| `HTTP_MAX_BODY_BYTES` | `5242880` | 推送体最大字节数 |
| `AI_ENABLED` | `true` | 是否启用 AI 健康报告；关闭后自动使用本地规则报告 |
| `AI_API_URL` | `https://ch.at/v1/chat/completions` | OpenAI 兼容 Chat Completions 地址 |
| `AI_MODEL` | `gpt-4o` | 模型名 |
| `AI_API_KEY` | 空 | 上游 API Key；为空时不发送 `Authorization`（本项目演示接口默认无鉴权） |
| `AI_TIMEOUT` | `45s` | AI 请求超时 |
| `DEMO_ENABLED` | `false` | 是否开放 `POST /api/v1/demo/scenario`；仅演示环境开启 |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_JSON` | `false` | `true` 时输出 JSON 日志 |

## TLS 方案

### 方案 A：Go 直接终止 TLS（推荐给单机部署）

```bash
./scripts/gen-dev-cert.sh 116.62.55.142
# 生成 certs/ca.crt、certs/server.crt、certs/server.key

export TLS_CERT_FILE=$PWD/certs/server.crt
export TLS_KEY_FILE=$PWD/certs/server.key
```

把 `certs/ca.crt` 上传到 IoTDA 的 **CA 证书** 页面，并将推送 URL 配置为：

```text
https://116.62.55.142:11294/data/push
```

生产环境请替换为正式 CA 签发的、包含该 IP/域名的服务器证书。

### 方案 B：反向代理终止 TLS

Go 只监听内网 HTTP：

```bash
LISTEN_ADDR=127.0.0.1:8080 ./server
```

由 Nginx/Caddy 监听 `:443` 或 `:11294`，把 `/data/push` 与 `/api/v1/` 反代到 `127.0.0.1:8080`。IoTDA 的推送 URL 指向代理的 HTTPS 地址即可。

## 数据库

迁移文件：`internal/store/migrations/0001_init.sql`、`0002_posture_frames.sql`、`0003_messages.sql`。

五张核心表：

- `iotda_events`：每一条 IoTDA 推送的原始 JSON、资源类型、设备标识、去重键（`sha256(raw body)`）。
- `devices`：设备维度信息、在线状态、最后上报时间、标签。
- `telemetry`：属性归一化后的时序数据，`value_json` 保留原 JSON 类型，`value_num` 用于数值查询与聚合。
- `posture_frames`：本地识别服务上传的标注 JPEG、状态、摘要、关键点元数据；每台设备保留最新 80 帧。
- `messages`：家长/孩子留言、作者身份、创建时间、ESP 送达/已读时间。

`iotda_events.dedupe_key` 有唯一约束：IoTDA 重试同一报文时不会重复写入，重复推送返回 `200 {"inserted":false}`。

## API 摘要

所有查询接口返回 JSON；`/api/v1/*` 在设置 `API_TOKEN` 后要求：

```http
Authorization: Bearer <API_TOKEN>
```

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 存活检查 |
| GET | `/readyz` | 数据库可用性检查 |
| GET | `/api/v1/devices` | 设备列表（`search`、`status`、分页） |
| GET | `/api/v1/devices/{device_id}` | 设备详情 |
| GET | `/api/v1/devices/{device_id}/latest` | 每个属性的最新值 |
| GET | `/api/v1/devices/{device_id}/telemetry` | 设备时序数据 |
| GET | `/api/v1/telemetry` | 跨设备时序数据 |
| GET | `/api/v1/devices/{device_id}/series` | 数值聚合序列（minute/hour/day/week） |
| GET | `/api/v1/events` | 原始推送事件（可含 `raw_payload`） |
| GET | `/api/v1/insights` | 24h 家长统计 + 7×24 热力图 + 5 分钟回放时间轴 |
| POST | `/api/v1/ai/report` | 生成家长端 AI 健康报告（失败自动降级为本地规则） |
| POST | `/api/v1/frames` | 上传标注帧（multipart，供本地识别服务使用） |
| GET | `/api/v1/frames` | 最近标注帧列表 |
| GET | `/api/v1/frames/latest` | 最新标注帧元数据 |
| GET | `/api/v1/frames/{frame_id}/image` | 标注帧 JPEG 二进制 |
| POST | `/api/v1/messages` | 家长/孩子发送留言 |
| GET | `/api/v1/messages` | 客户端留言列表 |
| GET | `/api/v1/device/messages` | ESP 拉取未送达留言 |
| POST | `/api/v1/device/messages/ack` | ESP 确认留言已显示 |
| POST | `/api/v1/demo/scenario` | 注入良好/偏移/告警/离开场景（`DEMO_ENABLED=true`） |

示例：

```bash
curl -H "Authorization: Bearer $API_TOKEN" \
  "http://127.0.0.1:11294/api/v1/devices/6ae74bcd7f2e6c302fa9194e_hbot/latest"

curl -H "Authorization: Bearer $API_TOKEN" \
  "http://127.0.0.1:11294/api/v1/devices/6ae74bcd7f2e6c302fa9194e_hbot/series?service_id=Environment&property=light&from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z&bucket=hour"
```

完整字段、分页、错误码和 IoTDA 推送契约见 [`docs/api.md`](docs/api.md)。

## 演示数据

后端自带一个可重复运行的 mock 数据生成器，方便在没有真实设备时演示前端：

```bash
make seed
# 默认：两台设备 × 7 天 × 5 分钟粒度（约 7.2 万条属性、800 条事件）
# 或指定数据库与时间范围
DATABASE_URL='postgres://user:pass@host:5432/data_collect?sslmode=disable' \
  go run ./cmd/seed -hours 24 -interval 1m
```

默认生成两台设备 7 天的数据（便于前端 7×24 热力图与 24h 回放），并写入几条家庭留言演示数据：

- `6ae74bcd7f2e6c302fa9194e_hbot`（正常坐姿为主，包含示例设备标签）
- `demo-desk-02`（坐姿偏移更多）

种子会先删除这两台设备的旧数据再写入；使用 `-device` 可指定其他设备 ID。真实设备数据与演示数据可以共存。

演示注入接口：

```bash
curl -X POST http://127.0.0.1:11294/api/v1/demo/scenario \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"6ae74bcd7f2e6c302fa9194e_hbot","status":"bad","light":35}'
```

`status` 支持 `good` / `warning` / `bad` / `away`，事件会走真实的 IoTDA 解析与 `telemetry` 写入路径。

## 配套前端

同仓库兄弟目录 [`../data-demostrate-client/web`](../data-demostrate-client/web/README.md) 提供深色霓虹风格的数据仪表盘，直接消费本服务的只读 API。启动方式见前端 README。

## Docker

```bash
cp .env.example .env
# 编辑 .env 中的 DATABASE_URL；如果使用 compose 内置 PostgreSQL，可保留默认值
docker compose up --build
```

`docker-compose.yml` 自带一个 PostgreSQL 16 实例用于本地验证；生产环境可以删除该服务，只保留 `server` 并把 `DATABASE_URL` 指向外部数据库。

容器以内置的非 root 用户 `app` 运行。如果挂载 `certs/`，请确保该用户能读取证书和私钥（例如 `chmod 644 certs/server.crt`、`chmod 640 certs/server.key` 并匹配组），或在 compose 中显式设置 `user: "${UID}:${GID}"`。

## 安全与运维

- **不要提交 `.env`、`certs/`、数据库密码和 IoTDA Token**（`.gitignore` 已包含）。
- 生产环境必须使用 HTTPS；IoTDA 只接受 2xx 响应，非 2xx 会重试并在连续失败后拉黑目标地址。
- `IOTDA_SIGNATURE_MAX_AGE` 默认 10 分钟，防止重放；服务器需保持时钟同步。
- 查询 API 建议始终设置 `API_TOKEN`，并通过反向代理限制来源 IP。
- PostgreSQL 建议启用 `sslmode=require` 或 `verify-full`。
- 日志不记录 Token、密码或完整推送体；原始推送体仅写入数据库。

## Go 版本与漏洞扫描

本项目使用最新 Go：

```bash
go version        # go1.27.3
make vuln         # govulncheck ./...
make test         # 单元测试
make integration-test  # 需要 TEST_DATABASE_URL
```

`make vuln` 通过 `govulncheck` 检查依赖和标准库的已知漏洞；建议在 CI 中执行。

## 目录结构

```text
cmd/server/              程序入口
internal/config/         环境变量配置
internal/iotda/          IoTDA 推送结构、时间解析、签名校验
internal/store/          PostgreSQL 存储、迁移、查询
internal/handler/        HTTP 路由、签名校验、查询 API
docs/api.md              接口契约与示例
docs/openapi.yaml        OpenAPI 3.0 规范
scripts/                 证书生成与测试推送脚本
```
