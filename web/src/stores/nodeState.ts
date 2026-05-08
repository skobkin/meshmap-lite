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

function valueOrCurrent<T>(incoming: T | undefined, current: T | undefined): T | undefined {
  if (typeof incoming === 'string' && incoming.length === 0) {
    return current
  }

  return incoming ?? current
}

function mergeNode(current: Node, incoming: Node): Node {
  return {
    ...current,
    ...incoming,
    node_num: valueOrCurrent(incoming.node_num, current.node_num),
    long_name: valueOrCurrent(incoming.long_name, current.long_name),
    short_name: valueOrCurrent(incoming.short_name, current.short_name),
    role: valueOrCurrent(incoming.role, current.role),
    board_model: valueOrCurrent(incoming.board_model, current.board_model),
    firmware_version: valueOrCurrent(incoming.firmware_version, current.firmware_version),
    lora_region: valueOrCurrent(incoming.lora_region, current.lora_region),
    lora_frequency_desc: valueOrCurrent(incoming.lora_frequency_desc, current.lora_frequency_desc),
    modem_preset: valueOrCurrent(incoming.modem_preset, current.modem_preset),
    has_default_channel: valueOrCurrent(incoming.has_default_channel, current.has_default_channel),
    has_opted_report_location: valueOrCurrent(incoming.has_opted_report_location, current.has_opted_report_location),
    neighbor_nodes_count: valueOrCurrent(incoming.neighbor_nodes_count, current.neighbor_nodes_count),
    mqtt_gateway_capable: valueOrCurrent(incoming.mqtt_gateway_capable, current.mqtt_gateway_capable),
    first_seen_at: valueOrCurrent(incoming.first_seen_at, current.first_seen_at),
    last_seen_mqtt_gateway_at: valueOrCurrent(incoming.last_seen_mqtt_gateway_at, current.last_seen_mqtt_gateway_at),
    last_mqtt_uploader_node_id: valueOrCurrent(incoming.last_mqtt_uploader_node_id, current.last_mqtt_uploader_node_id),
    last_mqtt_uploader_display_name: valueOrCurrent(incoming.last_mqtt_uploader_display_name, current.last_mqtt_uploader_display_name),
    last_mqtt_uploader_at: valueOrCurrent(incoming.last_mqtt_uploader_at, current.last_mqtt_uploader_at),
    last_seen_position_at: valueOrCurrent(incoming.last_seen_position_at, current.last_seen_position_at),
    updated_at: valueOrCurrent(incoming.updated_at, current.updated_at)
  }
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
  clone[idx] = { ...current, node: mergeNode(current.node, node) }

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
