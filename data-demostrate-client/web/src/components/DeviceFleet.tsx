import { ChevronRight, MapPin, Router, Wifi } from 'lucide-react'

import type { Device } from '../types'
import { deviceTag, formatRelativeTime } from '../utils'

interface DeviceFleetProps {
  devices: Device[]
  selectedDeviceId: string
  onSelect: (deviceId: string) => void
}

export function DeviceFleet({ devices, selectedDeviceId, onSelect }: DeviceFleetProps) {
  return (
    <section className="fleet-section" id="devices">
      <div className="fleet-head">
        <div>
          <div className="panel-kicker">GUARDIAN / DEVICES</div>
          <h2 className="panel-title">孩子的设备</h2>
        </div>
        <span className="micro-badge">{devices.length} DEVICES</span>
      </div>

      {devices.length === 0 ? (
        <div className="fleet-empty">暂无设备，等待孩子书桌上的设备接入…</div>
      ) : (
        <div className="fleet-grid">
          {devices.map((device) => {
            const selected = device.device_id === selectedDeviceId
            const online = (device.status ?? '').toUpperCase() === 'ONLINE'
            const room = deviceTag(device, 'room')
            return (
              <button
                key={device.device_id}
                type="button"
                className={`fleet-card ${selected ? 'is-selected' : ''}`}
                onClick={() => onSelect(device.device_id)}
              >
                <div className="fleet-card-top">
                  <span className={`fleet-status ${online ? 'is-online' : 'is-offline'}`}>
                    <span className="fleet-status-dot" />
                    {online ? 'ONLINE' : 'OFFLINE'}
                  </span>
                  <ChevronRight size={16} className="fleet-chevron" />
                </div>
                <div className="fleet-name">
                  <Router size={17} />
                  <span>{device.node_id || device.device_id}</span>
                </div>
                <div className="fleet-id">{device.device_id}</div>
                <div className="fleet-meta">
                  <span>
                    <Wifi size={13} />
                    {device.product_id || 'unknown product'}
                  </span>
                  {room && (
                    <span>
                      <MapPin size={13} />
                      {room}
                    </span>
                  )}
                </div>
                <div className="fleet-foot">
                  <span>last seen</span>
                  <span>{formatRelativeTime(device.last_seen_at)}</span>
                </div>
              </button>
            )
          })}
        </div>
      )}
    </section>
  )
}
