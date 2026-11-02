import type {
  AIReport,
  ApiConfig,
  DemoStatus,
  Device,
  EventRow,
  Insights,
  LatestResponse,
  ListResponse,
  Message,
  PostureFrame,
  SeriesResponse,
  TelemetryRow,
} from './types'

const STORAGE_KEY = 'desk-pulse:api-config:v1'

const envBaseUrl = String(import.meta.env.VITE_API_BASE_URL ?? '')
const envToken = String(import.meta.env.VITE_API_TOKEN ?? '')

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function readStoredConfig(): ApiConfig | null {
  if (typeof window === 'undefined') {
    return null
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as Partial<ApiConfig>
    if (typeof parsed.baseUrl === 'string' && typeof parsed.token === 'string') {
      return { baseUrl: parsed.baseUrl, token: parsed.token }
    }
  } catch {
    // Ignore malformed local settings.
  }
  return null
}

let currentConfig: ApiConfig = readStoredConfig() ?? {
  baseUrl: envBaseUrl,
  token: envToken,
}

export function getApiConfig(): ApiConfig {
  return currentConfig
}

export function setApiConfig(config: ApiConfig): void {
  currentConfig = {
    baseUrl: config.baseUrl.trim().replace(/\/+$/, ''),
    token: config.token.trim(),
  }
  if (typeof window !== 'undefined') {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(currentConfig))
    } catch {
      // Private mode can reject writes; the in-memory config still works.
    }
  }
}

export function resetApiConfig(): ApiConfig {
  if (typeof window !== 'undefined') {
    try {
      window.localStorage.removeItem(STORAGE_KEY)
    } catch {
      // Ignore.
    }
  }
  currentConfig = { baseUrl: envBaseUrl, token: envToken }
  return currentConfig
}

type QueryValue = string | number | boolean | null | undefined

function requestURL(path: string, query?: Record<string, QueryValue>): string {
  const base = currentConfig.baseUrl
  const target = `${base}${path}`
  const url = new URL(target, window.location.origin)
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null || value === '') {
        continue
      }
      url.searchParams.set(key, String(value))
    }
  }
  return url.toString()
}

function authHeaders(): HeadersInit {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (currentConfig.token) {
    headers.Authorization = `Bearer ${currentConfig.token}`
  }
  return headers
}

async function request<T>(path: string, query?: Record<string, QueryValue>): Promise<T> {
  let response: Response
  try {
    response = await fetch(requestURL(path, query), {
      headers: authHeaders(),
      cache: 'no-store',
    })
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    throw new ApiError(0, `无法连接数据服务：${reason}`)
  }

  if (!response.ok) {
    let message = `HTTP ${response.status}`
    try {
      const body = (await response.json()) as {
        error?: { message?: string; code?: string }
      }
      if (body.error?.message) {
        message = body.error.message
      }
    } catch {
      // Keep the HTTP status fallback.
    }
    throw new ApiError(response.status, message)
  }
  return (await response.json()) as T
}

export function listDevices(): Promise<ListResponse<Device>> {
  return request<ListResponse<Device>>('/api/v1/devices', { limit: 100 })
}

export function getLatest(deviceId: string): Promise<LatestResponse> {
  return request<LatestResponse>(
    `/api/v1/devices/${encodeURIComponent(deviceId)}/latest`,
  )
}

export function getSeries(
  deviceId: string,
  serviceId: string,
  property: string,
  from: string,
  to: string,
  bucket: string,
): Promise<SeriesResponse> {
  return request<SeriesResponse>(
    `/api/v1/devices/${encodeURIComponent(deviceId)}/series`,
    {
      service_id: serviceId,
      property,
      from,
      to,
      bucket,
    },
  )
}

export function getTelemetry(
  deviceId: string,
  serviceId: string,
  property: string,
  from: string,
  to: string,
  limit: number,
): Promise<ListResponse<TelemetryRow>> {
  return request<ListResponse<TelemetryRow>>(
    `/api/v1/devices/${encodeURIComponent(deviceId)}/telemetry`,
    {
      service_id: serviceId,
      property,
      from,
      to,
      limit,
    },
  )
}

export function getEvents(deviceId: string, limit: number): Promise<ListResponse<EventRow>> {
  return request<ListResponse<EventRow>>('/api/v1/events', {
    device_id: deviceId,
    limit,
  })
}

export function getInsights(deviceId: string, hours = 24): Promise<Insights> {
  return request<{ data: Insights }>('/api/v1/insights', {
    device_id: deviceId,
    hours,
  }).then((response) => response.data)
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  let response: Response
  try {
    response = await fetch(requestURL(path), {
      method: 'POST',
      headers: {
        ...authHeaders(),
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
      cache: 'no-store',
    })
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    throw new ApiError(0, `无法连接数据服务：${reason}`)
  }
  if (!response.ok) {
    let message = `HTTP ${response.status}`
    try {
      const payload = (await response.json()) as {
        error?: { message?: string; code?: string }
      }
      if (payload.error?.message) {
        message = payload.error.message
      }
    } catch {
      // Keep the HTTP status fallback.
    }
    throw new ApiError(response.status, message)
  }
  return (await response.json()) as T
}

export function generateAIReport(deviceId: string, hours = 24): Promise<AIReport> {
  return postJSON<{ data: AIReport }>('/api/v1/ai/report', {
    device_id: deviceId,
    hours,
  }).then((response) => response.data)
}

export function runDemoScenario(
  deviceId: string,
  status: DemoStatus,
  light?: number,
): Promise<{ inserted: boolean; status: string; summary: string }> {
  return postJSON<{ data: { inserted: boolean; status: string; summary: string } }>(
    '/api/v1/demo/scenario',
    {
      device_id: deviceId,
      status,
      light,
      sound: 0,
    },
  ).then((response) => response.data)
}

export async function getLatestFrame(deviceId: string): Promise<PostureFrame | null> {
  try {
    const response = await request<{ data: PostureFrame }>('/api/v1/frames/latest', {
      device_id: deviceId,
    })
    return response.data
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return null
    }
    throw error
  }
}

export function listFrames(deviceId: string, limit = 12): Promise<PostureFrame[]> {
  return request<{ data: PostureFrame[] }>('/api/v1/frames', {
    device_id: deviceId,
    limit,
  }).then((response) => response.data ?? [])
}

export function listMessages(deviceId: string, limit = 30): Promise<Message[]> {
  return request<{ data: Message[] }>('/api/v1/messages', {
    device_id: deviceId,
    limit,
  }).then((response) => response.data ?? [])
}

export function createMessage(
  deviceId: string,
  author: 'parent' | 'child',
  authorName: string,
  content: string,
): Promise<Message> {
  return postJSON<{ data: Message }>('/api/v1/messages', {
    device_id: deviceId,
    author,
    author_name: authorName,
    content,
  }).then((response) => response.data)
}

export async function getFrameImage(frameId: number): Promise<Blob> {
  const headers: Record<string, string> = { Accept: 'image/*' }
  if (currentConfig.token) {
    headers.Authorization = `Bearer ${currentConfig.token}`
  }
  let response: Response
  try {
    response = await fetch(requestURL(`/api/v1/frames/${frameId}/image`), {
      headers,
      cache: 'no-store',
    })
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    throw new ApiError(0, `无法加载坐姿画面：${reason}`)
  }
  if (!response.ok) {
    throw new ApiError(response.status, `无法加载坐姿画面（HTTP ${response.status}）`)
  }
  return response.blob()
}

