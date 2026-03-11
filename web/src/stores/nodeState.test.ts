import { describe, expect, it } from 'vitest'
import type { Node, NodePosition } from '../api/types'
import { upsertMapNode, upsertNode, upsertPosition } from './nodeState'

describe('node state helpers', () => {
  it('creates a stub node when a position arrives before node details', () => {
    const position: NodePosition = {
      node_id: '!abcd',
      latitude: 10,
      longitude: 20,
      source_kind: 'telemetry',
      observed_at: '2026-03-11T10:00:00Z'
    }

    expect(upsertPosition([], position)).toEqual([
      {
        node: {
          node_id: '!abcd',
          last_seen_any_event_at: '2026-03-11T10:00:00Z'
        },
        position
      }
    ])
  })

  it('merges later node data into an existing positioned map node', () => {
    const position: NodePosition = {
      node_id: '!abcd',
      latitude: 10,
      longitude: 20,
      source_kind: 'telemetry',
      observed_at: '2026-03-11T10:00:00Z'
    }
    const node: Node = {
      node_id: '!abcd',
      long_name: 'Field Router',
      last_seen_any_event_at: '2026-03-11T10:05:00Z'
    }

    expect(upsertNode(upsertPosition([], position), node)).toEqual([
      {
        node,
        position
      }
    ])
  })

  it('replaces an existing map node entry on upsertMapNode', () => {
    expect(upsertMapNode([
      {
        node: {
          node_id: '!abcd',
          last_seen_any_event_at: '2026-03-11T10:00:00Z'
        }
      }
    ], {
      node: {
        node_id: '!abcd',
        long_name: 'Updated Node',
        last_seen_any_event_at: '2026-03-11T10:10:00Z'
      }
    })).toEqual([
      {
        node: {
          node_id: '!abcd',
          long_name: 'Updated Node',
          last_seen_any_event_at: '2026-03-11T10:10:00Z'
        }
      }
    ])
  })
})
