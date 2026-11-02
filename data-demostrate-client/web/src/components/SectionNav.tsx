import {
  FileHeart,
  FlaskConical,
  History,
  LayoutDashboard,
  MessageSquare,
  Router,
} from 'lucide-react'

const ITEMS = [
  { id: 'overview', label: '总览', icon: LayoutDashboard },
  { id: 'ai-report', label: 'AI 健康报告', icon: FileHeart },
  { id: 'messages', label: '家庭留言', icon: MessageSquare },
  { id: 'replay', label: '时间回放', icon: History },
  { id: 'demo', label: '演示控制台', icon: FlaskConical },
  { id: 'devices', label: '设备', icon: Router },
]

export function SectionNav() {
  return (
    <nav className="section-nav" aria-label="页面区块">
      <span className="section-nav-label">成长档案</span>
      {ITEMS.map((item) => {
        const Icon = item.icon
        return (
          <button
            key={item.id}
            type="button"
            onClick={() => {
              document.getElementById(item.id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
            }}
          >
            <Icon size={13} /> {item.label}
          </button>
        )
      })}
      <span className="section-nav-spacer" />
      <span className="section-nav-note">ESP32-S3 · 华为云 IoTDA · GPT-4o</span>
    </nav>
  )
}
