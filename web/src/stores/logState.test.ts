import { describe, expect, it } from 'vitest'

import { prependLiveLogItem } from './logState'

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

describe('log state helpers', () => {
  it('filters live events by channel and event kind', () => {
    let items: LogEvent[] = []

    items = prependLiveLogItem(items, { channel: 'ops', eventKinds: [7] }, event(1, { channel_name: 'mesh', event_kind_value: 7 }))
    items = prependLiveLogItem(items, { channel: 'ops', eventKinds: [7] }, event(2, { channel_name: 'ops', event_kind_value: 3 }))
    items = prependLiveLogItem(items, { channel: 'ops', eventKinds: [7] }, event(3, { channel_name: 'ops', event_kind_value: 7 }))

    expect(items.map((item) => item.id)).toEqual([3])
  })

  it('caps live history at 1000 events', () => {
    let items: LogEvent[] = []

    for (let index = 1; index <= 1001; index++) {
      items = prependLiveLogItem(items, { channel: '', eventKinds: [] }, event(index))
    }

    expect(items).toHaveLength(1000)
    expect(items[0]?.id).toBe(1001)
    expect(items.at(-1)?.id).toBe(2)
  })
})
