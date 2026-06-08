// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { LogEventList } from './LogEventList'

import type { LogEvent, MapNode, NodeDetails, NodeSummary } from '../api/types'

interface NodeStoreState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
}

const { useNodeStore } = vi.hoisted(() => {
  let state: NodeStoreState = {
    mapNodes: [],
    summaries: [],
    selectedId: undefined,
    details: undefined
  }
  const store = ((selector?: (value: NodeStoreState) => unknown) => (
    selector ? selector(state) : state
  )) as ((selector?: (value: NodeStoreState) => unknown) => unknown) & {
    setState: (partial: Partial<NodeStoreState>) => void
  }
  store.setState = (partial) => {
    state = { ...state, ...partial }
  }

  return {
    useNodeStore: store
  }
})

vi.mock('../stores/nodes', () => ({
  useNodeStore
}))

function mockViewport(matches: boolean): void {
  const listeners = new Set<(event: MediaQueryListEvent) => void>()

  vi.stubGlobal('window', {
    ...window,
    matchMedia: vi.fn().mockImplementation(() => ({
      matches,
      media: '(max-width: 768px)',
      onchange: null,
      addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
        listeners.add(listener)
      },
      removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener)
      },
      addListener: (listener: (event: MediaQueryListEvent) => void) => {
        listeners.add(listener)
      },
      removeListener: (listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener)
      },
      dispatchEvent: vi.fn()
    }))
  })
}

function makeEvent(
  id: number,
  hopStart: number,
  hopLimit: number,
  options: { mqttUploaderNodeId?: string; nodeId?: string } = {}
): LogEvent {
  return {
    id,
    observed_at: '2026-03-11T12:00:00Z',
    node_id: options.nodeId ?? '!abc',
    node_display_name: 'Alpha',
    event_kind_value: 1,
    event_kind_title: 'Telemetry',
    encrypted: false,
    channel_name: 'mesh',
    details: {},
    hop_start: hopStart,
    hop_limit: hopLimit,
    mqtt_uploader_node_id: options.mqttUploaderNodeId
  }
}

function renderList(items: LogEvent[]): ReturnType<typeof render> {
  return render(
    <LogEventList
      items={items}
      showNodeColumn={true}
      onOpenNodeDetails={() => undefined}
    />
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('LogEventList mobile hop badge', () => {
  it('applies the log-hop-badge class and signal-quality class to mobile cards', () => {
    mockViewport(true)

    renderList([
      makeEvent(1, 5, 4),   // 1 hop traversed -> signal-good
      makeEvent(2, 5, 2),   // 3 hops traversed -> signal-warn
      makeEvent(3, 5, 0)    // 5 hops traversed, budget exhausted -> signal-bad + signal-exhausted
    ])

    const good = screen.getByTitle('Hops traversed: 1')
    expect(good.textContent).toBe('↓1')
    expect(good.className).toContain('log-hop-badge')
    expect(good.className).toContain('signal-good')

    const warn = screen.getByTitle('Hops traversed: 3')
    expect(warn.textContent).toBe('↓3')
    expect(warn.className).toContain('log-hop-badge')
    expect(warn.className).toContain('signal-warn')

    const exhausted = screen.getByTitle('Hops traversed: 5 (hop budget exhausted)')
    expect(exhausted.textContent).toBe('↓5')
    expect(exhausted.className).toContain('log-hop-badge')
    expect(exhausted.className).toContain('signal-bad')
    expect(exhausted.className).toContain('signal-exhausted')
  })

  it('omits the badge entirely for events with no hop metadata', () => {
    mockViewport(true)

    renderList([makeEvent(1, 5, 4), makeEvent(2, 0, 0)])

    // Event 1 has a badge.
    expect(screen.getByTitle('Hops traversed: 1')).toBeTruthy()

    // Event 2 has no hop metadata at all — only one badge should render
    // (for event 1). The Hops row for event 2 should not exist.
    const badges = screen.getAllByText(/↓\d/)
    expect(badges).toHaveLength(1)
  })

  it('renders a neutral ↓0 badge on mobile cards for direct-to-neighbor-gateway packets', () => {
    mockViewport(true)

    // hop_start == hop_limit === 7, with a different node_id and
    // mqtt_uploader_node_id: the originator is NOT the uploader, so the
    // packet was sent by a different node whose direct LoRa neighbour
    // is the gateway. The log view shows a ↓0 badge with the bare
    // .log-hop-badge class (no signal-* modifier).
    renderList([
      makeEvent(1, 7, 7, { nodeId: '!sender', mqttUploaderNodeId: '!gateway' })
    ])

    const direct = screen.getByTitle('Hops traversed: 0 (direct transmission to uploader)')
    expect(direct.textContent).toBe('↓0')
    expect(direct.className).toContain('log-hop-badge')
    expect(direct.className).not.toContain('signal-good')
    expect(direct.className).not.toContain('signal-warn')
    expect(direct.className).not.toContain('signal-bad')
  })

  it('omits the badge when originator and uploader are the same node (self-upload)', () => {
    mockViewport(true)

    // hop_start == hop_limit === 7, with node_id === mqtt_uploader_node_id:
    // the packet was generated by the gateway itself and uploaded over
    // MQTT without any LoRa hop at all. The hop header in that case is
    // structurally meaningless — we don't surface a ↓0 badge.
    renderList([
      makeEvent(1, 7, 7, { nodeId: '!gateway', mqttUploaderNodeId: '!gateway' })
    ])

    expect(screen.queryByText(/↓\d/)).toBeNull()
    expect(screen.queryByTitle('Hops traversed: 0 (direct transmission to uploader)')).toBeNull()
    // The Hops row should not exist at all on the mobile card.
    expect(screen.queryByText('Hops')).toBeNull()
  })
})
