export interface Tag {
  tag_key: string
  tag_value?: string
}

export interface Device {
  device_id: string
  node_id?: string | null
  product_id?: string | null
  app_id?: string | null
  gateway_id?: string | null
  status?: string | null
  status_updated_at?: string | null
  last_online_at?: string | null
  first_seen_at: string
  last_seen_at: string
  tags?: Tag[] | null
  created_at: string
  updated_at: string
}

export interface TelemetryRow {
  id: number
  device_id: string
  service_id: string
  property: string
  value: unknown
  value_type: string
  value_num?: number | null
  event_time: string
  received_at: string
  request_id?: string | null
}

export interface EventRow {
  id: number
  resource: string
  event?: string | null
  request_id?: string | null
  device_id?: string | null
  node_id?: string | null
  product_id?: string | null
  app_id?: string | null
  gateway_id?: string | null
  event_time?: string | null
  event_time_ms?: string | null
  tags?: Tag[] | null
  body: unknown
  raw_payload?: unknown
  received_at: string
}

export interface SeriesPoint {
  bucket: string
  count: number
  avg?: number
  min?: number
  max?: number
}

export interface Pagination {
  limit: number
  offset: number
  has_more: boolean
  total?: number
}

export interface ListResponse<T> {
  data: T[]
  pagination: Pagination
}

export interface LatestResponse {
  device_id: string
  data: TelemetryRow[]
}

export interface SeriesResponse {
  device_id: string
  service_id: string
  property: string
  bucket: string
  from: string
  to: string
  data: SeriesPoint[]
}

export type StatusKind = 'good' | 'warning' | 'bad' | 'unknown'

export type RangeKey = '1h' | '6h' | '24h' | '7d'

export interface ApiConfig {
  baseUrl: string
  token: string
}

export interface ChartPoint {
  time: string
  value: number
  min?: number
  max?: number
}

export interface PosturePoint {
  time: string
  status: StatusKind
}

export interface InsightsSummary {
  posture_samples: number
  good: number
  warning: number
  bad: number
  unknown: number
  good_rate: number
  alert_rate: number
  person_rate: number
  alert_minutes: number
  alert_events: number
  sound_events: number
  avg_light: number
  min_light: number
  max_light: number
  light_comfort: number
  avg_rssi: number
  avg_heap_kb: number
  longest_bad_streak_minutes: number
}

export interface InsightsHourly {
  bucket: string
  good_rate: number
  alert_rate: number
  avg_light: number
  samples: number
}

export interface InsightsHeatmapCell {
  weekday: number
  hour: number
  good_rate: number
  alert_rate: number
  avg_light: number
  samples: number
}

export interface InsightsTimelinePoint {
  time: string
  status: StatusKind
  alert: boolean
  light: number
  sound: boolean
  person: boolean
}

export interface InsightsRiskWindow {
  weekday: number
  hour: number
  good_rate: number
  alert_rate: number
  avg_light: number
  samples: number
}

export interface Insights {
  device_id: string
  from: string
  to: string
  hours: number
  has_data: boolean
  health_score: number
  summary: InsightsSummary
  hourly: InsightsHourly[]
  heatmap: InsightsHeatmapCell[]
  timeline: InsightsTimelinePoint[]
  risk_windows: InsightsRiskWindow[]
}

export type AILevel = 'excellent' | 'good' | 'attention' | 'risk'

export interface AIFinding {
  title: string
  detail: string
  severity: 'info' | 'warn' | 'alert' | string
}

export interface AIReport {
  device_id: string
  hours: number
  score: number
  level: AILevel | string
  headline: string
  summary: string
  findings: AIFinding[]
  risk_windows: string[]
  suggestions: string[]
  encouragement: string
  disclaimer: string
  source: string
  model: string
  generated_at: string
}

export interface PostureFrame {
  id: number
  device_id: string
  captured_at: string
  status?: string | null
  summary?: string | null
  person_detected: number
  request_id?: string | null
  image_mime: string
  metadata?: unknown
  created_at: string
  image_url?: string
}

export interface Message {
  id: number
  device_id: string
  author: 'parent' | 'child'
  author_name: string
  content: string
  created_at: string
  delivered_at?: string | null
  read_at?: string | null
}

export type DemoStatus = 'good' | 'warning' | 'bad' | 'away'

