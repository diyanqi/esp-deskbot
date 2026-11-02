# 坐姿矫正示意图

每个问题一张卡片，文件名与 `posture.py` 里的 issue code 一一对应，客户端可直接按 code 取图。

| code | 文件 | 问题 |
| --- | --- | --- |
| `forward_head` | forward_head.svg / .png | 头部前伸 |
| `head_drooped` | head_drooped.svg / .png | 低头含胸 |
| `torso_forward` | torso_forward.svg / .png | 躯干前倾 |
| `torso_side` | torso_side.svg / .png | 躯干侧倾 |
| `shoulder_tilt` | shoulder_tilt.svg / .png | 双肩不平 |
| `head_tilt` | head_tilt.svg / .png | 头部歪斜 |
| `hand_on_head` | hand_on_head.svg / .png | 手撑头 / 托腮 |

- SVG：480×320 矢量稿，`viewBox="0 0 480 320"`，任意尺寸缩放不糊。
- PNG：960×640（2×），圆角外为透明，适合不支持 SVG 的屏幕直接贴图。
- 每张卡片左侧红色为当前姿势，右侧绿色为正确姿势；灰色虚线是正确对齐参考线。
- 膝角异常（`knee_angle`）与双腿交叉（`legs_together`）属于下半身，按“只画上半身”的要求未绘制。
