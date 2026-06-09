import { relativeTime } from './time'

import type { NodeDetails, NodeNeighbor } from '../api/types'

// Tier names go worst -> best as: poor, weak, good, strong.
// "Poor" stays the floor — a real but barely-working link, never absence.
// "No SNR" / "Inferred" / "MQTT direct" are 5th-state buckets (no data),
// not a degraded tier.
export const TOPOLOGY_COLOR = {
  inferred: '#94a3b8',
  mqttDirect: '#38bdf8',
  noSNR: '#2563eb',
  poor: '#991b1b',
  weak: '#dc2626',
  good: '#eab308',
  strong: '#16a34a'
} as const

// TopologyColorInput is the minimal structural shape that drives the colour
// buckets. Both the per-neighbour layer (NodeNeighbor) and the global edge
// layer (TopologyEdge) map onto it at the call site, so the colour policy
// stays in one place.
export interface TopologyColorInput {
  inferred: boolean
  mqttDirect: boolean
  snr?: number
}

export function topologyColor(input: TopologyColorInput): string {
  if (input.inferred) {
    return TOPOLOGY_COLOR.inferred
  }
  if (typeof input.snr !== 'number') {
    if (input.mqttDirect) {
      return TOPOLOGY_COLOR.mqttDirect
    }

    return TOPOLOGY_COLOR.noSNR
  }
  if (input.snr < -15) {
    return TOPOLOGY_COLOR.poor
  }
  if (input.snr < -7) {
    return TOPOLOGY_COLOR.weak
  }
  if (input.snr < 10) {
    return TOPOLOGY_COLOR.good
  }

  return TOPOLOGY_COLOR.strong
}

export function topologySignalLabel(neighbor: NodeNeighbor): string {
  if (neighbor.evidence_kind === 'inferred') {
    return 'Inferred'
  }
  if (typeof neighbor.snr !== 'number') {
    if (neighbor.evidence_kind === 'mqtt_direct') {
      return 'Direct upload'
    }

    return 'No SNR'
  }

  return `SNR ${neighbor.snr.toFixed(1)} dB`
}

export function topologyEvidenceLabel(neighbor: NodeNeighbor): string {
  if (neighbor.evidence_kind === 'neighbor_info') {
    return 'Neighbor info'
  }
  if (neighbor.evidence_kind === 'mqtt_direct') {
    return 'MQTT direct'
  }

  return 'Inferred'
}

export function sortedNeighbors(details?: NodeDetails): NodeNeighbor[] {
  const neighbors = details?.neighbors ?? []

  return [...neighbors].sort(compareNeighbors)
}

function compareNeighbors(a: NodeNeighbor, b: NodeNeighbor): number {
  const rankDelta = neighborRank(a) - neighborRank(b)
  if (rankDelta !== 0) {
    return rankDelta
  }

  const aSNR = typeof a.snr === 'number' ? a.snr : Number.NEGATIVE_INFINITY
  const bSNR = typeof b.snr === 'number' ? b.snr : Number.NEGATIVE_INFINITY
  if (aSNR !== bSNR) {
    return bSNR - aSNR
  }

  const aObserved = Date.parse(a.last_observed_at)
  const bObserved = Date.parse(b.last_observed_at)
  if (Number.isFinite(aObserved) && Number.isFinite(bObserved) && aObserved !== bObserved) {
    return bObserved - aObserved
  }

  return a.display_name.localeCompare(b.display_name)
}

function neighborRank(neighbor: NodeNeighbor): number {
  if (neighbor.evidence_kind === 'inferred') {
    return 3
  }
  if (neighbor.evidence_kind === 'mqtt_direct') {
    return 2
  }
  if (typeof neighbor.snr !== 'number') {
    return 1
  }

  return 0
}

export function neighborTimeLabel(value?: string): string | null {
  return value ? relativeTime(value) : null
}
