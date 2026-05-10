import { parseDurationMs } from './duration'

import type { NodeDetails, NodeNeighbor } from '../api/types'

const cacheKey = 'meshmap-lite.node-details-cache.v1'
const cacheMaxEntries = 100

export interface CachedNodeDetailsEntry {
  fetchedAt: number
  details: NodeDetails
}

export type NodeDetailsCache = Record<string, CachedNodeDetailsEntry>

function isNodeDetails(value: unknown): value is NodeDetails {
  if (!value || typeof value !== 'object') {return false}
  const details = value as Record<string, unknown>
  const node = details.node

  return !!node && typeof node === 'object' && typeof (node as Record<string, unknown>).node_id === 'string'
}

function normalizeCacheEntry(value: unknown): CachedNodeDetailsEntry | null {
  if (!value || typeof value !== 'object') {return null}
  const entry = value as Record<string, unknown>
  if (typeof entry.fetchedAt !== 'number' || !Number.isFinite(entry.fetchedAt)) {return null}
  if (!isNodeDetails(entry.details)) {return null}

  return {
    fetchedAt: entry.fetchedAt,
    details: entry.details
  }
}

function trimCacheEntries(entries: NodeDetailsCache): NodeDetailsCache {
  return Object.fromEntries(
    Object.entries(entries)
      .sort(([, left], [, right]) => right.fetchedAt - left.fetchedAt)
      .slice(0, cacheMaxEntries)
  )
}

export function readNodeDetailsCache(storage: Storage = localStorage): NodeDetailsCache {
  const raw = storage.getItem(cacheKey)
  if (!raw) {return {}}

  try {
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') {return {}}

    return trimCacheEntries(
      Object.fromEntries(
        Object.entries(parsed as Record<string, unknown>)
          .map(([nodeID, value]) => {
            const entry = normalizeCacheEntry(value)

            return entry ? [nodeID, entry] as const : null
          })
          .filter((item): item is readonly [string, CachedNodeDetailsEntry] => item !== null)
      )
    )
  } catch {
    return {}
  }
}

export function persistNodeDetailsCache(entries: NodeDetailsCache, storage: Storage = localStorage): void {
  storage.setItem(cacheKey, JSON.stringify(trimCacheEntries(entries)))
}

export function upsertNodeDetailsCache(
  entries: NodeDetailsCache,
  details: NodeDetails,
  fetchedAt: number = Date.now()
): NodeDetailsCache {
  const next: NodeDetailsCache = {
    ...entries,
    [details.node.node_id]: {
      fetchedAt,
      details: mergeNodeDetails(entries[details.node.node_id]?.details, details)
    }
  }

  for (const neighbor of details.neighbors ?? []) {
    const reverseNeighbor = makeReverseNeighbor(details, neighbor)
    const existing = next[neighbor.node_id]
    next[neighbor.node_id] = {
      fetchedAt: existing?.fetchedAt ?? fetchedAt,
      details: mergeNodeDetails(existing?.details, {
        node: existing?.details.node ?? {
          node_id: neighbor.node_id,
          long_name: neighbor.long_name,
          short_name: neighbor.short_name,
          last_seen_any_event_at: reverseNeighbor.last_observed_at
        },
        position: existing?.details.position,
        telemetry: existing?.details.telemetry,
        neighbors: [reverseNeighbor]
      })
    }
  }

  return trimCacheEntries(next)
}

export function isNodeDetailsCacheFresh(entry: CachedNodeDetailsEntry | undefined, ttl: string, now: number = Date.now()): boolean {
  if (!entry) {
    return false
  }
  const ttlMs = parseDurationMs(ttl)
  if (typeof ttlMs !== 'number') {
    return true
  }

  return now-entry.fetchedAt <= ttlMs
}

function mergeNodeDetails(current: NodeDetails | undefined, incoming: NodeDetails): NodeDetails {
  const mergedNeighbors = mergeNeighbors(current?.neighbors, incoming.neighbors)

  return {
    node: {
      ...current?.node,
      ...incoming.node
    },
    position: incoming.position ?? current?.position,
    telemetry: incoming.telemetry ?? current?.telemetry,
    neighbors: mergedNeighbors,
    previous_names: incoming.previous_names ?? current?.previous_names
  }
}

function mergeNeighbors(current: NodeNeighbor[] | undefined, incoming: NodeNeighbor[] | undefined): NodeNeighbor[] | undefined {
  if ((!current || current.length === 0) && (!incoming || incoming.length === 0)) {
    return undefined
  }

  const merged = new Map<string, NodeNeighbor>()
  for (const neighbor of current ?? []) {
    merged.set(neighbor.node_id, neighbor)
  }
  for (const neighbor of incoming ?? []) {
    const existing = merged.get(neighbor.node_id)
    merged.set(neighbor.node_id, chooseBetterNeighbor(existing, neighbor))
  }

  return [...merged.values()]
}

function chooseBetterNeighbor(current: NodeNeighbor | undefined, incoming: NodeNeighbor): NodeNeighbor {
  if (!current) {
    return incoming
  }

  if (neighborRank(incoming) < neighborRank(current)) {
    return incoming
  }
  if (neighborRank(incoming) > neighborRank(current)) {
    return current
  }
  const currentObserved = Date.parse(current.last_observed_at)
  const incomingObserved = Date.parse(incoming.last_observed_at)
  if (Number.isFinite(currentObserved) && Number.isFinite(incomingObserved) && incomingObserved > currentObserved) {
    return incoming
  }
  if (typeof incoming.snr === 'number' && typeof current.snr === 'number' && incoming.snr > current.snr) {
    return incoming
  }
  if (typeof incoming.snr === 'number' && typeof current.snr !== 'number') {
    return incoming
  }

  return {
    ...current,
    ...incoming,
    display_name: incoming.display_name || current.display_name
  }
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

function makeReverseNeighbor(details: NodeDetails, neighbor: NodeNeighbor): NodeNeighbor {
  return {
    node_id: details.node.node_id,
    display_name: displayNameForNode(details),
    long_name: details.node.long_name,
    short_name: details.node.short_name,
    has_position: Boolean(details.position),
    evidence_kind: neighbor.evidence_kind,
    channel_name: neighbor.channel_name,
    reported_by_node_id: neighbor.reported_by_node_id,
    neighbor_last_rx_at: neighbor.neighbor_last_rx_at,
    neighbor_broadcast_interval_secs: neighbor.neighbor_broadcast_interval_secs,
    last_observed_at: neighbor.last_observed_at,
    last_reported_at: neighbor.last_reported_at,
    updated_at: neighbor.updated_at
  }
}

function displayNameForNode(details: NodeDetails): string {
  const longName = details.node.long_name?.trim()
  if (longName) {
    return longName
  }
  const shortName = details.node.short_name?.trim()
  if (shortName) {
    return shortName
  }

  return details.node.node_id
}
