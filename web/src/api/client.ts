import type { ChannelItem, ChatEvent, MapNode, Meta, NodeDetails, NodeSummary } from './types'

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
  nodes: () => request<NodeSummary[]>('/api/v1/nodes'),
  node: (id: string) => request<NodeDetails>(`/api/v1/nodes/${encodeURIComponent(id)}`)
}
