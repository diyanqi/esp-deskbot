import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Sun } from 'lucide-react'

import type { ChartPoint, RangeKey } from '../types'
import { formatClock, formatNumber, rangeLabel } from '../utils'
import { ChartEmpty } from './ChartEmpty'
import { PanelHeader } from './PanelHeader'

interface LightChartProps {
  data: ChartPoint[]
  range: RangeKey
}

export function LightChart({ data, range }: LightChartProps) {
  const values = data.map((point) => point.value)
  const current = values.length ? values[values.length - 1] : null
  const average = values.length ? values.reduce((sum, value) => sum + value, 0) / values.length : null
  const peak = values.length ? Math.max(...values) : null
  const bucketLabel = range === '1h' || range === '6h' ? '每分钟聚合' : '每小时聚合'

  return (
    <section className="panel chart-panel light-panel">
      <PanelHeader
        kicker="ENVIRONMENT / LIGHT"
        title="桌面光照"
        subtitle={`最近 ${rangeLabel(range)} · ${bucketLabel}`}
        actions={
          <div className="chart-stat-row">
            <div className="chart-stat">
              <span className="chart-stat-label">NOW</span>
              <span className="chart-stat-value">
                {current === null ? '—' : `${formatNumber(current, 0)}%`}
              </span>
            </div>
            <div className="chart-stat">
              <span className="chart-stat-label">AVG</span>
              <span className="chart-stat-value">
                {average === null ? '—' : `${formatNumber(average, 0)}%`}
              </span>
            </div>
            <div className="chart-stat">
              <span className="chart-stat-label">PEAK</span>
              <span className="chart-stat-value">
                {peak === null ? '—' : `${formatNumber(peak, 0)}%`}
              </span>
            </div>
          </div>
        }
      />

      {data.length < 2 ? (
        <ChartEmpty label="等待光照时序数据" />
      ) : (
        <div className="chart-shell">
          <ResponsiveContainer width="100%" height={266}>
            <AreaChart data={data} margin={{ top: 16, right: 12, bottom: 4, left: -18 }}>
              <defs>
                <linearGradient id="lightFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#22d3ee" stopOpacity={0.34} />
                  <stop offset="60%" stopColor="#22d3ee" stopOpacity={0.08} />
                  <stop offset="100%" stopColor="#22d3ee" stopOpacity={0} />
                </linearGradient>
                <filter id="lightGlow" x="-20%" y="-20%" width="140%" height="140%">
                  <feGaussianBlur stdDeviation="3" result="blur" />
                  <feMerge>
                    <feMergeNode in="blur" />
                    <feMergeNode in="SourceGraphic" />
                  </feMerge>
                </filter>
              </defs>
              <CartesianGrid
                strokeDasharray="3 7"
                stroke="rgba(148, 163, 184, 0.09)"
                vertical={false}
              />
              <XAxis
                dataKey="time"
                tickFormatter={(value) => formatClock(String(value))}
                tick={{ fill: '#5b6b82', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                minTickGap={42}
              />
              <YAxis
                domain={[0, 100]}
                tickFormatter={(value) => `${value}%`}
                tick={{ fill: '#5b6b82', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                width={46}
              />
              <Tooltip
                cursor={{ stroke: 'rgba(34, 211, 238, 0.25)', strokeWidth: 1 }}
                contentStyle={{
                  background: 'rgba(6, 11, 20, 0.94)',
                  border: '1px solid rgba(34, 211, 238, 0.24)',
                  borderRadius: 12,
                  boxShadow: '0 18px 40px rgba(0, 0, 0, 0.45)',
                  color: '#e2e8f0',
                  fontSize: 12,
                }}
                labelStyle={{ color: '#7c8aa0' }}
                labelFormatter={(value) => formatClock(String(value))}
                formatter={(value) => [`${formatNumber(Number(value), 0)}%`, '光照']}
              />
              <Area
                type="monotone"
                dataKey="value"
                stroke="#22d3ee"
                strokeWidth={2.4}
                fill="url(#lightFill)"
                filter="url(#lightGlow)"
                dot={false}
                activeDot={{ r: 4, fill: '#a5f3fc', stroke: '#0891b2', strokeWidth: 2 }}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
          <div className="chart-foot">
            <span>
              <Sun size={13} /> 光照来自 Environment.light
            </span>
            <span>{data.length} 个聚合点</span>
          </div>
        </div>
      )}
    </section>
  )
}
