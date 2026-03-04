import type { ChannelItem, ChatEvent, LogEvent, MapNode, Meta, NodeDetails, NodeSummary } from './types'

interface RequestOptions {
  signal?: AbortSignal
}

async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  const r = await fetch(path, { signal: options?.signal })
  if (!r.ok) {
    throw new Error(`request failed: ${r.status}`)
  }
  return await r.json() as T
}

export const api = {
  meta: (options?: RequestOptions) => request<Meta>('/api/v1/meta', options),
  channels: (options?: RequestOptions) => request<ChannelItem[]>('/api/v1/channels', options),
  mapNodes: (options?: RequestOptions) => request<MapNode[]>('/api/v1/map/nodes', options),
  chatMessages: (channel: string, limit: number, options?: RequestOptions) => request<ChatEvent[]>(`/api/v1/chat/messages?channel=${encodeURIComponent(channel)}&limit=${limit}`, options),
  logEvents: (params: { limit?: number; before?: number; eventKinds?: number[]; channel?: string }, options?: RequestOptions) => {
    const q = new URLSearchParams()
    if (params.limit && params.limit > 0) q.set('limit', String(params.limit))
    if (params.before && params.before > 0) q.set('before', String(params.before))
    if (params.channel) q.set('channel', params.channel)
    for (const kind of params.eventKinds ?? []) {
      q.append('event_kind', String(kind))
    }
    const suffix = q.toString()
    return request<LogEvent[]>(`/api/v1/log/events${suffix ? `?${suffix}` : ''}`, options)
  },
  nodes: (options?: RequestOptions) => request<NodeSummary[]>('/api/v1/nodes', options),
  node: (id: string, options?: RequestOptions) => request<NodeDetails>(`/api/v1/nodes/${encodeURIComponent(id)}`, options)
}
