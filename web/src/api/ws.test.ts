import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ChatEvent, LogEvent, Meta, Node, NodePosition, WSState, WSStats } from './types'

interface ChatStoreState {
  messages: ChatEvent[]
  pushMessage: (item: ChatEvent) => void
}

interface LogStoreState {
  items: LogEvent[]
  prependLive: (item: LogEvent) => void
}

interface MetaStoreState {
  meta?: Meta
}

interface NodeStoreState {
  nodes: Node[]
  positions: NodePosition[]
  upsertNode: (item: Node) => void
  upsertPosition: (item: NodePosition) => void
}

interface WSStoreState {
  state: WSState
  stats: WSStats | null
  setState: (state: WSState) => void
  setStats: (stats: WSStats) => void
}

let chatStore: ChatStoreState
let logStore: LogStoreState
let metaStore: MetaStoreState
let nodeStore: NodeStoreState
let wsStore: WSStoreState

vi.mock('../stores/chat', () => ({
  useChatStore: {
    getState: () => chatStore
  }
}))

vi.mock('../stores/log', () => ({
  useLogStore: {
    getState: () => logStore
  }
}))

vi.mock('../stores/meta', () => ({
  useMetaStore: {
    getState: () => metaStore
  }
}))

vi.mock('../stores/nodes', () => ({
  useNodeStore: {
    getState: () => nodeStore
  }
}))

vi.mock('../stores/ws', () => ({
  useWSStore: {
    getState: () => wsStore
  }
}))

class MockWebSocket {
  static instances: MockWebSocket[] = []

  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent<string>) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null

  constructor(readonly url: string) {
    MockWebSocket.instances.push(this)
  }

  emitOpen(): void {
    this.onopen?.()
  }

  emitMessage(payload: unknown): void {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent<string>)
  }

  emitClose(): void {
    this.onclose?.()
  }

  close(): void {
    this.emitClose()
  }

  static reset(): void {
    MockWebSocket.instances = []
  }
}

const { startWS } = await import('./ws')

describe('startWS', () => {
  beforeEach(() => {
    chatStore = {
      messages: [],
      pushMessage(item) {
        this.messages = [item, ...this.messages]
      }
    }
    logStore = {
      items: [],
      prependLive(item) {
        this.items = [item, ...this.items]
      }
    }
    metaStore = {}
    nodeStore = {
      nodes: [],
      positions: [],
      upsertNode(item) {
        this.nodes = [...this.nodes, item]
      },
      upsertPosition(item) {
        this.positions = [...this.positions, item]
      }
    }
    wsStore = {
      state: 'connecting',
      stats: null,
      setState(state) {
        this.state = state
      },
      setStats(stats) {
        this.stats = stats
      }
    }

    MockWebSocket.reset()
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  it('routes incoming events into the relevant stores', () => {
    const stop = startWS('/api/v1/ws')
    const socket = MockWebSocket.instances[0]

    socket?.emitOpen()
    socket?.emitMessage({
      type: 'chat.message',
      payload: {
        id: 7,
        event_type: 'message',
        observed_at: '2026-03-11T10:00:00Z'
      }
    })
    socket?.emitMessage({
      type: 'stats',
      payload: {
        known_nodes_count: 10,
        online_nodes_count: 4,
        ws_clients_count: 2
      }
    })
    socket?.emitMessage({
      type: 'node.upsert',
      payload: {
        node_id: '!abcd',
        long_name: 'Node Alpha',
        last_seen_any_event_at: '2026-03-11T10:00:00Z'
      }
    })
    socket?.emitMessage({
      type: 'node.position',
      payload: {
        node_id: '!abcd',
        latitude: 1,
        longitude: 2,
        source_kind: 'telemetry',
        observed_at: '2026-03-11T10:01:00Z'
      }
    })
    socket?.emitMessage({
      type: 'log.event',
      payload: {
        id: 9,
        observed_at: '2026-03-11T10:02:00Z',
        event_kind_value: 1,
        event_kind_title: 'message',
        encrypted: false
      }
    })

    expect(wsStore.state).toBe('connected')
    expect(chatStore.messages.map((item) => item.id)).toEqual([7])
    expect(wsStore.stats?.known_nodes_count).toBe(10)
    expect(nodeStore.nodes).toEqual([
      {
        node_id: '!abcd',
        long_name: 'Node Alpha',
        last_seen_any_event_at: '2026-03-11T10:00:00Z'
      }
    ])
    expect(nodeStore.positions).toEqual([
      {
        node_id: '!abcd',
        latitude: 1,
        longitude: 2,
        source_kind: 'telemetry',
        observed_at: '2026-03-11T10:01:00Z'
      }
    ])
    expect(logStore.items.map((item) => item.id)).toEqual([9])

    stop()
  })

  it('suppresses live log events when meta disables them', () => {
    metaStore.meta = {
      app_name: 'meshmap-lite',
      version: '0.1.0',
      websocket_path: '/api/v1/ws',
      default_chat_channel: 'mesh',
      show_recent_messages: 10,
      log_live_updates: false,
      log_page_size_default: 50,
      disconnected_threshold: '10m',
      map: {
        clustering: true,
        hide_position_after: '30m',
        precision_circles_mode: 'selected',
        default_view: {
          latitude: 0,
          longitude: 0,
          zoom: 2
        }
      }
    }

    const stop = startWS('/api/v1/ws')
    MockWebSocket.instances[0]?.emitMessage({
      type: 'log.event',
      payload: {
        id: 9,
        observed_at: '2026-03-11T10:02:00Z',
        event_kind_value: 1,
        event_kind_title: 'message',
        encrypted: false
      }
    })

    expect(logStore.items).toEqual([])

    stop()
  })

  it('reconnects with exponential backoff and eventually gives up', () => {
    startWS('/api/v1/ws')

    for (let retry = 1; retry <= 10; retry++) {
      MockWebSocket.instances.at(-1)?.emitClose()
      expect(wsStore.state).toBe('reconnecting')

      const expectedDelay = Math.min(5000 * Math.pow(2, retry - 1), 300000)
      vi.advanceTimersByTime(expectedDelay)
      expect(MockWebSocket.instances).toHaveLength(retry + 1)
    }

    MockWebSocket.instances.at(-1)?.emitClose()

    expect(wsStore.state).toBe('disconnected')
  })
})
