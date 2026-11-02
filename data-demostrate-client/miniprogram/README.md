# 成长守护 · 微信小程序客户端

家长端儿童坐姿健康看板，数据链路与 Web / HarmonyOS 客户端一致：

```text
ESP32-S3 / 本地识别服务 → Go + PostgreSQL ← 华为云 IoTDA
                                   │
                                   ▼
                       微信小程序（WXML / WXSS / JS）
```

## 功能

- **总览**：孩子档案、设备切换、1H/6H/24H/7D、坐姿表情、标注画面、成长评分、环境/设备指标、光照 Canvas 曲线、坐姿分布、IoTDA 事件流。
- **AI 报告**：自动生成 GPT-4o 家长健康报告，展示关键发现、高风险时段、可执行建议、鼓励语；支持重新分析。
- **家庭留言**：家长/孩子身份切换，发送留言、查看 ESP 送达状态；ESP 表情屏会显示最新留言。
- **久坐提醒**：ESP 连续在座 30 分钟后通过 `postureSummary/alert` 上报，主视觉显示“久坐提醒”chip。
- **7×24 热力图**：告警率 / 良好率 / 光照三种指标，原生 `scroll-view` + 网格，点击单元格查看详细数据。
- **时间回放**：24 小时 / 5 分钟采样点，原生 `slider` 拖动、播放/暂停、回到实时、采样点跳转。
- **演示控制台**：良好 / 偏移 / 告警 / 离开场景注入，走真实 IoTDA `Parse → Ingest → Postgres` 链路；支持每 5 秒自动演示。
- **连接设置**：Base URL / Token 使用 `wx.setStorageSync` 持久化，半模态输入面板。
- 下拉刷新、5 秒自动刷新、网络状态提示、`wx.showToast`、告警 `wx.vibrateShort`。

## 原生能力

- 自定义导航栏，适配状态栏与右上角胶囊按钮
- `canvas type="2d"` 原生绘制光照 / 回放曲线
- `wx.downloadFile` 带 `Authorization` 下载标注 JPEG，再交给 `<image>`
- `scroll-view` 下拉刷新与横向滚动
- `picker`、`slider`、`switch`、`input`、`button` 原生控件
- `wx.previewImage` 查看大图
- 没有引入第三方 UI 组件库或 WebView

## 用微信开发者工具打开

1. 打开微信开发者工具 → **导入项目**。
2. 项目目录选择本目录：

```text
/Users/diyanqi/Developer/esp-server/data-demostrate-client/miniprogram
```

3. AppID 使用 `project.config.json` 中的 `wxf74b4c22321f09dd`，或替换成你自己的 AppID。
4. 本地调试需要关闭域名校验：
   - 本项目 `project.private.config.json` 已设置 `"urlCheck": false`；
   - 也可以在开发者工具右上角 **详情 → 本地设置** 勾选「不校验合法域名、web-view（业务域名）、TLS 版本以及 HTTPS 证书」。

> 真机预览/发布时，微信要求 request / downloadFile 使用 HTTPS 且域名在后台白名单。当前默认地址是局域网 HTTP，只适合开发者工具或开启「不校验合法域名」的真机调试。生产环境请给 Go 服务配置 HTTPS 域名，并把域名加入小程序后台。

## 后端连接

默认配置：

```text
http://10.196.144.134:11294
test-api-token
```

Go 服务要监听局域网：

```bash
cd /Users/diyanqi/Developer/esp-server/data-collect-server
set -a; . ./.env; set +a
LISTEN_ADDR=0.0.0.0:11294 make run
```

如果 Mac 的局域网 IP 变化，在小程序右上角 **设置** 里修改 Base URL 和 Token，配置会保存在本机。

演示场景需要后端：

```bash
DEMO_ENABLED=true
```

## 目录

```text
app.js / app.json / app.wxss      全局入口、暗色主题、自定义导航
pages/index/
  index.js                        数据加载、AI 报告、回放、演示、设置
  index.wxml                      总览 / 报告 / 回放 / 演示
  index.wxss                      深色霓虹、玻璃卡片、表情、底部导航
  index.json                      自定义导航、组件注册
components/line-chart/            Canvas 2D 折线图
components/heatmap/               7×24 原生热力图
utils/api.js                      wx.request / wx.downloadFile 契约
utils/theme.js                    状态、时间、分数、格式化
```

## 开发校验

```bash
find . -name '*.js' -print | xargs -n1 node --check
find . -name '*.json' -print | xargs -n1 python3 -m json.tool
```

微信开发者工具 CLI 的「服务端口」默认关闭，若要用 `cli preview` / `cli upload`，请先在
**设置 → 安全设置** 打开服务端口。

## 说明

- 底部导航未选中项为白色，选中项为青色。
- AI 报告由 Go 后端调用 `gpt-4o` 生成；AI 失败时后端返回本地规则报告，小程序仍可用。
- 标注图请求带 Bearer Token，不把 Token 拼到图片 URL 中。
- 数据来自当前 Go 后端的 `/api/v1` 接口，和 Web / HarmonyOS 客户端共用同一套契约。
