import type { ChannelItem, ChatEvent, LogEvent, MapNode, Meta, NodeDetails, NodeSummary } from './types'

async function request<T>(path: string): Promise<T> {
  const r = await fetch(path)
  if (!r.ok) {
    throw new Error(`request failed: ${r.status}`)
  }
  return await r.json() as T
}

export const api = {
  meta: () => request<Meta>('/api/v1/meta'),
  channels: () => request<ChannelItem[]>('/api/v1/channels'),
  mapNodes: () => request<MapNode[]>('/api/v1/map/nodes'),
  chatMessages: (channel: string, limit: number) => request<ChatEvent[]>(`/api/v1/chat/messages?channel=${encodeURIComponent(channel)}&limit=${limit}`),
  logEvents: (params: { limit?: number; before?: number; eventKinds?: number[]; channel?: string }) => {
    const q = new URLSearchParams()
    if (params.limit && params.limit > 0) q.set('limit', String(params.limit))
    if (params.before && params.before > 0) q.set('before', String(params.before))
    if (params.channel) q.set('channel', params.channel)
    for (const kind of params.eventKinds ?? []) {
      q.append('event_kind', String(kind))
    }
    const suffix = q.toString()
    return request<LogEvent[]>(`/api/v1/log/events${suffix ? `?${suffix}` : ''}`)
  },
  nodes: () => request<NodeSummary[]>('/api/v1/nodes'),
  node: (id: string) => request<NodeDetails>(`/api/v1/nodes/${encodeURIComponent(id)}`)
}
