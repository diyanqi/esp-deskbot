import { BellRing, ShieldCheck, Sparkles, Sun, Timer } from 'lucide-react'

import type { AIReport, Insights } from '../types'
import { formatNumber } from '../utils'
import { PanelHeader } from './PanelHeader'

interface GrowthOverviewProps {
  insights: Insights | null
  insightsError: string | null
  aiReport: AIReport | null
  aiLoading: boolean
  onGenerateReport: () => void
}

const LEVEL_LABEL: Record<string, string> = {
  excellent: '优秀',
  good: '良好',
  attention: '需关注',
  risk: '需干预',
}

function scoreColor(score: number): string {
  if (score >= 85) {
    return '#34d399'
  }
  if (score >= 70) {
    return '#22d3ee'
  }
  if (score >= 55) {
    return '#fbbf24'
  }
  return '#fb7185'
}

export function GrowthOverview({
  insights,
  insightsError,
  aiReport,
  aiLoading,
  onGenerateReport,
}: GrowthOverviewProps) {
  const summary = insights?.summary
  const score = aiReport?.score ?? insights?.health_score ?? 0
  const hasScore = Boolean(aiReport || insights?.has_data)
  const level = aiReport?.level ?? (score >= 85 ? 'excellent' : score >= 70 ? 'good' : score >= 55 ? 'attention' : 'risk')
  const color = scoreColor(score)
  const circumference = 2 * Math.PI * 52
  const offset = circumference * (1 - Math.min(100, Math.max(0, score)) / 100)

  const stats = [
    {
      icon: <ShieldCheck size={15} />,
      label: '今日良好率',
      value: summary ? `${formatNumber(summary.good_rate * 100, 0)}%` : '—',
      hint: summary ? `${summary.good} 个良好样本` : '等待统计',
      tone: 'emerald',
    },
    {
      icon: <BellRing size={15} />,
      label: '今日提醒',
      value: summary ? `${formatNumber(summary.alert_events, 0)} 次` : '—',
      hint: summary ? `累计 ${summary.alert_minutes} 分钟` : '等待统计',
      tone: 'rose',
    },
    {
      icon: <Timer size={15} />,
      label: '最长连续告警',
      value: summary ? `${formatNumber(summary.longest_bad_streak_minutes, 0)} 分钟` : '—',
      hint: '连续不良坐姿',
      tone: 'amber',
    },
    {
      icon: <Sun size={15} />,
      label: '平均桌面光照',
      value: summary ? `${formatNumber(summary.avg_light, 0)}%` : '—',
      hint: summary ? `舒适度 ${formatNumber(summary.light_comfort * 100, 0)}%` : '等待统计',
      tone: 'cyan',
    },
  ]

  return (
    <section className="panel growth-panel">
      <PanelHeader
        kicker="GROWTH / SCORE"
        title="成长守护评分"
        subtitle="最近 24 小时 · 坐姿 + 环境综合评估"
        actions={<span className="micro-badge">PARENT VIEW</span>}
      />

      <div className="child-card">
        <div className="child-avatar" aria-hidden="true">
          宇
        </div>
        <div className="child-copy">
          <strong>小宇</strong>
          <span>9 岁 · 三年级 · 演示档案</span>
        </div>
        <span className="child-goal">今日目标 4h</span>
      </div>

      <div className="score-row">
        <div className="score-ring-wrap">
          <svg className="score-ring" viewBox="0 0 120 120" role="img" aria-label={`成长评分 ${score}`}>
            <circle className="score-ring-track" cx="60" cy="60" r="52" />
            <circle
              className="score-ring-value"
              cx="60"
              cy="60"
              r="52"
              style={{
                stroke: color,
                strokeDasharray: circumference,
                strokeDashoffset: offset,
                filter: `drop-shadow(0 0 8px ${color}88)`,
              }}
            />
          </svg>
          <div className="score-ring-center">
            <strong style={{ color }}>{hasScore ? score : '—'}</strong>
            <span>成长评分</span>
          </div>
        </div>
        <div className="score-copy">
          <span className="score-level" style={{ color }}>
            {hasScore ? LEVEL_LABEL[level] ?? '成长中' : '等待数据'}
          </span>
          <h3>{aiReport?.headline ?? (hasScore ? '今天也在认真守护成长' : '还没有足够的今日数据')}</h3>
          <p>
            {aiReport?.summary ??
              (hasScore
                ? '评分综合坐姿良好率、提醒频率、光照舒适度与在座时长，帮助家长快速了解孩子今天的状态。'
                : '孩子坐到书桌前，设备上报几分钟后就会生成成长评分。')}
          </p>
          <button
            type="button"
            className="btn btn-primary growth-ai-btn"
            onClick={onGenerateReport}
            disabled={aiLoading}
          >
            <Sparkles size={15} className={aiLoading ? 'spin' : ''} />
            {aiLoading ? 'AI 正在分析…' : aiReport ? '重新生成 AI 报告' : '生成 AI 健康报告'}
          </button>
          {insightsError && <span className="growth-error">{insightsError}</span>}
        </div>
      </div>

      <div className="growth-stats">
        {stats.map((stat) => (
          <div key={stat.label} className={`growth-stat tone-${stat.tone}`}>
            <span className="growth-stat-icon">{stat.icon}</span>
            <span className="growth-stat-label">{stat.label}</span>
            <strong>{stat.value}</strong>
            <span className="growth-stat-hint">{stat.hint}</span>
          </div>
        ))}
      </div>

      <div className="growth-foot">
        评分 = 坐姿良好率 60% + 提醒频率 20% + 光照舒适度 10% + 在座时长 10%
      </div>
    </section>
  )
}
