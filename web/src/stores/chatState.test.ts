import { describe, expect, it } from 'vitest'

import { appendOlderChatMessages, chatStorageKey, pushChatMessage, readStoredChatChannel } from './chatState'

import type { ChatEvent } from '../api/types'

function message(id: number): ChatEvent {
  return {
    id,
    event_type: 'message',
    observed_at: '2026-03-11T10:00:00Z'
  }
}

describe('chat state helpers', () => {
  it('reads the stored channel and falls back to empty string', () => {
    const storage = {
      getItem: (key: string) => key === chatStorageKey ? 'mesh' : null
    }

    expect(readStoredChatChannel(storage)).toBe('mesh')
    expect(readStoredChatChannel({ getItem: () => null })).toBe('')
  })

  it('keeps only the latest 500 live messages', () => {
    let messages: ChatEvent[] = []

    for (let index = 1; index <= 501; index++) {
      messages = pushChatMessage(messages, message(index))
    }

    expect(messages).toHaveLength(500)
    expect(messages[0]?.id).toBe(501)
    expect(messages.at(-1)?.id).toBe(2)
  })

  it('appends older history without trimming live capacity or duplicating rows', () => {
    const messages = appendOlderChatMessages(
      [message(5), message(4)],
      [message(4), message(3), message(2)]
    )

    expect(messages.map((item) => item.id)).toEqual([5, 4, 3, 2])
  })
})
