import type { CSSProperties } from 'react'
import { AlertTriangle, Clock3, Cpu, Hash, History, Radio, ScanEye, UserRound } from 'lucide-react'

import type { PosturePoint, StatusKind } from '../types'
import { STATUS_META, formatClock, formatRelativeTime, formatUptime } from '../utils'
import { PanelHeader } from './PanelHeader'

interface PostureHeroProps {
  status: StatusKind
  summary: string
  personDetected: boolean
  alert: number
  issueCount: number
  lastSeq: number
  points: PosturePoint[]
  lastSeen: string | null
  uptime: number
  firmware: string
  reportInterval: number
  replayActive?: boolean
  replayTime?: string | null
}

const STATUS_ORDER: StatusKind[] = ['good', 'warning', 'bad', 'unknown']

export function PostureHero({
  status,
  summary,
  personDetected,
  alert,
  issueCount,
  lastSeq,
  points,
  lastSeen,
  uptime,
  firmware,
  reportInterval,
  replayActive = false,
  replayTime = null,
}: PostureHeroProps) {
  const meta = STATUS_META[status]
  const timeline = points.slice(-72)
  const style = {
    '--status-color': meta.color,
    '--status-soft': meta.soft,
  } as CSSProperties

  return (
    <section className={`panel posture-hero ${meta.className}`} style={style}>
      <PanelHeader
        kicker="POSTURE / CHILD"
        title="孩子当前坐姿"
        subtitle="实时姿态识别 · 家长端守护"
        actions={
          <div className="posture-actions">
            {replayActive && (
              <span className="replay-badge">
                <History size={12} />
                {replayTime ? `回放 ${formatClock(replayTime)}` : '回放中'}
              </span>
            )}
            <span className="status-badge" style={{ color: meta.color }}>
              <span className="status-badge-dot" />
              {meta.label}
            </span>
          </div>
        }
      />

      <div className="posture-stage">
        <div className="posture-orb-wrap">
          <div className="orbit orbit-a" />
          <div className="orbit orbit-b" />
          <div className="posture-orb">
            <div className="orb-grid" />
            <div className="orb-scan" />
            <div className="face" aria-hidden="true">
              <span className="face-brow brow-left" />
              <span className="face-brow brow-right" />
              <span className="face-eye eye-left" />
              <span className="face-eye eye-right" />
              <span className="face-mouth" />
            </div>
          </div>
          <div className="orb-caption">
            <ScanEye size={14} />
            <span>{replayActive ? 'REPLAY SNAPSHOT' : personDetected ? 'CHILD DETECTED' : 'NO CHILD'}</span>
          </div>
        </div>

        <div className="posture-copy">
          <div className="posture-eyebrow">
            <span className="pulse-dot" />
            {replayActive ? `历史回放 · ${replayTime ? formatClock(replayTime) : '—'}` : meta.zh}
          </div>
          <h2>{summary || meta.zh}</h2>
          <p>
            {personDetected
              ? '已检测到孩子，正在分析坐姿与身体偏移。'
              : '孩子暂时离开书桌，设备保持守护状态。'}
          </p>

          <div className="posture-chips">
            <div className={`chip ${alert ? 'chip-alert' : ''}`}>
              <AlertTriangle size={14} />
              <span>{alert ? '告警中' : '无告警'}</span>
            </div>
            <div className="chip">
              <UserRound size={14} />
              <span>{personDetected ? '有人' : '无人'}</span>
            </div>
            <div className="chip">
              <Hash size={14} />
              <span>issue {issueCount}</span>
            </div>
            <div className="chip">
              <ScanEye size={14} />
              <span>seq {lastSeq || '—'}</span>
            </div>
            <div className="chip">
              <Clock3 size={14} />
              <span>{formatRelativeTime(lastSeen)}</span>
            </div>
          </div>
        </div>
      </div>

      <div className="timeline-block">
        <div className="timeline-head">
          <span>STATUS TIMELINE</span>
          <span>{timeline.length ? `最近 ${timeline.length} 个样本` : '等待姿态样本'}</span>
        </div>
        <div className="status-timeline">
          {timeline.length === 0 && <div className="timeline-placeholder" />}
          {timeline.map((point, index) => (
            <span
              key={`${point.time}-${index}`}
              className="timeline-segment"
              style={{
                backgroundColor: STATUS_META[point.status].color,
                opacity: 0.35 + (0.65 * (index + 1)) / timeline.length,
              }}
              title={`${STATUS_META[point.status].zh} · ${formatRelativeTime(point.time)}`}
            />
          ))}
        </div>
        <div className="timeline-legend">
          {STATUS_ORDER.map((key) => (
            <span key={key} className="legend-item">
              <span className="legend-dot" style={{ backgroundColor: STATUS_META[key].color }} />
              {STATUS_META[key].zh}
            </span>
          ))}
        </div>
      </div>

      <div className="posture-device-info">
        <span>
          <Cpu size={13} /> uptime {formatUptime(uptime)}
        </span>
        <span>
          <Radio size={13} /> {firmware || '—'}
        </span>
        <span>report {reportInterval}s</span>
      </div>
    </section>
  )
}
