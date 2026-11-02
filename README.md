# esp-deskbot · 成长守护

基于 ESP-Claw 的 AI 桌面坐姿助手完整项目源码。

- **ESP 桌面端**：ESP32-S3、摄像头、320×240 屏幕、本地姿态识别、家庭留言、久坐提醒
- **Go 后端**：IoTDA 数据接入、PostgreSQL、家长端聚合、AI 报告、标注帧、留言
- **Python 视觉**：MediaPipe Pose Landmarker、坐姿评估、标注图生成
- **三端客户端**：Web、HarmonyOS 7、微信小程序
- **AI 报告**：OpenAI 兼容接口，默认 `gpt-4o`

## 目录

```text
esp-claw/                  ESP-IDF 固件、板级配置、face/worker Lua、FATFS 资源
data-collect-server/       Go 数据后端 + PostgreSQL migrations
vision/                    Python MediaPipe 视觉服务、模型、坐姿示意图
data-demostrate-client/
  web/                     Vite + React 家长端
  harmonyosclient/         HarmonyOS 7 / ArkUI 原生客户端
  miniprogram/             微信小程序原生客户端
deploy/                    云服务器部署清单
video/                     项目介绍视频脚本
```

## 快速开始

```bash
# 1. Go 后端
cd data-collect-server
cp .env.example .env
make run

# 2. Python 视觉
cd ../vision
python3.12 -m venv venv
./venv/bin/pip install -r requirements.txt
./venv/bin/python server.py --port 8000

# 3. Web
cd ../data-demostrate-client/web
npm ci
npm run dev
```

## 云部署

完整步骤见：

- [`deploy/cloud-deployment-checklist.md`](deploy/cloud-deployment-checklist.md)
- [`video/intro-video-script.md`](video/intro-video-script.md)

## 重要说明

- 本仓库是**完整项目快照**，不是 `esp-claw` 上游 Git 历史的 fork。`esp-claw` 的官方上游仍是 `https://github.com/espressif/esp-claw`。
- 真实设备配置、`storage/config/iotda.json`、`storage/config/messages.json`、`.env` 和 HarmonyOS 签名材料**没有提交**；请按 `deploy/cloud-deployment-checklist.md` 在本地创建。
- HarmonyOS 的 `build-profile.json5` 已移除签名配置，需要在 DevEco Studio 里重新生成自动签名。
- 默认 Token（`test-api-token`）只用于本地演示；生产必须更换。
- 儿童图像/姿态数据请按当地法规和隐私政策处理。
