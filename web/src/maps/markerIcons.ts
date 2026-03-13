import clientBaseMarkerSvgTemplate from './marker-icons/client-base.svg?raw'
import clientMuteMarkerSvgTemplate from './marker-icons/client-mute.svg?raw'
import clientMarkerSvgTemplate from './marker-icons/client.svg?raw'
import defaultMarkerSvgTemplate from './marker-icons/default.svg?raw'
import routerLateMarkerSvgTemplate from './marker-icons/router-late.svg?raw'
import routerMarkerSvgTemplate from './marker-icons/router.svg?raw'

export const MARKER_FRESHNESS = {
  mqttRecent: 'mqtt-recent',
  heardRecent: 'heard-recent',
  stale: 'stale',
  cold: 'cold'
} as const

export type MarkerFreshness = typeof MARKER_FRESHNESS[keyof typeof MARKER_FRESHNESS]

export const MARKER_ICON_KEY = {
  default: 'default',
  router: 'router',
  routerLate: 'router-late',
  client: 'client',
  clientBase: 'client-base',
  clientMute: 'client-mute'
} as const

export type MarkerIconKey = typeof MARKER_ICON_KEY[keyof typeof MARKER_ICON_KEY]

export const SELECTED_MARKER_SCALE = 1.15
export const MARKER_ICON_SIZE: [number, number] = [30, 42]
export const MARKER_ICON_ANCHOR: [number, number] = [15, 36]
export const MARKER_POPUP_ANCHOR: [number, number] = [0, -28]
export const MARKER_TOOLTIP_ANCHOR: [number, number] = [10, -18]
export const DEFAULT_MARKER_ICON_KEY: MarkerIconKey = MARKER_ICON_KEY.default
export const DEFAULT_MARKER_FRESHNESS: MarkerFreshness = MARKER_FRESHNESS.stale

const markerSvgTemplates: Record<MarkerIconKey, string> = {
  [MARKER_ICON_KEY.default]: defaultMarkerSvgTemplate,
  [MARKER_ICON_KEY.router]: routerMarkerSvgTemplate,
  [MARKER_ICON_KEY.routerLate]: routerLateMarkerSvgTemplate,
  [MARKER_ICON_KEY.client]: clientMarkerSvgTemplate,
  [MARKER_ICON_KEY.clientBase]: clientBaseMarkerSvgTemplate,
  [MARKER_ICON_KEY.clientMute]: clientMuteMarkerSvgTemplate
}

export function markerDataUrl(iconKey: MarkerIconKey, freshness: MarkerFreshness, selected: boolean): string {
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

export function markerIconKeyForRole(role?: string): MarkerIconKey {
  switch (role) {
    case undefined:
      return MARKER_ICON_KEY.default
    case 'ROUTER':
      return MARKER_ICON_KEY.router
    case 'ROUTER_LATE':
      return MARKER_ICON_KEY.routerLate
    case 'CLIENT':
      return MARKER_ICON_KEY.client
    case 'CLIENT_BASE':
      return MARKER_ICON_KEY.clientBase
    case 'CLIENT_MUTE':
      return MARKER_ICON_KEY.clientMute
    default:
      return MARKER_ICON_KEY.default
  }
}

export function defaultMarkerDataUrl(): string {
  return markerDataUrl(DEFAULT_MARKER_ICON_KEY, DEFAULT_MARKER_FRESHNESS, false)
}

function markerSvg(
  iconKey: MarkerIconKey,
  { fill, stroke, width, height }: { fill: string; stroke: string; width: number; height: number }
): string {
  const template = markerSvgTemplates[iconKey] ?? markerSvgTemplates[MARKER_ICON_KEY.default]

  return template
    .replaceAll('__MARKER_FILL__', fill)
    .replaceAll('__MARKER_STROKE__', stroke)
    .replaceAll('__MARKER_WIDTH__', String(width))
    .replaceAll('__MARKER_HEIGHT__', String(height))
}

function markerColors(freshness: MarkerFreshness): [string, string] {
  switch (freshness) {
    case MARKER_FRESHNESS.mqttRecent:
      return ['#4fbc6a', '#2f8142']
    case MARKER_FRESHNESS.heardRecent:
      return ['#1f7a39', '#124a22']
    case MARKER_FRESHNESS.cold:
      return ['#7b8794', '#4b5563']
    case MARKER_FRESHNESS.stale:
    default:
      return ['#1f6ae5', '#0b3f97']
  }
}

function scalePoint(value: [number, number], scale: number): [number, number] {
  const [x, y] = value

  return [Math.round(x * scale), Math.round(y * scale)]
}
