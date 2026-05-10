import { describe, expect, it } from 'vitest'

import {
  isNodeDetailsCacheFresh,
  persistNodeDetailsCache,
  readNodeDetailsCache,
  upsertNodeDetailsCache
} from './nodeDetailsCache'

import type { NodeDetails } from '../api/types'

function details(nodeID: string): NodeDetails {
  return {
    node: {
      node_id: nodeID,
      last_seen_any_event_at: '2026-03-11T12:00:00Z'
    }
  }
}

function detailsWithNeighbors(nodeID: string, neighbors: NodeDetails['neighbors']): NodeDetails {
  return {
    node: {
      node_id: nodeID,
      long_name: `Node ${nodeID}`,
      short_name: nodeID.slice(1, 5),
      last_seen_any_event_at: '2026-03-11T12:00:00Z'
    },
    position: {
      node_id: nodeID,
      latitude: 64.5,
      longitude: 40.6,
      source_kind: 'gps',
      observed_at: '2026-03-11T12:00:00Z'
    },
    neighbors
  }
}

describe('nodeDetailsCache', () => {
  it('stores and reloads cache entries', () => {
    const cache = upsertNodeDetailsCache({}, details('!alpha'), 1000)

    persistNodeDetailsCache(cache)
    const loaded = readNodeDetailsCache()

    expect(loaded['!alpha']?.details.node.node_id).toBe('!alpha')
    expect(loaded['!alpha']?.fetchedAt).toBe(1000)
  })

  it('reports freshness from ttl', () => {
    const cache = upsertNodeDetailsCache({}, details('!alpha'), 1000)

    expect(isNodeDetailsCacheFresh(cache['!alpha'], '500ms', 1400)).toBe(true)
    expect(isNodeDetailsCacheFresh(cache['!alpha'], '300ms', 1400)).toBe(false)
  })

  it('keeps the newest 100 entries', () => {
    let cache = {} as Record<string, { fetchedAt: number; details: NodeDetails }>
    for (let index = 0; index < 105; index++) {
      cache = upsertNodeDetailsCache(cache, details(`!node-${index}`), index)
    }

    expect(Object.keys(cache)).toHaveLength(100)
    expect(cache['!node-0']).toBeUndefined()
    expect(cache['!node-104']?.details.node.node_id).toBe('!node-104')
  })

  it('mirrors fetched neighbors into peer cache entries', () => {
    let cache = upsertNodeDetailsCache({}, detailsWithNeighbors('!x', [{
      node_id: '!y',
      display_name: 'Node !y',
      has_position: true,
      evidence_kind: 'neighbor_info',
      snr: 4.2,
      last_observed_at: '2026-03-11T12:00:00Z'
    }]), 1000)

    cache = upsertNodeDetailsCache(cache, detailsWithNeighbors('!z', [{
      node_id: '!x',
      display_name: 'Node !x',
      has_position: true,
      evidence_kind: 'neighbor_info',
      snr: 12.5,
      last_observed_at: '2026-03-11T12:05:00Z'
    }]), 2000)

    expect(cache['!x']?.details.neighbors).toEqual(expect.arrayContaining([
      expect.objectContaining({ node_id: '!y' }),
      expect.objectContaining({
        node_id: '!z',
        evidence_kind: 'neighbor_info'
      })
    ]))
    expect(cache['!x']?.details.neighbors?.find((neighbor) => neighbor.node_id === '!z')).not.toHaveProperty('snr')
  })

  it('keeps mqtt direct cache evidence above newer inferred evidence', () => {
    let cache = upsertNodeDetailsCache({}, detailsWithNeighbors('!origin', [{
      node_id: '!peer',
      display_name: 'Peer',
      has_position: true,
      evidence_kind: 'mqtt_direct',
      last_observed_at: '2026-03-11T12:00:00Z'
    }]), 1000)

    cache = upsertNodeDetailsCache(cache, detailsWithNeighbors('!peer', [{
      node_id: '!origin',
      display_name: 'Origin',
      has_position: true,
      evidence_kind: 'inferred',
      last_observed_at: '2026-03-11T12:10:00Z'
    }]), 2000)

    expect(cache['!peer']?.details.neighbors?.find((neighbor) => neighbor.node_id === '!origin')?.evidence_kind).toBe('mqtt_direct')
  })

  it('preserves previous names when merging refreshed partial details', () => {
    let cache = upsertNodeDetailsCache({}, {
      ...details('!alpha'),
      previous_names: [{
        previous_long_name: 'Old Alpha',
        new_long_name: 'Alpha',
        changed_at: '2026-03-11T12:00:00Z'
      }]
    }, 1000)

    cache = upsertNodeDetailsCache(cache, details('!alpha'), 2000)

    expect(cache['!alpha']?.details.previous_names).toEqual([{
      previous_long_name: 'Old Alpha',
      new_long_name: 'Alpha',
      changed_at: '2026-03-11T12:00:00Z'
    }])
  })
})
