import { describe, expect, it } from 'vitest'

import { pruneMapNodesByRelevance, pruneNodeDetailsByRelevance, pruneNodeDetailsCacheByRelevance, pruneNodeSummariesByRelevance } from './relevance'

import type { MapNode, Meta, NodeDetails, NodeSummary } from '../api/types'

const now = Date.parse('2026-03-12T12:00:00Z')

function meta(): Meta {
  return {
    app_name: 'MeshMap Lite',
    version: 'dev',
    websocket_path: '/api/v1/ws',
    default_chat_channel: 'mesh',
    show_recent_messages: 50,
    log_live_updates: true,
    log_page_size_default: 100,
    disconnected_threshold: '60m',
    relevance: {
      telemetry_max_age: '24h',
      topology_evidence_max_age: '72h',
      map_position_max_age: '336h'
    },
    map: {
      clustering: false,
      topology_cache_ttl: '10m',
      precision_circles_mode: 'selected',
      default_view: {
        latitude: 64.5,
        longitude: 40.6,
        zoom: 13
      }
    }
  }
}

describe('relevance pruning', () => {
  it('drops stale map nodes and stale telemetry without touching logs', () => {
    const items: MapNode[] = [
      {
        node: { node_id: '!fresh', last_seen_any_event_at: '2026-03-12T12:00:00Z' },
        position: { node_id: '!fresh', latitude: 1, longitude: 2, source_kind: 'channel', observed_at: '2026-03-12T11:00:00Z' },
        telemetry: { node_id: '!fresh', power: {}, environment: {}, air_quality: {}, observed_at: '2026-03-11T11:59:59Z', updated_at: '2026-03-11T11:59:59Z' }
      },
      {
        node: { node_id: '!stale', last_seen_any_event_at: '2026-03-12T12:00:00Z' },
        position: { node_id: '!stale', latitude: 1, longitude: 2, source_kind: 'channel', observed_at: '2026-02-20T11:00:00Z' }
      }
    ]

    const pruned = pruneMapNodesByRelevance(items, meta(), now)

    expect(pruned.map((item) => item.node.node_id)).toEqual(['!fresh'])
    expect(pruned[0]?.telemetry).toBeUndefined()
  })

  it('clears stale node summaries, details, cached details, and neighbors', () => {
    const summaries: NodeSummary[] = [{
      node_id: '!node',
      display_name: 'Node',
      last_seen_any_event_at: '2026-03-12T12:00:00Z',
      last_seen_position_at: '2026-02-20T11:00:00Z',
      has_position: true
    }]
    const details: NodeDetails = {
      node: { node_id: '!node', last_seen_any_event_at: '2026-03-12T12:00:00Z' },
      position: { node_id: '!node', latitude: 1, longitude: 2, source_kind: 'channel', observed_at: '2026-02-20T11:00:00Z' },
      telemetry: { node_id: '!node', power: {}, environment: {}, air_quality: {}, observed_at: '2026-03-10T12:00:00Z', updated_at: '2026-03-10T12:00:00Z' },
      neighbors: [{
        node_id: '!peer',
        display_name: 'Peer',
        has_position: true,
        evidence_kind: 'neighbor_info',
        last_observed_at: '2026-03-10T12:00:00Z',
        updated_at: '2026-03-09T11:59:59Z'
      }]
    }

    const prunedSummary = pruneNodeSummariesByRelevance(summaries, meta(), now)[0]
    expect(prunedSummary).toEqual(expect.objectContaining({ has_position: false }))
    expect(prunedSummary).not.toHaveProperty('last_seen_position_at')
    expect(pruneNodeDetailsByRelevance(details, meta(), now)).toEqual(expect.objectContaining({
      position: undefined,
      telemetry: undefined,
      neighbors: undefined
    }))
    expect(pruneNodeDetailsCacheByRelevance({ '!node': { fetchedAt: now, details } }, meta(), now)['!node']?.details.neighbors).toBeUndefined()
  })
})
