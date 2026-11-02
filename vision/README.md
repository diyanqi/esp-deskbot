# esp-server

接收图片上传 → 保存到 `./uploads/` → 人体关键点检测（MediaPipe Pose Landmarker lite）→
坐姿评估 → 返回 JSON 反馈。

## 环境

```bash
python3.12 -m venv venv
./venv/bin/pip install -r requirements.txt

# 关键点模型（5.5MB，已放在 models/ 下则跳过）
mkdir -p models
curl -L -o models/pose_landmarker_lite.task \
  https://storage.googleapis.com/mediapipe-models/pose_landmarker/pose_landmarker_lite/float16/1/pose_landmarker_lite.task
```

## 运行

```bash
./venv/bin/python server.py --port 8000
```

客户端上传的图片是上下颠倒的，服务端在解码后先自动旋转 180° 校正，再做保存、检测和标注，
因此 `uploads/` 里存的是摆正后的图。如果实际是镜像而不是 180° 倒置，改 `--flip`：

```bash
--flip 180        # 默认，旋转 180°
--flip vertical   # 上下镜像
--flip horizontal # 左右镜像
--flip none       # 不处理
```

## 上传

```bash
# 表单方式
curl -F 'file=@photo.jpg' http://<ip>:8000/upload

# 裸字节方式（ESP32 常用）
curl --data-binary @photo.jpg -H 'Content-Type: image/jpeg' http://<ip>:8000/upload
```

## 返回

```json
{
  "ok": true,
  "saved": "uploads/20261030_121615_448.jpg",
  "annotated_url": "/uploads/20261030_121615_448_annotated.jpg",
  "flip": "180",
  "person_detected": true,
  "posture": {
    "status": "bad",
    "summary": "坐姿存在问题：头部前伸、躯干前倾",
    "issues": [
      {"code": "forward_head", "label": "头部前伸", "severity": "warning",
       "value": 0.52, "threshold": 0.45, "advice": "下巴回收，让耳朵与肩膀回到同一条垂线上"}
    ],
    "metrics": {"torso_forward_deg": 26.4, "head_drop_ratio": 0.19}
  },
  "keypoints": {"nose": [0.51, 0.22, 0.99], "...": []},
  "elapsed_ms": 48
}
```

`status`：`good` 坐姿良好 / `warning` 轻微问题 / `bad` 存在明显问题 / `unknown` 未检测到人体。

标好的骨架图可通过 `GET /uploads/<文件名>` 取回。

## 评估规则

阈值集中在 `posture.py` 的 `THRESHOLDS`，按“关键点几何 + 人体坐标系”计算：

- `torso_forward` / `torso_side`：肩髋连线相对垂直方向的前倾 / 侧倾角度
- `forward_head` / `head_drooped`：头部相对肩中点的前移量、下沉量（以躯干长度为尺度）
- `shoulder_tilt` / `head_tilt`：肩线、耳线相对水平线的倾角
- `hand_on_head`：手腕到鼻子的距离
- `knee_angle` / `legs_together`：膝角（约 90° 为佳）、双踝/双膝间距

`warning` 级问题会把整体标记为 `bad`，`info` 级只标记为 `warning`。
