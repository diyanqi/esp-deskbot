import { ArrowUpRight, Clock3, Radio } from 'lucide-react'

import type { EventRow } from '../types'
import { eventColor, eventMeta, formatClock, formatRelativeTime } from '../utils'
import { ChartEmpty } from './ChartEmpty'
import { PanelHeader } from './PanelHeader'

interface EventStreamProps {
  events: EventRow[]
}

export function EventStream({ events }: EventStreamProps) {
  return (
    <section className="panel event-panel">
      <PanelHeader
        kicker="IOTDA / EVENT STREAM"
        title="守护事件流"
        subtitle="孩子的设备最近上报记录"
        actions={<span className="micro-badge">{events.length} EVENTS</span>}
      />

      {events.length === 0 ? (
        <ChartEmpty label="等待 IoTDA 推送事件" />
      ) : (
        <div className="event-list">
          {events.map((event) => {
            const color = eventColor(event)
            const { title, detail } = eventMeta(event)
            const resource = event.resource.replace('device.', '')
            return (
              <article key={event.id} className="event-row">
                <span
                  className="event-dot"
                  style={{ backgroundColor: color, boxShadow: `0 0 14px ${color}` }}
                />
                <div className="event-main">
                  <div className="event-title-row">
                    <span className="event-title">{title}</span>
                    <span className="event-resource">{resource}</span>
                  </div>
                  <div className="event-detail">{detail}</div>
                  <div className="event-meta">
                    <Clock3 size={12} />
                    <span>{formatClock(event.event_time ?? event.received_at)}</span>
                    <span className="event-meta-sep">/</span>
                    <span>{formatRelativeTime(event.event_time ?? event.received_at)}</span>
                    {event.request_id && (
                      <>
                        <span className="event-meta-sep">/</span>
                        <span className="event-request">{event.request_id}</span>
                      </>
                    )}
                  </div>
                </div>
                <ArrowUpRight size={15} className="event-arrow" />
              </article>
            )
          })}
        </div>
      )}

      <div className="event-foot">
        <Radio size={13} />
        <span>数据来源：`/api/v1/events` · 每 5 秒轮询</span>
      </div>
    </section>
  )
}
