import L from 'leaflet'

import 'leaflet.markercluster'
import { relativeTime } from '../utils/time'

import {
  MARKER_FRESHNESS,
  MARKER_ICON_ANCHOR,
  MARKER_ICON_SIZE,
  MARKER_POPUP_ANCHOR,
  MARKER_TOOLTIP_ANCHOR,
  type MarkerFreshness,
  type MarkerIconKey,
  SELECTED_MARKER_SCALE,
  markerDataUrl,
  markerIconKeyForRole
} from './markerIcons'

import type { MapNode, MapPrecisionCirclesMode } from '../api/types'
import type { Map } from 'leaflet'

type MarkerMap = Record<string, L.Marker>

interface LeafletMapOptions {
  clustering?: boolean
  precisionCirclesMode?: MapPrecisionCirclesMode
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

const MARKER_SHADOW_URL = '/static/images/node-marker-shadow.svg'
const markerIconCache = new globalThis.Map<string, L.Icon>()
const COLD_NODE_AGE_MS = 7 * 24 * 60 * 60 * 1000
const MARKER_CACHE_VARIANT = {
  selected: 'selected',
  default: 'default'
} as const

export class LeafletMapAdapter {
  private map: Map
  private readonly markerLayer: L.FeatureGroup | L.MarkerClusterGroup
  private markers: MarkerMap = {}
  private mapNodesByID = new globalThis.Map<string, MapNode>()
  private readonly precisionCircleLayer: L.FeatureGroup
  private precisionCircles = new globalThis.Map<string, L.Circle>()
  private readonly precisionCirclesMode: MapPrecisionCirclesMode
  private lastDisconnectedThreshold?: string
  private selectedID?: string
  private readonly onOpenNodeDetails?: (id: string) => void
  private readonly onSelectNode?: (id?: string) => void

  public constructor(el: HTMLElement, center: [number, number], zoom: number, opts: LeafletMapOptions = {}) {
    this.onOpenNodeDetails = opts.onOpenNodeDetails
    this.onSelectNode = opts.onSelectNode
    this.precisionCirclesMode = opts.precisionCirclesMode ?? 'none'
    this.map = L.map(el).setView(center, zoom)
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(this.map)
    this.precisionCircleLayer = L.featureGroup().addTo(this.map)
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

  public setView(center: [number, number], zoom: number): void {
    const current = this.map.getCenter()
    if (this.map.getZoom() === zoom && Math.abs(current.lat - center[0]) < 1e-8 && Math.abs(current.lng - center[1]) < 1e-8) {
      return
    }
    this.map.setView(center, zoom)
  }

  public render(nodes: MapNode[], disconnectedThreshold?: string): void {
    this.lastDisconnectedThreshold = disconnectedThreshold
    this.mapNodesByID = new globalThis.Map(nodes.map((node) => [node.node.node_id, node]))
    const visibleNodeIDs = new Set<string>()
    const visibleCircleIDs = new Set<string>()
    for (const n of nodes) {
      if (!n.position) {continue}
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
          popupEl?.addEventListener('click', this.handlePopupClick)
          marker.setIcon(buildMarkerIcon(markerIconKey, markerFreshness, true))
          this.selectedID = id
          this.render(Array.from(this.mapNodesByID.values()), this.lastDisconnectedThreshold)
          this.onSelectNode?.(id)
        })
        marker.on('popupclose', () => {
          const popupEl = marker.getPopup()?.getElement()
          popupEl?.removeEventListener('click', this.handlePopupClick)
          if (this.selectedID !== id) {return}
          marker.setIcon(buildMarkerIcon(markerIconKey, markerFreshness, false))
          this.selectedID = undefined
          this.render(Array.from(this.mapNodesByID.values()), this.lastDisconnectedThreshold)
          this.onSelectNode?.(undefined)
        })
        this.markers[id] = marker.addTo(this.markerLayer)
        if (this.selectedID === id) {
          marker.openPopup()
        }
      }

      if (!this.shouldRenderPrecisionCircle(id, n.position.position_precision)) {
        continue
      }

      visibleCircleIDs.add(id)
      const circle = this.precisionCircles.get(id)
      const circleLatLng: L.LatLngExpression = [n.position.latitude, n.position.longitude]
      const radiusMeters = precisionBitsToRadiusMeters(n.position.position_precision)
      if (radiusMeters === undefined) {
        continue
      }
      if (circle) {
        circle.setLatLng(circleLatLng)
        circle.setRadius(radiusMeters)
        continue
      }

      this.precisionCircles.set(id, L.circle(circleLatLng, {
        radius: radiusMeters,
        color: '#0b3f97',
        weight: 1.5,
        opacity: 0.45,
        fillColor: '#1f6ae5',
        fillOpacity: 0.14,
        interactive: false,
        bubblingMouseEvents: false
      }).addTo(this.precisionCircleLayer))
    }

    for (const [id, marker] of Object.entries(this.markers)) {
      if (visibleNodeIDs.has(id)) {continue}
      if (this.selectedID === id) {
        marker.closePopup()
      }
      this.markerLayer.removeLayer(marker)
      delete this.markers[id]
    }

    for (const [id, circle] of this.precisionCircles.entries()) {
      if (visibleCircleIDs.has(id)) {continue}
      this.precisionCircleLayer.removeLayer(circle)
      this.precisionCircles.delete(id)
    }
  }

  public setSelectedNode(id?: string): void {
    if (id === this.selectedID) {return}
    if (!id) {
      this.selectedID = undefined
      this.map.closePopup()
      this.render(Array.from(this.mapNodesByID.values()), this.lastDisconnectedThreshold)

      return
    }
    const marker = this.markers[id]
    if (!marker) {
      this.selectedID = undefined
      this.map.closePopup()
      this.render(Array.from(this.mapNodesByID.values()), this.lastDisconnectedThreshold)

      return
    }
    marker.openPopup()
  }

  public focusNode(id: string): void {
    const marker = this.markers[id]
    if (!marker) {
      return
    }

    const openMarker = (): void => {
      this.map.panTo(marker.getLatLng())
      marker.openPopup()
    }

    if (this.markerLayer instanceof L.MarkerClusterGroup) {
      this.markerLayer.zoomToShowLayer(marker, openMarker)

      return
    }

    openMarker()
  }

  public destroy(): void {
    for (const marker of Object.values(this.markers)) {
      const popupEl = marker.getPopup()?.getElement()
      popupEl?.removeEventListener('click', this.handlePopupClick)
      marker.off('popupopen')
      marker.off('popupclose')
    }
    this.precisionCircleLayer.clearLayers()
    this.precisionCircles.clear()
    this.map.remove()
  }

  private readonly handlePopupClick = (event: Event): void => {
    const target = event.target
    if (!(target instanceof Element)) {
      return
    }
    const detailsLink = target.closest<HTMLElement>('[data-node-details-link]')
    if (!detailsLink) {
      return
    }
    event.preventDefault()
    event.stopPropagation()
    const id = detailsLink.dataset.nodeDetailsLink
    if (!id) {
      return
    }
    this.onOpenNodeDetails?.(id)
  }

  private shouldRenderPrecisionCircle(id: string, bits?: number): boolean {
    if (precisionBitsToRadiusMeters(bits) === undefined) {
      return false
    }
    switch (this.precisionCirclesMode) {
      case 'always':
        return true
      case 'selected':
        return this.selectedID === id
      case 'none':
      default:
        return false
    }
  }
}

function parseDurationMs(raw?: string): number | undefined {
  if (!raw) {return undefined}
  const token = /([0-9]+(?:\.[0-9]+)?)(ns|us|µs|ms|s|m|h)/g
  let total = 0
  let found = false
  for (const match of raw.matchAll(token)) {
    found = true
    const n = Number(match[1])
    const unit = match[2]
    if (!Number.isFinite(n)) {continue}
    if (unit === 'h') {total += n * 3600000}
    if (unit === 'm') {total += n * 60000}
    if (unit === 's') {total += n * 1000}
    if (unit === 'ms') {total += n}
    if (unit === 'us' || unit === 'µs') {total += n / 1000}
    if (unit === 'ns') {total += n / 1000000}
  }
  if (!found) {return undefined}

  return Math.max(0, Math.floor(total))
}

function scalePoint(value: L.PointExpression, scale: number): [number, number] {
  const [x, y] = Array.isArray(value) ? value : [value.x, value.y]

  return [Math.round(x * scale), Math.round(y * scale)]
}

function mqttStatus(lastSeen?: string, disconnectedThreshold?: string): { status: 'Connected' | 'Disconnected'; age?: string } {
  if (!lastSeen) {return { status: 'Disconnected' }}
  const t = new Date(lastSeen)
  if (Number.isNaN(t.getTime())) {return { status: 'Disconnected' }}
  const thresholdMs = parseDurationMs(disconnectedThreshold)
  const ageMs = Date.now() - t.getTime()
  const age = relativeTime(lastSeen)
  if (typeof thresholdMs !== 'number') {return { status: 'Connected', age }}

  return ageMs <= thresholdMs ? { status: 'Connected', age } : { status: 'Disconnected', age }
}

function markerFreshnessState(node: MapNode, disconnectedThreshold?: string): MarkerFreshness {
  const thresholdMs = parseDurationMs(disconnectedThreshold)
  const lastMQTTSeenAt = parseTimestampMs(node.node.last_seen_mqtt_gateway_at)
  if (typeof thresholdMs === 'number' && lastMQTTSeenAt !== undefined && Date.now() - lastMQTTSeenAt <= thresholdMs) {
    return MARKER_FRESHNESS.mqttRecent
  }

  const lastAnySeenAt = parseTimestampMs(node.node.last_seen_any_event_at)
  if (lastAnySeenAt === undefined) {
    return MARKER_FRESHNESS.cold
  }

  const ageMs = Date.now() - lastAnySeenAt
  if (typeof thresholdMs === 'number' && ageMs <= thresholdMs) {
    return MARKER_FRESHNESS.heardRecent
  }
  if (ageMs < COLD_NODE_AGE_MS) {
    return MARKER_FRESHNESS.stale
  }

  return MARKER_FRESHNESS.cold
}

function parseTimestampMs(raw?: string): number | undefined {
  if (!raw) {return undefined}
  const value = new Date(raw).getTime()
  if (Number.isNaN(value)) {return undefined}

  return value
}

function buildMarkerIcon(iconKey: MarkerIconKey, freshness: MarkerFreshness, selected: boolean): L.Icon {
  const scale = selected ? SELECTED_MARKER_SCALE : 1
  const [width, height] = scalePoint(MARKER_ICON_SIZE, scale)
  const iconAnchor = scalePoint(MARKER_ICON_ANCHOR, scale)
  const popupAnchor = scalePoint(MARKER_POPUP_ANCHOR, scale)
  const tooltipAnchor = scalePoint(MARKER_TOOLTIP_ANCHOR, scale)
  const cacheVariant = selected ? MARKER_CACHE_VARIANT.selected : MARKER_CACHE_VARIANT.default
  const cacheKey = `${iconKey}:${freshness}:${cacheVariant}:${width}x${height}`
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

function precisionBitsToRadiusMeters(bits?: number): number | undefined {
  if (typeof bits !== 'number' || !Number.isFinite(bits)) {
    return undefined
  }
  if (bits <= 0 || bits >= 32) {
    return undefined
  }

  return 23905787.925008 * Math.pow(0.5, bits)
}

function displayValue(v: string | number | boolean | undefined): string | null {
  if (typeof v === 'boolean') {return v ? 'yes' : 'no'}
  if (typeof v === 'number') {return String(v)}

  return v && v.length > 0 ? v : null
}

function displayRelativeTime(v?: string): string | null {
  return v ? relativeTime(v) : null
}

function row(label: string, value: string | null): PopupRow | null {
  return value === null ? null : { label, value }
}

function compactRows(rows: (PopupRow | null)[]): PopupRow[] {
  return rows.filter((item): item is PopupRow => item !== null)
}

function compactValues(values: (string | null)[]): string[] {
  return values.filter((value): value is string => value !== null)
}

function section(title: string, rows: PopupRow[]): PopupSection | null {
  return rows.length > 0 ? { title, rows } : null
}

function compactSections(sections: (PopupSection | null)[]): PopupSection[] {
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
