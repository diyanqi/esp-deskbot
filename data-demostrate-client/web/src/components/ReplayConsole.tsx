import { Clock3, Pause, Play, Radio, RotateCcw, ScanEye, Sun, UserRound } from 'lucide-react'

import type { InsightsTimelinePoint } from '../types'
import { STATUS_META, formatDateTime, formatNumber, normalizeStatus } from '../utils'
import { PanelHeader } from './PanelHeader'
import { Sparkline } from './Sparkline'

interface ReplayConsoleProps {
  timeline: InsightsTimelinePoint[]
  active: boolean
  playing: boolean
  index: number
  point: InsightsTimelinePoint | null
  onStart: () => void
  onExit: () => void
  onSeek: (index: number) => void
  onTogglePlaying: () => void
}

export function ReplayConsole({
  timeline,
  active,
  playing,
  index,
  point,
  onStart,
  onExit,
  onSeek,
  onTogglePlaying,
}: ReplayConsoleProps) {
  const displayIndex = active ? index : Math.max(0, timeline.length - 1)
  const current = active ? point ?? timeline[index] ?? null : timeline[displayIndex] ?? null
  const status = normalizeStatus(current?.status)
  const meta = STATUS_META[status]
  const lightValues = timeline.map((entry) => entry.light)

  return (
    <section className="panel replay-panel" id="replay">
      <PanelHeader
        kicker="REPLAY / 24H"
        title="24 小时时间回放"
        subtitle="拖动时间轴，回看孩子今天的坐姿变化"
        actions={
          <span className="micro-badge">
            {timeline.length ? `${timeline.length} 个 5 分钟采样点` : '等待数据'}
          </span>
        }
      />

      {timeline.length === 0 ? (
        <div className="replay-empty">
          <Clock3 size={24} />
          <strong>还没有可回放的数据</strong>
          <span>设备持续上报后，时间轴会自动出现。</span>
        </div>
      ) : (
        <>
          <div className="replay-now">
            <div className="replay-time-block">
              <span className="replay-label">{active ? 'REPLAY TIME' : 'LATEST SAMPLES'}</span>
              <strong>{current ? formatDateTime(current.time) : '—'}</strong>
              <span className="replay-sub">
                {active
                  ? `第 ${index + 1} / ${timeline.length} 个采样点`
                  : '点击开始回放，查看今天的变化轨迹'}
              </span>
            </div>
            <div className="replay-status" style={{ color: meta.color }}>
              <span className="replay-status-dot" style={{ backgroundColor: meta.color }} />
              <strong>{meta.zh}</strong>
              <span>{meta.label}</span>
            </div>
          </div>

          <div className="replay-spark">
            <Sparkline data={lightValues} color="#22d3ee" height={54} />
            <span className="replay-spark-label">LIGHT TRACE</span>
          </div>

          <div className="replay-chips">
            <span className="replay-chip">
              <Sun size={13} /> 光照 {current ? `${formatNumber(current.light, 0)}%` : '—'}
            </span>
            <span className={`replay-chip ${current?.person ? '' : 'is-muted'}`}>
              <UserRound size={13} /> {current?.person ? '在座' : '离开'}
            </span>
            <span className={`replay-chip ${current?.alert ? 'is-alert' : ''}`}>
              <ScanEye size={13} /> {current?.alert ? '坐姿提醒' : '无提醒'}
            </span>
            <span className={`replay-chip ${current?.sound ? 'is-muted' : ''}`}>
              <Radio size={13} /> {current?.sound ? '环境有声音' : '环境安静'}
            </span>
          </div>

          <div className="replay-slider-row">
            <input
              className="replay-slider"
              type="range"
              min={0}
              max={Math.max(0, timeline.length - 1)}
              value={Math.min(displayIndex, Math.max(0, timeline.length - 1))}
              onChange={(event) => onSeek(Number(event.target.value))}
              aria-label="回放时间轴"
            />
            <span className="replay-slider-value">
              {current ? formatDateTime(current.time) : '—'}
            </span>
          </div>

          <div className="replay-actions">
            {active ? (
              <>
                <button type="button" className="btn btn-primary btn-small" onClick={onTogglePlaying}>
                  {playing ? <Pause size={14} /> : <Play size={14} />}
                  {playing ? '暂停' : '继续'}
                </button>
                <button type="button" className="btn btn-ghost btn-small" onClick={onExit}>
                  <RotateCcw size={14} /> 回到实时
                </button>
              </>
            ) : (
              <button type="button" className="btn btn-primary btn-small" onClick={onStart}>
                <Play size={14} /> 开始回放
              </button>
            )}
            <span className="replay-speed">每 220ms 前进 1 个采样点（5 分钟）</span>
          </div>
        </>
      )}
    </section>
  )
}
