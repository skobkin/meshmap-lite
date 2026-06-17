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

  it('renders the integer rr as a labelled enum code and derives the role', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 7,
          from: '!aabbccdd',
          stats: {
            messages_total: 1
          }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    // ROUTER_STATS = 7.
    expect(rowForLabel(container, 'Request/Response').textContent).toBe('ROUTER_STATS')
    // Role is derived from rr (7 < 64 → "router"), not stored.
    expect(rowForLabel(container, 'Role').textContent).toBe('router')
  })

  it('derives client role for rr values >= 64', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 65,
          history: { history_messages: 3 }
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    expect(rowForLabel(container, 'Request/Response').textContent).toBe('CLIENT_HISTORY')
    expect(rowForLabel(container, 'Role').textContent).toBe('client')
  })

  it('falls back to a "RR N" label for unknown rr values', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 200
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    expect(rowForLabel(container, 'Request/Response').textContent).toBe('RR 200')
  })

  it('renders the human-readable label for ROUTER_HEARTBEAT (rr=2) and does not duplicate the raw value', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 2
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    // ROUTER_HEARTBEAT = 2. The custom view should show the label,
    // not the raw enum code — operators do not have the proto enum
    // table in their head.
    expect(rowForLabel(container, 'Request/Response').textContent).toBe('ROUTER_HEARTBEAT')

    // Only one "Request/Response" row should be rendered; the raw
    // integer is hidden in the custom view (it stays available in
    // the Raw tab).
    const requestResponseRows = Array.from(container.querySelectorAll('.log-details-label'))
      .filter((el) => el.textContent === 'Request/Response')
    expect(requestResponseRows.length).toBe(1)
  })

  it('renders the text length and skips the body when text_bytes is present', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 8,
          text_bytes: 5
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    // 5 bytes plural.
    expect(container.textContent).toContain('5 bytes')
    expect(container.textContent).toContain('body not stored')
    // No replayed-text pre block should be present.
    expect(container.querySelector('.store-forward-text-block')?.tagName).toBe('P')
  })

  it('uses singular "byte" for text_bytes of 1', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 8,
          text_bytes: 1
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    expect(container.textContent).toContain('1 byte (body not stored)')
    expect(container.textContent).not.toContain('1 bytes')
  })

  it('renders "UNKNOWN" for the RR sentinel and surfaces the raw value', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: -1,
          raw_rr: 'ROUTER_PING_PONG_2027'
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    expect(rowForLabel(container, 'Request/Response').textContent).toBe('UNKNOWN')
    expect(rowForLabel(container, 'Role').textContent).toBe('unknown')
    expect(rowForLabel(container, 'Request/Response (raw)').textContent).toBe('ROUTER_PING_PONG_2027')
  })

  it('surfaces raw_role when the wire role is not a known value', () => {
    const container = render(
      storeForwardLogDetailsRenderer.render(
        event({
          rr: 7,
          raw_role: 'repeater'
        }),
        { onOpenNodeDetails: () => undefined }
      )
    ).container

    expect(rowForLabel(container, 'Role').textContent).toBe('router')
    expect(rowForLabel(container, 'Role (raw)').textContent).toBe('repeater')
  })
})
