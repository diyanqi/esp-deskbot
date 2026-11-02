# 华为云 IoTDA 产品模型与设备接入

本目录包含桌面健康助手接入华为云 IoTDA 的产品模型和设备侧配置说明。

## 文件

| 文件 | 用途 |
|---|---|
| `DeskHealthAssistant_product_model.xls` | 按 `excel2Profile_zh.xls` 模板填写的产品模型，可用于华为云 Excel 转 Profile 工具 |
| `profile/DeskHealthAssistant_Espressif.zip` | 可直接在 IoTDA 控制台导入的产品模型包 |
| `profile_src/` | Profile 包的未压缩内容，便于检查 JSON |
| `iotda.example.json` | 设备侧连接配置示例，不含真实密钥 |

设备实际使用的配置位于 DATA 分区的 `config/iotda.json`：

```json
{
  "device_id": "6ae74bcd7f2e6c302fa9194e_hbot",
  "secret": "******",
  "host": "21e54825de.st1.iotda-device.cn-north-4.myhuaweicloud.com",
  "port": 8883,
  "client_id": "6ae74bcd7f2e6c302fa9194e_hbot_0_0_2026110114",
  "report_interval": 5,
  "keepalive": 120
}
```

> `secret` 是设备密钥，不要提交到版本库；种子镜像中的 `storage/config/iotda.json` 属于本机配置。

## 导入产品模型

1. 登录 IoTDA 控制台，进入设备实例 `21e54825de`。
2. 打开产品 `6ae74bcd7f2e6c302fa9194e`，确认协议为 **MQTT**（设备使用 MQTTS 接入）。
3. 在“产品模型”页面选择“导入”或“上传模型文件”，选择 `profile/DeskHealthAssistant_Espressif.zip`。
4. 导入后应能看到三个服务：`Environment`、`Posture`、`DeviceStatus`。
5. 如果控制台要求模型与已有服务合并，请保持服务标识和属性名与下表一致。

## 服务与属性

### Environment

| 属性 | 类型 | 单位 | 范围 | 说明 |
|---|---|---|---|---|
| `temperature` | decimal | C | -40 ~ 80 | 温度；传感器故障，当前固定上报 `0` |
| `humidity` | int | % | 0 ~ 100 | 湿度；传感器故障，当前固定上报 `0` |
| `light` | int | % | 0 ~ 100 | 光照百分比 |
| `sound` | int | - | 0 ~ 1 | 声音检测：0 安静，1 有声音 |

### Posture

| 属性 | 类型 | 说明 |
|---|---|---|
| `personDetected` | int | 是否检测到人：0/1 |
| `postureStatus` | string | `good` / `warning` / `bad` / `unknown` |
| `postureSummary` | string | 坐姿摘要或最近一次采集错误 |
| `issueCount` | int | 问题数量 |
| `alert` | int | 当前是否坐姿异常：0/1 |
| `detecting` | int | 是否正在采样：0/1 |
| `lastSeq` | int | 最近一次采集序号 |

### DeviceStatus

| 属性 | 类型 | 单位 | 说明 |
|---|---|---|---|
| `wifiConnected` | int | - | Wi-Fi 是否连接：0/1 |
| `rssi` | int | dBm | Wi-Fi 信号强度 |
| `uptime` | int | s | 运行时长 |
| `freeHeap` | int | byte | 空闲堆内存 |
| `reportInterval` | int | s | 属性上报最小间隔，可写 |
| `firmware` | string | - | 固件版本 |
| `lastReport` | DateTime | - | 最近上报时间（UTC） |

## 云端命令

| 命令 | 参数 | 设备行为 |
|---|---|---|
| `SET_REPORT_INTERVAL` | `interval` int 5~3600 秒 | 修改属性上报最小间隔，并立即返回结果 |
| `TRIGGER_REMINDER` | `reason` string，可选 | 在设备屏幕上显示“云端提醒”，蜂鸣并播放休息表情 |

响应分别是 `SET_REPORT_INTERVAL_RSP` / `TRIGGER_REMINDER_RSP`，参数 `result`：0 成功，1 失败。

## 设备侧 MQTT 参数

| 参数 | 值 |
|---|---|
| 地址 | `21e54825de.st1.iotda-device.cn-north-4.myhuaweicloud.com:8883` |
| 协议 | MQTTS（TLS） |
| ClientId | `{device_id}_0_0_{YYYYMMDDHH}` |
| Username | `{device_id}` |
| Password | `HMAC-SHA256(key=时间戳, message=secret)`，64 位小写十六进制 |
| TLS 根证书 | GlobalSign Root CA - R3（ESP-IDF 证书包已内置） |

设备固件在每次启动时根据 `config/iotda.json` 生成 ClientId 和 Password；如果配置中带有合法的 `client_id`，则直接使用其中的时间戳。密码计算方式：

```text
password = hex(hmac_sha256(key = timestamp, msg = secret))
```

## 上报 Topic

| 用途 | Topic |
|---|---|
| 属性上报 | `$oc/devices/{device_id}/sys/properties/report` |
| 命令订阅 | `$oc/devices/{device_id}/sys/commands/#` |
| 命令响应 | `$oc/devices/{device_id}/sys/commands/response/request_id={request_id}` |
| 事件上报 | `$oc/devices/{device_id}/sys/events/up` |

属性上报示例：

```json
{
  "services": [
    {
      "service_id": "Environment",
      "properties": {
        "temperature": 0,
        "humidity": 0,
        "light": 62,
        "sound": 0
      }
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
      }
    },
    {
      "service_id": "DeviceStatus",
      "properties": {
        "wifiConnected": 1,
        "rssi": -55,
        "uptime": 1234,
        "freeHeap": 180000,
        "reportInterval": 5,
        "firmware": "0.1.0",
        "lastReport": "20261101T143000Z"
      }
    }
  ]
}
```

## 数据来源

- `posture_worker.lua` 每 5 秒拍照并请求坐姿分析，结果写入 `posture_state.json`，同时通过 `iotda.publish()` 上报。
- 温度/湿度传感器当前不可用，`Environment.temperature` 和 `humidity` 固定为 `0`；其余字段来自 `sensors.lua` 和坐姿分析结果。
- `face.lua` 在本地循环中轮询 `iotda.take_reminder()`；云端下发 `TRIGGER_REMINDER` 后，屏幕会显示休息提醒并蜂鸣。

## 验证

1. 设备启动后串口应出现 `lua_iotda: connected to IoTDA` 和 `subscribed to command topic`。
2. IoTDA 控制台“设备详情 → 设备影子/运行状态”应能看到属性值。
3. 在控制台“命令下发”中选择 `DeviceStatus`，下发 `TRIGGER_REMINDER`，设备屏幕应立即弹出休息提醒。
