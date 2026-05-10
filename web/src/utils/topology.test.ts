import { describe, expect, it } from 'vitest'

import { sortedNeighbors, topologyColor, topologyEvidenceLabel, topologySignalLabel } from './topology'

import type { NodeDetails, NodeNeighbor } from '../api/types'

function neighbor(overrides: Partial<NodeNeighbor> = {}): NodeNeighbor {
  return {
    node_id: overrides.node_id ?? '!peer',
    display_name: overrides.display_name ?? 'Peer',
    has_position: overrides.has_position ?? true,
    evidence_kind: overrides.evidence_kind ?? 'neighbor_info',
    last_observed_at: overrides.last_observed_at ?? '2026-03-11T12:00:00Z',
    ...overrides
  }
}

describe('topology helpers', () => {
  it('maps evidence and SNR to colors and labels', () => {
    expect(topologyColor(neighbor({ evidence_kind: 'inferred' }))).toBe('#94a3b8')
    expect(topologyColor(neighbor({ evidence_kind: 'mqtt_direct' }))).toBe('#0f766e')
    expect(topologyColor(neighbor({ evidence_kind: 'mqtt_direct', snr: 12 }))).toBe('#16a34a')
    expect(topologyColor(neighbor())).toBe('#2563eb')
    expect(topologyColor(neighbor({ snr: -1 }))).toBe('#dc2626')
    expect(topologyColor(neighbor({ snr: 5 }))).toBe('#eab308')
    expect(topologyColor(neighbor({ snr: 12 }))).toBe('#16a34a')

    expect(topologySignalLabel(neighbor({ evidence_kind: 'inferred' }))).toBe('Inferred')
    expect(topologySignalLabel(neighbor({ evidence_kind: 'mqtt_direct' }))).toBe('Direct upload')
    expect(topologySignalLabel(neighbor({ evidence_kind: 'mqtt_direct', snr: -2.25 }))).toBe('SNR -2.3 dB')
    expect(topologySignalLabel(neighbor())).toBe('No SNR')
    expect(topologySignalLabel(neighbor({ snr: 12.34 }))).toBe('SNR 12.3 dB')
    expect(topologyEvidenceLabel(neighbor({ evidence_kind: 'mqtt_direct' }))).toBe('MQTT direct')
    expect(topologyEvidenceLabel(neighbor({ evidence_kind: 'inferred' }))).toBe('Inferred')
  })

  it('sorts neighbors by evidence quality, snr, and freshness', () => {
    const details: NodeDetails = {
      node: {
        node_id: '!origin',
        last_seen_any_event_at: '2026-03-11T12:00:00Z'
      },
      neighbors: [
        neighbor({ node_id: '!inferred', display_name: 'Inferred', evidence_kind: 'inferred' }),
        neighbor({ node_id: '!mqtt', display_name: 'MQTT direct', evidence_kind: 'mqtt_direct' }),
        neighbor({ node_id: '!unknown', display_name: 'No SNR' }),
        neighbor({ node_id: '!weak', display_name: 'Weak', snr: 1 }),
        neighbor({ node_id: '!strong', display_name: 'Strong', snr: 12 }),
        neighbor({ node_id: '!newer', display_name: 'Newer', snr: 12, last_observed_at: '2026-03-11T12:01:00Z' })
      ]
    }

    expect(sortedNeighbors(details).map((item) => item.node_id)).toEqual([
      '!newer',
      '!strong',
      '!weak',
      '!unknown',
      '!mqtt',
      '!inferred'
    ])
  })
})
