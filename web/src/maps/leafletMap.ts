import L from 'leaflet'

import 'leaflet.markercluster'
import { formatBattery } from '../utils/battery'
import { parseDurationMs } from '../utils/duration'
import { relativeTime } from '../utils/time'
import { topologyColor } from '../utils/topology'

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

import type { MapNode, MapPrecisionCirclesMode, NodeNeighbor, TopologyEdge } from '../api/types'
import type { Map } from 'leaflet'

type MarkerMap = Record<string, L.Marker>
type SpiderfiedMarker = L.Marker & {
  _preSpiderfyLatlng?: L.LatLng
}

interface LeafletMapOptions {
  clustering?: boolean
  precisionCirclesMode?: MapPrecisionCirclesMode
  onHoverNode?: (id?: string) => void
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
export const OVERLAP_CLUSTER_RADIUS_PX = 18
export const OVERLAP_SPIDERFY_DISTANCE_MULTIPLIER = 1.15
const markerIconCache = new globalThis.Map<string, L.Icon>()
const VISUAL_COLD_NODE_AGE_MS = 7 * 24 * 60 * 60 * 1000
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
  private readonly topologyLayer: L.FeatureGroup
  private readonly topologyAllLayer: L.FeatureGroup
  private precisionCircles = new globalThis.Map<string, L.Circle>()
  private readonly precisionCirclesMode: MapPrecisionCirclesMode
  private lastDisconnectedThreshold?: string
  private selectedID?: string
  private readonly onHoverNode?: (id?: string) => void
  private readonly onOpenNodeDetails?: (id: string) => void
  private readonly onSelectNode?: (id?: string) => void

  public constructor(el: HTMLElement, center: [number, number], zoom: number, opts: LeafletMapOptions = {}) {
    this.onHoverNode = opts.onHoverNode
    this.onOpenNodeDetails = opts.onOpenNodeDetails
    this.onSelectNode = opts.onSelectNode
    this.precisionCirclesMode = opts.precisionCirclesMode ?? 'none'
    this.map = L.map(el).setView(center, zoom)
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(this.map)
    this.precisionCircleLayer = L.featureGroup().addTo(this.map)
    // The "all topology" layer sits below the focal-node layer so the
    // per-node polylines read first when both are visible.
    this.topologyAllLayer = L.featureGroup().addTo(this.map)
    this.topologyLayer = L.featureGroup().addTo(this.map)
    this.markerLayer = L.markerClusterGroup(markerClusterOptions(opts.clustering ?? true))
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
      if (!n.position) {
        continue
      }
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
          row('Online local nodes', displayValue(n.node.neighbor_nodes_count))
        ])),
        section('Connectivity', compactRows([
          row('MQTT', `${mqtt.status}${mqtt.age ? ` (${mqtt.age})` : ''}`),
          row('Last GW', displayUploader(n.node.last_mqtt_uploader_node_id, n.node.last_mqtt_uploader_display_name, n.node.last_mqtt_uploader_at)),
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
      ]), n.telemetry)
      const latlng: [number, number] = [n.position.latitude, n.position.longitude]
      const markerIcon = buildMarkerIcon(markerIconKey, markerFreshness, this.selectedID === id)
      const m = this.markers[id]
      if (m) {
        if (!sameLatLng(storedMarkerLatLng(m), latlng)) {
          m.setLatLng(latlng)
        }
        m.setIcon(markerIcon)
        m.getPopup()?.setContent(html)
        if (this.selectedID === id && !m.getPopup()?.isOpen()) {
          m.openPopup()
        }
      } else {
        const marker = L.marker(latlng, { icon: markerIcon }).bindPopup(html, {
          autoPan: false,
          closeButton: false,
          // Class lets styles.css target the Leaflet popup container with
          // theme-aware overrides (dark mode in particular — Leaflet renders
          // outside the React tree, so data-theme on <html> doesn't reach it
          // without an explicit class on the popup element).
          className: 'map-popup'
        })
        marker.on('popupopen', () => {
          const popupEl = marker.getPopup()?.getElement()
          popupEl?.addEventListener('click', this.handlePopupClick)
          this.setSelection(id)
        })
        marker.on('popupclose', () => {
          const popupEl = marker.getPopup()?.getElement()
          popupEl?.removeEventListener('click', this.handlePopupClick)
          if (this.selectedID !== id) {return}
          this.setSelection(undefined)
        })
        marker.on('mouseover', () => {
          this.onHoverNode?.(id)
        })
        marker.on('mouseout', () => {
          this.onHoverNode?.(undefined)
        })
        this.markers[id] = marker.addTo(this.markerLayer)
        if (this.selectedID === id) {
          marker.openPopup()
        }
      }

      if (this.shouldRenderPrecisionCircle(id, n.position.position_precision)) {
        visibleCircleIDs.add(id)
      }
    }

    for (const [id, marker] of Object.entries(this.markers)) {
      if (visibleNodeIDs.has(id)) {continue}
      if (this.selectedID === id) {
        marker.closePopup()
      }
      this.markerLayer.removeLayer(marker)
      delete this.markers[id]
    }

    this.syncPrecisionCircles(visibleCircleIDs)
  }

  public renderTopology(nodeID?: string, neighbors: NodeNeighbor[] = []): void {
    this.topologyLayer.clearLayers()
    if (!nodeID) {
      return
    }

    const origin = this.mapNodesByID.get(nodeID)?.position
    if (!origin) {
      return
    }

    for (const neighbor of neighbors) {
      const peer = this.mapNodesByID.get(neighbor.node_id)?.position
      if (!peer) {
        continue
      }

      L.polyline([
        [origin.latitude, origin.longitude],
        [peer.latitude, peer.longitude]
      ], {
        color: topologyColor({
          inferred: neighbor.evidence_kind === 'inferred',
          mqttDirect: neighbor.evidence_kind === 'mqtt_direct',
          snr: neighbor.snr
        }),
        weight: this.selectedID === nodeID ? 3 : 2.5,
        opacity: 0.82,
        interactive: false,
        bubblingMouseEvents: false
      }).addTo(this.topologyLayer)
    }
  }

  public renderAllTopology(edges: TopologyEdge[] = []): void {
    this.topologyAllLayer.clearLayers()
    if (edges.length === 0) {
      return
    }

    for (const edge of edges) {
      const from = this.mapNodesByID.get(edge.from_node_id)?.position
      const to = this.mapNodesByID.get(edge.to_node_id)?.position
      if (!from || !to) {
        continue
      }

      L.polyline([
        [from.latitude, from.longitude],
        [to.latitude, to.longitude]
      ], {
        color: topologyColor({
          inferred: edge.inferred === true,
          mqttDirect: edge.source_kind === 'mqtt_direct',
          snr: edge.snr
        }),
        weight: 2.5,
        opacity: 0.82,
        interactive: false,
        bubblingMouseEvents: false
      }).addTo(this.topologyAllLayer)
    }
  }

  public setSelectedNode(id?: string): void {
    if (id === this.selectedID) {return}
    if (!id) {
      this.setSelection(undefined)
      this.map.closePopup()

      return
    }
    const marker = this.markers[id]
    if (!marker) {
      this.setSelection(undefined)
      this.map.closePopup()

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
      marker.off('mouseover')
      marker.off('mouseout')
    }
    this.precisionCircleLayer.clearLayers()
    this.precisionCircles.clear()
    this.topologyLayer.clearLayers()
    this.topologyAllLayer.clearLayers()
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

  private setSelection(id?: string): void {
    if (id === this.selectedID) {
      return
    }

    const previousID = this.selectedID
    this.selectedID = id
    this.setMarkerSelectedIcon(previousID, false)
    this.setMarkerSelectedIcon(id, true)
    this.syncPrecisionCircles()
    this.onSelectNode?.(id)
  }

  private setMarkerSelectedIcon(id: string | undefined, selected: boolean): void {
    if (!id) {
      return
    }
    const marker = this.markers[id]
    const node = this.mapNodesByID.get(id)
    if (!marker || !node) {
      return
    }

    marker.setIcon(buildMarkerIcon(
      markerIconKeyForRole(node.node.role),
      markerFreshnessState(node, this.lastDisconnectedThreshold),
      selected
    ))
  }

  private syncPrecisionCircles(visibleCircleIDs?: Set<string>): void {
    const nextVisibleCircleIDs = visibleCircleIDs ?? new Set<string>()

    if (!visibleCircleIDs) {
      for (const [id, node] of this.mapNodesByID.entries()) {
        if (node.position && this.shouldRenderPrecisionCircle(id, node.position.position_precision)) {
          nextVisibleCircleIDs.add(id)
        }
      }
    }

    for (const id of nextVisibleCircleIDs) {
      const position = this.mapNodesByID.get(id)?.position
      if (!position) {
        continue
      }

      const radiusMeters = precisionBitsToRadiusMeters(position.position_precision)
      if (radiusMeters === undefined) {
        continue
      }

      const circleLatLng: L.LatLngExpression = [position.latitude, position.longitude]
      const circle = this.precisionCircles.get(id)
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

    for (const [id, circle] of this.precisionCircles.entries()) {
      if (nextVisibleCircleIDs.has(id)) {continue}
      this.precisionCircleLayer.removeLayer(circle)
      this.precisionCircles.delete(id)
    }
  }
}

export function markerClusterOptions(clustering: boolean): L.MarkerClusterGroupOptions {
  const options: L.MarkerClusterGroupOptions = {
    chunkedLoading: true,
    removeOutsideVisibleBounds: true,
    showCoverageOnHover: false,
    spiderfyOnMaxZoom: true
  }

  if (clustering) {
    return options
  }

  return {
    ...options,
    maxClusterRadius: OVERLAP_CLUSTER_RADIUS_PX,
    spiderfyOnEveryZoom: true,
    zoomToBoundsOnClick: false,
    spiderfyDistanceMultiplier: OVERLAP_SPIDERFY_DISTANCE_MULTIPLIER
  }
}

function storedMarkerLatLng(marker: L.Marker): L.LatLng {
  return (marker as SpiderfiedMarker)._preSpiderfyLatlng ?? marker.getLatLng()
}

function sameLatLng(current: L.LatLng, next: L.LatLngExpression): boolean {
  const [lat, lng] = Array.isArray(next) ? next : [next.lat, next.lng]

  return Math.abs(current.lat - lat) < 1e-8 && Math.abs(current.lng - lng) < 1e-8
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
  if (ageMs < VISUAL_COLD_NODE_AGE_MS) {
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

function displayUploader(nodeID?: string, displayName?: string, seenAt?: string): string | null {
  if (!nodeID) {return null}
  const label = displayName && displayName !== nodeID ? displayName : nodeID
  const age = displayRelativeTime(seenAt)
  const escapedNodeID = escapeHtml(nodeID)
  const link = `<a href="#" data-node-details-link="${escapedNodeID}" title="${escapedNodeID}" class="map-popup-node-link"><code>${escapeHtml(label)}</code></a>`

  return age ? `${link}, ${age}` : link
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
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

function formatBatteryInfo(power: { voltage?: number; battery_level?: number }): string {
  const { voltage, level, qualityClass, hasData } = formatBattery(power)
  if (!hasData) {return ''}

  // Layout: icon, then a small pill wrapping the percentage (coloured by
  // qualityClass), then middot, then voltage. The pill keeps the colored
  // area small and matches the .chat-hop-badge / .log-hop-badge pill style
  // used elsewhere. The .map-popup-battery class on the outer span keeps
  // the line itself muted (icon and voltage stay in --pico-muted-color).
  const pillClass = ['battery-pill', qualityClass].filter(Boolean).join(' ')
  const pill = level ? `<span class="${pillClass}">${level}</span>` : ''
  const middle = level && voltage ? ' · ' : ''

  return `<span class="map-popup-battery">🔋 ${pill}${middle}${voltage ?? ''}</span>`
}

function popupHtml(
  id: string,
  title: string,
  sections: PopupSection[],
  telemetry?: { power: { voltage?: number; battery_level?: number }; observed_at: string },
): string {
  const sectionsHtml = sections
    .map(
      (item) =>
        `<div class="map-popup-section">` +
        `<div class="map-popup-section-title"><strong>${item.title}</strong></div>` +
        `<div class="map-popup-section-body">${item.rows.map((row) => `${row.label}: ${row.value}`).join('<br/>')}</div>` +
        `</div>`,
    )
    .join('')

  const power = telemetry?.power
  const batteryInfo =
    power && (power.voltage !== undefined || power.battery_level !== undefined) ? formatBatteryInfo(power) : ''

  return (
    `<div class="map-popup-title"><b>${title}</b></div>` +
    sectionsHtml +
    `<div class="map-popup-details-section">` +
      `<a href="#" data-node-details-link="${id}" class="map-popup-details-link">Details</a>` +
      batteryInfo +
    `</div>`
  )
}
