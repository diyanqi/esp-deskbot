#!/usr/bin/env python3
"""接收图片上传、保存、做坐姿（人体关键点）检测并返回反馈的简单 HTTP 服务器。

上传方式：
  1. multipart/form-data 表单（浏览器、curl -F）
  2. 直接 POST 原始图片字节（ESP32 等嵌入式设备常用）

图片保存到 ./uploads/<时间戳>.<ext>，标注图保存为 <时间戳>_annotated.jpg，
可通过 GET /uploads/<文件名> 取回。

识别完成后，服务器还会把标注图以 multipart 自动上传到 Go 数据收集服务的
POST /api/v1/frames，供家长端看板展示。上传失败只记录日志，不影响本服务
向 ESP 返回识别结果。相关参数：
  --data-collect-url      Go 服务地址（默认 http://127.0.0.1:11294，空字符串禁用）
  --data-collect-token    Go 查询 API 的 Bearer Token
  --device-id             标注帧关联的设备 ID
"""

import argparse
import datetime
import email
import json
import logging
import mimetypes
import os
import time
import urllib.error
import urllib.request
import uuid
from email import policy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import cv2
import numpy as np

from posture import PoseAnalyzer, draw_annotated

MAX_BYTES_DEFAULT = 10 * 1024 * 1024
EXTENSIONS = {"jpeg": "jpg", "png": "png", "gif": "gif", "webp": "webp", "bmp": "bmp"}
DEFAULT_DATA_COLLECT_URL = os.environ.get("DATA_COLLECT_URL", "http://127.0.0.1:11294")
DEFAULT_DATA_COLLECT_TOKEN = os.environ.get("DATA_COLLECT_TOKEN", "test-api-token")
DEFAULT_DEVICE_ID = os.environ.get("DEVICE_ID", "6ae74bcd7f2e6c302fa9194e_hbot")


def detect_image(data):
    """根据魔数判断图片格式，返回 (格式名, MIME) 或 (None, None)。"""
    if data[:3] == b"\xff\xd8\xff":
        return "jpeg", "image/jpeg"
    if data[:8] == b"\x89PNG\r\n\x1a\n":
        return "png", "image/png"
    if data[:6] in (b"GIF87a", b"GIF89a"):
        return "gif", "image/gif"
    if data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        return "webp", "image/webp"
    if data[:2] == b"BM":
        return "bmp", "image/bmp"
    return None, None


FLIP_MODES = ("180", "vertical", "horizontal", "none")


def apply_flip(image, mode):
    """客户端图像上下颠倒，默认旋转 180° 校正。"""
    if mode == "180":
        return cv2.rotate(image, cv2.ROTATE_180)
    if mode == "vertical":
        return cv2.flip(image, 0)
    if mode == "horizontal":
        return cv2.flip(image, 1)
    return image


def extract_image(body, content_type):
    """从请求体中取出图片字节，返回 (data, filename) 或 (None, None)。"""
    if not content_type.lower().startswith("multipart/form-data"):
        return body, None

    raw = (
        b"Content-Type: " + content_type.encode("latin-1") + b"\r\n"
        b"MIME-Version: 1.0\r\n\r\n" + body
    )
    msg = email.message_from_bytes(raw, policy=policy.default)
    for part in msg.iter_parts():
        data = part.get_payload(decode=True)
        if data:
            return data, part.get_filename()
    return None, None


def timestamp_name(ext, directory):
    while True:
        now = time.time()
        stamp = time.strftime("%Y%m%d_%H%M%S", time.localtime(now))
        name = f"{stamp}_{int(now * 1000) % 1000:03d}.{ext}"
        if not os.path.exists(os.path.join(directory, name)):
            return name
        time.sleep(0.002)


def captured_at_from_stem(stem):
    """将 uploads/YYYYmmdd_HHMMSS_mmm 转成带时区的 ISO 时间。"""
    try:
        parsed = datetime.datetime.strptime(stem, "%Y%m%d_%H%M%S_%f")
        return parsed.astimezone().astimezone(datetime.timezone.utc)
    except ValueError:
        return datetime.datetime.now(datetime.timezone.utc)


def upload_annotated_frame(server, annotated_path, analysis, stem):
    """把标注图推送到 Go 数据收集服务，失败时只记录错误，不阻塞 ESP 响应。"""
    base_url = (server.data_collect_url or "").rstrip("/")
    if not base_url:
        return None
    try:
        with open(annotated_path, "rb") as handle:
            image = handle.read()
    except OSError as exc:
        return {"ok": False, "error": f"read annotated image: {exc}"}

    boundary = "----PostureFrame" + uuid.uuid4().hex
    parts = []

    def field(name, value):
        parts.append(
            f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n'
            f"{value}\r\n".encode("utf-8")
        )

    def file_part(name, filename, content, content_type):
        parts.append(
            f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"; '
            f'filename="{filename}"\r\nContent-Type: {content_type}\r\n\r\n'.encode("utf-8")
        )
        parts.append(content)
        parts.append(b"\r\n")

    metadata = {
        "issues": analysis.get("issues", []),
        "metrics": analysis.get("metrics", {}),
        "keypoints_count": len(analysis.get("keypoints", {}) or {}),
    }
    field("device_id", server.device_id)
    field("captured_at", captured_at_from_stem(stem).isoformat())
    field("status", str(analysis.get("status", "")))
    field("summary", str(analysis.get("summary", "")))
    field("person_detected", "true" if analysis.get("person_detected") else "false")
    field("request_id", stem)
    field("metadata", json.dumps(metadata, ensure_ascii=False))
    file_part("image", os.path.basename(annotated_path), image, "image/jpeg")

    body = b"".join(parts) + f"--{boundary}--\r\n".encode("utf-8")
    headers = {
        "Content-Type": f"multipart/form-data; boundary={boundary}",
        "Content-Length": str(len(body)),
    }
    if server.data_collect_token:
        headers["Authorization"] = f"Bearer {server.data_collect_token}"

    request = urllib.request.Request(
        base_url + "/api/v1/frames", data=body, headers=headers, method="POST"
    )
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(request, timeout=3) as response:
            payload = json.loads(response.read().decode("utf-8"))
        data = payload.get("data", {})
        return {"ok": True, "frame_id": data.get("id"), "image_url": data.get("image_url")}
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", "replace")[:300]
        return {"ok": False, "error": f"HTTP {exc.code}: {detail}"}
    except Exception as exc:  # noqa: BLE001 - best-effort side channel
        return {"ok": False, "error": str(exc)}


class ImageHandler(BaseHTTPRequestHandler):
    server_version = "ImageReceiver/2.0"
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        if self.path == "/":
            self._send_json(
                200,
                {
                    "ok": True,
                    "message": "POST an image to /upload",
                    "examples": [
                        "curl -F 'file=@photo.jpg' http://<host>:8000/upload",
                        "curl --data-binary @photo.jpg -H 'Content-Type: image/jpeg' "
                        "http://<host>:8000/upload",
                    ],
                },
            )
            return
        if self.path.startswith("/uploads/"):
            self._serve_upload(self.path[len("/uploads/"):])
            return
        self._send_json(404, {"ok": False, "error": "not found"})

    def _serve_upload(self, name):
        name = os.path.basename(name)
        path = os.path.join(self.server.upload_dir, name)
        if not name or not os.path.isfile(path):
            self._send_json(404, {"ok": False, "error": "file not found"})
            return
        ctype = mimetypes.guess_type(path)[0] or "application/octet-stream"
        with open(path, "rb") as f:
            data = f.read()
        self.send_response(200)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        if self.path != "/upload":
            self._send_json(404, {"ok": False, "error": "not found"})
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self._send_json(400, {"ok": False, "error": "invalid Content-Length"})
            return

        if length <= 0:
            self._send_json(400, {"ok": False, "error": "empty body"})
            return
        if length > self.server.max_bytes:
            self._send_json(
                413,
                {"ok": False, "error": f"body too large (limit {self.server.max_bytes} bytes)"},
            )
            return

        body = self.rfile.read(length)
        if len(body) != length:
            self._send_json(400, {"ok": False, "error": "incomplete body"})
            return

        raw, filename = extract_image(body, self.headers.get("Content-Type", ""))
        if not raw:
            self._send_json(400, {"ok": False, "error": "no file part in body"})
            return

        fmt, _mime = detect_image(raw)
        if fmt is None:
            self._send_json(415, {"ok": False, "error": "unsupported or corrupt image"})
            return

        image = cv2.imdecode(np.frombuffer(raw, np.uint8), cv2.IMREAD_COLOR)
        if image is None:
            self._send_json(415, {"ok": False, "error": "cannot decode image"})
            return

        image = apply_flip(image, self.server.flip)

        started = time.time()
        stem = os.path.splitext(timestamp_name(EXTENSIONS[fmt], self.server.upload_dir))[0]
        original_name = f"{stem}.{EXTENSIONS[fmt]}"
        original_path = os.path.join(self.server.upload_dir, original_name)
        ok, encoded = cv2.imencode(f".{EXTENSIONS[fmt]}", image)
        if not ok:
            self._send_json(500, {"ok": False, "error": "cannot re-encode image"})
            return
        with open(original_path, "wb") as f:
            f.write(encoded.tobytes())

        annotated_name = f"{stem}_annotated.jpg"
        annotated_path = os.path.join(self.server.upload_dir, annotated_name)
        try:
            landmarks, analysis = self.server.analyzer.infer(image)
            annotated = draw_annotated(image, landmarks or [], analysis)
            cv2.imwrite(annotated_path, annotated, [int(cv2.IMWRITE_JPEG_QUALITY), 90])
        except Exception:
            logging.exception("posture analysis failed for %s", original_name)
            self._send_json(
                500,
                {
                    "ok": False,
                    "error": "posture analysis failed",
                    "saved": f"uploads/{original_name}",
                },
            )
            return

        elapsed_ms = int((time.time() - started) * 1000)
        logging.info(
            "image %s (%d bytes, %s) from %s -> posture=%s in %dms",
            original_name, len(raw), fmt, self.client_address[0], analysis["status"], elapsed_ms,
        )

        collect = None
        if self.server.data_collect_url:
            collect = upload_annotated_frame(self.server, annotated_path, analysis, stem)
            if collect and collect.get("ok"):
                logging.info(
                    "uploaded annotated frame %s to data collector (frame_id=%s)",
                    annotated_name, collect.get("frame_id"),
                )
            else:
                logging.warning(
                    "upload annotated frame %s failed: %s",
                    annotated_name, (collect or {}).get("error", "unknown error"),
                )

        self._send_json(
            200,
            {
                "ok": True,
                "message": analysis["summary"] if analysis["person_detected"] else "image received",
                "bytes": len(raw),
                "format": fmt,
                "filename": filename,
                "flip": self.server.flip,
                "saved": f"uploads/{original_name}",
                "annotated": f"uploads/{annotated_name}",
                "annotated_url": f"/uploads/{annotated_name}",
                "person_detected": analysis["person_detected"],
                "posture": {
                    "status": analysis["status"],
                    "summary": analysis["summary"],
                    "issues": analysis["issues"],
                    "metrics": analysis["metrics"],
                },
                "keypoints": analysis.get("keypoints", {}),
                "elapsed_ms": elapsed_ms,
                "data_collect": collect,
            },
        )

    def _send_json(self, status, payload):
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        logging.info("%s - %s", self.client_address[0], fmt % args)


def main():
    parser = argparse.ArgumentParser(description="Image upload receiver with posture analysis")
    parser.add_argument("--host", default="0.0.0.0", help="bind address (default 0.0.0.0)")
    parser.add_argument("--port", type=int, default=8000, help="port (default 8000)")
    parser.add_argument("--max-mb", type=float, default=10, help="max body size in MB (default 10)")
    parser.add_argument("--uploads", default="uploads", help="upload directory (default ./uploads)")
    parser.add_argument("--model", default=None, help="path to pose_landmarker .task file")
    parser.add_argument(
        "--data-collect-url",
        default=DEFAULT_DATA_COLLECT_URL,
        help="Go data collector base URL; empty disables annotated frame upload",
    )
    parser.add_argument(
        "--data-collect-token",
        default=DEFAULT_DATA_COLLECT_TOKEN,
        help="Bearer token for the Go data collector API",
    )
    parser.add_argument(
        "--device-id",
        default=DEFAULT_DEVICE_ID,
        help="device id attached to uploaded annotated frames",
    )
    parser.add_argument(
        "--flip",
        choices=FLIP_MODES,
        default="180",
        help="correct client-side orientation: 180 (default), vertical, horizontal, none",
    )
    args = parser.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    upload_dir = os.path.abspath(args.uploads)
    os.makedirs(upload_dir, exist_ok=True)

    analyzer = PoseAnalyzer(args.model) if args.model else PoseAnalyzer()

    server = ThreadingHTTPServer((args.host, args.port), ImageHandler)
    server.max_bytes = int(args.max_mb * 1024 * 1024)
    server.upload_dir = upload_dir
    server.analyzer = analyzer
    server.flip = args.flip
    server.data_collect_url = (args.data_collect_url or "").strip()
    server.data_collect_token = (args.data_collect_token or "").strip()
    server.device_id = (args.device_id or "").strip()

    logging.info("listening on http://%s:%d  (POST /upload)", args.host, args.port)
    logging.info("saving images to %s", upload_dir)
    if server.data_collect_url:
        logging.info(
            "uploading annotated frames to %s as device %s",
            server.data_collect_url, server.device_id,
        )
    else:
        logging.info("annotated frame upload disabled")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logging.info("shutting down")
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
