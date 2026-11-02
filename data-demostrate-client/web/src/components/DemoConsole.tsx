import type { CSSProperties } from 'react'
import { AlertTriangle, CheckCircle2, FlaskConical, Play, ShieldAlert, UserRound } from 'lucide-react'

import type { DemoStatus } from '../types'
import { PanelHeader } from './PanelHeader'

interface DemoConsoleProps {
  busy: boolean
  message: string | null
  autoDemo: boolean
  onToggleAuto: (enabled: boolean) => void
  onRun: (status: DemoStatus) => void
}

const SCENARIOS: Array<{
  status: DemoStatus
  label: string
  hint: string
  icon: typeof CheckCircle2
  color: string
}> = [
  {
    status: 'good',
    label: '良好坐姿',
    hint: '绿色 · 无提醒',
    icon: CheckCircle2,
    color: '#34d399',
  },
  {
    status: 'warning',
    label: '轻微偏移',
    hint: '黄色 · 一次提醒',
    icon: AlertTriangle,
    color: '#fbbf24',
  },
  {
    status: 'bad',
    label: '坐姿告警',
    hint: '红色 · 强提醒',
    icon: ShieldAlert,
    color: '#fb7185',
  },
  {
    status: 'away',
    label: '离开书桌',
    hint: '灰色 · 无人',
    icon: UserRound,
    color: '#94a3b8',
  },
]

export function DemoConsole({
  busy,
  message,
  autoDemo,
  onToggleAuto,
  onRun,
}: DemoConsoleProps) {
  return (
    <section className="panel demo-panel" id="demo">
      <PanelHeader
        kicker="DEMO / SCENARIO"
        title="演示控制台"
        subtitle="通过真实 IoTDA 事件路径写入 Postgres · 页面轮询后更新"
        actions={<span className="micro-badge">DEMO ENABLED</span>}
      />

      <div className="demo-intro">
        <span className="demo-intro-icon">
          <FlaskConical size={17} />
        </span>
        <p>
          点击场景按钮，后端会生成一条带签名的设备属性事件并写入数据库。前端不是假数据覆盖，而是走完整的
          <code> ingest → Postgres → API </code>链路。
        </p>
      </div>

      <div className="demo-scenarios">
        {SCENARIOS.map((scenario) => {
          const Icon = scenario.icon
          return (
            <button
              key={scenario.status}
              type="button"
              className="demo-scenario"
              style={{ '--demo-color': scenario.color } as CSSProperties}
              disabled={busy}
              onClick={() => onRun(scenario.status)}
            >
              <span className="demo-scenario-icon">
                <Icon size={18} />
              </span>
              <span className="demo-scenario-copy">
                <strong>{scenario.label}</strong>
                <span>{scenario.hint}</span>
              </span>
              <Play size={13} className="demo-scenario-play" />
            </button>
          )
        })}
      </div>

      <label className={`demo-auto ${autoDemo ? 'is-on' : ''}`}>
        <span className="demo-auto-switch" aria-hidden="true">
          <span />
        </span>
        <input
          type="checkbox"
          checked={autoDemo}
          onChange={(event) => onToggleAuto(event.target.checked)}
        />
        <span>
          <strong>自动演示</strong>
          <span>每 5 秒循环注入 良好 → 偏移 → 告警 → 离开</span>
        </span>
      </label>

      {message && <div className={`demo-message ${message.includes('失败') || message.includes('disabled') ? 'is-error' : ''}`}>{message}</div>}

      <div className="demo-foot">
        <span>需要后端 `DEMO_ENABLED=true`</span>
        <span>演示数据会保留在数据库中</span>
      </div>
    </section>
  )
}
