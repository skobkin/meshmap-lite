// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { MapPage } from './MapPage'

import type { ChatEvent, MapNode, NodeDetails, NodeSummary } from '../api/types'

interface NodeStoreState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
  setSelectedId: (id?: string) => void
}

interface ChatStoreState {
  messages: ChatEvent[]
  channel: string
  setChannel: (channel: string) => void
}

const { useNodeStore, useChatStore, resetStores } = vi.hoisted(() => {
  let nodeState: NodeStoreState = {
    mapNodes: [],
    summaries: [],
    selectedId: undefined,
    details: undefined,
    setSelectedId: () => undefined
  }
  let chatState: ChatStoreState = {
    messages: [],
    channel: 'mesh',
    setChannel: () => undefined
  }

  const nodeStore = ((selector?: (value: NodeStoreState) => unknown) => (
    selector ? selector(nodeState) : nodeState
  )) as ((selector?: (value: NodeStoreState) => unknown) => unknown) & {
    setState: (partial: Partial<NodeStoreState>) => void
  }
  nodeStore.setState = (partial) => {
    nodeState = { ...nodeState, ...partial }
  }

  const chatStore = ((selector?: (value: ChatStoreState) => unknown) => (
    selector ? selector(chatState) : chatState
  )) as ((selector?: (value: ChatStoreState) => unknown) => unknown) & {
    setState: (partial: Partial<ChatStoreState>) => void
  }
  chatStore.setState = (partial) => {
    chatState = { ...chatState, ...partial }
  }

  return {
    useNodeStore: nodeStore,
    useChatStore: chatStore,
    resetStores: () => {
      nodeState = {
        mapNodes: [],
        summaries: [],
        selectedId: undefined,
        details: undefined,
        setSelectedId: () => undefined
      }
      chatState = {
        messages: [],
        channel: 'mesh',
        setChannel: () => undefined
      }
    }
  }
})

vi.mock('../stores/nodes', () => ({
  useNodeStore
}))

vi.mock('../stores/chat', () => ({
  useChatStore
}))

vi.mock('../maps/leafletMap', () => ({
  LeafletMapAdapter: class {
    public destroy(): void {
      return undefined
    }

    public focusNode(): void {
      return undefined
    }

    public render(): void {
      return undefined
    }

    public renderTopology(): void {
      return undefined
    }

    public setSelectedNode(): void {
      return undefined
    }

    public setView(): void {
      return undefined
    }
  }
}))

describe('MapPage chat timeline', () => {
  afterEach(() => {
    resetStores()
    localStorage.clear()
  })

  it('resolves chat node labels from the shared node store and falls back to payload labels', async () => {
    useNodeStore.setState({
      mapNodes: [
        {
          node: {
            node_id: '!alpha',
            long_name: 'Field Router',
            last_seen_any_event_at: '2026-03-11T17:58:00Z'
          }
        }
      ]
    })
    useChatStore.setState({
      messages: [
        {
          id: 7,
          event_type: 'message',
          node_id: '!alpha',
          node_display_name: 'Payload Alpha',
          message_text: 'Hello',
          observed_at: '2026-03-11T17:58:00Z'
        },
        {
          id: 8,
          event_type: 'message',
          node_id: '!bravo',
          node_display_name: 'Payload Bravo',
          mqtt_uploader_node_id: '!gateway',
          mqtt_uploader_display_name: 'Gateway',
          message_text: 'World',
          observed_at: '2026-03-11T18:00:00Z'
        },
        {
          id: 9,
          event_type: 'system',
          system_code: 'node_discovered',
          observed_at: '2026-03-11T18:01:00Z'
        }
      ]
    })

    const user = userEvent.setup()
    const onOpenNodeDetails = vi.fn()

    render(
      <MapPage
        center={[0, 0]}
        zoom={7}
        clustering={true}
        precisionCirclesMode="selected"
        channels={['mesh']}
        onFocusNodeHandled={() => undefined}
        onHoverTopologyNode={() => undefined}
        onLoadMoreChat={() => undefined}
        onOpenNodeDetails={onOpenNodeDetails}
        onViewChange={() => undefined}
        chatHasMore={false}
        chatLoadingMore={false}
        chatLoadMoreError=""
      />
    )

    const resolvedButton = screen.getByRole('button', { name: 'Field Router' })
    expect(resolvedButton.getAttribute('title')).toBe('!alpha')
    expect(screen.getByRole('button', { name: 'Payload Bravo' }).getAttribute('title')).toBe('!bravo')
    expect(screen.queryByText('via')).toBeNull()
    const gatewayButton = screen.getByRole('button', { name: 'Gateway: Gateway (!gateway)' })
    expect(gatewayButton.getAttribute('title')).toBe('Gateway: Gateway (!gateway)')
    expect(screen.getByText('system')).toBeTruthy()

    await user.click(resolvedButton)
    await user.click(screen.getByRole('button', { name: 'Payload Bravo' }))
    await user.click(gatewayButton)

    expect(onOpenNodeDetails).toHaveBeenNthCalledWith(1, '!alpha')
    expect(onOpenNodeDetails).toHaveBeenNthCalledWith(2, '!bravo')
    expect(onOpenNodeDetails).toHaveBeenNthCalledWith(3, '!gateway')
  })

  it('renders chat pagination controls and calls the load handler', async () => {
    useChatStore.setState({
      messages: [
        {
          id: 7,
          event_type: 'message',
          message_text: 'Hello',
          observed_at: '2026-03-11T17:58:00Z'
        }
      ]
    })
    const user = userEvent.setup()
    const onLoadMoreChat = vi.fn()

    const { rerender } = render(
      <MapPage
        center={[0, 0]}
        zoom={7}
        clustering={true}
        precisionCirclesMode="selected"
        channels={['mesh']}
        onFocusNodeHandled={() => undefined}
        onHoverTopologyNode={() => undefined}
        onLoadMoreChat={onLoadMoreChat}
        onOpenNodeDetails={() => undefined}
        onViewChange={() => undefined}
        chatHasMore={true}
        chatLoadingMore={false}
        chatLoadMoreError=""
      />
    )

    await user.click(screen.getByRole('button', { name: 'Load more' }))
    expect(onLoadMoreChat).toHaveBeenCalledTimes(1)

    rerender(
      <MapPage
        center={[0, 0]}
        zoom={7}
        clustering={true}
        precisionCirclesMode="selected"
        channels={['mesh']}
        onFocusNodeHandled={() => undefined}
        onHoverTopologyNode={() => undefined}
        onLoadMoreChat={onLoadMoreChat}
        onOpenNodeDetails={() => undefined}
        onViewChange={() => undefined}
        chatHasMore={false}
        chatLoadingMore={false}
        chatLoadMoreError="Failed to load older chat messages."
      />
    )

    expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull()
    expect(screen.getByRole('alert').textContent).toContain('Failed to load older chat messages.')
  })

  it('renders MQTT direct topology legend entry', () => {
    render(
      <MapPage
        center={[0, 0]}
        zoom={7}
        clustering={true}
        precisionCirclesMode="selected"
        channels={['mesh']}
        topologyNodeId="!origin"
        topologyDetails={{
          node: { node_id: '!origin', last_seen_any_event_at: '2026-03-11T12:00:00Z' },
          neighbors: [{
            node_id: '!peer',
            display_name: 'Peer',
            has_position: true,
            evidence_kind: 'mqtt_direct',
            last_observed_at: '2026-03-11T12:00:00Z',
            updated_at: '2026-03-11T12:00:00Z'
          }]
        }}
        onFocusNodeHandled={() => undefined}
        onHoverTopologyNode={() => undefined}
        onLoadMoreChat={() => undefined}
        onOpenNodeDetails={() => undefined}
        onViewChange={() => undefined}
        chatHasMore={false}
        chatLoadingMore={false}
        chatLoadMoreError=""
      />
    )

    expect(screen.getByText('MQTT direct')).toBeTruthy()
  })

  it('renders hop badges with the correct signal-quality class', () => {
    useNodeStore.setState({
      mapNodes: [
        {
          node: {
            node_id: '!alpha',
            long_name: 'Alpha',
            last_seen_any_event_at: '2026-03-11T17:58:00Z'
          }
        }
      ]
    })
    useChatStore.setState({
      messages: [
        {
          id: 1,
          event_type: 'message',
          node_id: '!alpha',
          node_display_name: 'Alpha',
          message_text: 'close',
          observed_at: '2026-03-11T17:58:00Z',
          hop_start: 5,
          hop_limit: 4
        },
        {
          id: 2,
          event_type: 'message',
          node_id: '!alpha',
          node_display_name: 'Alpha',
          message_text: 'mid',
          observed_at: '2026-03-11T17:58:01Z',
          hop_start: 5,
          hop_limit: 2
        },
        {
          id: 3,
          event_type: 'message',
          node_id: '!alpha',
          node_display_name: 'Alpha',
          message_text: 'exhausted',
          observed_at: '2026-03-11T17:58:02Z',
          hop_start: 5,
          hop_limit: 0
        }
      ]
    })

    render(
      <MapPage
        center={[0, 0]}
        zoom={7}
        clustering={true}
        precisionCirclesMode="selected"
        channels={['mesh']}
        onFocusNodeHandled={() => undefined}
        onHoverTopologyNode={() => undefined}
        onLoadMoreChat={() => undefined}
        onOpenNodeDetails={() => undefined}
        onViewChange={() => undefined}
        chatHasMore={false}
        chatLoadingMore={false}
        chatLoadMoreError=""
      />
    )

    const goodBadge = screen.getByTitle('Hops traversed: 1')
    expect(goodBadge.textContent).toBe('↓1')
    expect(goodBadge.className).toContain('chat-hop-badge')
    expect(goodBadge.className).toContain('signal-good')

    const warnBadge = screen.getByTitle('Hops traversed: 3')
    expect(warnBadge.textContent).toBe('↓3')
    expect(warnBadge.className).toContain('signal-warn')

    const exhaustedBadge = screen.getByTitle('Hops traversed: 5 (hop budget exhausted)')
    expect(exhaustedBadge.textContent).toBe('↓5')
    expect(exhaustedBadge.className).toContain('signal-bad')
    expect(exhaustedBadge.className).toContain('signal-exhausted')
  })

  it('hides the hop badge in the chat panel for direct-to-uploader (↓0) packets', () => {
    useNodeStore.setState({
      mapNodes: [
        {
          node: {
            node_id: '!alpha',
            long_name: 'Alpha',
            last_seen_any_event_at: '2026-03-11T17:58:00Z'
          }
        }
      ]
    })
    useChatStore.setState({
      messages: [
        {
          id: 1,
          event_type: 'message',
          node_id: '!alpha',
          node_display_name: 'Alpha',
          message_text: 'direct',
          observed_at: '2026-03-11T17:58:00Z',
          hop_start: 7,
          hop_limit: 7
        }
      ]
    })

    render(
      <MapPage
        center={[0, 0]}
        zoom={7}
        clustering={true}
        precisionCirclesMode="selected"
        channels={['mesh']}
        onFocusNodeHandled={() => undefined}
        onHoverTopologyNode={() => undefined}
        onLoadMoreChat={() => undefined}
        onOpenNodeDetails={() => undefined}
        onViewChange={() => undefined}
        chatHasMore={false}
        chatLoadingMore={false}
        chatLoadMoreError=""
      />
    )

    // The classifier returns traversed: 0 for hop_start == hop_limit, but
    // the chat renderer explicitly hides the ↓0 case to keep the per-line
    // visual weight down. The log view does show it (see LogEventList tests).
    expect(screen.queryByText('↓0')).toBeNull()
    expect(screen.queryByTitle('Hops traversed: 0 (direct transmission to uploader)')).toBeNull()
  })
})
