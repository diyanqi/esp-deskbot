import { Activity, ChevronDown, RefreshCw, Settings, Wifi, WifiOff } from 'lucide-react'

import type { Device, RangeKey } from '../types'
import { RANGE_OPTIONS, formatRelativeTime } from '../utils'

interface HeaderProps {
  devices: Device[]
  selectedDeviceId: string
  onSelectDevice: (deviceId: string) => void
  range: RangeKey
  onRangeChange: (range: RangeKey) => void
  autoRefresh: boolean
  onToggleAutoRefresh: () => void
  refreshInterval: number
  onRefreshIntervalChange: (seconds: number) => void
  refreshing: boolean
  lastUpdated: Date | null
  onRefresh: () => void
  onOpenSettings: () => void
  connected: boolean
  demo?: boolean
}

function deviceLabel(device: Device): string {
  if (device.device_id.includes('hbot')) {
    return '孩子的书桌'
  }
  if (device.node_id && device.node_id !== device.device_id) {
    return device.node_id
  }
  return device.device_id
}

export function Header({
  devices,
  selectedDeviceId,
  onSelectDevice,
  range,
  onRangeChange,
  autoRefresh,
  onToggleAutoRefresh,
  refreshInterval,
  onRefreshIntervalChange,
  refreshing,
  lastUpdated,
  onRefresh,
  onOpenSettings,
  connected,
  demo = false,
}: HeaderProps) {
  return (
    <header className="topbar">
      <div className="brand">
        <div className="brand-mark" aria-hidden="true">
          <span className="brand-pupil" />
        </div>
        <div className="brand-copy">
          <span className="brand-kicker">GROWTH GUARDIAN · HUAWEI IOTDA</span>
          <div className="brand-title-row">
            <h1>成长守护</h1>
            <span className="parent-badge">家长端</span>
            {demo && <span className="demo-badge">DEMO</span>}
          </div>
        </div>
      </div>

      <div className="child-chip" title="演示档案">
        <span className="child-chip-avatar">宇</span>
        <span className="child-chip-copy">
          <strong>小宇</strong>
          <span>9 岁 · 演示档案</span>
        </span>
      </div>

      <div className="topbar-center">
        <button
          type="button"
          className={`live-pill ${autoRefresh ? 'is-live' : 'is-paused'}`}
          onClick={onToggleAutoRefresh}
          title={autoRefresh ? '暂停自动刷新' : '恢复自动刷新'}
        >
          <span className="live-dot" />
          {autoRefresh ? 'LIVE' : 'PAUSED'}
          <span className="live-sep">/</span>
          <span className="live-interval">{refreshInterval}s</span>
        </button>

        <div className="device-select">
          <Wifi size={14} className="device-select-icon" />
          <select
            value={selectedDeviceId}
            onChange={(event) => onSelectDevice(event.target.value)}
            aria-label="选择孩子的设备"
          >
            {devices.length === 0 && <option value="">等待设备…</option>}
            {devices.map((device) => (
              <option key={device.device_id} value={device.device_id}>
                {deviceLabel(device)}
              </option>
            ))}
          </select>
          <ChevronDown size={14} className="device-select-chevron" />
        </div>

        <div className="range-group" role="group" aria-label="时间范围">
          {RANGE_OPTIONS.map((option) => (
            <button
              key={option.key}
              type="button"
              className={option.key === range ? 'is-active' : ''}
              onClick={() => onRangeChange(option.key)}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div className="topbar-actions">
        <div className="sync-status">
          {connected ? <Wifi size={14} /> : <WifiOff size={14} />}
          <span>{lastUpdated ? formatRelativeTime(lastUpdated.toISOString()) : '等待数据'}</span>
        </div>
        <div className="interval-select" title="自动刷新间隔">
          <Activity size={14} />
          <select
            value={refreshInterval}
            onChange={(event) => onRefreshIntervalChange(Number(event.target.value))}
            aria-label="自动刷新间隔"
          >
            <option value={5}>5s</option>
            <option value={10}>10s</option>
            <option value={30}>30s</option>
          </select>
        </div>
        <button
          type="button"
          className="icon-button"
          onClick={onRefresh}
          title="立即刷新"
          aria-label="立即刷新"
        >
          <RefreshCw size={17} className={refreshing ? 'spin' : ''} />
        </button>
        <button
          type="button"
          className="icon-button"
          onClick={onOpenSettings}
          title="API 设置"
          aria-label="API 设置"
        >
          <Settings size={17} />
        </button>
      </div>
    </header>
  )
}
