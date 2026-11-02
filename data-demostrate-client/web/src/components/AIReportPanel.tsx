import type { CSSProperties } from 'react'
import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  Clock3,
  Heart,
  Lightbulb,
  Printer,
  RefreshCw,
  ShieldAlert,
  Sparkles,
} from 'lucide-react'

import type { AIFinding, AIReport } from '../types'
import { formatDateTime } from '../utils'
import { PanelHeader } from './PanelHeader'

interface AIReportPanelProps {
  report: AIReport | null
  loading: boolean
  error: string | null
  onGenerate: () => void
}

const LEVEL_LABEL: Record<string, string> = {
  excellent: '优秀',
  good: '良好',
  attention: '需关注',
  risk: '需干预',
}

function severityMeta(severity: string): { color: string; icon: typeof CheckCircle2 } {
  if (severity === 'alert') {
    return { color: '#fb7185', icon: ShieldAlert }
  }
  if (severity === 'warn') {
    return { color: '#fbbf24', icon: AlertTriangle }
  }
  return { color: '#34d399', icon: CheckCircle2 }
}

function FindingRow({ finding }: { finding: AIFinding }) {
  const meta = severityMeta(finding.severity)
  const Icon = meta.icon
  return (
    <li className="ai-finding" style={{ '--finding-color': meta.color } as CSSProperties}>
      <span className="ai-finding-icon">
        <Icon size={15} />
      </span>
      <div>
        <strong>{finding.title}</strong>
        <p>{finding.detail}</p>
      </div>
    </li>
  )
}

export function AIReportPanel({ report, loading, error, onGenerate }: AIReportPanelProps) {
  const sourceLabel =
    report?.source === 'ai' ? `AI · ${report.model || 'gpt-4o'}` : '本地规则分析'

  return (
    <section className="panel ai-report-panel report-print" id="ai-report">
      <PanelHeader
        kicker="AI / GROWTH ADVISOR"
        title="AI 健康报告"
        subtitle="基于最近 24 小时坐姿与环境数据 · 给家长的可执行建议"
        actions={
          report ? (
            <span className={`ai-source ${report.source === 'ai' ? 'is-ai' : 'is-local'}`}>
              <Bot size={13} /> {sourceLabel}
            </span>
          ) : (
            <span className="micro-badge">GPT-4O</span>
          )
        }
      />

      {loading ? (
        <div className="ai-loading">
          <div className="ai-loading-head">
            <span className="ai-orbit">
              <Sparkles size={22} />
            </span>
            <div>
              <strong>AI 正在阅读孩子的坐姿数据</strong>
              <span>分析良好率、告警时段、光照舒适度与连续坐姿…</span>
            </div>
          </div>
          <div className="ai-loading-bars" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
        </div>
      ) : !report ? (
        <div className="ai-empty">
          <span className="ai-empty-icon">
            <Sparkles size={26} />
          </span>
          <h3>生成一份给家长的坐姿健康报告</h3>
          <p>
            AI 会结合最近 24 小时的坐姿良好率、提醒次数、风险时段与光照数据，给出温和、可执行的建议。
          </p>
          {error && (
            <div className="ai-error">
              <AlertTriangle size={14} />
              <span>{error}</span>
            </div>
          )}
          <button type="button" className="btn btn-primary" onClick={onGenerate}>
            <Sparkles size={15} /> 生成 AI 报告
          </button>
        </div>
      ) : (
        <>
          <div className={`ai-hero level-${report.level}`}>
            <div className="ai-score">
              <strong>{report.score}</strong>
              <span>综合评分</span>
            </div>
            <div className="ai-hero-copy">
              <div className="ai-level-row">
                <span className="ai-level">{LEVEL_LABEL[report.level] ?? report.level}</span>
                <span className="ai-hero-sep">/</span>
                <span>{report.hours}H WINDOW</span>
              </div>
              <h3>{report.headline}</h3>
              <p>{report.summary}</p>
            </div>
          </div>

          {error && (
            <div className="ai-error ai-error-inline">
              <AlertTriangle size={14} />
              <span>{error}，下面展示的是上一次报告。</span>
            </div>
          )}

          {report.findings.length > 0 && (
            <div className="ai-section">
              <div className="ai-section-head">
                <span>
                  <Bot size={14} /> 关键发现
                </span>
                <span>{report.findings.length} 条</span>
              </div>
              <ul className="ai-findings">
                {report.findings.map((finding, index) => (
                  <FindingRow key={`${finding.title}-${index}`} finding={finding} />
                ))}
              </ul>
            </div>
          )}

          {report.risk_windows.length > 0 && (
            <div className="ai-section">
              <div className="ai-section-head">
                <span>
                  <Clock3 size={14} /> 高风险时段
                </span>
                <span>建议重点关注</span>
              </div>
              <div className="ai-risk-windows">
                {report.risk_windows.map((window) => (
                  <span key={window} className="ai-risk-chip">
                    {window}
                  </span>
                ))}
              </div>
            </div>
          )}

          {report.suggestions.length > 0 && (
            <div className="ai-section">
              <div className="ai-section-head">
                <span>
                  <Lightbulb size={14} /> 给家长的建议
                </span>
                <span>可执行</span>
              </div>
              <ol className="ai-suggestions">
                {report.suggestions.map((suggestion, index) => (
                  <li key={`${suggestion}-${index}`}>
                    <span className="ai-suggestion-index">{index + 1}</span>
                    <span>{suggestion}</span>
                  </li>
                ))}
              </ol>
            </div>
          )}

          {report.encouragement && (
            <blockquote className="ai-encouragement">
              <Heart size={16} />
              <span>{report.encouragement}</span>
            </blockquote>
          )}

          <div className="ai-foot">
            <span className="ai-disclaimer">{report.disclaimer}</span>
            <div className="ai-foot-meta">
              <span>
                {sourceLabel} · {formatDateTime(report.generated_at)}
              </span>
              <div className="ai-foot-actions">
                <button type="button" className="btn btn-ghost btn-small" onClick={onGenerate}>
                  <RefreshCw size={13} /> 重新分析
                </button>
                <button
                  type="button"
                  className="btn btn-ghost btn-small"
                  onClick={() => window.print()}
                >
                  <Printer size={13} /> 打印报告
                </button>
              </div>
            </div>
          </div>
        </>
      )}
    </section>
  )
}
