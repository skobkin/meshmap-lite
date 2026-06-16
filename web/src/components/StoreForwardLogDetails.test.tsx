// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { storeForwardLogDetailsRenderer } from './StoreForwardLogDetails'

import type { LogEvent, MapNode } from '../api/types'

const { useNodeStore, resetNodeStore } = vi.hoisted(() => {
  const initialState = {
    mapNodes: [] as MapNode[],
    summaries: [] as never[],
    details: undefined
  }
  let state = initialState
  const store = ((selector?: (value: typeof state) => unknown) => (
    selector ? selector(state) : state
  )) as ((selector?: (value: typeof state) => unknown) => unknown) & {
    setState: (partial: Partial<typeof state>) => void
  }
  store.setState = (partial) => {
    state = { ...state, ...partial }
  }

  return {
    useNodeStore: store,
    resetNodeStore: () => {
      state = {
        mapNodes: [] as MapNode[],
        summaries: [] as never[],
        details: undefined
      }
    }
  }
})

vi.mock('../stores/nodes', () => ({
  useNodeStore
}))

function mapNode(nodeId: string, longName: string): MapNode {
  return {
    node: {
      node_id: nodeId,
      long_name: longName,
      last_seen_any_event_at: '2026-06-17T10:00:00Z'
    }
  }
}

function event(details: Record<string, unknown>): LogEvent {
  return {
    id: 42,
    observed_at: '2026-06-17T10:00:00Z',
    event_kind_value: 12,
    event_kind_title: 'S&F',
    encrypted: false,
    details
  }
}

function rowForLabel(container: Element, label: string): HTMLElement {
  const dt = Array.from(container.querySelectorAll('.log-details-label'))
    .find((el) => el.textContent === label)
  if (!dt) {
    throw new Error(`row label not found: ${label}`)
  }
  const dd = dt.nextElementSibling
  if (!(dd instanceof HTMLElement)) {
    throw new Error(`value cell not found for label: ${label}`)
  }

  return dd
}

describe('storeForwardLogDetailsRenderer', () => {
  afterEach(() => {
    resetNodeStore()
  })

  it('renders "broadcast" instead of a node link when to is the broadcast node id', () => {
    useNodeStore.setState({
      mapNodes: [mapNode('!aabbccdd', 'Field Router')]
    })

    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 'ROUTER_STATS',
          role: 'router',
          from: '!aabbccdd',
          to: '!ffffffff',
          stats: {
            messages_total: 1
          }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    const toCell = rowForLabel(container, 'To')
    expect(toCell.textContent).toBe('broadcast')
    expect(toCell.querySelector('.log-details-node-link')).toBeNull()
    expect(toCell.querySelector('button')).toBeNull()
  })

  it('keeps the from row as a resolvable node link when to is the broadcast node id', () => {
    useNodeStore.setState({
      mapNodes: [mapNode('!aabbccdd', 'Field Router')]
    })

    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 'ROUTER_STATS',
          role: 'router',
          from: '!aabbccdd',
          to: '!ffffffff',
          stats: {
            messages_total: 1
          }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    const fromCell = rowForLabel(container, 'From')
    expect(fromCell.querySelector('.log-details-node-link')).not.toBeNull()
  })

  it('treats a case-insensitive broadcast node id as broadcast', () => {
    useNodeStore.setState({
      mapNodes: [mapNode('!aabbccdd', 'Field Router')]
    })

    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 'ROUTER_STATS',
          role: 'router',
          from: '!aabbccdd',
          to: '!FFFFffff',
          stats: {
            messages_total: 1
          }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    const toCell = rowForLabel(container, 'To')
    expect(toCell.textContent).toBe('broadcast')
    expect(toCell.querySelector('button')).toBeNull()
  })

  it('renders a node link when to is a real (non-broadcast) node id', () => {
    useNodeStore.setState({
      mapNodes: [mapNode('!11223344', 'Other Router')]
    })

    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 'ROUTER_STATS',
          role: 'router',
          from: '!aabbccdd',
          to: '!11223344',
          stats: {
            messages_total: 1
          }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    const toCell = rowForLabel(container, 'To')
    expect(toCell.textContent).not.toBe('broadcast')
    expect(toCell.querySelector('.log-details-node-link')).not.toBeNull()
  })

  it('surfaces the broadcast label within the rendered document', () => {
    useNodeStore.setState({
      mapNodes: [mapNode('!aabbccdd', 'Field Router')]
    })

    render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 'ROUTER_STATS',
          role: 'router',
          from: '!aabbccdd',
          to: '!ffffffff',
          stats: {
            messages_total: 1
          }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    )

    expect(screen.getByText('broadcast')).toBeTruthy()
  })
})
