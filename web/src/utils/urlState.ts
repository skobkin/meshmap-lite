import { defaultLogHopRange, isDefaultLogHopRange, normalizeLogHopRange } from './logHops'

export type FragmentPage = 'map' | 'nodes' | 'stats' | 'log'

export interface MapViewState {
  center: [number, number]
  zoom: number
}

export interface FragmentState {
  page: FragmentPage
  infoRequested?: boolean
  updatesRequestedSource?: string
  map?: {
    view?: MapViewState
    node?: string
    chatChannel?: string
    chatPanel?: 'open' | 'collapsed'
  }
  nodes?: {
    node?: string
    q?: string
  }
  log?: {
    eventKinds: number[]
    channel: string
    nodeID: string
    hopRange: {
      min: number
      max: number
    }
    eventID?: number
  }
}

const pages = new Set<FragmentPage>(['map', 'nodes', 'stats', 'log'])

function parseURLNumber(raw: string | null): number | undefined {
  if (raw === null) {return undefined}
  const n = Number(raw)
  if (!Number.isFinite(n)) {return undefined}

  return n
}

function parseMapView(params: URLSearchParams): MapViewState | undefined {
  const lat = parseURLNumber(params.get('lat'))
  const lng = parseURLNumber(params.get('lng'))
  const zoom = parseURLNumber(params.get('z'))
  if (lat === undefined || lng === undefined || zoom === undefined) {return undefined}
  if (lat < -90 || lat > 90) {return undefined}
  if (lng < -180 || lng > 180) {return undefined}
  if (zoom < 0 || zoom > 24) {return undefined}

  return { center: [lat, lng], zoom }
}

function nonEmpty(value: string | null): string | undefined {
  return value && value.length > 0 ? value : undefined
}

function parsePage(raw: string): {
  page: FragmentPage
  params: URLSearchParams
  routed: boolean
  infoRequested: boolean
  updatesRequestedSource: string
} {
  const trimmed = raw.startsWith('#') ? raw.slice(1) : raw
  if (!trimmed.startsWith('/')) {
    return { page: 'map', params: new URLSearchParams(), routed: false, infoRequested: false, updatesRequestedSource: '' }
  }
  const queryStart = trimmed.indexOf('?')
  const path = queryStart >= 0 ? trimmed.slice(1, queryStart) : trimmed.slice(1)
  const query = queryStart >= 0 ? trimmed.slice(queryStart + 1) : ''
  if (path === 'info') {
    return { page: 'map', params: new URLSearchParams(query), routed: true, infoRequested: true, updatesRequestedSource: '' }
  }
  if (path.startsWith('updates/')) {
    const source = path.slice('updates/'.length).trim()

    return {
      page: 'map',
      params: new URLSearchParams(query),
      routed: true,
      infoRequested: false,
      updatesRequestedSource: source
    }
  }
  const page = pages.has(path as FragmentPage) ? path as FragmentPage : 'map'

  return { page, params: new URLSearchParams(query), routed: true, infoRequested: false, updatesRequestedSource: '' }
}

export function parseFragmentState(hash: string): FragmentState {
  const { page, params, routed, infoRequested, updatesRequestedSource } = parsePage(hash)
  const state: FragmentState = {
    page,
    infoRequested: infoRequested || undefined,
    updatesRequestedSource: updatesRequestedSource || undefined
  }
  if (infoRequested) {return state}
  if (updatesRequestedSource) {return state}
  if (!routed) {return state}

  if (page === 'map') {
    const panel = params.get('chat_panel')
    state.map = {
      view: parseMapView(params),
      node: nonEmpty(params.get('node')),
      chatChannel: nonEmpty(params.get('chat')),
      chatPanel: panel === 'open' || panel === 'collapsed' ? panel : undefined
    }
  }

  if (page === 'nodes') {
    state.nodes = {
      node: nonEmpty(params.get('node')),
      q: nonEmpty(params.get('q'))
    }
  }

  if (page === 'log') {
    const eventID = parseURLNumber(params.get('event_id'))
    const hopsMin = parseURLNumber(params.get('hops_min'))
    const hopsMax = parseURLNumber(params.get('hops_max'))
    state.log = {
      eventKinds: params.getAll('event_kind')
        .map((value) => Number(value))
        .filter((value) => Number.isInteger(value) && value > 0),
      channel: params.get('channel') ?? '',
      nodeID: params.get('node_id') ?? '',
      hopRange: normalizeLogHopRange({
        min: Number.isInteger(hopsMin) ? hopsMin : defaultLogHopRange.min,
        max: Number.isInteger(hopsMax) ? hopsMax : defaultLogHopRange.max
      })
    }
    if (eventID && Number.isInteger(eventID) && eventID > 0) {
      state.log.eventID = eventID
    }
  }

  return state
}

function setMapView(params: URLSearchParams, view?: MapViewState): void {
  if (!view) {return}
  params.set('lat', Number(view.center[0].toFixed(6)).toString())
  params.set('lng', Number(view.center[1].toFixed(6)).toString())
  params.set('z', Number(view.zoom.toFixed(2)).toString())
}

export function serializeFragmentState(state: FragmentState): string {
  if (state.infoRequested) {
    return '#/info'
  }
  if (state.updatesRequestedSource) {
    return `#/updates/${encodeURIComponent(state.updatesRequestedSource)}`
  }

  const params = new URLSearchParams()

  switch (state.page) {
    case 'map':
      setMapView(params, state.map?.view)
      if (state.map?.node) {params.set('node', state.map.node)}
      if (state.map?.chatChannel) {params.set('chat', state.map.chatChannel)}
      if (state.map?.chatPanel) {params.set('chat_panel', state.map.chatPanel)}
      break
    case 'nodes':
      if (state.nodes?.node) {params.set('node', state.nodes.node)}
      if (state.nodes?.q) {params.set('q', state.nodes.q)}
      break
    case 'log':
      for (const kind of state.log?.eventKinds ?? []) {
        params.append('event_kind', String(kind))
      }
      if (state.log?.channel) {params.set('channel', state.log.channel)}
      if (state.log?.nodeID) {params.set('node_id', state.log.nodeID)}
      if (state.log?.hopRange && !isDefaultLogHopRange(state.log.hopRange)) {
        params.set('hops_min', String(state.log.hopRange.min))
        params.set('hops_max', String(state.log.hopRange.max))
      }
      if (state.log?.eventID) {params.set('event_id', String(state.log.eventID))}
      break
    case 'stats':
      break
  }

  const query = params.toString()

  return `#/${state.page}${query ? `?${query}` : ''}`
}
