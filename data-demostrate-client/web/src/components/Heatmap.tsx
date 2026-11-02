import { useState } from 'react'
import { AlertTriangle, Grid3X3, ShieldCheck, Sun } from 'lucide-react'

import type { InsightsHeatmapCell, InsightsRiskWindow } from '../types'
import { formatNumber } from '../utils'
import { PanelHeader } from './PanelHeader'

interface HeatmapProps {
  cells: InsightsHeatmapCell[]
  riskWindows: InsightsRiskWindow[]
}

type Metric = 'good' | 'alert' | 'light'

const WEEKDAYS = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

const METRICS: Array<{ key: Metric; label: string; icon: typeof ShieldCheck }> = [
  { key: 'good', label: '良好率', icon: ShieldCheck },
  { key: 'alert', label: '告警率', icon: AlertTriangle },
  { key: 'light', label: '光照', icon: Sun },
]

function metricValue(cell: InsightsHeatmapCell, metric: Metric): number {
  if (metric === 'good') {
    return cell.good_rate
  }
  if (metric === 'alert') {
    return cell.alert_rate
  }
  return Math.min(1, Math.max(0, cell.avg_light / 100))
}

function cellColor(value: number | null, metric: Metric): string {
  if (value === null) {
    return 'rgba(148, 163, 184, 0.055)'
  }
  const alpha = 0.08 + Math.min(1, Math.max(0, value)) * 0.82
  if (metric === 'good') {
    return `rgba(52, 211, 153, ${alpha})`
  }
  if (metric === 'alert') {
    return `rgba(251, 113, 133, ${alpha})`
  }
  return `rgba(251, 191, 36, ${alpha})`
}

function weekdayName(weekday: number): string {
  return WEEKDAYS[weekday - 1] ?? '—'
}

export function Heatmap({ cells, riskWindows }: HeatmapProps) {
  const [metric, setMetric] = useState<Metric>('alert')
  const matrix: Array<Array<InsightsHeatmapCell | null>> = Array.from({ length: 7 }, () =>
    Array.from({ length: 24 }, () => null),
  )
  for (const cell of cells) {
    if (cell.weekday >= 1 && cell.weekday <= 7 && cell.hour >= 0 && cell.hour < 24) {
      matrix[cell.weekday - 1][cell.hour] = cell
    }
  }

  const activeMetric = METRICS.find((entry) => entry.key === metric) ?? METRICS[0]
  const topRisk = riskWindows[0]

  return (
    <section className="panel heatmap-panel">
      <PanelHeader
        kicker="PATTERN / 7 × 24"
        title="一周坐姿热力图"
        subtitle="按星期与小时聚合 · 颜色越深表示问题越集中"
        actions={
          <div className="heatmap-toggle" role="group" aria-label="热力图指标">
            {METRICS.map((entry) => {
              const Icon = entry.icon
              return (
                <button
                  key={entry.key}
                  type="button"
                  className={metric === entry.key ? 'is-active' : ''}
                  onClick={() => setMetric(entry.key)}
                >
                  <Icon size={12} /> {entry.label}
                </button>
              )
            })}
          </div>
        }
      />

      {cells.length === 0 ? (
        <div className="heatmap-empty">
          <Grid3X3 size={24} />
          <strong>还没有一周的数据</strong>
          <span>设备持续上报后，这里会显示 7 × 24 小时的坐姿规律。</span>
        </div>
      ) : (
        <>
          <div className="heatmap-scroll">
            <div className="heatmap-grid">
              <div className="heatmap-corner" />
              {Array.from({ length: 24 }, (_, hour) => (
                <div key={`hour-${hour}`} className="heatmap-hour">
                  {hour % 3 === 0 ? `${hour}:00` : ''}
                </div>
              ))}
              {matrix.map((row, rowIndex) => (
                <div key={`row-${rowIndex}`} className="heatmap-row">
                  <div className="heatmap-weekday">{WEEKDAYS[rowIndex]}</div>
                  {row.map((cell, hour) => {
                    const value = cell ? metricValue(cell, metric) : null
                    const label = cell
                      ? `${WEEKDAYS[rowIndex]} ${hour}:00 · 良好率 ${formatNumber(cell.good_rate * 100, 0)}% · 告警率 ${formatNumber(cell.alert_rate * 100, 0)}% · 光照 ${formatNumber(cell.avg_light, 0)}% · ${cell.samples} 样本`
                      : `${WEEKDAYS[rowIndex]} ${hour}:00 · 无数据`
                    return (
                      <div
                        key={`cell-${rowIndex}-${hour}`}
                        className="heatmap-cell"
                        style={{ backgroundColor: cellColor(value, metric) }}
                        title={label}
                        aria-label={label}
                      />
                    )
                  })}
                </div>
              ))}
            </div>
          </div>

          <div className="heatmap-footer">
            <div className="heatmap-legend">
              <span>{activeMetric.label}低</span>
              <span className="heatmap-gradient" data-metric={metric} />
              <span>高</span>
            </div>
            <span className="heatmap-source">7 天聚合 · 每格 = 1 小时</span>
          </div>

          <div className="heatmap-insight">
            <span className="heatmap-insight-icon">
              <AlertTriangle size={15} />
            </span>
            <div>
              <strong>高风险时段</strong>
              <span>
                {topRisk
                  ? `${weekdayName(topRisk.weekday)} ${String(topRisk.hour).padStart(2, '0')}:00–${String((topRisk.hour + 1) % 24).padStart(2, '0')}:00 · 告警率 ${formatNumber(topRisk.alert_rate * 100, 0)}%`
                  : '暂未发现明显的高风险时段'}
              </span>
            </div>
          </div>
        </>
      )}
    </section>
  )
}
