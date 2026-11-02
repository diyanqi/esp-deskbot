import { Radar } from 'lucide-react'

export function ChartEmpty({ label }: { label: string }) {
  return (
    <div className="chart-empty">
      <span className="chart-empty-radar">
        <Radar size={22} />
      </span>
      <span>{label}</span>
    </div>
  )
}
