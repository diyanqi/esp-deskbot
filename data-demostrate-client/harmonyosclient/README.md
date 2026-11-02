# 成长守护 · HarmonyOS 7 原生客户端

面向家长的儿童坐姿健康看板，数据与 Web 端一致：ESP32-S3 → 华为云 IoTDA / 本地识别服务 → Go + PostgreSQL → HarmonyOS 客户端。

- **HarmonyOS 7 / API 26**
- **ArkUI 声明式 UI**，不是 WebView
- `HdsNavigation` + `HdsTabs` 沉浸光感标题栏 / 悬浮页签
- `hdsMaterial` 系统材质（`IMMERSIVE / EXQUISITE`），低版本自动回退
- `bindSheet` + `uiMaterial.ImmersiveMaterial` 连接设置半模态
- `PixelMap` 实时展示带 Bearer Token 的标注 JPEG
- `Shape` / `Path` 原生折线图、`Progress(Ring)` 成长评分、`Slider` 回放
- `SymbolGlyph` 系统符号、`Vibrator` 告警轻振、`Preferences` 持久化
- `Refresh` 下拉刷新、`PromptAction` 沉浸 Toast、暗色渐变背景

## 页面

底部悬浮页签：

| 页签 | 内容 |
|---|---|
| 总览 | 孩子档案、设备切换、1H/6H/24H/7D、坐姿表情、标注画面、成长评分、六项环境/设备指标、光照趋势、坐姿分布、事件流 |
| AI 报告 | 自动生成 GPT-4o 家长健康报告、关键发现、高风险时段、家长建议、7×24 热力图 |
| 留言 | 家长/孩子身份切换、家庭留言列表、发送留言并查看 ESP 送达状态 |
| 时间回放 | 24h / 5 分钟采样点、播放/暂停、拖动、状态时间条、采样点跳转 |
| 演示 | 良好/偏移/告警/离开场景注入、自动演示、连接信息、原生能力说明 |

## 构建

环境：DevEco Studio / HarmonyOS SDK 26.0.0。

```bash
cd /Users/diyanqi/Developer/esp-server/data-demostrate-client/harmonyosclient
hvigorw --mode module -p product=default -p module=entry@default \
  -p buildMode=debug assembleHap --no-daemon
```

产物：

```text
entry/build/default/outputs/default/entry-default-signed.hap
```

## 真机安装

```bash
HDC=/Applications/DevEco-Studio.app/Contents/sdk/default/openharmony/toolchains/hdc
$HDC tconn 10.196.177.40:12345
$HDC install -r entry/build/default/outputs/default/entry-default-signed.hap
$HDC shell aa start -a EntryAbility -b com.deskbot.hmclient
```

设备：HUAWEI Mate 80 Pro，HarmonyOS 7.0.0，API 26。工程已带本地 debug 签名配置。

## 后端连接

客户端默认连接：

```text
http://10.196.144.134:11294
```

即当前 Mac 的局域网地址。真机与 Mac 需要同一网络；Go 服务必须监听 `0.0.0.0:11294`，例如：

```bash
cd /Users/diyanqi/Developer/esp-server/data-collect-server
set -a; . ./.env; set +a
LISTEN_ADDR=0.0.0.0:11294 make run
```

API Token 默认 `test-api-token`。如果网络/端口变化，在客户端右上角 **设置** 里修改 Base URL 与 Token，配置会保存到本机 `Preferences`。

演示场景需要后端开启：

```bash
DEMO_ENABLED=true
```

## 目录

```text
entry/src/main/ets/
  entryability/EntryAbility.ets     Ability 入口、设置初始化
  model/DashboardModels.ets         API 数据契约
  service/ApiService.ets            HTTP、PixelMap、Preferences
  utils/Theme.ets                   颜色、状态、时间/数值格式化
  view/DashboardWidgets.ets         GlassCard、Face、SparkLine、Heatmap、AIReportView 等
  pages/Index.ets                   主页面、HdsNavigation/HdsTabs、四个页签与交互
entry/src/main/module.json5         INTERNET / VIBRATE 权限
```

## 说明

- AI 报告由后端调用 `gpt-4o` 生成；首次打开自动生成一份，也可手动重新分析。AI 失败时后端返回本地规则报告，客户端仍然可用。
- 标注画面通过带 `Authorization: Bearer` 的 HTTP 请求取 `ArrayBuffer`，经 `@kit.ImageKit` 解码为 `PixelMap`，因此 Token 不会出现在图片 URL 里。
- Hds 系统材质通过 `hdsMaterial.getSystemMaterialTypes()` 检测；设备不支持时自动退化为普通背景，不会崩溃。
- 底部页签未选中项为白色，选中项为主题青色。
