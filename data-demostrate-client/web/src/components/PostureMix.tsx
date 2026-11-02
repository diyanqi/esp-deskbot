import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'

import type { PosturePoint, StatusKind } from '../types'
import { STATUS_META, formatNumber } from '../utils'
import { ChartEmpty } from './ChartEmpty'
import { PanelHeader } from './PanelHeader'

interface PostureMixProps {
  points: PosturePoint[]
}

const STATUS_ORDER: StatusKind[] = ['good', 'warning', 'bad', 'unknown']

export function PostureMix({ points }: PostureMixProps) {
  const counts = STATUS_ORDER.map((status) => ({
    status,
    name: STATUS_META[status].zh,
    value: points.filter((point) => point.status === status).length,
    color: STATUS_META[status].color,
  }))
  const total = counts.reduce((sum, entry) => sum + entry.value, 0)
  const visible = counts.filter((entry) => entry.value > 0)

  return (
    <section className="panel chart-panel mix-panel">
      <PanelHeader
        kicker="POSTURE / DISTRIBUTION"
        title="坐姿分布"
        subtitle="最近姿态样本占比 · 家长视角"
        actions={<span className="micro-badge">{formatNumber(total, 0)} SAMPLES</span>}
      />

      {total === 0 ? (
        <ChartEmpty label="等待姿态样本" />
      ) : (
        <>
          <div className="donut-wrap">
            <ResponsiveContainer width="100%" height={210}>
              <PieChart>
                <Pie
                  data={visible}
                  dataKey="value"
                  nameKey="name"
                  innerRadius={62}
                  outerRadius={86}
                  paddingAngle={4}
                  stroke="rgba(5, 7, 12, 0.9)"
                  strokeWidth={3}
                  isAnimationActive={false}
                >
                  {visible.map((entry) => (
                    <Cell key={entry.status} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    background: 'rgba(6, 11, 20, 0.94)',
                    border: '1px solid rgba(148, 163, 184, 0.18)',
                    borderRadius: 12,
                    color: '#e2e8f0',
                    fontSize: 12,
                  }}
                  formatter={(value, name) => [
                    `${formatNumber(Number(value), 0)} 次`,
                    String(name),
                  ]}
                />
              </PieChart>
            </ResponsiveContainer>
            <div className="donut-center">
              <span className="donut-value">{formatNumber(total, 0)}</span>
              <span className="donut-label">SAMPLES</span>
            </div>
          </div>

          <div className="mix-legend">
            {counts.map((entry) => {
              const percent = total > 0 ? Math.round((entry.value / total) * 100) : 0
              return (
                <div key={entry.status} className="mix-row">
                  <span className="mix-dot" style={{ backgroundColor: entry.color }} />
                  <span className="mix-name">{entry.name}</span>
                  <span className="mix-bar">
                    <span
                      className="mix-bar-fill"
                      style={{ width: `${percent}%`, backgroundColor: entry.color }}
                    />
                  </span>
                  <span className="mix-percent">{percent}%</span>
                </div>
              )
            })}
          </div>
        </>
      )}
    </section>
  )
}
