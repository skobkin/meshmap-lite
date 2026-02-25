import L, { Map } from 'leaflet'
import type { MapNode } from '../api/types'
import { relativeTime } from '../utils/time'

type MarkerMap = Record<string, L.Marker>

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
  private markers: MarkerMap = {}

  constructor(el: HTMLElement, center: [number, number], zoom: number, onViewChange?: (center: [number, number], zoom: number) => void) {
    this.map = L.map(el).setView(center, zoom)
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(this.map)
    if (onViewChange) {
      this.map.on('moveend', () => {
        const c = this.map.getCenter()
        onViewChange([c.lat, c.lng], this.map.getZoom())
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

  render(nodes: MapNode[]): void {
    for (const n of nodes) {
      if (!n.position) continue
      const id = n.node.node_id
      const html = `<b>${n.node.long_name ?? n.node.short_name ?? id}</b><br/>ID: ${id}<br/>Role: ${n.node.role ?? '-'}<br/>FW: ${n.node.firmware_version ?? '-'}<br/>Last update: ${relativeTime(n.node.last_seen_any_event_at)}`
      const latlng: [number, number] = [n.position.latitude, n.position.longitude]
      const m = this.markers[id]
      if (m) {
        m.setLatLng(latlng)
        m.bindTooltip(html)
      } else {
        this.markers[id] = L.marker(latlng).bindTooltip(html).addTo(this.map)
      }
    }
  }

  destroy(): void {
    this.map.remove()
  }
}
