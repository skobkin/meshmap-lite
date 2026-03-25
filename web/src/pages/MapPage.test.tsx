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
        onOpenNodeDetails={onOpenNodeDetails}
        onViewChange={() => undefined}
      />
    )

    const resolvedButton = screen.getByRole('button', { name: 'Field Router' })
    expect(resolvedButton.getAttribute('title')).toBe('!alpha')
    expect(screen.getByRole('button', { name: 'Payload Bravo' }).getAttribute('title')).toBe('!bravo')
    expect(screen.getByText('system')).toBeTruthy()

    await user.click(resolvedButton)
    await user.click(screen.getByRole('button', { name: 'Payload Bravo' }))

    expect(onOpenNodeDetails).toHaveBeenNthCalledWith(1, '!alpha')
    expect(onOpenNodeDetails).toHaveBeenNthCalledWith(2, '!bravo')
  })
})
