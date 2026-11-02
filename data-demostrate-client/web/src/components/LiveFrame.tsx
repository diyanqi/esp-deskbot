import {
  Camera,
  Clock3,
  ImageOff,
  ScanEye,
  Sparkles,
  UserRound,
} from 'lucide-react'

import type { PostureFrame } from '../types'
import { STATUS_META, formatClock, formatRelativeTime, normalizeStatus } from '../utils'
import { PanelHeader } from './PanelHeader'

interface LiveFrameProps {
  frames: PostureFrame[]
  selectedFrame: PostureFrame | undefined
  imageUrl: string | null
  loading: boolean
  error: string | null
  onSelect: (frameId: number) => void
}

function keypointCount(frame: PostureFrame | undefined): number | null {
  if (!frame?.metadata || typeof frame.metadata !== 'object' || Array.isArray(frame.metadata)) {
    return null
  }
  const value = (frame.metadata as Record<string, unknown>).keypoints_count
  return typeof value === 'number' ? value : null
}

export function LiveFrame({
  frames,
  selectedFrame,
  imageUrl,
  loading,
  error,
  onSelect,
}: LiveFrameProps) {
  const status = normalizeStatus(selectedFrame?.status)
  const meta = STATUS_META[status]
  const points = keypointCount(selectedFrame)
  const recent = frames.slice(0, 8)

  return (
    <section className="panel live-frame-panel">
      <PanelHeader
        kicker="CAMERA / POSTURE VISION"
        title="孩子的实时坐姿画面"
        subtitle="本地 MediaPipe 关键点识别 · 标注图自动上传"
        actions={
          <div className="frame-actions">
            <span className="status-badge" style={{ color: meta.color }}>
              <span className="status-badge-dot" />
              {meta.label}
            </span>
            <span className="micro-badge">{frames.length} FRAMES</span>
          </div>
        }
      />

      <div className={`frame-stage ${imageUrl ? 'has-image' : ''}`}>
        {imageUrl ? (
          <img
            className="frame-image"
            src={imageUrl}
            alt={`坐姿标注画面 · ${selectedFrame?.summary ?? meta.zh}`}
          />
        ) : (
          <div className="frame-placeholder">
            {loading ? (
              <>
                <span className="frame-loader" />
                <strong>正在加载标注画面…</strong>
                <span>从数据服务拉取最新帧</span>
              </>
            ) : (
              <>
                <span className="frame-placeholder-icon">
                  <ImageOff size={26} />
                </span>
                <strong>还没有标注画面</strong>
                <span>
                  本地识别服务生成 <code>_annotated.jpg</code> 后会自动上传到这里。
                </span>
              </>
            )}
          </div>
        )}

        {imageUrl && (
          <>
            <div className="frame-overlay-top">
              <span className="frame-chip">
                <Sparkles size={13} /> AI 姿态标注
              </span>
              <span className="frame-chip frame-chip-live">
                <span className="live-dot" /> SNAPSHOT
              </span>
            </div>
            <div className="frame-overlay-bottom">
              <div>
                <span className="frame-overlay-status" style={{ color: meta.color }}>
                  {meta.zh}
                </span>
                <strong>{selectedFrame?.summary || '已收到新的坐姿标注画面'}</strong>
              </div>
              <div className="frame-overlay-meta">
                <span>
                  <Clock3 size={12} />
                  {selectedFrame ? formatClock(selectedFrame.captured_at) : '—'}
                </span>
                <span>
                  <UserRound size={12} />
                  {selectedFrame?.person_detected ? '检测到孩子' : '画面中无人'}
                </span>
                {points !== null && (
                  <span>
                    <ScanEye size={12} />
                    {points} 个关键点
                  </span>
                )}
              </div>
            </div>
          </>
        )}

        {error && <div className="frame-error">{error}</div>}
      </div>

      <div className="frame-strip">
        <div className="frame-strip-head">
          <span>
            <Camera size={13} /> 最近画面
          </span>
          <span>{recent.length ? '点击切换 · 标注图来自本地识别服务' : '等待识别服务上传'}</span>
        </div>
        {recent.length > 0 ? (
          <div className="frame-strip-list">
            {recent.map((frame) => {
              const itemMeta = STATUS_META[normalizeStatus(frame.status)]
              return (
                <button
                  key={frame.id}
                  type="button"
                  className={`frame-thumb ${frame.id === selectedFrame?.id ? 'is-active' : ''}`}
                  onClick={() => onSelect(frame.id)}
                >
                  <span
                    className="frame-thumb-dot"
                    style={{ backgroundColor: itemMeta.color }}
                  />
                  <span>{formatClock(frame.captured_at)}</span>
                  <span className="frame-thumb-status">{itemMeta.label}</span>
                  <span className="frame-thumb-ago">
                    {formatRelativeTime(frame.captured_at)}
                  </span>
                </button>
              )
            })}
          </div>
        ) : (
          <div className="frame-strip-empty">识别服务运行后，这里会显示最近 8 帧。</div>
        )}
      </div>
    </section>
  )
}
