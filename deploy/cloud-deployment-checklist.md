# 成长守护 · 云服务器部署清单

> 目标：把 `data-collect-server`（Go）、PostgreSQL、`server.py`（MediaPipe 视觉）、Web 前端、Nginx/TLS 和 IoTDA 接入全部部署到一台云服务器，并让 HarmonyOS / 小程序 / ESP 指向云端。
>
> 占位符约定：`<domain>`、`<server-ip>`、`<API_TOKEN>`、`<IOTDA_PUSH_TOKEN>`、`<DB_PASSWORD>`、`<AI_API_KEY>`、`<IOTDA_DEVICE_SECRET>` 全部替换为真实值；**不要把真实值写进本文件或仓库**。

---

## 0. 先定部署形态

- [ ] 选择云厂商和区域：连接华为云 IoTDA 建议同区域/低延迟；国内域名需 ICP 备案。
- [ ] 选择单机部署还是拆分部署：
  - **演示/单机**：一台 4 vCPU / 8 GB / 100 GB SSD，Docker Compose + Nginx + 本机 PostgreSQL。
  - **生产/多设备**：Go + Nginx 一台；Python 视觉单独一台；PostgreSQL 用云数据库；对象存储可选。
- [ ] 准备域名和 DNS：
  - `api.example.com` → 云服务器公网 IP
  - 如果 Web 和 API 同域，Web 直接用 `https://api.example.com/`
- [ ] 准备 TLS 证书（Let's Encrypt / 云厂商证书）；**ESP 和小程序都只信任公共 CA，不要用自签名证书**。
- [ ] 云服务器开启 NTP 时间同步（IoTDA 签名校验依赖时间）。
- [ ] 明确儿童图像/姿态数据的保存期限、访问权限和删除流程。

---

## 1. 资源与端口规划

| 项目 | 建议 |
|---|---|
| CPU / 内存 | Go + Postgres 约 2C/2–4G；Python MediaPipe 建议 4C/8G 起步 |
| 磁盘 | 100 GB SSD 起；标注帧存 PostgreSQL `bytea`，会随设备数增长 |
| 公网端口 | `443`（Nginx TLS）；`22` 仅管理员 IP |
| 内网端口 | `11294` Go、`8000` Python、`5432` PostgreSQL，只监听 `127.0.0.1` 或私网 |
| 出站 | HTTPS 443（AI、IoTDA、包管理）；MQTTS 8883（ESP → IoTDA） |
| 不建议 | 公网直接暴露 `11294`、`8000`、`5432`；不要用 HTTP 传儿童图像 |

---

## 2. 必须更换的默认密钥

先在本地/服务器生成强随机值：

```bash
openssl rand -hex 24   # API_TOKEN
openssl rand -hex 24   # IOTDA_PUSH_TOKEN
openssl rand -base64 32  # DB_PASSWORD
```

- [ ] `API_TOKEN`：替换当前的 `test-api-token`；Go、Web/HarmonyOS/小程序、ESP、Python 回调全部同步。
- [ ] `IOTDA_PUSH_TOKEN`：与华为云 IoTDA 转发配置完全一致；不要用 `change-me`。
- [ ] PostgreSQL 密码：不要用 `change-me`。
- [ ] `IOTDA_DEVICE_SECRET`：当前 ESP `iotda.json` 里的密钥；如果曾进入过公开仓库/截图，立即在 IoTDA 轮换。
- [ ] `AI_API_KEY`：生产用正式 Key，不要用演示接口。
- [ ] 不要把 `.env`、`iotda.json`、`messages.json`、`project.private.config.json`、证书私钥提交到 Git。

**需要检查的本地文件：**

```text
data-collect-server/.env
esp-claw/application/edge_agent/fatfs_image/storage/config/iotda.json
esp-claw/application/edge_agent/fatfs_image/storage/config/messages.json
esp-claw/application/edge_agent/fatfs_image/system/.recovery/config/iotda.json
esp-claw/application/edge_agent/fatfs_image/system/.recovery/config/messages.json
data-demostrate-client/web/.env.local
data-demostrate-client/miniprogram/project.private.config.json
```

---

## 3. 需要部署/配置的服务清单

| # | 服务 | 位置 | 部署方式 | 是否公网 |
|---|---|---|---|---|
| 1 | PostgreSQL 16 | 云数据库或本机 Docker | 托管/容器 | 否 |
| 2 | Go 数据后端 | `data-collect-server/` | systemd 或 Docker | 经 Nginx |
| 3 | Python 视觉识别 | `server.py` + `posture.py` + `models/` | venv + systemd | 经 Nginx `/vision/` |
| 4 | Web 家长端 | `data-demostrate-client/web/` | 静态 `dist/` | 是，`/` |
| 5 | Nginx / TLS | 云服务器 | 系统服务 | 是，443 |
| 6 | AI 健康报告 | 外部 OpenAI 兼容接口 | 出站 HTTPS | 否 |
| 7 | 华为云 IoTDA | 华为云控制台 | 已有云服务 | 推送 Go |
| 8 | HarmonyOS 客户端 | `harmonyosclient/` | 签名 HAP/AGC | 否 |
| 9 | 微信小程序 | `miniprogram/` | 微信平台发布 | 否 |
| 10 | ESP 设备 | `esp-claw/` | 改 URL 后 build/flash | 出站访问云端 |

---

## 4. Go 后端

### 4.1 生产 `.env`

```dotenv
LISTEN_ADDR=127.0.0.1:11294
TLS_CERT_FILE=
TLS_KEY_FILE=

IOTDA_PUSH_TOKEN=<IOTDA_PUSH_TOKEN>
IOTDA_SIGNATURE_MAX_AGE=10m
DEV_ALLOW_UNSIGNED_PUSH=false

API_TOKEN=<API_TOKEN>
CORS_ALLOW_ORIGINS=https://<domain>

DATABASE_URL=postgres://data_collect:<DB_PASSWORD>@<db-host>:5432/data_collect?sslmode=require
DB_MAX_CONNS=10
DB_MIN_CONNS=1
RUN_MIGRATIONS=true

HTTP_READ_TIMEOUT=15s
HTTP_WRITE_TIMEOUT=60s
HTTP_IDLE_TIMEOUT=60s
HTTP_SHUTDOWN_TIMEOUT=10s
HTTP_MAX_BODY_BYTES=5242880

AI_ENABLED=true
AI_API_URL=https://<official-openai-compatible-endpoint>/v1/chat/completions
AI_MODEL=gpt-4o
AI_API_KEY=<AI_API_KEY>
AI_TIMEOUT=45s

DEMO_ENABLED=false
LOG_LEVEL=info
LOG_JSON=true
```

说明：

- [ ] Nginx 终止 TLS 时，Go 只监听 `127.0.0.1:11294`，不要把 11294 暴露公网。
- [ ] `IOTDA_SIGNATURE_MAX_AGE` 保持 10 分钟；服务器时间必须准。
- [ ] `HTTP_WRITE_TIMEOUT` 必须大于 `AI_TIMEOUT`；Nginx 的 `proxy_read_timeout` 建议 90s。
- [ ] `DEMO_ENABLED=false`；演示环境如开启，必须限制来源 IP 或加 Basic Auth。
- [ ] 生产不要执行 `make seed`；需要真实设备数据。

### 4.2 构建与启动

```bash
cd data-collect-server
make build
# 产物 bin/server；用 systemd 或 Docker 启动
curl -fsS http://127.0.0.1:11294/healthz
curl -fsS http://127.0.0.1:11294/readyz
```

- [ ] 使用 systemd/容器重启策略，开机自启。
- [ ] 日志落到 journald/文件，不打印 Token、DB 密码、完整推送体。
- [ ] 数据库连接使用私网；云数据库开启 TLS（`sslmode=require`）。

---

## 5. PostgreSQL

- [ ] 使用云数据库 PostgreSQL 16，或 `docker-compose` 的本机实例；生产建议托管。
- [ ] 建库/用户：

```sql
CREATE USER data_collect WITH PASSWORD '<DB_PASSWORD>';
CREATE DATABASE data_collect OWNER data_collect;
```

- [ ] 迁移：Go 启动时 `RUN_MIGRATIONS=true` 自动执行 `0001`–`0003`；确认三张业务表存在。
- [ ] 不要对公网开放 5432；只允许 Go 所在私网/IP。
- [ ] 备份：每日 `pg_dump`，保留 7–30 天；定期做恢复演练。
- [ ] 容量关注：
  - `posture_frames.image_bytes`：每台设备保留最新 80 帧，单帧几十 KB 到几百 KB。
  - `telemetry`：5 秒级数据，7 天约 12 万行/设备。
  - `iotda_events`：原始推送，可按保留期归档/清理。
- [ ] 如果设备数增长，考虑把标注帧迁到对象存储（OSS/S3），数据库只存 URL/元数据。

---

## 6. Python 视觉识别服务

### 6.1 依赖与模型

- [ ] Python 3.10/3.11/3.12（以 MediaPipe 实际支持为准）。
- [ ] 安装：

```bash
cd /opt/growth-guardian
python3.11 -m venv venv
./venv/bin/pip install -r requirements.txt
./venv/bin/python -c "import cv2, mediapipe; print('vision deps ok')"
```

- [ ] 模型文件必须在：

```text
/opt/growth-guardian/models/pose_landmarker_lite.task
```

  仓库已有 5.5MB 模型；没有时按根 README 的 URL 下载。
- [ ] `requirements.txt` 目前只写 `mediapipe`；如果云上 `import cv2` 失败，补 `opencv-python-headless` 并重新验证。
- [ ] 上传目录使用持久盘：`/data/uploads`；不要放在容器临时层。

### 6.2 启动参数

```bash
/opt/growth-guardian/venv/bin/python /opt/growth-guardian/server.py \
  --host 127.0.0.1 \
  --port 8000 \
  --uploads /data/uploads \
  --model /opt/growth-guardian/models/pose_landmarker_lite.task \
  --data-collect-url https://<domain> \
  --data-collect-token '<API_TOKEN>' \
  --device-id '<DEVICE_ID>'
```

- [ ] `--data-collect-url` 指向 Go 的公网 HTTPS；标注帧上传走 `/api/v1/frames`。
- [ ] `--device-id` 与 ESP 的真实设备 ID 一致。
- [ ] `/vision/upload` 只允许 ESP 来源；当前 Python 服务本身没有鉴权，建议在 Nginx 加 IP 白名单或上传 Token 校验。
- [ ] 给 `uploads/` 设置保留策略/定时清理；不要公开目录列表。
- [ ] 视觉服务是 CPU 密集；多设备时单独扩一台，不要在 Go 容器里跑。
- [ ] 识别失败不能影响 ESP 返回；日志记录但不要阻塞。

### 6.3 Nginx 反代

- [ ] 把公网 `https://<domain>/vision/upload` 反代到 `http://127.0.0.1:8000/upload`。
- [ ] 请求体上限 ≥ 10MB（ESP 图片）。
- [ ] 不要把 `/uploads/` 直接暴露公网；面板展示用 Go 的 `/api/v1/frames/{id}/image`。

---

## 7. Web 家长端

```bash
cd data-demostrate-client/web
npm ci
npm run build
# 部署 dist/ 到 Nginx root
```

- [ ] 同域部署时：`VITE_API_BASE_URL` 留空，前端请求 `/api/v1/...`，由 Nginx 反代。
- [ ] 独立 API 域时：`VITE_API_BASE_URL=https://<domain>`，并把 Web 域加入 `CORS_ALLOW_ORIGINS`。
- [ ] `VITE_API_TOKEN` 会被编译进前端 JS，**不是秘密**；生产建议：
  - 私有演示：Nginx Basic Auth 保护整个站点；
  - 正式产品：用户登录 + 后端签发短期 Token；
  - 至少不要把长期 `API_TOKEN` 直接写在公开页面。
- [ ] Nginx 加 SPA fallback：`try_files $uri $uri/ /index.html`。
- [ ] 静态资源可加缓存；`index.html` 不缓存。
- [ ] 确认 `/api` 代理的 `proxy_read_timeout` ≥ 90s（AI 报告最长 45s）。

---

## 8. Nginx / TLS 参考配置

```nginx
server {
    listen 443 ssl http2;
    server_name <domain>;

    ssl_certificate     /etc/letsencrypt/live/<domain>/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/<domain>/privkey.pem;

    client_max_body_size 12m;

    # Web
    root /var/www/growth-guardian;
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Go API
    location /api/ {
        proxy_pass http://127.0.0.1:11294;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 90s;
    }

    # IoTDA 推送
    location = /data/push {
        proxy_pass http://127.0.0.1:11294;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /healthz { proxy_pass http://127.0.0.1:11294; }
    location = /readyz  { proxy_pass http://127.0.0.1:11294; }

    # Python 视觉
    location /vision/ {
        proxy_pass http://127.0.0.1:8000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 60s;
    }
}
```

- [ ] 80 端口只做 301 到 443。
- [ ] 防火墙：`443` 全开；`22` 限制管理员 IP；`11294/8000/5432` 不开放。
- [ ] 如果 IoTDA 推送走 IP 白名单，确认 Nginx 保留真实来源头。
- [ ] 不要在前面再加一层会修改/压缩 `/data/push` 请求体的代理。

---

## 9. 华为云 IoTDA

- [ ] 产品模型保留现有服务/属性：`Posture`、`Environment`、`DeviceStatus`（以项目实际模型为准）。
- [ ] 数据转发目标 URL：

```text
https://<domain>/data/push
```

- [ ] 转发 Token = `IOTDA_PUSH_TOKEN`。
- [ ] 签名算法/时间戳/随机数按 IoTDA 控制台配置；服务器 NTP 必须准确。
- [ ] IoTDA 要求 HTTPS 时，在 IoTDA 的 CA 配置中上传服务器证书链。
- [ ] 用 `scripts/send-test-push.sh` 或控制台“测试推送”验证一条事件进入数据库。
- [ ] ESP 的 `iotda.json` 保持云端 MQTTS 地址/设备密钥；不要把 `secret` 提交到 Git。
- [ ] 设备端 MQTTS 8883 出站；确认 Wi-Fi/防火墙允许。

---

## 10. ESP 设备端

### 10.1 需要改的 URL

`esp-claw/application/edge_agent/fatfs_image/storage/scripts/posture_worker.lua`：

```lua
local UPLOAD_URL = "https://<domain>/vision/upload"
```

`.../storage/config/messages.json`：

```json
{
  "base_url": "https://<domain>",
  "token": "<API_TOKEN>",
  "device_id": "<DEVICE_ID>"
}
```

- [ ] `storage/config/messages.json` 与 `.recovery/config/messages.json` 都要同步；两者都不要提交真实 Token。
- [ ] `iotda.json` 保持设备 ID、secret、MQTTS host/port；只在设备本地保存。
- [ ] 如果设备端要访问 HTTPS 自签证书，当前 `http_request` 使用 ESP 证书包，**不能用自签**；请用公共 CA。

### 10.2 构建/烧录

```bash
. $IDF_PATH/export.sh
cd application/edge_agent
idf.py bmgr -b esp32_s3_devkitc_1_breadboard
idf.py build
idf.py -p <PORT> flash
```

- [ ] 先修正 Board Manager 的相机配置提示：`CONFIG_ESP_BOARD_DEV_CAMERA_SUPPORT` 当前为 not set，但 board 默认期望 y；否则摄像头/坐姿识别不可用。
- [ ] 修改 URL 后必须重新 build/flash；只改 `storage/` 文件而固件没打包不会生效。
- [ ] 烧录前备份 storage：`esptool read_flash 0x7aa000 0x800000 backup.bin`。
- [ ] 烧录后确认 Wi-Fi 已连接、NTP 已同步、IoTDA 上线、消息轮询成功。
- [ ] 修改 `SEDENTARY_SECONDS` 做演示后记得改回 `30 * 60`。

---

## 11. 三端客户端

### HarmonyOS

- [ ] `ApiService.ets`：

```ts
DEFAULT_BASE_URL = 'https://<domain>'
DEFAULT_API_TOKEN = '<API_TOKEN>'
```

- [ ] 使用公共 CA 的 HTTPS；设备时间准确。
- [ ] 重新签名 HAP；通过 AGC/企业分发/本地安装。
- [ ] 确认真机能访问 `<domain>`，不要再用 `10.196.x.x` 内网 IP。

### 微信小程序

- [ ] `utils/api.js` 修改 `DEFAULT_BASE_URL=https://<domain>` 和 Token。
- [ ] 微信公众平台 → 开发管理 → 服务器域名：
  - request 合法域名：`https://<domain>`
  - downloadFile 合法域名：`https://<domain>`（标注帧走 `wx.downloadFile`）
- [ ] 域名必须 HTTPS、已备案；不能用 IP。
- [ ] 发布前关闭 `project.private.config.json` 里的本地校验，不要把该文件提交。
- [ ] 补充隐私政策：儿童图像、姿态数据、家长/孩子留言。
- [ ] `test-api-token` 必须替换；小程序包内的 Token 可被提取，正式产品应加登录/短期 Token。

### Web

- [ ] 见第 7 节；生产域名加入 `CORS_ALLOW_ORIGINS`。
- [ ] 如果 Web 与 API 不同域，检查预检请求和 `Authorization` 头。

---

## 12. 安全与隐私（儿童数据）

- [ ] 原始视频/图片不公开；后端只保留姿态结果、标注帧和设备事件。
- [ ] `/uploads/` 不公开；标注帧接口必须带 Token。
- [ ] 开启磁盘加密/云盘加密；数据库备份加密。
- [ ] 访问日志不记录 Token、密码、完整儿童图像。
- [ ] 设置数据保留期：标注帧、原始上传、AI 报告、留言分别定义。
- [ ] AI 只发送统计摘要，不发送原始图片；确认供应商 DPA/数据地域。
- [ ] 家长/孩子留言属于儿童相关数据，提供删除/导出机制。
- [ ] 生产关闭 `DEMO_ENABLED`；演示控制台加 Basic Auth 或 IP 白名单。
- [ ] 定期轮换 API Token、IoTDA 密钥、AI Key、数据库密码。

---

## 13. 上线后验证清单

```bash
# 1. 健康检查
curl -fsS https://<domain>/healthz
curl -fsS https://<domain>/readyz

# 2. 查询 API
curl -fsS -H "Authorization: Bearer <API_TOKEN>" \
  "https://<domain>/api/v1/devices/<DEVICE_ID>/latest"

# 3. AI 报告（注意 45s）
curl -fsS -X POST "https://<domain>/api/v1/ai/report" \
  -H "Authorization: Bearer <API_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"device_id":"<DEVICE_ID>","hours":24}'

# 4. 视觉上传
curl -fsS -F "file=@sample.jpg" "https://<domain>/vision/upload"

# 5. 留言
curl -fsS -X POST "https://<domain>/api/v1/messages" \
  -H "Authorization: Bearer <API_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"device_id":"<DEVICE_ID>","author":"parent","author_name":"妈妈","content":"坐直一点"}'
```

- [ ] IoTDA 测试推送 → `iotda_events` 有记录 → `/latest` 有数据。
- [ ] Python 上传 → 返回 `posture` → Go `posture_frames` 有记录 → Web 显示标注图。
- [ ] AI 报告在 Nginx 90s 超时内返回；失败时有本地规则兜底。
- [ ] 留言：Web/Harmony/小程序发送 → ESP 约 10 秒内拉取 → ack 后显示已送达。
- [ ] ESP：默认眼睛、坏坐姿示意图、久坐提醒、留言显示正常。
- [ ] TLS 证书链完整；小程序 `downloadFile` 正常；HarmonyOS 真机不报证书错误。
- [ ] 断网/重启/数据库不可用时有明确错误日志和恢复路径。

---

## 14. 备份与回滚

- [ ] 发布前打 Git tag / 记镜像 digest。
- [ ] `pg_dump` 备份；备份 `/data/uploads`、`.env`、Nginx 证书、ESP storage。
- [ ] 保留上一版 Go 二进制/镜像和 Web `dist`。
- [ ] 回滚顺序：Nginx → Go → Python → DB migration（迁移不可逆时用备份恢复）。
- [ ] 恢复演练：在测试库恢复一次备份，确认 `/latest`、`/insights`、`/frames` 可用。

---

## 15. 明确不要做

- [ ] 不要在生产执行 `make seed`（会写入/删除演示设备数据）。
- [ ] 不要保留 `DEMO_ENABLED=true`。
- [ ] 不要使用 `test-api-token`、`change-me` 或示例 IoTDA 设备密钥。
- [ ] 不要把 `11294`、`8000`、`5432` 直接暴露公网。
- [ ] 不要把 `.env`、`iotda.json`、`messages.json`、证书私钥、`project.private.config.json` 提交到 Git。
- [ ] 不要把 `ch.at` 演示 AI 接口当生产服务。
- [ ] 不要公开 `/uploads/` 或未鉴权的标注帧。
- [ ] 不要用自签名证书给 ESP/小程序/HarmonyOS 使用。
