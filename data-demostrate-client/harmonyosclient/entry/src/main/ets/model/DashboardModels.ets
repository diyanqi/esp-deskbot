export interface Tag {
  tag_key: string;
  tag_value?: string | null;
}

export interface Device {
  device_id: string;
  node_id?: string | null;
  product_id?: string | null;
  status?: string | null;
  last_seen_at: string;
  first_seen_at: string;
  tags?: Tag[] | null;
}

export interface TelemetryRow {
  id: number;
  device_id: string;
  service_id: string;
  property: string;
  value?: Object | null;
  value_num?: number | null;
  event_time: string;
  received_at: string;
  request_id?: string | null;
}

export interface SeriesPoint {
  bucket: string;
  count: number;
  avg?: number | null;
  min?: number | null;
  max?: number | null;
}

export interface EventRow {
  id: number;
  resource: string;
  event?: string | null;
  request_id?: string | null;
  device_id?: string | null;
  event_time?: string | null;
  received_at: string;
  body?: Object | null;
}

export interface InsightsSummary {
  posture_samples: number;
  good: number;
  warning: number;
  bad: number;
  unknown: number;
  good_rate: number;
  alert_rate: number;
  person_rate: number;
  alert_minutes: number;
  alert_events: number;
  sound_events: number;
  avg_light: number;
  min_light: number;
  max_light: number;
  light_comfort: number;
  avg_rssi: number;
  avg_heap_kb: number;
  longest_bad_streak_minutes: number;
}

export interface InsightsHourly {
  bucket: string;
  good_rate: number;
  alert_rate: number;
  avg_light: number;
  samples: number;
}

export interface InsightsHeatmapCell {
  weekday: number;
  hour: number;
  good_rate: number;
  alert_rate: number;
  avg_light: number;
  samples: number;
}

export interface InsightsTimelinePoint {
  time: string;
  status: string;
  alert: boolean;
  light: number;
  sound: boolean;
  person: boolean;
}

export interface InsightsRiskWindow {
  weekday: number;
  hour: number;
  good_rate: number;
  alert_rate: number;
  avg_light: number;
  samples: number;
}

export interface Insights {
  device_id: string;
  from: string;
  to: string;
  hours: number;
  has_data: boolean;
  health_score: number;
  summary: InsightsSummary;
  hourly: InsightsHourly[];
  heatmap: InsightsHeatmapCell[];
  timeline: InsightsTimelinePoint[];
  risk_windows: InsightsRiskWindow[];
}

export interface AIFinding {
  title: string;
  detail: string;
  severity: string;
}

export interface AIReport {
  device_id: string;
  hours: number;
  score: number;
  level: string;
  headline: string;
  summary: string;
  findings: AIFinding[];
  risk_windows: string[];
  suggestions: string[];
  encouragement: string;
  disclaimer: string;
  source: string;
  model: string;
  generated_at: string;
}

export interface PostureFrame {
  id: number;
  device_id: string;
  captured_at: string;
  status?: string | null;
  summary?: string | null;
  person_detected: number;
  request_id?: string | null;
  image_mime: string;
  created_at: string;
  image_url?: string;
}

export interface Message {
  id: number;
  device_id: string;
  author: string;
  author_name: string;
  content: string;
  created_at: string;
  delivered_at?: string | null;
  read_at?: string | null;
}

export interface DemoResult {
  inserted: boolean;
  device_id: string;
  status: string;
  summary: string;
  event_time: string;
}

export interface ChartPoint {
  time: string;
  value: number;
}

export interface ListResponse<T> {
  data: T[];
}

export interface LatestResponse {
  data: TelemetryRow[];
}

export interface SeriesResponse {
  data: SeriesPoint[];
}

export interface InsightsResponse {
  data: Insights;
}

export interface AIReportResponse {
  data: AIReport;
}

export interface FramesResponse {
  data: PostureFrame[];
}

export interface MessagesResponse {
  data: Message[];
}

export interface MessageResponse {
  data: Message;
}

export interface DemoResponse {
  data: DemoResult;
}

export interface RangeParams {
  from: string;
  to: string;
  bucket: string;
  hours: number;
}
