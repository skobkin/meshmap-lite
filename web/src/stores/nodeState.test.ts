import { describe, expect, it } from 'vitest'

import { upsertMapNode, upsertNode, upsertPosition, upsertTelemetry } from './nodeState'

import type { MapNode, Node, NodePosition, NodeTelemetry } from '../api/types'

const fixedTime = '2026-03-11T10:00:00Z'

function telemetry(nodeId: string, overrides: Partial<NodeTelemetry> = {}): NodeTelemetry {
  return {
    node_id: nodeId,
    power: {},
    environment: {},
    air_quality: {},
    observed_at: fixedTime,
    updated_at: fixedTime,
    ...overrides
  }
}

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

  it('ignores telemetry for unknown node_id and does not create a new map node entry', () => {
    const initial: MapNode[] = [
      {
        node: {
          node_id: '!known',
          last_seen_any_event_at: fixedTime
        }
      }
    ]

    const next = upsertTelemetry(initial, telemetry('!unknown', {
      power: { battery_level: 89 }
    }))

    expect(next).toBe(initial)
    expect(next).toHaveLength(1)
    expect(next.find((item) => item.node.node_id === '!unknown')).toBeUndefined()
  })

  it('updates telemetry for matching node without affecting unrelated map nodes', () => {
    const alpha: MapNode = {
      node: {
        node_id: '!alpha',
        last_seen_any_event_at: fixedTime
      },
      telemetry: telemetry('!alpha', { power: { battery_level: 35 } })
    }
    const bravo: MapNode = {
      node: {
        node_id: '!bravo',
        last_seen_any_event_at: fixedTime
      },
      telemetry: telemetry('!bravo', { power: { battery_level: 72 } })
    }
    const incoming = telemetry('!alpha', {
      power: { battery_level: 54, voltage: 4.01 },
      environment: { temperature_c: 22.4 }
    })

    const next = upsertTelemetry([alpha, bravo], incoming)

    expect(next).toHaveLength(2)
    expect(next[0]).toEqual({ ...alpha, telemetry: incoming })
    expect(next[1]).toBe(bravo)
  })

  it('preserves battery info on live partial telemetry update when backend sends merged snapshot', () => {
    const existing: MapNode = {
      node: {
        node_id: '!alpha',
        last_seen_any_event_at: fixedTime
      },
      telemetry: telemetry('!alpha', {
        power: { battery_level: 64, voltage: 3.98 },
        environment: { temperature_c: 19.5 }
      })
    }
    const mergedSnapshot = telemetry('!alpha', {
      power: { battery_level: 64 },
      environment: { temperature_c: 21.1, humidity: 40 }
    })

    const next = upsertTelemetry([existing], mergedSnapshot)
    const nextTelemetry = next[0]?.telemetry

    expect(nextTelemetry?.power.battery_level).toBe(64)
    expect(nextTelemetry).toEqual(mergedSnapshot)
  })
})
