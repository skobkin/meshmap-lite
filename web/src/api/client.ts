import type { ActivityStats, ChannelItem, ChatEvent, FirmwareHistory, FirmwareSnapshot, HardwareHistory, HardwareSnapshot, InfoFormat, InfoResponse, LogEvent, MapNode, Meta, NodeDetails, NodeSummary, TopologyEdge, TopologyEdgesResponse, UpdatesResponse } from './types'

interface RequestOptions {
  signal?: AbortSignal
}

export class APIError extends Error {
  public readonly status: number

  public constructor(status: number) {
    super(`request failed: ${status}`)
    this.name = 'APIError'
    this.status = status
  }
}

async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  const r = await fetch(path, { signal: options?.signal })
  if (!r.ok) {
    throw new APIError(r.status)
  }

  return await r.json() as T
}

export const api = {
  meta: (options?: RequestOptions) => request<Meta>('/api/v1/meta', options),
  info: (format: InfoFormat = 'html', options?: RequestOptions) => request<InfoResponse>(`/api/v1/info?format=${encodeURIComponent(format)}`, options),
  updates: (source: string, format: 'html' | 'markdown' = 'html', options?: RequestOptions) => {
    const q = new URLSearchParams()
    q.set('source', source)
    q.set('format', format)

    return request<UpdatesResponse>(`/api/v1/updates?${q.toString()}`, options)
  },
  channels: (options?: RequestOptions) => request<ChannelItem[]>('/api/v1/channels', options),
  mapNodes: (options?: RequestOptions) => request<MapNode[]>('/api/v1/map/nodes', options),
  chatMessages: (params: { channel: string; limit?: number; before?: number }, options?: RequestOptions) => {
    const q = new URLSearchParams()
    if (params.channel) {q.set('channel', params.channel)}
    if (params.limit && params.limit > 0) {q.set('limit', String(params.limit))}
    if (params.before && params.before > 0) {q.set('before', String(params.before))}
    const suffix = q.toString()

    return request<ChatEvent[]>(`/api/v1/chat/messages${suffix ? `?${suffix}` : ''}`, options)
  },
  logEvents: (params: { limit?: number; before?: number; eventKinds?: number[]; channel?: string; nodeID?: string; hopsMin?: number; hopsMax?: number }, options?: RequestOptions) => {
    const q = new URLSearchParams()
    if (params.limit && params.limit > 0) {q.set('limit', String(params.limit))}
    if (params.before && params.before > 0) {q.set('before', String(params.before))}
    if (params.channel) {q.set('channel', params.channel)}
    if (params.nodeID) {q.set('node_id', params.nodeID)}
    if (typeof params.hopsMin === 'number') {q.set('hops_min', String(params.hopsMin))}
    if (typeof params.hopsMax === 'number') {q.set('hops_max', String(params.hopsMax))}
    for (const kind of params.eventKinds ?? []) {
      q.append('event_kind', String(kind))
    }
    const suffix = q.toString()

    return request<LogEvent[]>(`/api/v1/log/events${suffix ? `?${suffix}` : ''}`, options)
  },
  topologyEdges: (params: { nodeID?: string; channel?: string; sourceKinds?: TopologyEdge['source_kind'][] }, options?: RequestOptions) => {
    const q = new URLSearchParams()
    if (params.nodeID) {q.set('node_id', params.nodeID)}
    if (params.channel) {q.set('channel', params.channel)}
    for (const kind of params.sourceKinds ?? []) {
      q.append('source_kind', kind)
    }
    const suffix = q.toString()

    return request<TopologyEdgesResponse>(`/api/v1/topology/edges${suffix ? `?${suffix}` : ''}`, options)
  },
  statsActivity: (options?: RequestOptions) => request<ActivityStats>('/api/v1/stats/activity', options),
  firmwareSnapshot: (options?: RequestOptions) => request<FirmwareSnapshot>('/api/v1/stats/firmware', options),
  // The server controls the window shape via web.stats.software.history_weeks
  // and web.stats.software.top_versions; the endpoint does not accept query
  // parameters. Keeping this client signature parameterless matches that
  // contract and avoids spreading config-derived values through the API.
  firmwareHistory: (options?: RequestOptions) => request<FirmwareHistory>('/api/v1/stats/firmware/history', options),
  // See firmwareSnapshot/firmwareHistory: the window shape is server-controlled
  // via web.stats.hardware.history_weeks and web.stats.hardware.top_models; no
  // query parameters.
  hardwareSnapshot: (options?: RequestOptions) => request<HardwareSnapshot>('/api/v1/stats/hardware', options),
  hardwareHistory: (options?: RequestOptions) => request<HardwareHistory>('/api/v1/stats/hardware/history', options),
  nodes: (options?: RequestOptions) => request<NodeSummary[]>('/api/v1/nodes', options),
  node: (id: string, options?: RequestOptions) => request<NodeDetails>(`/api/v1/nodes/${encodeURIComponent(id)}`, options)
}
