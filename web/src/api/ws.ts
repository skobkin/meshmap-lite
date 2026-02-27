import { useChatStore } from '../stores/chat'
import { useLogStore } from '../stores/log'
import { useMetaStore } from '../stores/meta'
import { useNodeStore } from '../stores/nodes'
import { useWSStore } from '../stores/ws'
import type { LogEvent, Node, NodePosition, WSStats } from './types'

interface EventEnvelope {
  type: string
  payload: unknown
}

function isNodePayload(payload: unknown): payload is Node {
  if (!payload || typeof payload !== 'object') return false
  const node = payload as Record<string, unknown>
  return typeof node.node_id === 'string' && typeof node.last_seen_any_event_at === 'string'
}

function isNodePositionPayload(payload: unknown): payload is NodePosition {
  if (!payload || typeof payload !== 'object') return false
  const p = payload as Record<string, unknown>
  return typeof p.node_id === 'string' &&
    typeof p.latitude === 'number' &&
    typeof p.longitude === 'number' &&
    typeof p.source_kind === 'string' &&
    typeof p.observed_at === 'string'
}

function isStatsPayload(payload: unknown): payload is WSStats {
  if (!payload || typeof payload !== 'object') return false
  const stats = payload as Record<string, unknown>
  return typeof stats.known_nodes_count === 'number' &&
    typeof stats.online_nodes_count === 'number' &&
    typeof stats.ws_clients_count === 'number'
}

function isLogEventPayload(payload: unknown): payload is LogEvent {
  if (!payload || typeof payload !== 'object') return false
  const row = payload as Record<string, unknown>
  return typeof row.id === 'number' &&
    typeof row.observed_at === 'string' &&
    typeof row.event_kind_value === 'number' &&
    typeof row.event_kind_title === 'string' &&
    typeof row.encrypted === 'boolean'
}

export function startWS(path: string): () => void {
  let stop = false
  let retries = 0
  let timer = 0
  const maxRetries = 10

  const connect = (): void => {
    if (stop) return
    useWSStore.getState().setState(retries === 0 ? 'connecting' : 'reconnecting')
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}${path}`)

    ws.onopen = () => {
      retries = 0
      useWSStore.getState().setState('connected')
    }

    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data) as EventEnvelope
      if (msg.type === 'chat.message' || msg.type === 'chat.system') {
        useChatStore.getState().pushMessage(msg.payload as never)
        return
      }
      if (msg.type === 'stats' && isStatsPayload(msg.payload)) {
        useWSStore.getState().setStats(msg.payload)
        return
      }
      if (msg.type === 'node.upsert' && isNodePayload(msg.payload)) {
        useNodeStore.getState().upsertNode(msg.payload)
        return
      }
      if (msg.type === 'node.position' && isNodePositionPayload(msg.payload)) {
        useNodeStore.getState().upsertPosition(msg.payload)
        return
      }
      if (msg.type === 'log.event' && isLogEventPayload(msg.payload)) {
        if (useMetaStore.getState().meta?.log_live_updates ?? true) {
          useLogStore.getState().prependLive(msg.payload)
        }
      }
    }

    ws.onerror = () => {
      ws.close()
    }

    ws.onclose = () => {
      if (stop) return
      retries++
      if (retries > maxRetries) {
        useWSStore.getState().setState('disconnected')
        return
      }
      useWSStore.getState().setState('reconnecting')
      const delay = Math.min(5000 * Math.pow(2, retries - 1), 300000)
      timer = window.setTimeout(connect, delay)
    }
  }

  connect()
  return () => {
    stop = true
    if (timer) window.clearTimeout(timer)
  }
}
