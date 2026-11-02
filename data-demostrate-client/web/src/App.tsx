import { useState } from 'react'
import {
  AlertTriangle,
  Cpu,
  Droplets,
  RefreshCw,
  Sun,
  Thermometer,
  Timer,
  Volume2,
  Wifi,
} from 'lucide-react'

import { AIReportPanel } from './components/AIReportPanel'
import { DemoConsole } from './components/DemoConsole'
import { DeviceFleet } from './components/DeviceFleet'
import { EmptyState } from './components/EmptyState'
import { EventStream } from './components/EventStream'
import { GrowthOverview } from './components/GrowthOverview'
import { Header } from './components/Header'
import { HealthChart } from './components/HealthChart'
import { Heatmap } from './components/Heatmap'
import { LightChart } from './components/LightChart'
import { LiveFrame } from './components/LiveFrame'
import { LoadingState } from './components/LoadingState'
import { MessageBoard } from './components/MessageBoard'
import { MetricCard } from './components/MetricCard'
import { PostureHero } from './components/PostureHero'
import { PostureMix } from './components/PostureMix'
import { ReplayConsole } from './components/ReplayConsole'
import { SectionNav } from './components/SectionNav'
import { SettingsDrawer } from './components/SettingsDrawer'
import { useDashboard } from './hooks/useDashboard'
import {
  STATUS_META,
  formatDateTime,
  formatNumber,
  formatUptime,
  latestNumber,
  latestString,
  latestValue,
  normalizeStatus,
} from './utils'

function App() {
  const dashboard = useDashboard()
  const [settingsOpen, setSettingsOpen] = useState(false)

  const {
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
    selectFrame,
    frameImageUrl,
    frameLoading,
    frameError,
    messages,
    messagesError,
    sendingMessage,
    sendMessage,
    aiReport,
    aiLoading,
    aiError,
    generateReport,
    demoBusy,
    demoMessage,
    runDemo,
    autoDemo,
    setAutoDemo,
    timeline,
    replayActive,
    replayPlaying,
    replayIndex,
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
  } = dashboard

  const status = normalizeStatus(latestValue(latest, 'Posture', 'postureStatus')?.value)
  const summary = latestString(latest, 'Posture', 'postureSummary')
  const personDetected = latestNumber(latest, 'Posture', 'personDetected', 0) === 1
  const alertFlag = latestNumber(latest, 'Posture', 'alert', 0)
  const issueCount = latestNumber(latest, 'Posture', 'issueCount', 0)
  const lastSeq = latestNumber(latest, 'Posture', 'lastSeq', 0)

  const light = latestNumber(latest, 'Environment', 'light', 0)
  const sound = latestNumber(latest, 'Environment', 'sound', 0)
  const temperature = latestNumber(latest, 'Environment', 'temperature', 0)
  const humidity = latestNumber(latest, 'Environment', 'humidity', 0)

  const rssi = latestNumber(latest, 'DeviceStatus', 'rssi', 0)
  const freeHeap = latestNumber(latest, 'DeviceStatus', 'freeHeap', 0)
  const uptime = latestNumber(latest, 'DeviceStatus', 'uptime', 0)
  const reportInterval = latestNumber(latest, 'DeviceStatus', 'reportInterval', 0)
  const firmware = latestString(latest, 'DeviceStatus', 'firmware', '—')

  const displayStatus = replayPoint ? normalizeStatus(replayPoint.status) : status
  const displaySummary = replayPoint
    ? `${STATUS_META[displayStatus].zh} · 历史回放`
    : summary
  const displayPersonDetected = replayPoint ? replayPoint.person : personDetected
  const displayAlert = replayPoint ? (replayPoint.alert ? 1 : 0) : alertFlag
  const displayIssueCount = replayPoint ? (replayPoint.alert ? 1 : 0) : issueCount
  const displayLastSeq = replayPoint ? 0 : lastSeq
  const displayLastSeen = replayPoint?.time ?? selectedDevice?.last_seen_at ?? null
  const displayLight = replayPoint ? replayPoint.light : light
  const displaySound = replayPoint ? (replayPoint.sound ? 1 : 0) : sound

  const lightSpark = lightSeries.map((point) => point.value)
  const rssiSpark = rssiSeries.map((point) => point.value)
  const heapSpark = heapSeries.map((point) => point.value / 1024)

  const heapKb = freeHeap / 1024
  const hasDevices = devices.length > 0
  const showLoading = loading && !hasDevices
  const showEmpty = !loading && !hasDevices
  const isDemo =
    selectedDevice?.tags?.some((tag) => tag.tag_key === 'demo' && tag.tag_value === 'true') ??
    false
  const lightHint = displayLight >= 60 ? '光照充足' : displayLight >= 30 ? '光照适中' : '光照偏暗'
  const rssiHint = rssi >= -55 ? '信号优秀' : rssi >= -70 ? '信号良好' : '信号偏弱'
  const sedentary = summary.includes('久坐')

  return (
    <div className="app">
      <div className="ambient" aria-hidden="true">
        <span className="ambient-orb ambient-orb-a" />
        <span className="ambient-orb ambient-orb-b" />
        <span className="ambient-orb ambient-orb-c" />
        <span className="ambient-grid" />
        <span className="ambient-vignette" />
      </div>

      <div className="app-shell">
        <Header
          devices={devices}
          selectedDeviceId={selectedDeviceId}
          onSelectDevice={setSelectedDeviceId}
          range={range}
          onRangeChange={setRange}
          autoRefresh={autoRefresh}
          onToggleAutoRefresh={() => setAutoRefresh(!autoRefresh)}
          refreshInterval={refreshInterval}
          onRefreshIntervalChange={setRefreshInterval}
          refreshing={refreshing}
          lastUpdated={lastUpdated}
          onRefresh={refresh}
          onOpenSettings={() => setSettingsOpen(true)}
          connected={hasDevices && !error}
          demo={isDemo}
        />

        {hasDevices && <SectionNav />}

        {hasDevices && error && (
          <div className="error-banner" role="status">
            <AlertTriangle size={16} />
            <span>{error}</span>
            <button type="button" className="btn btn-ghost btn-small" onClick={refresh}>
              <RefreshCw size={14} /> 重试
            </button>
          </div>
        )}

        {showLoading && <LoadingState />}

        {showEmpty && (
          <EmptyState
            title={error ? '无法连接数据服务' : '还没有孩子的坐姿数据'}
            description={
              error ??
              '等待书桌设备上报，或先在后端生成演示数据，再刷新此页。'
            }
            hint={error ? '检查 API Token、Base URL 与后端 /healthz' : 'cd data-collect-server && make seed'}
            actionLabel="打开 API 设置"
            onAction={() => setSettingsOpen(true)}
          />
        )}

        {hasDevices && (
          <main className="dashboard" id="overview">
            <section className="hero-grid">
              <PostureHero
                status={displayStatus}
                summary={displaySummary}
                personDetected={displayPersonDetected}
                alert={displayAlert}
                issueCount={displayIssueCount}
                lastSeq={displayLastSeq}
                points={posturePoints}
                lastSeen={displayLastSeen}
                uptime={uptime}
                firmware={firmware}
                reportInterval={reportInterval}
                replayActive={replayActive}
                replayTime={replayPoint?.time ?? null}
              />

              <div className="metric-grid">
                <MetricCard
                  icon={<Sun size={18} />}
                  label="桌面光照"
                  value={formatNumber(displayLight, 0)}
                  unit="%"
                  hint={lightHint}
                  tone="amber"
                  spark={lightSpark}
                  live={!replayActive}
                />
                <MetricCard
                  icon={<Volume2 size={18} />}
                  label="环境声音"
                  value={formatNumber(displaySound, 0)}
                  unit="LEVEL"
                  hint={displaySound === 1 ? '检测到声音' : '环境安静'}
                  tone="violet"
                />
                <MetricCard
                  icon={<Thermometer size={18} />}
                  label="桌面温度"
                  value={formatNumber(temperature, 1)}
                  unit="°C"
                  hint="传感器离线 · 固定上报 0"
                  tone="rose"
                />
                <MetricCard
                  icon={<Droplets size={18} />}
                  label="桌面湿度"
                  value={formatNumber(humidity, 0)}
                  unit="%"
                  hint="传感器离线 · 固定上报 0"
                  tone="cyan"
                />
                <MetricCard
                  icon={<Wifi size={18} />}
                  label="设备信号"
                  value={formatNumber(rssi, 0)}
                  unit="dBm"
                  hint={rssiHint}
                  tone="emerald"
                  spark={rssiSpark}
                  live={!replayActive}
                />
                <MetricCard
                  icon={<Cpu size={18} />}
                  label="设备内存"
                  value={formatNumber(heapKb, 0)}
                  unit="KB"
                  hint={`uptime ${formatUptime(uptime)}`}
                  tone="slate"
                  spark={heapSpark}
                />
              </div>
            </section>

            {sedentary && (
              <div className="sedentary-banner" role="status">
                <span className="sedentary-icon">
                  <Timer size={18} />
                </span>
                <div>
                  <strong>久坐提醒</strong>
                  <span>{summary}</span>
                </div>
                <span className="sedentary-badge">ESP 已提醒站起来活动</span>
              </div>
            )}

            <section className="growth-grid">
              <LiveFrame
                frames={frames}
                selectedFrame={selectedFrame}
                imageUrl={frameImageUrl}
                loading={frameLoading}
                error={frameError}
                onSelect={selectFrame}
              />
              <GrowthOverview
                insights={insights}
                insightsError={insightsError}
                aiReport={aiReport}
                aiLoading={aiLoading}
                onGenerateReport={() => {
                  void generateReport()
                }}
              />
            </section>

            <section className="chart-grid">
              <LightChart data={lightSeries} range={range} />
              <PostureMix points={posturePoints} />
            </section>

            <section className="analysis-grid">
              <AIReportPanel
                report={aiReport}
                loading={aiLoading}
                error={aiError}
                onGenerate={() => {
                  void generateReport()
                }}
              />
              <Heatmap
                cells={insights?.heatmap ?? []}
                riskWindows={insights?.risk_windows ?? []}
              />
            </section>

            <MessageBoard
              messages={messages}
              error={messagesError}
              sending={sendingMessage}
              onSend={sendMessage}
            />

            <section className="bottom-grid">
              <HealthChart rssi={rssiSeries} heap={heapSeries} range={range} />
              <EventStream events={events} />
            </section>

            <section className="replay-demo-grid">
              <ReplayConsole
                timeline={timeline}
                active={replayActive}
                playing={replayPlaying}
                index={replayIndex}
                point={replayPoint}
                onStart={startReplay}
                onExit={exitReplay}
                onSeek={setReplayIndex}
                onTogglePlaying={toggleReplayPlaying}
              />
              <DemoConsole
                busy={demoBusy}
                message={demoMessage}
                autoDemo={autoDemo}
                onToggleAuto={setAutoDemo}
                onRun={(scenario) => {
                  void runDemo(scenario)
                }}
              />
            </section>

            <DeviceFleet
              devices={devices}
              selectedDeviceId={selectedDeviceId}
              onSelect={setSelectedDeviceId}
            />
          </main>
        )}

        <footer className="footer">
          <div className="footer-brand">
            <span className="footer-dot" />
            <span>成长守护</span>
            <span className="footer-sep">/</span>
            <span>ESP-CLAW · HUAWEI IOTDA · POSTGRESQL · GPT-4O</span>
          </div>
          <div className="footer-meta">
            <span>{apiConfig.baseUrl || 'same-origin'}</span>
            <span className="footer-sep">/</span>
            <span>{lastUpdated ? formatDateTime(lastUpdated.toISOString()) : '等待首次同步'}</span>
            <span className="footer-sep">/</span>
            <span>AI 报告仅供参考，不构成医疗建议</span>
          </div>
        </footer>
      </div>

      <SettingsDrawer
        key={settingsOpen ? 'settings-open' : 'settings-closed'}
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        config={apiConfig}
        onSave={saveApiConfig}
        onReset={resetConfig}
      />
    </div>
  )
}

export default App
