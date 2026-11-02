import { useId } from 'react'

interface SparklineProps {
  data: number[]
  color?: string
  height?: number
  filled?: boolean
}

export function Sparkline({
  data,
  color = '#22d3ee',
  height = 38,
  filled = true,
}: SparklineProps) {
  const gradientId = useId().replace(/:/g, '')
  if (data.length < 2) {
    return <div className="sparkline-empty" style={{ height }} />
  }

  const min = Math.min(...data)
  const max = Math.max(...data)
  const span = max - min || 1
  const points = data.map((value, index) => {
    const x = (index / (data.length - 1)) * 100
    const y = 30 - ((value - min) / span) * 26
    return `${x.toFixed(2)},${y.toFixed(2)}`
  })
  const line = `M${points.join(' L')}`
  const area = `${line} L100,32 L0,32 Z`

  return (
    <svg
      className="sparkline"
      viewBox="0 0 100 32"
      preserveAspectRatio="none"
      style={{ height }}
      aria-hidden="true"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.34" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      {filled && <path d={area} fill={`url(#${gradientId})`} />}
      <path
        d={line}
        fill="none"
        stroke={color}
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
