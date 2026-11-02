import { useState } from 'react'
import { CheckCircle2, Clock3, MessageSquare, Send } from 'lucide-react'

import type { Message } from '../types'
import { formatDateTime, formatRelativeTime } from '../utils'
import { PanelHeader } from './PanelHeader'

interface MessageBoardProps {
  messages: Message[]
  error: string | null
  sending: boolean
  onSend: (
    author: 'parent' | 'child',
    authorName: string,
    content: string,
  ) => Promise<boolean>
}

const ROLES: Array<{ key: 'parent' | 'child'; name: string; label: string }> = [
  { key: 'parent', name: '妈妈', label: '家长' },
  { key: 'child', name: '小宇', label: '孩子' },
]

export function MessageBoard({ messages, error, sending, onSend }: MessageBoardProps) {
  const [author, setAuthor] = useState<'parent' | 'child'>('parent')
  const [content, setContent] = useState('')

  const role = ROLES.find((item) => item.key === author) ?? ROLES[0]

  const submit = async () => {
    const trimmed = content.trim()
    if (!trimmed || sending) {
      return
    }
    const ok = await onSend(author, role.name, trimmed)
    if (ok) {
      setContent('')
    }
  }

  return (
    <section className="panel message-panel" id="messages">
      <PanelHeader
        kicker="FAMILY / MESSAGES"
        title="家庭留言"
        subtitle="家长和孩子的话会出现在 ESP 屏幕上 · 已送达会标记"
        actions={<span className="micro-badge">{messages.length} MESSAGES</span>}
      />

      <div className="message-body">
        <div className="message-list">
          {messages.length === 0 && (
            <div className="message-empty">
              <MessageSquare size={22} />
              <strong>还没有家庭留言</strong>
              <span>写一句鼓励的话，ESP 会在几秒内显示。</span>
            </div>
          )}
          {messages.slice(0, 8).map((message) => {
            const isParent = message.author === 'parent'
            return (
              <article key={message.id} className={`message-row ${isParent ? 'is-parent' : 'is-child'}`}>
                <span className="message-avatar">{isParent ? '妈' : '宇'}</span>
                <div className="message-main">
                  <div className="message-head">
                    <span className="message-author">{message.author_name}</span>
                    <span className="message-role-tag">{isParent ? '家长' : '孩子'}</span>
                    <span className="message-time" title={formatDateTime(message.created_at)}>
                      {formatRelativeTime(message.created_at)}
                    </span>
                  </div>
                  <p className="message-content">{message.content}</p>
                  <div className="message-meta">
                    {message.delivered_at ? (
                      <>
                        <CheckCircle2 size={11} /> 已送达 ESP 屏幕
                      </>
                    ) : (
                      <>
                        <Clock3 size={11} /> 等待 ESP 拉取
                      </>
                    )}
                  </div>
                </div>
              </article>
            )
          })}
        </div>

        <form
          className="message-composer"
          onSubmit={(event) => {
            event.preventDefault()
            void submit()
          }}
        >
          <div className="message-role-group" role="group" aria-label="留言身份">
            {ROLES.map((item) => (
              <button
                key={item.key}
                type="button"
                className={author === item.key ? 'is-active' : ''}
                onClick={() => setAuthor(item.key)}
              >
                {item.label} · {item.name}
              </button>
            ))}
          </div>
          <div className="message-input-row">
            <input
              className="message-input"
              type="text"
              value={content}
              maxLength={500}
              placeholder={`以${role.label}身份写一句留言…`}
              onChange={(event) => setContent(event.target.value)}
              aria-label="留言内容"
            />
            <button
              type="submit"
              className="btn btn-primary message-send"
              disabled={sending || !content.trim()}
            >
              <Send size={14} /> {sending ? '发送中…' : '发送'}
            </button>
          </div>
          {error && <div className="message-error">{error}</div>}
          <div className="message-hint">
            <span>ESP 每 10 秒左右拉取一次；留言会出现在表情屏底部。</span>
            <span>{content.length}/500</span>
          </div>
        </form>
      </div>
    </section>
  )
}
