import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Cpu, Wifi } from 'lucide-react'

import type { ChartPoint, RangeKey } from '../types'
import { formatClock, formatNumber, rangeLabel } from '../utils'
import { ChartEmpty } from './ChartEmpty'
import { PanelHeader } from './PanelHeader'

interface HealthChartProps {
  rssi: ChartPoint[]
  heap: ChartPoint[]
  range: RangeKey
}

interface HealthPoint {
  time: string
  rssi?: number
  heap?: number
}

function mergeSeries(rssi: ChartPoint[], heap: ChartPoint[]): HealthPoint[] {
  const merged = new Map<string, HealthPoint>()
  for (const point of rssi) {
    const current = merged.get(point.time) ?? { time: point.time }
    current.rssi = point.value
    merged.set(point.time, current)
  }
  for (const point of heap) {
    const current = merged.get(point.time) ?? { time: point.time }
    current.heap = point.value / 1024
    merged.set(point.time, current)
  }
  return Array.from(merged.values()).sort((left, right) => left.time.localeCompare(right.time))
}

export function HealthChart({ rssi, heap, range }: HealthChartProps) {
  const data = mergeSeries(rssi, heap)
  const rssiValues = rssi.map((point) => point.value)
  const heapValues = heap.map((point) => point.value / 1024)

  const rssiDomain: [number, number] = rssiValues.length
    ? [
        Math.max(-100, Math.floor(Math.min(...rssiValues)) - 4),
        Math.min(-20, Math.ceil(Math.max(...rssiValues)) + 4),
      ]
    : [-100, -20]
  const heapDomain: [number, number] = heapValues.length
    ? [
        Math.max(0, Math.floor(Math.min(...heapValues)) - 20),
        Math.ceil(Math.max(...heapValues)) + 20,
      ]
    : [100, 260]

  return (
    <section className="panel chart-panel health-panel">
      <PanelHeader
        kicker="DEVICE / HEALTH"
        title="设备连接健康度"
        subtitle={`最近 ${rangeLabel(range)} · RSSI 与空闲内存`}
        actions={
          <div className="chart-legends">
            <span className="chart-legend">
              <span className="legend-dot" style={{ backgroundColor: '#38bdf8' }} />
              <Wifi size={13} /> RSSI
            </span>
            <span className="chart-legend">
              <span className="legend-dot" style={{ backgroundColor: '#a78bfa' }} />
              <Cpu size={13} /> HEAP
            </span>
          </div>
        }
      />

      {data.length < 2 ? (
        <ChartEmpty label="等待设备健康数据" />
      ) : (
        <div className="chart-shell">
          <ResponsiveContainer width="100%" height={242}>
            <ComposedChart data={data} margin={{ top: 14, right: 6, bottom: 4, left: -18 }}>
              <defs>
                <linearGradient id="rssiFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#38bdf8" stopOpacity={0.26} />
                  <stop offset="100%" stopColor="#38bdf8" stopOpacity={0} />
                </linearGradient>
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
                minTickGap={48}
              />
              <YAxis
                yAxisId="rssi"
                domain={rssiDomain}
                tickFormatter={(value) => `${value}`}
                tick={{ fill: '#5b6b82', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                width={42}
              />
              <YAxis
                yAxisId="heap"
                orientation="right"
                domain={heapDomain}
                tickFormatter={(value) => `${value}`}
                tick={{ fill: '#5b6b82', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                width={42}
              />
              <Tooltip
                cursor={{ stroke: 'rgba(148, 163, 184, 0.2)', strokeWidth: 1 }}
                contentStyle={{
                  background: 'rgba(6, 11, 20, 0.94)',
                  border: '1px solid rgba(148, 163, 184, 0.18)',
                  borderRadius: 12,
                  color: '#e2e8f0',
                  fontSize: 12,
                }}
                labelStyle={{ color: '#7c8aa0' }}
                labelFormatter={(value) => formatClock(String(value))}
                formatter={(value, name) => {
                  const numeric = Number(value)
                  if (name === 'rssi') {
                    return [`${formatNumber(numeric, 0)} dBm`, 'RSSI']
                  }
                  return [`${formatNumber(numeric, 0)} KB`, 'Free heap']
                }}
              />
              <Area
                yAxisId="rssi"
                type="monotone"
                dataKey="rssi"
                stroke="#38bdf8"
                strokeWidth={2.1}
                fill="url(#rssiFill)"
                dot={false}
                activeDot={{ r: 4, fill: '#bae6fd', stroke: '#0284c7', strokeWidth: 2 }}
                connectNulls
                isAnimationActive={false}
              />
              <Line
                yAxisId="heap"
                type="monotone"
                dataKey="heap"
                stroke="#a78bfa"
                strokeWidth={1.8}
                dot={false}
                activeDot={{ r: 3.5, fill: '#ddd6fe', stroke: '#7c3aed', strokeWidth: 2 }}
                connectNulls
                isAnimationActive={false}
              />
            </ComposedChart>
          </ResponsiveContainer>
          <div className="chart-foot">
            <span>左轴 RSSI (dBm) · 右轴 Free heap (KB)</span>
            <span>{data.length} 个采样点</span>
          </div>
        </div>
      )}
    </section>
  )
}
