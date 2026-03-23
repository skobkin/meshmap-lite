// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ResolvedNodeData, resolveNodeDataValue } from './ResolvedNodeData'

import type { MapNode, NodeDetails, NodeSummary } from '../api/types'

interface NodeStoreState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
}

const { useNodeStore, resetNodeStore } = vi.hoisted(() => {
  const initialState: NodeStoreState = {
    mapNodes: [],
    summaries: [],
    selectedId: undefined,
    details: undefined
  }
  let state: NodeStoreState = initialState
  const store = ((selector?: (value: NodeStoreState) => unknown) => (
    selector ? selector(state) : state
  )) as ((selector?: (value: NodeStoreState) => unknown) => unknown) & {
    setState: (partial: Partial<NodeStoreState>) => void
  }
  store.setState = (partial) => {
    state = { ...state, ...partial }
  }

  return {
    useNodeStore: store,
    resetNodeStore: () => {
      state = {
        mapNodes: [],
        summaries: [],
        selectedId: undefined,
        details: undefined
      }
    }
  }
})

vi.mock('../stores/nodes', () => ({
  useNodeStore
}))

const fixedTime = '2026-03-11T12:00:00Z'

function mapNode(nodeId: string, overrides: Partial<MapNode> = {}): MapNode {
  return {
    node: {
      node_id: nodeId,
      long_name: 'Map Node',
      last_seen_any_event_at: fixedTime
    },
    ...overrides
  }
}

function summary(nodeId: string, overrides: Partial<NodeSummary> = {}): NodeSummary {
  return {
    node_id: nodeId,
    display_name: 'Summary Node',
    has_position: false,
    last_seen_any_event_at: fixedTime,
    ...overrides
  }
}

function details(nodeId: string, overrides: Partial<NodeDetails> = {}): NodeDetails {
  return {
    node: {
      node_id: nodeId,
      long_name: 'Detailed Node',
      last_seen_any_event_at: fixedTime
    },
    ...overrides
  }
}

describe('resolveNodeDataValue', () => {
  afterEach(() => {
    resetNodeStore()
  })

  it('prefers selected node details over other data sources', () => {
    const value = resolveNodeDataValue(
      '!alpha',
      details('!alpha', { node: { node_id: '!alpha', short_name: 'A1', last_seen_any_event_at: fixedTime } }),
      mapNode('!alpha', { node: { node_id: '!alpha', long_name: 'Map Name', last_seen_any_event_at: fixedTime } }),
      summary('!alpha', { display_name: 'Summary Name' })
    )

    expect(value.label).toBe('A1')
    expect(value.title).toBe('!alpha')
    expect(value.resolved).toBe(true)
  })

  it('falls back from map nodes to summaries and then raw node id', () => {
    expect(resolveNodeDataValue(
      '!alpha',
      undefined,
      mapNode('!alpha', { node: { node_id: '!alpha', long_name: 'Map Name', last_seen_any_event_at: fixedTime } }),
      summary('!alpha', { display_name: 'Summary Name' })
    ).label).toBe('Map Name')

    expect(resolveNodeDataValue(
      '!bravo',
      undefined,
      undefined,
      summary('!bravo', { display_name: 'Summary Name' })
    ).label).toBe('Summary Name')

    const raw = resolveNodeDataValue('!charlie')
    expect(raw.label).toBe('!charlie')
    expect(raw.title).toBeUndefined()
    expect(raw.resolved).toBe(false)
  })
})

describe('ResolvedNodeData', () => {
  afterEach(() => {
    resetNodeStore()
  })

  it('renders a resolved label and raw node id tooltip from node store data', () => {
    useNodeStore.setState({
      mapNodes: [mapNode('!alpha', {
        node: {
          node_id: '!alpha',
          long_name: 'Field Router',
          last_seen_any_event_at: fixedTime
        }
      })],
      summaries: [],
      details: undefined
    })

    render(
      <ResolvedNodeData nodeId="!alpha">
        {({ label, title }) => <span title={title}>{label}</span>}
      </ResolvedNodeData>
    )

    expect(screen.getByText('Field Router').getAttribute('title')).toBe('!alpha')
  })
})
