import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Eye, EyeOff, KeyRound, RotateCcw, Save, Server, ShieldCheck, X } from 'lucide-react'

import type { ApiConfig } from '../types'

interface SettingsDrawerProps {
  open: boolean
  onClose: () => void
  config: ApiConfig
  onSave: (config: ApiConfig) => void
  onReset: () => void
}

export function SettingsDrawer({ open, onClose, config, onSave, onReset }: SettingsDrawerProps) {
  const [baseUrl, setBaseUrl] = useState(config.baseUrl)
  const [token, setToken] = useState(config.token)
  const [showToken, setShowToken] = useState(false)

  useEffect(() => {
    if (!open) {
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose()
      }
    }
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onClose])

  if (!open) {
    return null
  }

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    onSave({ baseUrl, token })
    onClose()
  }

  const handleReset = () => {
    onReset()
    onClose()
  }

  const preview = `${baseUrl.replace(/\/+$/, '') || window.location.origin}/api/v1`

  return (
    <div className="settings-overlay" role="presentation" onClick={onClose}>
      <aside
        className="settings-panel"
        role="dialog"
        aria-modal="true"
        aria-label="API 设置"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="settings-head">
          <div>
            <div className="panel-kicker">CONNECTION</div>
            <h2>API 设置</h2>
            <p>配置查询接口地址与 Bearer Token。</p>
          </div>
          <button type="button" className="icon-button" onClick={onClose} aria-label="关闭设置">
            <X size={18} />
          </button>
        </div>

        <form className="settings-form" onSubmit={handleSubmit}>
          <label className="field">
            <span className="field-label">
              <Server size={14} /> API Base URL
            </span>
            <input
              type="url"
              value={baseUrl}
              placeholder="留空使用同源 /api（Vite 代理或反向代理）"
              onChange={(event) => setBaseUrl(event.target.value)}
              spellCheck={false}
            />
            <span className="field-help">
              留空时请求 <code>/api/v1/...</code>；也可以填写 <code>https://host:11294</code>。
            </span>
          </label>

          <label className="field">
            <span className="field-label">
              <KeyRound size={14} /> API Token
            </span>
            <span className="token-input">
              <input
                type={showToken ? 'text' : 'password'}
                value={token}
                placeholder="与后端 API_TOKEN 一致"
                onChange={(event) => setToken(event.target.value)}
                spellCheck={false}
                autoComplete="off"
              />
              <button
                type="button"
                className="token-toggle"
                onClick={() => setShowToken((visible) => !visible)}
                aria-label={showToken ? '隐藏 Token' : '显示 Token'}
              >
                {showToken ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </span>
            <span className="field-help">Token 只保存在当前浏览器的 localStorage。</span>
          </label>

          <div className="settings-preview">
            <span className="preview-label">请求目标</span>
            <code>{preview}</code>
          </div>

          <div className="settings-note">
            <ShieldCheck size={15} />
            <span>
              演示环境可直接使用 <code>.env.local</code> 中的 Token；生产环境请使用独立 Token，并通过 HTTPS
              访问。
            </span>
          </div>

          <div className="settings-actions">
            <button type="button" className="btn btn-ghost" onClick={handleReset}>
              <RotateCcw size={15} /> 恢复默认
            </button>
            <button type="submit" className="btn btn-primary">
              <Save size={15} /> 保存并刷新
            </button>
          </div>
        </form>
      </aside>
    </div>
  )
}
