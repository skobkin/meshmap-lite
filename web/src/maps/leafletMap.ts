import L, { Map } from 'leaflet'
import 'leaflet.markercluster'
import type { MapNode } from '../api/types'
import { relativeTime } from '../utils/time'

type MarkerMap = Record<string, L.Marker>

interface LeafletMapOptions {
  clustering?: boolean
  onOpenNodeDetails?: (id: string) => void
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

const SELECTED_MARKER_SCALE = 1 / 0.6

const DEFAULT_MARKER_OPTIONS = {
  iconUrl: '/static/images/node-marker.svg',
  iconRetinaUrl: '/static/images/node-marker.svg',
  shadowUrl: '/static/images/node-marker-shadow.svg',
  iconSize: [18, 25] as L.PointExpression,
  iconAnchor: [9, 25] as L.PointExpression,
  popupAnchor: [0, -20] as L.PointExpression,
  tooltipAnchor: [10, -12] as L.PointExpression,
  shadowSize: [25, 12] as L.PointExpression,
  shadowAnchor: [13, 6] as L.PointExpression
}

const DEFAULT_MARKER_ICON = L.icon(DEFAULT_MARKER_OPTIONS)
const SELECTED_MARKER_ICON = L.icon(scaleMarkerOptions(DEFAULT_MARKER_OPTIONS, SELECTED_MARKER_SCALE))

export class LeafletMapAdapter {
  private map: Map
  private readonly markerLayer: L.FeatureGroup | L.MarkerClusterGroup
  private markers: MarkerMap = {}
  private selectedID?: string
  private readonly onOpenNodeDetails?: (id: string) => void
  private readonly onSelectNode?: (id?: string) => void

  constructor(el: HTMLElement, center: [number, number], zoom: number, opts: LeafletMapOptions = {}) {
    this.onOpenNodeDetails = opts.onOpenNodeDetails
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
      const html = popupHtml(id, n.node.long_name ?? id, compactSections([
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
        m.setIcon(this.selectedID === id ? SELECTED_MARKER_ICON : DEFAULT_MARKER_ICON)
        m.getPopup()?.setContent(html)
        if (this.selectedID === id) {
          m.openPopup()
        }
      } else {
        const marker = L.marker(latlng, { icon: this.selectedID === id ? SELECTED_MARKER_ICON : DEFAULT_MARKER_ICON }).bindPopup(html, {
          autoPan: false,
          closeButton: false
        })
        marker.on('popupopen', () => {
          const popupEl = marker.getPopup()?.getElement()
          const detailsLink = popupEl?.querySelector<HTMLElement>('[data-node-details-link]')
          detailsLink?.addEventListener('click', this.handleDetailsLinkClick)
          marker.setIcon(SELECTED_MARKER_ICON)
          this.selectedID = id
          this.onSelectNode?.(id)
        })
        marker.on('popupclose', () => {
          const popupEl = marker.getPopup()?.getElement()
          const detailsLink = popupEl?.querySelector<HTMLElement>('[data-node-details-link]')
          detailsLink?.removeEventListener('click', this.handleDetailsLinkClick)
          if (this.selectedID !== id) return
          marker.setIcon(DEFAULT_MARKER_ICON)
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

  focusNode(id: string): void {
    const marker = this.markers[id]
    if (!marker) {
      return
    }

    const openMarker = () => {
      this.map.panTo(marker.getLatLng())
      marker.openPopup()
    }

    if (this.markerLayer instanceof L.MarkerClusterGroup) {
      this.markerLayer.zoomToShowLayer(marker, openMarker)
      return
    }

    openMarker()
  }

  destroy(): void {
    for (const marker of Object.values(this.markers)) {
      const popupEl = marker.getPopup()?.getElement()
      const detailsLink = popupEl?.querySelector<HTMLElement>('[data-node-details-link]')
      detailsLink?.removeEventListener('click', this.handleDetailsLinkClick)
      marker.off('popupopen')
      marker.off('popupclose')
    }
    this.map.remove()
  }

  private readonly handleDetailsLinkClick = (event: Event): void => {
    event.preventDefault()
    const target = event.currentTarget
    if (!(target instanceof HTMLElement)) {
      return
    }
    const id = target.dataset.nodeDetailsLink
    if (!id) {
      return
    }
    this.onOpenNodeDetails?.(id)
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

function scalePoint(value: L.PointExpression, scale: number): [number, number] {
  const [x, y] = Array.isArray(value) ? value : [value.x, value.y]
  return [Math.round(x * scale), Math.round(y * scale)]
}

function scaleMarkerOptions(options: typeof DEFAULT_MARKER_OPTIONS, scale: number): typeof DEFAULT_MARKER_OPTIONS {
  return {
    ...options,
    iconSize: scalePoint(options.iconSize, scale),
    iconAnchor: scalePoint(options.iconAnchor, scale),
    popupAnchor: scalePoint(options.popupAnchor, scale),
    tooltipAnchor: scalePoint(options.tooltipAnchor, scale),
    shadowSize: scalePoint(options.shadowSize, scale),
    shadowAnchor: scalePoint(options.shadowAnchor, scale)
  }
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

function popupHtml(id: string, title: string, sections: PopupSection[]): string {
  return [
    `<b>${title}</b>`,
    ...sections.map((item) => `<div><strong>${item.title}</strong></div>${item.rows.map((row) => `${row.label}: ${row.value}`).join('<br/>')}`),
    `<div class="map-popup-actions"><a href="#" data-node-details-link="${id}">Details</a></div>`
  ].join('<br/>')
}
