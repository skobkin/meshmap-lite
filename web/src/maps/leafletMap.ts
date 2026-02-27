import L, { Map } from 'leaflet'
import 'leaflet.markercluster'
import type { MapNode } from '../api/types'
import { relativeTime } from '../utils/time'

type MarkerMap = Record<string, L.Marker>

interface LeafletMapOptions {
  clustering?: boolean
  onViewChange?: (center: [number, number], zoom: number) => void
  onSelectNode?: (id?: string) => void
}

interface PopupRow {
  label: string
  value: string
}

interface PopupSection {
  title: string
  rows: PopupRow[]
}

L.Icon.Default.mergeOptions({
  iconUrl: '/static/images/node-marker.svg',
  iconRetinaUrl: '/static/images/node-marker.svg',
  shadowUrl: '/static/images/node-marker-shadow.svg',
  iconSize: [30, 42],
  iconAnchor: [15, 42],
  popupAnchor: [0, -34],
  tooltipAnchor: [16, -20],
  shadowSize: [42, 20],
  shadowAnchor: [21, 10]
})

export class LeafletMapAdapter {
  private map: Map
  private readonly markerLayer: L.FeatureGroup
  private markers: MarkerMap = {}
  private selectedID?: string
  private readonly onSelectNode?: (id?: string) => void

  constructor(el: HTMLElement, center: [number, number], zoom: number, opts: LeafletMapOptions = {}) {
    this.onSelectNode = opts.onSelectNode
    this.map = L.map(el).setView(center, zoom)
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(this.map)
    this.markerLayer = opts.clustering
      ? L.markerClusterGroup({
          chunkedLoading: true,
          removeOutsideVisibleBounds: true,
          showCoverageOnHover: false
        })
      : L.featureGroup()
    this.markerLayer.addTo(this.map)
    if (opts.onViewChange) {
      this.map.on('moveend', () => {
        const c = this.map.getCenter()
        opts.onViewChange?.([c.lat, c.lng], this.map.getZoom())
      })
    }
  }

  setView(center: [number, number], zoom: number): void {
    const current = this.map.getCenter()
    if (this.map.getZoom() === zoom && Math.abs(current.lat - center[0]) < 1e-8 && Math.abs(current.lng - center[1]) < 1e-8) {
      return
    }
    this.map.setView(center, zoom)
  }

  render(nodes: MapNode[], disconnectedThreshold?: string): void {
    const visibleNodeIDs = new Set<string>()
    for (const n of nodes) {
      if (!n.position) continue
      const id = n.node.node_id
      visibleNodeIDs.add(id)
      const mqtt = mqttStatus(n.node.last_seen_mqtt_gateway_at, disconnectedThreshold)
      const lora = compactValues([displayValue(n.node.lora_region), displayValue(n.node.lora_frequency_desc)]).join(' / ')
      const html = popupHtml(n.node.long_name ?? id, compactSections([
        section('Identity', compactRows([
          row('Short', displayValue(n.node.short_name)),
          row('ID', id),
          row('Role', displayValue(n.node.role)),
          row('Neighbors', displayValue(n.node.neighbor_nodes_count))
        ])),
        section('Connectivity', compactRows([
          row('MQTT', `${mqtt.status}${mqtt.age ? ` (${mqtt.age})` : ''}`),
          row('Last update', displayRelativeTime(n.node.last_seen_any_event_at)),
          row('Last position', displayRelativeTime(n.node.last_seen_position_at))
        ])),
        section('Radio', compactRows([
          row('LoRa', lora || null),
          row('Modem', displayValue(n.node.modem_preset)),
          row('Default channel', displayValue(n.node.has_default_channel)),
          row('Location reports', displayValue(n.node.has_opted_report_location)),
          row('Board', displayValue(n.node.board_model)),
          row('FW', displayValue(n.node.firmware_version))
        ]))
      ]))
      const latlng: [number, number] = [n.position.latitude, n.position.longitude]
      const m = this.markers[id]
      if (m) {
        m.setLatLng(latlng)
        m.getPopup()?.setContent(html)
        if (this.selectedID === id) {
          m.openPopup()
        }
      } else {
        const marker = L.marker(latlng).bindPopup(html, {
          autoPan: false,
          closeButton: false
        })
        marker.on('popupopen', () => {
          this.selectedID = id
          this.onSelectNode?.(id)
        })
        marker.on('popupclose', () => {
          if (this.selectedID !== id) return
          this.selectedID = undefined
          this.onSelectNode?.(undefined)
        })
        this.markers[id] = marker.addTo(this.markerLayer)
        if (this.selectedID === id) {
          marker.openPopup()
        }
      }
    }

    for (const [id, marker] of Object.entries(this.markers)) {
      if (visibleNodeIDs.has(id)) continue
      if (this.selectedID === id) {
        marker.closePopup()
      }
      this.markerLayer.removeLayer(marker)
      delete this.markers[id]
    }
  }

  setSelectedNode(id?: string): void {
    if (id === this.selectedID) return
    if (!id) {
      this.map.closePopup()
      return
    }
    const marker = this.markers[id]
    if (!marker) {
      this.map.closePopup()
      return
    }
    marker.openPopup()
  }

  destroy(): void {
    this.map.remove()
  }
}

function parseDurationMs(raw?: string): number | undefined {
  if (!raw) return undefined
  const token = /([0-9]+(?:\.[0-9]+)?)(ns|us|µs|ms|s|m|h)/g
  let total = 0
  let found = false
  for (const match of raw.matchAll(token)) {
    found = true
    const n = Number(match[1])
    const unit = match[2]
    if (!Number.isFinite(n)) continue
    if (unit === 'h') total += n * 3600000
    if (unit === 'm') total += n * 60000
    if (unit === 's') total += n * 1000
    if (unit === 'ms') total += n
    if (unit === 'us' || unit === 'µs') total += n / 1000
    if (unit === 'ns') total += n / 1000000
  }
  if (!found) return undefined
  return Math.max(0, Math.floor(total))
}

function mqttStatus(lastSeen?: string, disconnectedThreshold?: string): { status: 'Connected' | 'Disconnected'; age?: string } {
  if (!lastSeen) return { status: 'Disconnected' }
  const t = new Date(lastSeen)
  if (Number.isNaN(t.getTime())) return { status: 'Disconnected' }
  const thresholdMs = parseDurationMs(disconnectedThreshold)
  const ageMs = Date.now() - t.getTime()
  const age = relativeTime(lastSeen)
  if (typeof thresholdMs !== 'number') return { status: 'Connected', age }
  return ageMs <= thresholdMs ? { status: 'Connected', age } : { status: 'Disconnected', age }
}

function displayValue(v: string | number | boolean | undefined): string | null {
  if (typeof v === 'boolean') return v ? 'yes' : 'no'
  if (typeof v === 'number') return String(v)
  return v && v.length > 0 ? v : null
}

function displayRelativeTime(v?: string): string | null {
  return v ? relativeTime(v) : null
}

function row(label: string, value: string | null): PopupRow | null {
  return value === null ? null : { label, value }
}

function compactRows(rows: Array<PopupRow | null>): PopupRow[] {
  return rows.filter((item): item is PopupRow => item !== null)
}

function compactValues(values: Array<string | null>): string[] {
  return values.filter((value): value is string => value !== null)
}

function section(title: string, rows: PopupRow[]): PopupSection | null {
  return rows.length > 0 ? { title, rows } : null
}

function compactSections(sections: Array<PopupSection | null>): PopupSection[] {
  return sections.filter((item): item is PopupSection => item !== null)
}

function popupHtml(title: string, sections: PopupSection[]): string {
  return [
    `<b>${title}</b>`,
    ...sections.map((item) => `<div><strong>${item.title}</strong></div>${item.rows.map((row) => `${row.label}: ${row.value}`).join('<br/>')}`)
  ].join('<br/>')
}
