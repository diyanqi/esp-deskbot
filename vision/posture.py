"""基于 MediaPipe Pose Landmarker (lite) 的关键点检测与坐姿评估。"""

import os
import threading

import cv2
import mediapipe as mp
import numpy as np
from mediapipe.tasks.python import BaseOptions, vision

HERE = os.path.dirname(os.path.abspath(__file__))
MODEL_PATH = os.path.join(HERE, "models", "pose_landmarker_lite.task")

IDX = {
    "nose": 0,
    "left_eye": 2,
    "right_eye": 5,
    "left_ear": 7,
    "right_ear": 8,
    "left_shoulder": 11,
    "right_shoulder": 12,
    "left_elbow": 13,
    "right_elbow": 14,
    "left_wrist": 15,
    "right_wrist": 16,
    "left_hip": 23,
    "right_hip": 24,
    "left_knee": 25,
    "right_knee": 26,
    "left_ankle": 27,
    "right_ankle": 28,
}

BODY_CONNECTIONS = [
    (11, 12), (11, 13), (13, 15), (12, 14), (14, 16),
    (11, 23), (12, 24), (23, 24),
    (23, 25), (25, 27), (24, 26), (26, 28),
    (27, 31), (28, 32), (27, 29), (29, 31), (28, 30), (30, 32),
    (0, 11), (0, 12),
]

THRESHOLDS = {
    "torso_forward_deg": 20.0,
    "torso_side_deg": 12.0,
    "shoulder_tilt_deg": 10.0,
    "head_tilt_deg": 12.0,
    "head_offset_ratio": 0.45,
    "head_drop_ratio": 0.22,
    "hand_head_ratio": 0.45,
    "knee_angle_min": 70.0,
    "knee_angle_max": 130.0,
    "legs_together_ratio": 0.30,
}

VISIBLE = 0.5

ISSUE_INFO = {
    "forward_head": ("头部前伸", "下巴回收，让耳朵与肩膀回到同一条垂线上"),
    "head_drooped": ("低头/含胸", "抬头，把视线抬到屏幕上半部"),
    "torso_forward": ("躯干前倾", "坐直，让肩关节落在髋关节正上方"),
    "torso_side": ("躯干侧倾", "调正坐姿或调整座椅高度，避免长期单侧倚靠"),
    "shoulder_tilt": ("双肩不平", "放松较高一侧肩膀，检查座椅是否水平"),
    "head_tilt": ("头部歪斜", "把头摆正，避免长期偏头看屏幕"),
    "hand_on_head": ("手撑头/托腮", "让双手回到桌面，减少颈部单侧压力"),
    "knee_angle": ("腿部角度异常", "大腿与小腿保持约 90 度，双脚平放地面"),
    "legs_together": ("双腿交叉/并拢", "双脚与髋同宽，避免跷腿"),
}

STATUS_TEXT = {"good": "坐姿良好", "warning": "坐姿需注意", "bad": "坐姿存在问题"}


class _Issue:
    def __init__(self, code, value, threshold, severity="warning", detail=None):
        self.code = code
        self.value = round(float(value), 3)
        self.threshold = threshold
        self.severity = severity
        self.detail = detail
        self.label, self.advice = ISSUE_INFO[code]

    def as_dict(self):
        item = {
            "code": self.code,
            "label": self.label,
            "severity": self.severity,
            "value": self.value,
            "threshold": self.threshold,
            "advice": self.advice,
        }
        if self.detail:
            item["detail"] = self.detail
        return item


def _dist(a, b):
    return float(np.linalg.norm(np.asarray(a, dtype=float) - np.asarray(b, dtype=float)))


def _angle_at(vertex, a, b):
    v1 = np.asarray(a, dtype=float) - np.asarray(vertex, dtype=float)
    v2 = np.asarray(b, dtype=float) - np.asarray(vertex, dtype=float)
    n1, n2 = np.linalg.norm(v1), np.linalg.norm(v2)
    if n1 == 0 or n2 == 0:
        return 0.0
    cos = float(np.clip(np.dot(v1, v2) / (n1 * n2), -1.0, 1.0))
    return float(np.degrees(np.arccos(cos)))


def _tilt_from_horizontal(a, b):
    v = np.asarray(b, dtype=float) - np.asarray(a, dtype=float)
    horiz = float(np.hypot(v[0], v[2]))
    if horiz == 0:
        return 90.0
    return float(np.degrees(np.arctan2(abs(v[1]), horiz)))


def _body_axes(left_shoulder, right_shoulder):
    """人体坐标系：lateral 沿双肩方向，forward 与其水平正交，用于区分前倾/侧倾。"""
    lateral = np.asarray(right_shoulder, dtype=float) - np.asarray(left_shoulder, dtype=float)
    lateral[1] = 0.0
    norm = float(np.linalg.norm(lateral))
    lateral = lateral / norm if norm > 1e-6 else np.array([1.0, 0.0, 0.0])
    up = np.array([0.0, -1.0, 0.0])
    forward = np.cross(up, lateral)
    return lateral, forward / np.linalg.norm(forward)


class PoseAnalyzer:
    """线程不安全的推理器，用锁串行化调用。"""

    def __init__(self, model_path=MODEL_PATH):
        if not os.path.exists(model_path):
            raise FileNotFoundError(
                f"pose model not found: {model_path}. "
                "Download pose_landmarker_lite.task into ./models/"
            )
        options = vision.PoseLandmarkerOptions(
            base_options=BaseOptions(model_asset_path=model_path),
            running_mode=vision.RunningMode.IMAGE,
            num_poses=1,
            min_pose_detection_confidence=0.5,
            min_pose_presence_confidence=0.5,
        )
        self._landmarker = vision.PoseLandmarker.create_from_options(options)
        self._lock = threading.Lock()

    def detect(self, image_bgr):
        rgb = cv2.cvtColor(image_bgr, cv2.COLOR_BGR2RGB)
        mp_image = mp.Image(image_format=mp.ImageFormat.SRGB, data=np.ascontiguousarray(rgb))
        with self._lock:
            return self._landmarker.detect(mp_image)

    def infer(self, image_bgr, thresholds=None):
        """返回 (landmarks 或 None, analysis)，只跑一次模型。"""
        th = {**THRESHOLDS, **(thresholds or {})}
        h, w = image_bgr.shape[:2]
        result = self.detect(image_bgr)

        if not result.pose_landmarks:
            return None, {
                "person_detected": False,
                "status": "unknown",
                "summary": "未检测到人体",
                "issues": [],
                "metrics": {},
                "keypoints": {},
            }

        landmarks = result.pose_landmarks[0]
        world = result.pose_world_landmarks[0] if result.pose_world_landmarks else None
        analysis = analyze_landmarks(landmarks, world, w, h, th)
        analysis["keypoints"] = {
            name: [
                round(float(landmarks[i].x), 4),
                round(float(landmarks[i].y), 4),
                round(float(landmarks[i].visibility), 3),
            ]
            for name, i in IDX.items()
        }
        return landmarks, analysis

    def analyze(self, image_bgr, thresholds=None):
        return self.infer(image_bgr, thresholds)[1]


def analyze_landmarks(landmarks, world_landmarks, width, height, thresholds=None):
    th = {**THRESHOLDS, **(thresholds or {})}
    vis = np.array([float(p.visibility) for p in landmarks])

    if world_landmarks is not None:
        pts = np.array([[p.x, p.y, p.z] for p in world_landmarks], dtype=float)
    else:
        pts = np.array([[p.x * width, p.y * height, 0.0] for p in landmarks], dtype=float)

    def P(name):
        return pts[IDX[name]]

    def seen(*names):
        return all(vis[IDX[n]] >= VISIBLE for n in names)

    issues = []
    metrics = {}

    sh_mid = (P("left_shoulder") + P("right_shoulder")) / 2
    has_hips = seen("left_hip", "right_hip")
    hip_mid = (P("left_hip") + P("right_hip")) / 2 if has_hips else None
    shoulder_w = _dist(P("left_shoulder"), P("right_shoulder"))

    if has_hips:
        torso_len = _dist(sh_mid, hip_mid)
    else:
        torso_len = shoulder_w * 1.5
    if torso_len <= 1e-6:
        return {
            "person_detected": True,
            "status": "unknown",
            "summary": "关键点不足，无法评估",
            "issues": [],
            "metrics": {},
        }

    if has_hips:
        lateral, forward = _body_axes(P("left_shoulder"), P("right_shoulder"))
        v = sh_mid - hip_mid
        up = -v[1]
        if up > 0:
            side = abs(float(np.dot(v, lateral)))
            fwd = abs(float(np.dot(v, forward)))
            metrics["torso_lean_deg"] = round(float(np.degrees(np.arctan2(np.hypot(side, fwd), up))), 1)
            metrics["torso_forward_deg"] = round(float(np.degrees(np.arctan2(fwd, up))), 1)
            metrics["torso_side_deg"] = round(float(np.degrees(np.arctan2(side, up))), 1)
            if metrics["torso_forward_deg"] > th["torso_forward_deg"]:
                issues.append(
                    _Issue("torso_forward", metrics["torso_forward_deg"], th["torso_forward_deg"])
                )
            if metrics["torso_side_deg"] > th["torso_side_deg"]:
                issues.append(
                    _Issue("torso_side", metrics["torso_side_deg"], th["torso_side_deg"])
                )

    if seen("left_shoulder", "right_shoulder"):
        tilt = _tilt_from_horizontal(P("left_shoulder"), P("right_shoulder"))
        metrics["shoulder_tilt_deg"] = round(tilt, 1)
        if tilt > th["shoulder_tilt_deg"]:
            issues.append(_Issue("shoulder_tilt", tilt, th["shoulder_tilt_deg"]))

    nose = P("nose")
    lateral, forward = _body_axes(P("left_shoulder"), P("right_shoulder"))
    head_vec = nose - sh_mid
    head_forward = abs(float(np.dot(head_vec, forward))) / torso_len
    head_lateral = abs(float(np.dot(head_vec, lateral))) / torso_len
    head_drop = (sh_mid[1] - nose[1]) / torso_len
    metrics["head_forward_ratio"] = round(head_forward, 3)
    metrics["head_lateral_ratio"] = round(head_lateral, 3)
    metrics["head_drop_ratio"] = round(head_drop, 3)

    if head_forward > th["head_offset_ratio"]:
        issues.append(_Issue("forward_head", head_forward, th["head_offset_ratio"]))
    if head_drop < th["head_drop_ratio"]:
        issues.append(_Issue("head_drooped", head_drop, th["head_drop_ratio"]))

    if seen("left_ear", "right_ear"):
        tilt = _tilt_from_horizontal(P("left_ear"), P("right_ear"))
        metrics["head_tilt_deg"] = round(tilt, 1)
        if tilt > th["head_tilt_deg"]:
            issues.append(_Issue("head_tilt", tilt, th["head_tilt_deg"], severity="info"))

    wrists = [n for n in ("left_wrist", "right_wrist") if vis[IDX[n]] >= VISIBLE]
    if wrists:
        near = min(_dist(P(n), nose) for n in wrists) / torso_len
        metrics["hand_head_ratio"] = round(near, 3)
        if near < th["hand_head_ratio"]:
            issues.append(
                _Issue("hand_on_head", near, th["hand_head_ratio"], severity="info")
            )

    knee_angles = {}
    for side in ("left", "right"):
        names = (f"{side}_hip", f"{side}_knee", f"{side}_ankle")
        if seen(*names):
            angle = _angle_at(P(names[1]), P(names[0]), P(names[2]))
            knee_angles[side] = angle
            metrics[f"{side}_knee_angle_deg"] = round(angle, 1)

    if knee_angles:
        bad = {
            side: angle
            for side, angle in knee_angles.items()
            if angle < th["knee_angle_min"] or angle > th["knee_angle_max"]
        }
        if bad:
            worst = max(
                bad,
                key=lambda s: max(th["knee_angle_min"] - bad[s], bad[s] - th["knee_angle_max"]),
            )
            issues.append(
                _Issue(
                    "knee_angle",
                    bad[worst],
                    [th["knee_angle_min"], th["knee_angle_max"]],
                    severity="info",
                    detail={
                        "side": worst,
                        "angles_deg": {s: round(a, 1) for s, a in knee_angles.items()},
                    },
                )
            )

    if seen("left_ankle", "right_ankle") and seen("left_knee", "right_knee"):
        ankle_gap = _dist(P("left_ankle"), P("right_ankle")) / torso_len
        knee_gap = _dist(P("left_knee"), P("right_knee")) / torso_len
        metrics["ankle_gap_ratio"] = round(ankle_gap, 3)
        metrics["knee_gap_ratio"] = round(knee_gap, 3)
        if ankle_gap < th["legs_together_ratio"] and knee_gap < th["legs_together_ratio"]:
            issues.append(
                _Issue("legs_together", ankle_gap, th["legs_together_ratio"], severity="info")
            )

    if any(i.severity == "warning" for i in issues):
        status = "bad"
    elif issues:
        status = "warning"
    else:
        status = "good"

    labels = "、".join(dict.fromkeys(i.label for i in issues))
    return {
        "person_detected": True,
        "status": status,
        "summary": STATUS_TEXT[status] + ("" if not labels else "：" + labels),
        "issues": [i.as_dict() for i in issues],
        "metrics": metrics,
    }


STATUS_COLORS = {"good": (80, 200, 80), "warning": (0, 190, 255), "bad": (60, 60, 230)}


def draw_annotated(image_bgr, landmarks, analysis):
    img = image_bgr.copy()
    h, w = img.shape[:2]
    status = analysis.get("status", "unknown")
    color = STATUS_COLORS.get(status, (200, 200, 200))

    if landmarks:
        pts = [(int(round(p.x * w)), int(round(p.y * h))) for p in landmarks]
        vis = [float(p.visibility) for p in landmarks]

        for a, b in BODY_CONNECTIONS:
            if vis[a] >= VISIBLE and vis[b] >= VISIBLE:
                cv2.line(img, pts[a], pts[b], color, 2, cv2.LINE_AA)
        for i, pt in enumerate(pts):
            if vis[i] >= VISIBLE:
                cv2.circle(img, pt, 4, (0, 255, 255), -1, cv2.LINE_AA)
                cv2.circle(img, pt, 4, (0, 0, 0), 1, cv2.LINE_AA)

        nose = pts[IDX["nose"]]
        ear_l, ear_r = pts[IDX["left_ear"]], pts[IDX["right_ear"]]
        if vis[IDX["left_ear"]] >= VISIBLE and vis[IDX["right_ear"]] >= VISIBLE:
            radius = max(12, int(_dist(ear_l, ear_r) * 0.9))
        else:
            radius = max(12, int(min(w, h) * 0.04))
        cv2.circle(img, nose, radius, color, 2, cv2.LINE_AA)

    font = cv2.FONT_HERSHEY_SIMPLEX
    cv2.rectangle(img, (0, 0), (w, 34), (0, 0, 0), -1)
    cv2.putText(img, f"POSTURE: {status.upper()}", (10, 24), font, 0.7, color, 2, cv2.LINE_AA)
    codes = ", ".join(dict.fromkeys(i["code"] for i in analysis.get("issues", [])))
    if codes:
        cv2.putText(img, codes[:80], (max(10, w - 10 - 10 * len(codes[:80])), 24),
                    font, 0.45, color, 1, cv2.LINE_AA)
    return img
