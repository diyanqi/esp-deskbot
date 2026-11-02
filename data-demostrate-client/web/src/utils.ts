import type {
  ChartPoint,
  Device,
  EventRow,
  PosturePoint,
  RangeKey,
  SeriesPoint,
  StatusKind,
  TelemetryRow,
} from './types'

export const RANGE_OPTIONS: ReadonlyArray<{
  key: RangeKey
  label: string
  hours: number
  bucket: 'minute' | 'hour'
}> = [
  { key: '1h', label: '1H', hours: 1, bucket: 'minute' },
  { key: '6h', label: '6H', hours: 6, bucket: 'minute' },
  { key: '24h', label: '24H', hours: 24, bucket: 'hour' },
  { key: '7d', label: '7D', hours: 168, bucket: 'hour' },
]

export function rangeParams(range: RangeKey): {
  from: string
  to: string
  bucket: 'minute' | 'hour'
  hours: number
} {
  const meta = RANGE_OPTIONS.find((option) => option.key === range) ?? RANGE_OPTIONS[2]
  const to = new Date()
  const from = new Date(to.getTime() - meta.hours * 60 * 60 * 1000)
  return {
    from: from.toISOString(),
    to: to.toISOString(),
    bucket: meta.bucket,
    hours: meta.hours,
  }
}

export function rangeLabel(range: RangeKey): string {
  return RANGE_OPTIONS.find((option) => option.key === range)?.label ?? '24H'
}

const relativeFormatter = new Intl.RelativeTimeFormat('zh-CN', { numeric: 'auto' })
const clockFormatter = new Intl.DateTimeFormat('zh-CN', {
  hour: '2-digit',
  minute: '2-digit',
})
const shortDateFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})
const dateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
})

export function formatRelativeTime(iso: string | null | undefined): string {
  if (!iso) {
    return '—'
  }
  const timestamp = new Date(iso).getTime()
  if (!Number.isFinite(timestamp)) {
    return '—'
  }
  const diffSeconds = (timestamp - Date.now()) / 1000
  const absolute = Math.abs(diffSeconds)
  if (absolute < 60) {
    return relativeFormatter.format(Math.round(diffSeconds), 'second')
  }
  if (absolute < 3600) {
    return relativeFormatter.format(Math.round(diffSeconds / 60), 'minute')
  }
  if (absolute < 86400) {
    return relativeFormatter.format(Math.round(diffSeconds / 3600), 'hour')
  }
  if (absolute < 86400 * 30) {
    return relativeFormatter.format(Math.round(diffSeconds / 86400), 'day')
  }
  return dateTimeFormatter.format(timestamp)
}

export function formatClock(iso: string): string {
  const timestamp = new Date(iso).getTime()
  return Number.isFinite(timestamp) ? clockFormatter.format(timestamp) : '—'
}

export function formatShortDate(iso: string): string {
  const timestamp = new Date(iso).getTime()
  return Number.isFinite(timestamp) ? shortDateFormatter.format(timestamp) : '—'
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) {
    return '—'
  }
  const timestamp = new Date(iso).getTime()
  return Number.isFinite(timestamp) ? dateTimeFormatter.format(timestamp) : '—'
}

export function formatNumber(value: number, maximumFractionDigits = 0): string {
  if (!Number.isFinite(value)) {
    return '—'
  }
  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits,
    minimumFractionDigits: maximumFractionDigits,
  }).format(value)
}

export function formatCompact(value: number): string {
  if (!Number.isFinite(value)) {
    return '—'
  }
  if (Math.abs(value) >= 1_000_000) {
    return `${formatNumber(value / 1_000_000, 1)}M`
  }
  if (Math.abs(value) >= 1_000) {
    return `${formatNumber(value / 1_000, 1)}K`
  }
  return formatNumber(value, 0)
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes)) {
    return '—'
  }
  if (bytes >= 1024 * 1024) {
    return `${formatNumber(bytes / (1024 * 1024), 1)} MB`
  }
  if (bytes >= 1024) {
    return `${formatNumber(bytes / 1024, 0)} KB`
  }
  return `${formatNumber(bytes, 0)} B`
}

export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return '—'
  }
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) {
    return `${days}d ${hours}h`
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  return `${minutes}m`
}

export function asNumber(value: unknown, fallback = 0): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return fallback
}

export function asString(value: unknown, fallback = ''): string {
  if (typeof value === 'string') {
    return value
  }
  if (value === null || value === undefined) {
    return fallback
  }
  return String(value)
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

export function normalizeStatus(value: unknown): StatusKind {
  const text = asString(value).toLowerCase()
  if (text === 'good') {
    return 'good'
  }
  if (text === 'warning') {
    return 'warning'
  }
  if (text === 'bad') {
    return 'bad'
  }
  return 'unknown'
}

export const STATUS_META: Record<
  StatusKind,
  { label: string; zh: string; color: string; soft: string; className: string }
> = {
  good: {
    label: 'GOOD',
    zh: '坐姿良好',
    color: '#34d399',
    soft: 'rgba(52, 211, 153, 0.16)',
    className: 'is-good',
  },
  warning: {
    label: 'WARNING',
    zh: '坐姿偏移',
    color: '#fbbf24',
    soft: 'rgba(251, 191, 36, 0.16)',
    className: 'is-warning',
  },
  bad: {
    label: 'ALERT',
    zh: '坐姿异常',
    color: '#fb7185',
    soft: 'rgba(251, 113, 133, 0.16)',
    className: 'is-bad',
  },
  unknown: {
    label: 'AWAY',
    zh: '暂时离开',
    color: '#94a3b8',
    soft: 'rgba(148, 163, 184, 0.14)',
    className: 'is-unknown',
  },
}

export function latestValue(
  latest: Map<string, TelemetryRow>,
  service: string,
  property: string,
): TelemetryRow | undefined {
  return latest.get(`${service}.${property}`)
}

export function latestNumber(
  latest: Map<string, TelemetryRow>,
  service: string,
  property: string,
  fallback = 0,
): number {
  return asNumber(latestValue(latest, service, property)?.value, fallback)
}

export function latestString(
  latest: Map<string, TelemetryRow>,
  service: string,
  property: string,
  fallback = '',
): string {
  return asString(latestValue(latest, service, property)?.value, fallback)
}

export function seriesToPoints(points: SeriesPoint[]): ChartPoint[] {
  return points
    .filter((point) => Number.isFinite(point.avg ?? NaN))
    .map((point) => ({
      time: point.bucket,
      value: point.avg ?? 0,
      min: point.min,
      max: point.max,
    }))
}

export function postureTelemetryToPoints(rows: TelemetryRow[], limit = 90): PosturePoint[] {
  return rows
    .slice(0, limit)
    .reverse()
    .map((row) => ({
      time: row.event_time,
      status: normalizeStatus(row.value),
    }))
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function serviceProperties(event: EventRow, serviceId: string): Record<string, unknown> | null {
  const body = asRecord(event.body)
  if (!body || !Array.isArray(body.services)) {
    return null
  }
  for (const entry of body.services) {
    const service = asRecord(entry)
    if (!service || service.service_id !== serviceId) {
      continue
    }
    return asRecord(service.properties)
  }
  return null
}

export function eventStatus(event: EventRow): StatusKind | null {
  const posture = serviceProperties(event, 'Posture')
  if (posture && typeof posture.postureStatus === 'string') {
    return normalizeStatus(posture.postureStatus)
  }
  if (event.resource === 'device.status') {
    const body = asRecord(event.body)
    const status = asString(body?.status).toUpperCase()
    if (status === 'ONLINE') {
      return 'good'
    }
    if (status === 'ABNORMAL') {
      return 'bad'
    }
    if (status === 'FROZEN') {
      return 'warning'
    }
    return 'unknown'
  }
  return null
}

export function eventMeta(event: EventRow): { title: string; detail: string } {
  const posture = serviceProperties(event, 'Posture')
  if (posture) {
    const summary = asString(posture.postureSummary)
    const status = normalizeStatus(posture.postureStatus)
    const detailParts: string[] = [STATUS_META[status].label]
    if (typeof posture.alert === 'number') {
      detailParts.push(`alert ${posture.alert}`)
    }
    if (typeof posture.lastSeq === 'number') {
      detailParts.push(`seq ${posture.lastSeq}`)
    }
    return {
      title: summary || STATUS_META[status].zh,
      detail: detailParts.join(' · '),
    }
  }

  const environment = serviceProperties(event, 'Environment')
  if (environment) {
    const parts: string[] = []
    if (typeof environment.light === 'number') {
      parts.push(`light ${environment.light}%`)
    }
    if (typeof environment.sound === 'number') {
      parts.push(environment.sound === 1 ? 'sound detected' : 'quiet')
    }
    return { title: '环境属性上报', detail: parts.join(' · ') || 'environment' }
  }

  if (event.resource === 'device.status') {
    const body = asRecord(event.body)
    const status = asString(body?.status, 'UNKNOWN')
    return { title: `设备状态 · ${status}`, detail: 'device lifecycle' }
  }

  return {
    title: event.resource,
    detail: event.event ? `event ${event.event}` : 'raw notification',
  }
}

export function deviceTag(device: Device | undefined, key: string): string | null {
  if (!device?.tags) {
    return null
  }
  const tag = device.tags.find((entry) => entry.tag_key === key)
  return tag?.tag_value ?? null
}

export function eventColor(event: EventRow): string {
  const status = eventStatus(event)
  if (status) {
    return STATUS_META[status].color
  }
  if (event.resource === 'device.message') {
    return '#a78bfa'
  }
  if (event.resource === 'device.status') {
    return '#38bdf8'
  }
  return '#64748b'
}
