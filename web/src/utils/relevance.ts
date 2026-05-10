import { parseDurationMs } from './duration'

import type { NodeDetailsCache } from './nodeDetailsCache'
import type { MapNode, Meta, NodeDetails, NodeNeighbor, NodeSummary } from '../api/types'

interface RelevanceCutoffs {
  telemetry: number
  topology: number
  position: number
}

function cutoff(now: number, raw: string): number {
  const duration = parseDurationMs(raw)

  return typeof duration === 'number' ? now - duration : Number.NEGATIVE_INFINITY
}

function cutoffs(meta: Meta, now: number): RelevanceCutoffs {
  return {
    telemetry: cutoff(now, meta.relevance.telemetry_max_age),
    topology: cutoff(now, meta.relevance.topology_evidence_max_age),
    position: cutoff(now, meta.relevance.map_position_max_age)
  }
}

function isRecent(raw: string | undefined, cutoffMs: number): boolean {
  if (!raw) {return false}
  const ts = Date.parse(raw)

  return Number.isFinite(ts) && ts >= cutoffMs
}

function pruneNeighbors(neighbors: NodeNeighbor[] | undefined, cutoffMs: number): NodeNeighbor[] | undefined {
  const next = (neighbors ?? []).filter((item) => isRecent(item.updated_at, cutoffMs))

  return next.length > 0 ? next : undefined
}

export function pruneMapNodesByRelevance(items: MapNode[], meta: Meta, now: number = Date.now()): MapNode[] {
  const c = cutoffs(meta, now)

  return items
    .filter((item) => isRecent(item.position?.observed_at, c.position))
    .map((item) => ({
      ...item,
      telemetry: isRecent(item.telemetry?.observed_at, c.telemetry) ? item.telemetry : undefined
    }))
}

export function pruneNodeSummariesByRelevance(items: NodeSummary[], meta: Meta, now: number = Date.now()): NodeSummary[] {
  const c = cutoffs(meta, now)

  return items.map((item) => {
    if (isRecent(item.last_seen_position_at, c.position)) {
      return item
    }

    const rest: NodeSummary = { ...item, has_position: false }
    delete rest.last_seen_position_at

    return rest
  })
}

export function pruneNodeDetailsByRelevance(details: NodeDetails | undefined, meta: Meta, now: number = Date.now()): NodeDetails | undefined {
  if (!details) {return undefined}
  const c = cutoffs(meta, now)

  return {
    ...details,
    position: isRecent(details.position?.observed_at, c.position) ? details.position : undefined,
    telemetry: isRecent(details.telemetry?.observed_at, c.telemetry) ? details.telemetry : undefined,
    neighbors: pruneNeighbors(details.neighbors, c.topology)
  }
}

export function pruneNodeDetailsCacheByRelevance(entries: NodeDetailsCache, meta: Meta, now: number = Date.now()): NodeDetailsCache {
  return Object.fromEntries(
    Object.entries(entries).map(([nodeID, entry]) => [
      nodeID,
      {
        ...entry,
        details: pruneNodeDetailsByRelevance(entry.details, meta, now) ?? entry.details
      }
    ])
  )
}
