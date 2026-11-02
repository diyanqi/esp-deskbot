# 成长守护 · 家长端坐姿健康看板

面向家长的儿童坐姿健康数据看板。数据链：

```text
ESP32-S3 摄像头 / 本地 MediaPipe 识别
        │  标注图 + 姿态结果
        ▼
server.py ── multipart ──► data-collect-server (PostgreSQL)
                                   │
ESP32-S3 ── MQTTS ──► 华为云 IoTDA ── HTTPS 转发 ──┘
                                   │
                                   ▼
                     成长守护看板（Vite + React）
```

页面为深色霓虹风格，核心是“家长视角”：不是只展示传感器，而是把坐姿、环境、识别画面和 AI 建议组织成一份可阅读的成长报告。

## 功能

- **实时坐姿画面**：展示本地识别服务上传的 `_annotated.jpg`，包含人体关键点/骨架标注、状态、拍摄时间和最近 8 帧切换。
- **成长守护评分**：按坐姿良好率、提醒频率、光照舒适度和在座时长综合计算 0–100 分，并展示今日良好率、提醒次数、最长连续告警和平均光照。
- **AI 健康报告**：调用 `gpt-4o`（OpenAI 兼容接口）生成给家长的结构化报告，包含评分、总体评价、关键发现、高风险时段、可执行建议和鼓励语；上游失败时自动降级为本地规则报告。首次打开页面会自动生成一份，也可手动“重新分析”。
- **7×24 热力图**：按星期与小时聚合坐姿良好率 / 告警率 / 光照，快速发现高风险时段。
- **24 小时时间回放**：5 分钟粒度时间轴，可播放、暂停、拖动，主视觉、光照/声音/在座状态同步回放。
- **家庭留言**：家长/孩子身份切换，发送留言、查看 ESP 送达状态；ESP 表情屏会显示最新留言。
- **久坐提醒**：ESP 连续在座 30 分钟后通过 `postureSummary/alert` 上报，页面顶部显示久坐横幅。
- **演示控制台**：一键注入“良好 / 偏移 / 告警 / 离开”场景，数据走真实 IoTDA 解析与 Postgres 写入路径，不是前端假状态；支持每 5 秒自动演示。
- 光照趋势、坐姿分布、RSSI + Free heap 双轴图、IoTDA 事件流、设备切换。
- 1H / 6H / 24H / 7D 范围、5s / 10s / 30s 自动刷新、API Base URL / Token 设置抽屉。
- AI 报告可打印（浏览器打印会自动隐藏其他面板）。

## 技术栈

- Vite 8 + React 19 + TypeScript（strict）
- Recharts 3
- Lucide React
- 纯 CSS：玻璃拟态、霓虹渐变、扫描线、骨架 shimmer、响应式布局

## 快速开始

### 1. 启动后端并生成演示数据

```bash
cd ../data-collect-server
cp .env.example .env        # 填写 DATABASE_URL、API_TOKEN；本地演示可设 DEMO_ENABLED=true
make seed                   # 两台设备 × 7 天 × 5 分钟粒度
make run                    # 监听 :11294
```

`make seed` 默认生成：

- `6ae74bcd7f2e6c302fa9194e_hbot`（正常坐姿为主）
- `demo-desk-02`（坐姿偏移更多）

### 2. 启动本地识别服务（可选，用于实时标注画面）

```bash
cd ..                       # esp-server 根目录
venv/bin/python server.py   # 监听 :8000
```

识别服务默认把标注图上传到 `http://127.0.0.1:11294/api/v1/frames`，设备 ID 默认是
`6ae74bcd7f2e6c302fa9194e_hbot`。可通过参数覆盖：

```bash
venv/bin/python server.py \
  --data-collect-url http://127.0.0.1:11294 \
  --data-collect-token test-api-token \
  --device-id 6ae74bcd7f2e6c302fa9194e_hbot
```

不上传标注图时可传 `--data-collect-url ''`。

### 3. 启动前端

```bash
cd data-demostrate-client/web
npm install
npm run dev
```

打开 `http://localhost:5173`。

Vite 会把 `/api` 代理到 `VITE_PROXY_TARGET`（默认 `http://127.0.0.1:11294`）。

## 环境变量

复制 `.env.example` 为 `.env.local`：

```bash
cp .env.example .env.local
```

| 变量 | 说明 |
|---|---|
| `VITE_API_BASE_URL` | 留空表示使用同源 `/api`（Vite 代理或反向代理）；也可以填写 `https://host:11294` |
| `VITE_API_TOKEN` | 本地演示用查询 Token；生产环境不要打包进前端，建议通过设置抽屉填写 |
| `VITE_PROXY_TARGET` | Vite 开发代理的后端地址，默认 `http://127.0.0.1:11294` |

页面右上角“设置”也可以修改 Base URL 与 Token；配置保存在浏览器 localStorage。

## 后端接口

前端调用：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/devices` | 设备列表 |
| GET | `/api/v1/devices/{id}/latest` | 最新属性 |
| GET | `/api/v1/devices/{id}/series` | 光照/RSSI/内存聚合 |
| GET | `/api/v1/devices/{id}/telemetry` | 坐姿状态样本 |
| GET | `/api/v1/events` | 原始事件流 |
| GET | `/api/v1/messages` | 家庭留言列表 |
| POST | `/api/v1/messages` | 发送家长/孩子留言 |
| GET | `/api/v1/insights` | 成长评分 / 热力图 / 回放时间轴 |
| POST | `/api/v1/ai/report` | AI 健康报告 |
| GET | `/api/v1/frames` / `/latest` | 标注帧元数据 |
| GET | `/api/v1/frames/{id}/image` | 标注帧 JPEG（前端带 Token fetch 成 Blob） |
| POST | `/api/v1/demo/scenario` | 演示场景注入 |

完整契约见后端 [`docs/api.md`](../../data-collect-server/docs/api.md) 与
[`docs/openapi.yaml`](../../data-collect-server/docs/openapi.yaml)。

## 构建

```bash
npm run build
npm run preview
npm run lint
```

生产环境建议：

- 用 Nginx/Caddy 将静态文件和 `/api` 放在同源，避免 CORS；
- 或给后端配置 `CORS_ALLOW_ORIGINS`，前端通过 `VITE_API_BASE_URL` 指向后端；
- Query API 必须使用 Bearer Token；不要把生产 Token 写进构建产物。

## 目录结构

```text
src/
  api.ts                 请求封装、Token、Base URL、AI/画面/演示接口
  types.ts               后端返回类型
  utils.ts               状态/时间/序列格式化
  hooks/useDashboard.ts  轮询、设备切换、AI 报告、标注帧、回放、演示
  components/
    Header.tsx           家长端品牌、孩子档案、设备/范围/刷新
    SectionNav.tsx       总览 / AI 报告 / 回放 / 演示 / 设备锚点
    PostureHero.tsx      坐姿主视觉与回放状态
    LiveFrame.tsx        实时标注画面与最近帧
    GrowthOverview.tsx   成长评分、孩子档案、KPI
    AIReportPanel.tsx    AI 健康报告与打印
    MessageBoard.tsx     家庭留言列表与发送
    Heatmap.tsx          7×24 热力图
    ReplayConsole.tsx    24h 时间回放
    DemoConsole.tsx      演示场景注入
    PostureMix.tsx
    LightChart.tsx
    HealthChart.tsx
    EventStream.tsx
    DeviceFleet.tsx
    MetricCard.tsx
    SettingsDrawer.tsx
    Sparkline.tsx
    EmptyState.tsx
    LoadingState.tsx
  App.tsx
  index.css              主题、背景、动画
  styles.css             组件样式、响应式与打印样式
```
