import { useCallback, useEffect, useRef, useState } from 'react'

import {
  ApiError,
  createMessage,
  generateAIReport,
  getApiConfig,
  getEvents,
  getFrameImage,
  getInsights,
  getLatest,
  getSeries,
  getTelemetry,
  listDevices,
  listFrames,
  listMessages,
  resetApiConfig,
  runDemoScenario,
  setApiConfig,
} from '../api'
import type {
  AIReport,
  ApiConfig,
  ChartPoint,
  DemoStatus,
  Device,
  EventRow,
  Insights,
  InsightsTimelinePoint,
  Message,
  PostureFrame,
  PosturePoint,
  RangeKey,
  TelemetryRow,
} from '../types'
import { postureTelemetryToPoints, rangeParams, seriesToPoints } from '../utils'

function messageOf(reason: unknown): string {
  if (reason instanceof ApiError) {
    if (reason.status === 401) {
      return 'API Token 无效或未设置，请打开设置填入查询 Token。'
    }
    if (reason.status === 0) {
      return reason.message
    }
    return reason.message
  }
  if (reason instanceof Error) {
    return reason.message
  }
  return String(reason)
}

export interface DashboardState {
  devices: Device[]
  selectedDevice: Device | undefined
  selectedDeviceId: string
  setSelectedDeviceId: (deviceId: string) => void
  latest: Map<string, TelemetryRow>
  lightSeries: ChartPoint[]
  rssiSeries: ChartPoint[]
  heapSeries: ChartPoint[]
  posturePoints: PosturePoint[]
  events: EventRow[]
  insights: Insights | null
  insightsError: string | null
  frames: PostureFrame[]
  selectedFrame: PostureFrame | undefined
  selectedFrameId: number | null
  selectFrame: (frameId: number) => void
  frameImageUrl: string | null
  frameLoading: boolean
  frameError: string | null
  messages: Message[]
  messagesError: string | null
  sendingMessage: boolean
  sendMessage: (
    author: 'parent' | 'child',
    authorName: string,
    content: string,
  ) => Promise<boolean>
  aiReport: AIReport | null
  aiLoading: boolean
  aiError: string | null
  generateReport: () => void
  demoBusy: boolean
  demoMessage: string | null
  runDemo: (status: DemoStatus) => void
  autoDemo: boolean
  setAutoDemo: (enabled: boolean) => void
  timeline: InsightsTimelinePoint[]
  replayActive: boolean
  replayPlaying: boolean
  replayIndex: number
  replayPoint: InsightsTimelinePoint | null
  startReplay: () => void
  exitReplay: () => void
  setReplayIndex: (index: number) => void
  toggleReplayPlaying: () => void
  loading: boolean
  refreshing: boolean
  error: string | null
  lastUpdated: Date | null
  range: RangeKey
  setRange: (range: RangeKey) => void
  autoRefresh: boolean
  setAutoRefresh: (enabled: boolean) => void
  refreshInterval: number
  setRefreshInterval: (seconds: number) => void
  refresh: () => void
  apiConfig: ApiConfig
  saveApiConfig: (config: ApiConfig) => void
  resetConfig: () => void
}

export function useDashboard(): DashboardState {
  const [devices, setDevices] = useState<Device[]>([])
  const [selectedDeviceId, setSelectedDeviceId] = useState('')
  const [latest, setLatest] = useState<Map<string, TelemetryRow>>(new Map())
  const [lightSeries, setLightSeries] = useState<ChartPoint[]>([])
  const [rssiSeries, setRssiSeries] = useState<ChartPoint[]>([])
  const [heapSeries, setHeapSeries] = useState<ChartPoint[]>([])
  const [posturePoints, setPosturePoints] = useState<PosturePoint[]>([])
  const [events, setEvents] = useState<EventRow[]>([])
  const [insights, setInsights] = useState<Insights | null>(null)
  const [insightsError, setInsightsError] = useState<string | null>(null)
  const [frames, setFrames] = useState<PostureFrame[]>([])
  const [selectedFrameId, setSelectedFrameId] = useState<number | null>(null)
  const [frameImageUrl, setFrameImageUrl] = useState<string | null>(null)
  const [frameLoading, setFrameLoading] = useState(false)
  const [frameError, setFrameError] = useState<string | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [messagesError, setMessagesError] = useState<string | null>(null)
  const [sendingMessage, setSendingMessage] = useState(false)
  const [aiReports, setAIReports] = useState<Record<string, AIReport>>({})
  const [aiLoading, setAILoading] = useState(false)
  const [aiError, setAIError] = useState<string | null>(null)
  const [demoBusy, setDemoBusy] = useState(false)
  const [demoMessage, setDemoMessage] = useState<string | null>(null)
  const [autoDemo, setAutoDemo] = useState(false)
  const [timeline, setTimeline] = useState<InsightsTimelinePoint[]>([])
  const [replayActive, setReplayActive] = useState(false)
  const [replayPlaying, setReplayPlaying] = useState(false)
  const [replayIndex, setReplayIndexState] = useState(0)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [range, setRange] = useState<RangeKey>('24h')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [refreshInterval, setRefreshInterval] = useState(5)
  const [apiConfig, setApiConfigState] = useState<ApiConfig>(() => getApiConfig())
  const [configVersion, setConfigVersion] = useState(0)

  const inFlight = useRef(false)
  const sequence = useRef(0)
  const frameURLRef = useRef<string | null>(null)
  const autoReportAttempted = useRef<Set<string>>(new Set())

  const load = useCallback(
    async (silent: boolean) => {
      if (inFlight.current) {
        return
      }
      inFlight.current = true
      const requestId = ++sequence.current
      if (!silent) {
        setLoading(true)
      }
      setRefreshing(true)

      try {
        const deviceResponse = await listDevices()
        if (requestId !== sequence.current) {
          return
        }
        const deviceList = deviceResponse.data ?? []
        setDevices(deviceList)

        if (deviceList.length === 0) {
          setSelectedDeviceId('')
          setLatest(new Map())
          setLightSeries([])
          setRssiSeries([])
          setHeapSeries([])
          setPosturePoints([])
          setEvents([])
          setInsights(null)
          setInsightsError(null)
          setTimeline([])
          setFrames([])
          setSelectedFrameId(null)
          setFrameError(null)
          setMessages([])
          setMessagesError(null)
          setError(null)
          setLastUpdated(new Date())
          return
        }

        const preferred =
          deviceList.find((device) => device.device_id === selectedDeviceId) ??
          deviceList.find((device) => device.device_id.includes('hbot')) ??
          deviceList[0]
        const deviceId = preferred.device_id
        if (deviceId !== selectedDeviceId) {
          setSelectedDeviceId(deviceId)
        }

        const { from, to, bucket } = rangeParams(range)
        const results = await Promise.allSettled([
          getLatest(deviceId),
          getSeries(deviceId, 'Environment', 'light', from, to, bucket),
          getSeries(deviceId, 'DeviceStatus', 'rssi', from, to, bucket),
          getSeries(deviceId, 'DeviceStatus', 'freeHeap', from, to, bucket),
          getTelemetry(deviceId, 'Posture', 'postureStatus', from, to, 240),
          getEvents(deviceId, 14),
          getInsights(deviceId, 24),
          listFrames(deviceId, 12),
          listMessages(deviceId, 30),
        ])
        if (requestId !== sequence.current) {
          return
        }

        let firstError: string | null = null
        const rememberError = (reason: unknown) => {
          if (!firstError) {
            firstError = messageOf(reason)
          }
        }

        const [
          latestResult,
          lightResult,
          rssiResult,
          heapResult,
          postureResult,
          eventsResult,
          insightsResult,
          framesResult,
          messagesResult,
        ] = results

        if (latestResult.status === 'fulfilled') {
          const map = new Map<string, TelemetryRow>()
          for (const row of latestResult.value.data) {
            map.set(`${row.service_id}.${row.property}`, row)
          }
          setLatest(map)
        } else {
          rememberError(latestResult.reason)
        }

        if (lightResult.status === 'fulfilled') {
          setLightSeries(seriesToPoints(lightResult.value.data))
        } else {
          rememberError(lightResult.reason)
        }
        if (rssiResult.status === 'fulfilled') {
          setRssiSeries(seriesToPoints(rssiResult.value.data))
        } else {
          rememberError(rssiResult.reason)
        }
        if (heapResult.status === 'fulfilled') {
          setHeapSeries(seriesToPoints(heapResult.value.data))
        } else {
          rememberError(heapResult.reason)
        }
        if (postureResult.status === 'fulfilled') {
          setPosturePoints(postureTelemetryToPoints(postureResult.value.data, 90))
        } else {
          rememberError(postureResult.reason)
        }
        if (eventsResult.status === 'fulfilled') {
          setEvents(eventsResult.value.data)
        } else {
          rememberError(eventsResult.reason)
        }
        if (insightsResult.status === 'fulfilled') {
          setInsights(insightsResult.value)
          setInsightsError(null)
          setTimeline(insightsResult.value.timeline ?? [])
        } else {
          setInsightsError(messageOf(insightsResult.reason))
        }
        if (framesResult.status === 'fulfilled') {
          const list = framesResult.value
          setFrames(list)
          setFrameError(null)
          if (list.length === 0) {
            setFrameLoading(false)
            setFrameImageUrl((previous) => {
              if (previous) {
                URL.revokeObjectURL(previous)
              }
              return null
            })
          }
          setSelectedFrameId((current) => {
            if (current !== null && list.some((frame) => frame.id === current)) {
              return current
            }
            return list[0]?.id ?? null
          })
        } else {
          setFrames([])
          setSelectedFrameId(null)
          setFrameError(messageOf(framesResult.reason))
        }
        if (messagesResult.status === 'fulfilled') {
          setMessages(messagesResult.value)
          setMessagesError(null)
        } else {
          setMessagesError(messageOf(messagesResult.reason))
        }

        setError(firstError)
        setLastUpdated(new Date())
      } catch (reason) {
        if (requestId === sequence.current) {
          setError(messageOf(reason))
        }
      } finally {
        if (requestId === sequence.current) {
          setLoading(false)
          setRefreshing(false)
        }
        inFlight.current = false
      }
    },
    [range, selectedDeviceId],
  )

  useEffect(() => {
    // Initial load and reload after the API configuration changes.
    // oxlint-disable-next-line react/set-state-in-effect
    void load(false)
  }, [load, configVersion])

  useEffect(() => {
    if (!autoRefresh) {
      return
    }
    const timer = window.setInterval(() => {
      void load(true)
    }, refreshInterval * 1000)
    return () => window.clearInterval(timer)
  }, [autoRefresh, refreshInterval, load])

  useEffect(() => {
    if (!selectedFrameId) {
      return
    }

    let cancelled = false
    // Loading an authenticated blob is an external-system synchronization.
    // oxlint-disable-next-line react/set-state-in-effect
    setFrameLoading(true)
    setFrameError(null)
    setFrameImageUrl((previous) => {
      if (previous) {
        URL.revokeObjectURL(previous)
      }
      return null
    })

    void getFrameImage(selectedFrameId)
      .then((blob) => {
        if (cancelled) {
          return
        }
        const url = URL.createObjectURL(blob)
        frameURLRef.current = url
        setFrameImageUrl(url)
      })
      .catch((reason) => {
        if (!cancelled) {
          setFrameError(messageOf(reason))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setFrameLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [selectedFrameId, configVersion])

  useEffect(() => {
    return () => {
      if (frameURLRef.current) {
        URL.revokeObjectURL(frameURLRef.current)
        frameURLRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    if (!replayActive || !replayPlaying || timeline.length === 0) {
      return
    }
    const timer = window.setInterval(() => {
      setReplayIndexState((current) => Math.min(current + 1, timeline.length - 1))
    }, 220)
    return () => window.clearInterval(timer)
  }, [replayActive, replayPlaying, timeline.length])

  const refresh = useCallback(() => {
    void load(false)
  }, [load])

  const saveApiConfig = useCallback((config: ApiConfig) => {
    setApiConfig(config)
    setApiConfigState(getApiConfig())
    setConfigVersion((version) => version + 1)
  }, [])

  const resetConfig = useCallback(() => {
    resetApiConfig()
    setApiConfigState(getApiConfig())
    setConfigVersion((version) => version + 1)
  }, [])

  const selectFrame = useCallback((frameId: number) => {
    setSelectedFrameId(frameId)
  }, [])

  const generateReport = useCallback(async () => {
    const deviceId = selectedDeviceId
    if (!deviceId || aiLoading) {
      return
    }
    setAILoading(true)
    setAIError(null)
    try {
      const report = await generateAIReport(deviceId, 24)
      setAIReports((reports) => ({ ...reports, [deviceId]: report }))
    } catch (reason) {
      setAIError(messageOf(reason))
    } finally {
      setAILoading(false)
    }
  }, [selectedDeviceId, aiLoading])

  useEffect(() => {
    if (!selectedDeviceId || !insights?.has_data || aiLoading) {
      return
    }
    if (aiReports[selectedDeviceId]) {
      return
    }
    if (autoReportAttempted.current.has(selectedDeviceId)) {
      return
    }
    autoReportAttempted.current.add(selectedDeviceId)
    // Auto-generate one report per device; the ref prevents a render loop.
    // oxlint-disable-next-line react/set-state-in-effect
    void generateReport()
  }, [selectedDeviceId, insights?.has_data, aiLoading, aiReports, generateReport])

  const runDemo = useCallback(
    async (status: DemoStatus) => {
      if (!selectedDeviceId || demoBusy) {
        return
      }
      setDemoBusy(true)
      setDemoMessage(null)
      try {
        const result = await runDemoScenario(selectedDeviceId, status)
        setDemoMessage(
          `已注入 ${result.summary} · ${new Date().toLocaleTimeString('zh-CN')}`,
        )
        await load(true)
      } catch (reason) {
        setDemoMessage(messageOf(reason))
      } finally {
        setDemoBusy(false)
      }
    },
    [selectedDeviceId, demoBusy, load],
  )

  useEffect(() => {
    if (!autoDemo || !selectedDeviceId) {
      return
    }
    const sequence: DemoStatus[] = ['good', 'good', 'warning', 'bad', 'good', 'away']
    let index = 0
    const timer = window.setInterval(() => {
      void runDemo(sequence[index % sequence.length])
      index += 1
    }, 5000)
    return () => window.clearInterval(timer)
  }, [autoDemo, selectedDeviceId, runDemo])

  const setReplayIndex = useCallback(
    (index: number) => {
      const max = Math.max(0, timeline.length - 1)
      setReplayIndexState(Math.min(Math.max(0, index), max))
      setReplayActive(true)
    },
    [timeline.length],
  )

  const startReplay = useCallback(() => {
    if (timeline.length === 0) {
      return
    }
    setReplayActive(true)
    setReplayPlaying(true)
    setReplayIndexState(0)
  }, [timeline.length])

  const exitReplay = useCallback(() => {
    setReplayActive(false)
    setReplayPlaying(false)
    setReplayIndexState(0)
  }, [])

  const toggleReplayPlaying = useCallback(() => {
    setReplayPlaying((playing) => !playing)
  }, [])

  const sendMessage = useCallback(
    async (author: 'parent' | 'child', authorName: string, content: string) => {
      const trimmed = content.trim()
      if (!selectedDeviceId || !trimmed || sendingMessage) {
        return false
      }
      setSendingMessage(true)
      setMessagesError(null)
      try {
        await createMessage(selectedDeviceId, author, authorName, trimmed)
        const list = await listMessages(selectedDeviceId, 30)
        setMessages(list)
        return true
      } catch (reason) {
        setMessagesError(messageOf(reason))
        return false
      } finally {
        setSendingMessage(false)
      }
    },
    [selectedDeviceId, sendingMessage],
  )

  const selectedDevice = devices.find((device) => device.device_id === selectedDeviceId)
  const selectedFrame = frames.find((frame) => frame.id === selectedFrameId)
  const visibleAIReport = aiReports[selectedDeviceId] ?? null
  const effectiveReplayActive = replayActive && timeline.length > 0
  const effectiveReplayIndex = Math.min(replayIndex, Math.max(0, timeline.length - 1))
  const replayPoint = effectiveReplayActive ? timeline[effectiveReplayIndex] ?? null : null

  return {
    devices,
    selectedDevice,
    selectedDeviceId,
    setSelectedDeviceId,
    latest,
    lightSeries,
    rssiSeries,
    heapSeries,
    posturePoints,
    events,
    insights,
    insightsError,
    frames,
    selectedFrame,
    selectedFrameId,
    selectFrame,
    frameImageUrl,
    frameLoading,
    frameError,
    messages,
    messagesError,
    sendingMessage,
    sendMessage,
    aiReport: visibleAIReport,
    aiLoading,
    aiError,
    generateReport,
    demoBusy,
    demoMessage,
    runDemo,
    autoDemo,
    setAutoDemo,
    timeline,
    replayActive: effectiveReplayActive,
    replayPlaying: effectiveReplayActive && replayPlaying,
    replayIndex: effectiveReplayIndex,
    replayPoint,
    startReplay,
    exitReplay,
    setReplayIndex,
    toggleReplayPlaying,
    loading,
    refreshing,
    error,
    lastUpdated,
    range,
    setRange,
    autoRefresh,
    setAutoRefresh,
    refreshInterval,
    setRefreshInterval,
    refresh,
    apiConfig,
    saveApiConfig,
    resetConfig,
  }
}
