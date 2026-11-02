import type { ReactNode } from 'react'

interface PanelHeaderProps {
  kicker: string
  title: string
  subtitle?: string
  actions?: ReactNode
}

export function PanelHeader({ kicker, title, subtitle, actions }: PanelHeaderProps) {
  return (
    <div className="panel-header">
      <div>
        <div className="panel-kicker">{kicker}</div>
        <h2 className="panel-title">{title}</h2>
        {subtitle && <p className="panel-subtitle">{subtitle}</p>}
      </div>
      {actions && <div className="panel-actions">{actions}</div>}
    </div>
  )
}
