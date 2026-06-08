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

function reaction(id: number): ChatEvent {
  return {
    id,
    event_type: 'reaction',
    reaction_emoji: '👍',
    reply_to_packet_id: 100,
    observed_at: '2026-03-11T10:00:30Z'
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

  it('keeps reactions in the same live ring buffer as messages', () => {
    const messages = pushChatMessage([], message(1))
    const withReaction = pushChatMessage(messages, reaction(2))
    // pushChatMessage prepends (newest first), so the reaction is at index 0.
    expect(withReaction.map((item) => item.event_type)).toEqual(['reaction', 'message'])
    expect(withReaction[0]?.reaction_emoji).toBe('👍')
    expect(withReaction[1]?.message_text).toBeUndefined()
  })

  it('deduplicates reactions when paginating older history', () => {
    const messages = appendOlderChatMessages(
      [message(5), reaction(4)],
      [reaction(4), message(3), message(2)]
    )

    expect(messages.map((item) => item.id)).toEqual([5, 4, 3, 2])
    const reactionCount = messages.filter((item) => item.event_type === 'reaction').length
    expect(reactionCount).toBe(1)
  })
})
