import type { ReactNode } from 'react'

import { Sparkline } from './Sparkline'

export type Tone = 'cyan' | 'violet' | 'amber' | 'emerald' | 'rose' | 'slate'

interface MetricCardProps {
  icon: ReactNode
  label: string
  value: string
  unit?: string
  hint?: string
  tone: Tone
  spark?: number[]
  live?: boolean
}

export function MetricCard({
  icon,
  label,
  value,
  unit,
  hint,
  tone,
  spark,
  live = false,
}: MetricCardProps) {
  return (
    <article className={`metric-card tone-${tone}`}>
      <div className="metric-head">
        <span className="metric-icon">{icon}</span>
        <span className="metric-label">{label}</span>
        {live && <span className="metric-live">LIVE</span>}
      </div>
      <div className="metric-value-row">
        <span className="metric-value">{value}</span>
        {unit && <span className="metric-unit">{unit}</span>}
      </div>
      {hint && <div className="metric-hint">{hint}</div>}
      {spark && spark.length > 1 && (
        <div className="metric-spark">
          <Sparkline data={spark} color="currentColor" height={34} />
        </div>
      )}
    </article>
  )
}
