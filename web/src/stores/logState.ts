import type { LogEvent } from '../api/types'

export interface LogFilters {
  eventKinds: number[]
  channel: string
  nodeID: string
}

export function prependLiveLogItem(items: LogEvent[], filters: LogFilters, item: LogEvent): LogEvent[] {
  if (filters.channel && (item.channel_name ?? '') !== filters.channel) {
    return items
  }
  if (filters.nodeID && item.node_id !== filters.nodeID) {
    return items
  }
  if (filters.eventKinds.length > 0 && !filters.eventKinds.includes(item.event_kind_value)) {
    return items
  }

  return [item, ...items].slice(0, 1000)
}
