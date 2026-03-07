import L, { Map } from 'leaflet'
import 'leaflet.markercluster'
import type { MapNode } from '../api/types'
import { relativeTime } from '../utils/time'
import clientMarkerSvgTemplate from './marker-icons/client.svg?raw'
import clientBaseMarkerSvgTemplate from './marker-icons/client-base.svg?raw'
import clientMuteMarkerSvgTemplate from './marker-icons/client-mute.svg?raw'
import defaultMarkerSvgTemplate from './marker-icons/default.svg?raw'
import routerLateMarkerSvgTemplate from './marker-icons/router-late.svg?raw'
import routerMarkerSvgTemplate from './marker-icons/router.svg?raw'

type MarkerMap = Record<string, L.Marker>
type MarkerFreshness = 'mqtt-recent' | 'heard-recent' | 'stale' | 'cold'
type MarkerIconKey = 'default' | 'router' | 'router-late' | 'client' | 'client-base' | 'client-mute'

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

const SELECTED_MARKER_SCALE = 1.15
const MARKER_ICON_SIZE: [number, number] = [30, 42]
const MARKER_ICON_ANCHOR: [number, number] = [15, 36]
const MARKER_POPUP_ANCHOR: [number, number] = [0, -28]
const MARKER_TOOLTIP_ANCHOR: [number, number] = [10, -18]
const MARKER_SHADOW_URL = '/static/images/node-marker-shadow.svg'
const markerIconCache = new globalThis.Map<string, L.Icon>()
const COLD_NODE_AGE_MS = 7 * 24 * 60 * 60 * 1000
const markerSvgTemplates: Record<MarkerIconKey, string> = {
  default: defaultMarkerSvgTemplate,
  router: routerMarkerSvgTemplate,
  'router-late': routerLateMarkerSvgTemplate,
  client: clientMarkerSvgTemplate,
  'client-base': clientBaseMarkerSvgTemplate,
  'client-mute': clientMuteMarkerSvgTemplate
}

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
      const markerFreshness = markerFreshnessState(n, disconnectedThreshold)
      const markerIconKey = markerIconKeyForRole(n.node.role)
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
      const markerIcon = buildMarkerIcon(markerIconKey, markerFreshness, this.selectedID === id)
      const m = this.markers[id]
      if (m) {
        m.setLatLng(latlng)
        m.setIcon(markerIcon)
        m.getPopup()?.setContent(html)
        if (this.selectedID === id) {
          m.openPopup()
        }
      } else {
        const marker = L.marker(latlng, { icon: markerIcon }).bindPopup(html, {
          autoPan: false,
          closeButton: false
        })
        marker.on('popupopen', () => {
          const popupEl = marker.getPopup()?.getElement()
          const detailsLink = popupEl?.querySelector<HTMLElement>('[data-node-details-link]')
          detailsLink?.addEventListener('click', this.handleDetailsLinkClick)
          marker.setIcon(buildMarkerIcon(markerIconKey, markerFreshness, true))
          this.selectedID = id
          this.onSelectNode?.(id)
        })
        marker.on('popupclose', () => {
          const popupEl = marker.getPopup()?.getElement()
          const detailsLink = popupEl?.querySelector<HTMLElement>('[data-node-details-link]')
          detailsLink?.removeEventListener('click', this.handleDetailsLinkClick)
          if (this.selectedID !== id) return
          marker.setIcon(buildMarkerIcon(markerIconKey, markerFreshness, false))
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

function markerFreshnessState(node: MapNode, disconnectedThreshold?: string): MarkerFreshness {
  const thresholdMs = parseDurationMs(disconnectedThreshold)
  const lastMQTTSeenAt = parseTimestampMs(node.node.last_seen_mqtt_gateway_at)
  if (typeof thresholdMs === 'number' && lastMQTTSeenAt !== undefined && Date.now() - lastMQTTSeenAt <= thresholdMs) {
    return 'mqtt-recent'
  }

  const lastAnySeenAt = parseTimestampMs(node.node.last_seen_any_event_at)
  if (lastAnySeenAt === undefined) {
    return 'cold'
  }

  const ageMs = Date.now() - lastAnySeenAt
  if (typeof thresholdMs === 'number' && ageMs <= thresholdMs) {
    return 'heard-recent'
  }
  if (ageMs < COLD_NODE_AGE_MS) {
    return 'stale'
  }
  return 'cold'
}

function parseTimestampMs(raw?: string): number | undefined {
  if (!raw) return undefined
  const value = new Date(raw).getTime()
  if (Number.isNaN(value)) return undefined
  return value
}

function markerIconKeyForRole(role?: string): MarkerIconKey {
  switch (role) {
    case 'ROUTER':
      return 'router'
    case 'ROUTER_LATE':
      return 'router-late'
    case 'CLIENT':
      return 'client'
    case 'CLIENT_BASE':
      return 'client-base'
    case 'CLIENT_MUTE':
      return 'client-mute'
    default:
      return 'default'
  }
}

function buildMarkerIcon(iconKey: MarkerIconKey, freshness: MarkerFreshness, selected: boolean): L.Icon {
  const scale = selected ? SELECTED_MARKER_SCALE : 1
  const [width, height] = scalePoint(MARKER_ICON_SIZE, scale)
  const iconAnchor = scalePoint(MARKER_ICON_ANCHOR, scale)
  const popupAnchor = scalePoint(MARKER_POPUP_ANCHOR, scale)
  const tooltipAnchor = scalePoint(MARKER_TOOLTIP_ANCHOR, scale)
  const cacheKey = `${iconKey}:${freshness}:${selected ? 'selected' : 'default'}:${width}x${height}`
  const cached = markerIconCache.get(cacheKey)
  if (cached) {
    return cached
  }

  const icon = L.icon({
    iconUrl: markerDataUrl(iconKey, freshness, selected),
    iconRetinaUrl: markerDataUrl(iconKey, freshness, selected),
    shadowUrl: MARKER_SHADOW_URL,
    iconSize: [width, height],
    iconAnchor,
    popupAnchor,
    tooltipAnchor,
    shadowSize: scalePoint([25, 12], scale),
    shadowAnchor: scalePoint([13, 6], scale),
    className: selected ? 'map-node-marker-selected' : 'map-node-marker'
  })
  markerIconCache.set(cacheKey, icon)
  return icon
}

function markerDataUrl(iconKey: MarkerIconKey, freshness: MarkerFreshness, selected: boolean): string {
  const [fill, stroke] = markerColors(freshness)
  const scale = selected ? SELECTED_MARKER_SCALE : 1
  const [width, height] = scalePoint(MARKER_ICON_SIZE, scale)
  const svg = markerSvg(iconKey, {
    fill,
    stroke,
    width,
    height
  })
  return `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(svg)}`
}

function markerSvg(
  iconKey: MarkerIconKey,
  { fill, stroke, width, height }: { fill: string; stroke: string; width: number; height: number }
): string {
  const template = markerSvgTemplates[iconKey] ?? markerSvgTemplates.default
  return template
    .replaceAll('__MARKER_FILL__', fill)
    .replaceAll('__MARKER_STROKE__', stroke)
    .replaceAll('__MARKER_WIDTH__', String(width))
    .replaceAll('__MARKER_HEIGHT__', String(height))
}

function markerColors(freshness: MarkerFreshness): [string, string] {
  switch (freshness) {
    case 'mqtt-recent':
      return ['#4fbc6a', '#2f8142']
    case 'heard-recent':
      return ['#1f7a39', '#124a22']
    case 'cold':
      return ['#7b8794', '#4b5563']
    case 'stale':
    default:
      return ['#1f6ae5', '#0b3f97']
  }
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
  const sectionsHtml = sections.map((item) => (
    `<div class="map-popup-section">` +
      `<div class="map-popup-section-title"><strong>${item.title}</strong></div>` +
      `<div class="map-popup-section-body">${item.rows.map((row) => `${row.label}: ${row.value}`).join('<br/>')}</div>` +
    `</div>`
  )).join('')

  return (
    `<div class="map-popup-title"><b>${title}</b></div>` +
    sectionsHtml +
    `<div class="map-popup-actions"><a href="#" data-node-details-link="${id}">Details</a></div>`
  )
}
