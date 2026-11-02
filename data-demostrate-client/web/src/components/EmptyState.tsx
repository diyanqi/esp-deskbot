import { Radar, Settings2 } from 'lucide-react'

interface EmptyStateProps {
  title: string
  description: string
  hint?: string
  actionLabel?: string
  onAction?: () => void
}

export function EmptyState({ title, description, hint, actionLabel, onAction }: EmptyStateProps) {
  return (
    <section className="empty-state">
      <div className="empty-radar" aria-hidden="true">
        <span className="empty-ring ring-1" />
        <span className="empty-ring ring-2" />
        <span className="empty-ring ring-3" />
        <span className="empty-sweep" />
        <Radar size={30} />
      </div>
      <h2>{title}</h2>
      <p>{description}</p>
      {hint && <code className="empty-hint">{hint}</code>}
      {actionLabel && onAction && (
        <button type="button" className="btn btn-primary" onClick={onAction}>
          <Settings2 size={16} /> {actionLabel}
        </button>
      )}
    </section>
  )
}
