import type { MapNode, Node, NodePosition, NodeTelemetry } from '../api/types'

export function upsertMapNode(mapNodes: MapNode[], item: MapNode): MapNode[] {
  const idx = mapNodes.findIndex((node) => node.node.node_id === item.node.node_id)
  if (idx < 0) {
    return [item, ...mapNodes]
  }
  const clone = mapNodes.slice()
  clone[idx] = item

  return clone
}

export function upsertNode(mapNodes: MapNode[], node: Node): MapNode[] {
  const idx = mapNodes.findIndex((item) => item.node.node_id === node.node_id)
  if (idx < 0) {
    return [{ node }, ...mapNodes]
  }
  const clone = mapNodes.slice()
  const current = clone[idx]
  if (!current) {
    return mapNodes
  }
  clone[idx] = { ...current, node }

  return clone
}

export function upsertPosition(mapNodes: MapNode[], position: NodePosition): MapNode[] {
  const idx = mapNodes.findIndex((item) => item.node.node_id === position.node_id)
  if (idx < 0) {
    const stubNode: Node = {
      node_id: position.node_id,
      last_seen_any_event_at: position.observed_at
    }

    return [{ node: stubNode, position }, ...mapNodes]
  }
  const clone = mapNodes.slice()
  const current = clone[idx]
  if (!current) {
    return mapNodes
  }
  clone[idx] = { ...current, position }

  return clone
}

export function upsertTelemetry(mapNodes: MapNode[], telemetry: NodeTelemetry): MapNode[] {
  const idx = mapNodes.findIndex((item) => item.node.node_id === telemetry.node_id)
  if (idx < 0) {
    return mapNodes
  }
  const clone = mapNodes.slice()
  const current = clone[idx]
  if (!current) {
    return mapNodes
  }
  clone[idx] = { ...current, telemetry }

  return clone
}
