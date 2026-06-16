import { defaultLogHopRange, eventMatchesLogHopRange, normalizeLogHopRange } from '../utils/logHops'

import type { LogEvent } from '../api/types'
import type { LogHopRange } from '../utils/logHops'

export interface LogFilters {
  eventKinds: number[]
  channel: string
  nodeID: string
  hopRange: LogHopRange
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
  if (!eventMatchesLogHopRange(item, normalizeLogHopRange(filters.hopRange))) {
    return items
  }

  return [item, ...items].slice(0, 1000)
}

export const defaultLogFilters: LogFilters = {
  eventKinds: [],
  channel: '',
  nodeID: '',
  hopRange: defaultLogHopRange
}
