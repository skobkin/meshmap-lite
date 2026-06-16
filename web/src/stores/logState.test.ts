import { describe, expect, it } from 'vitest'

import { defaultLogHopRange } from '../utils/logHops'

import { prependLiveLogItem } from './logState'

import type { LogFilters } from './logState'
import type { LogEvent } from '../api/types'

function event(id: number, overrides: Partial<LogEvent> = {}): LogEvent {
  return {
    id,
    observed_at: '2026-03-11T10:00:00Z',
    event_kind_value: 1,
    event_kind_title: 'text message',
    encrypted: false,
    channel_name: 'mesh',
    ...overrides
  }
}

function filters(overrides: Partial<LogFilters> = {}): LogFilters {
  return {
    channel: '',
    nodeID: '',
    eventKinds: [],
    hopRange: defaultLogHopRange,
    ...overrides
  }
}

describe('log state helpers', () => {
  it('filters live events by channel and event kind', () => {
    let items: LogEvent[] = []

    items = prependLiveLogItem(items, filters({ channel: 'ops', eventKinds: [7] }), event(1, { channel_name: 'mesh', event_kind_value: 7 }))
    items = prependLiveLogItem(items, filters({ channel: 'ops', eventKinds: [7] }), event(2, { channel_name: 'ops', event_kind_value: 3 }))
    items = prependLiveLogItem(items, filters({ channel: 'ops', eventKinds: [7] }), event(3, { channel_name: 'ops', event_kind_value: 7 }))

    expect(items.map((item) => item.id)).toEqual([3])
  })

  it('filters live events by exact node id', () => {
    let items: LogEvent[] = []

    items = prependLiveLogItem(items, filters({ nodeID: '!alpha' }), event(1, { node_id: '!bravo' }))
    items = prependLiveLogItem(items, filters({ nodeID: '!alpha' }), event(2, { node_id: '!alpha' }))

    expect(items.map((item) => item.id)).toEqual([2])
  })

  it('filters live events by traversed hop range and excludes missing metadata and self uploads', () => {
    let items: LogEvent[] = []
    const active = filters({ hopRange: { min: 0, max: 3 } })

    items = prependLiveLogItem(items, active, event(1, { node_id: '!zero', mqtt_uploader_node_id: '!gateway', hop_start: 7, hop_limit: 7 }))
    items = prependLiveLogItem(items, active, event(2, { node_id: '!three', mqtt_uploader_node_id: '!gateway', hop_start: 5, hop_limit: 2 }))
    items = prependLiveLogItem(items, active, event(3, { node_id: '!five', mqtt_uploader_node_id: '!gateway', hop_start: 5, hop_limit: 0 }))
    items = prependLiveLogItem(items, active, event(4, { node_id: '!missing', mqtt_uploader_node_id: '!gateway' }))
    items = prependLiveLogItem(items, active, event(5, { node_id: '!self', mqtt_uploader_node_id: '!self', hop_start: 7, hop_limit: 7 }))

    expect(items.map((item) => item.id)).toEqual([2, 1])
  })

  it('does not filter live events by hops when the default range is active', () => {
    let items: LogEvent[] = []

    items = prependLiveLogItem(items, filters(), event(1))
    items = prependLiveLogItem(items, filters(), event(2, { node_id: '!self', mqtt_uploader_node_id: '!self', hop_start: 7, hop_limit: 7 }))

    expect(items.map((item) => item.id)).toEqual([2, 1])
  })

  it('caps live history at 1000 events', () => {
    let items: LogEvent[] = []

    for (let index = 1; index <= 1001; index++) {
      items = prependLiveLogItem(items, filters(), event(index))
    }

    expect(items).toHaveLength(1000)
    expect(items[0]?.id).toBe(1001)
    expect(items.at(-1)?.id).toBe(2)
  })
})
