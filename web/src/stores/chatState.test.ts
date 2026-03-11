import { describe, expect, it } from 'vitest'
import type { ChatEvent } from '../api/types'
import { chatStorageKey, pushChatMessage, readStoredChatChannel } from './chatState'

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
})
