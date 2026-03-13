import { relativeTime } from './time'

import type { NodeDetails, NodeNeighbor } from '../api/types'

export const TOPOLOGY_COLOR = {
  inferred: '#94a3b8',
  noSNR: '#2563eb',
  poor: '#dc2626',
  fair: '#eab308',
  good: '#16a34a'
} as const

export function topologyColor(neighbor: NodeNeighbor): string {
  if (neighbor.evidence_kind === 'inferred') {
    return TOPOLOGY_COLOR.inferred
  }
  if (typeof neighbor.snr !== 'number') {
    return TOPOLOGY_COLOR.noSNR
  }
  if (neighbor.snr < 0) {
    return TOPOLOGY_COLOR.poor
  }
  if (neighbor.snr < 10) {
    return TOPOLOGY_COLOR.fair
  }

  return TOPOLOGY_COLOR.good
}

export function topologySignalLabel(neighbor: NodeNeighbor): string {
  if (neighbor.evidence_kind === 'inferred') {
    return 'Inferred'
  }
  if (typeof neighbor.snr !== 'number') {
    return 'No SNR'
  }

  return `SNR ${neighbor.snr.toFixed(1)} dB`
}

export function topologyEvidenceLabel(neighbor: NodeNeighbor): string {
  return neighbor.evidence_kind === 'neighbor_info' ? 'Neighbor info' : 'Inferred'
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
