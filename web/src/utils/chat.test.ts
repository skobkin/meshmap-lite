import { describe, expect, it } from 'vitest'

import { type ChatMessageEntry, type ChatReactionEntry, groupChatEvents } from './chat'

import type { ChatEvent } from '../api/types'

function makeMessage(overrides: Partial<ChatEvent> = {}): ChatEvent {
  return {
    id: 1,
    event_type: 'message',
    channel_name: 'LongFast',
    node_id: '!sender',
    node_display_name: 'sender',
    message_text: 'hello mesh',
    observed_at: '2026-05-10T12:00:00Z',
    packet_id: 100,
    ...overrides
  }
}

function makeReaction(overrides: Partial<ChatEvent> = {}): ChatEvent {
  return {
    id: 2,
    event_type: 'reaction',
    channel_name: 'LongFast',
    node_id: '!reactor',
    node_display_name: 'reactor',
    reaction_emoji: '👍',
    reply_to_packet_id: 100,
    observed_at: '2026-05-10T12:01:00Z',
    packet_id: 101,
    ...overrides
  }
}

describe('groupChatEvents', () => {
  it('returns each message and system row as its own entry when there are no reactions', () => {
    const events: ChatEvent[] = [
      makeMessage({ id: 1, packet_id: 100 }),
      makeMessage({ id: 2, event_type: 'system', system_code: 'node_discovered', message_text: undefined, packet_id: undefined })
    ]
    const out = groupChatEvents(events)
    expect(out).toHaveLength(2)
    expect(out[0]?.kind).toBe('message')
    expect(out[1]?.kind).toBe('system')
  })

  it('folds a reaction onto its target message via packet_id', () => {
    const events: ChatEvent[] = [makeMessage({ id: 1, packet_id: 100 }), makeReaction({ id: 2, reply_to_packet_id: 100 })]
    const out = groupChatEvents(events)
    expect(out).toHaveLength(1)
    const entry = out[0] as ChatMessageEntry
    expect(entry.reactions).toHaveLength(1)
    expect(entry.reactions[0]?.emoji).toBe('👍')
    expect(entry.reactions[0]?.reactors.map((r) => r.nodeId)).toEqual(['!reactor'])
  })

  it('aggregates two reactors under the same emoji on the same message', () => {
    const events: ChatEvent[] = [
      makeMessage({ id: 1, packet_id: 100 }),
      makeReaction({ id: 2, node_id: '!alice', node_display_name: 'alice', reply_to_packet_id: 100, observed_at: '2026-05-10T12:00:30Z' }),
      makeReaction({ id: 3, node_id: '!bob', node_display_name: 'bob', reply_to_packet_id: 100, observed_at: '2026-05-10T12:00:40Z' })
    ]
    const out = groupChatEvents(events)
    const entry = out[0] as ChatMessageEntry
    expect(entry.reactions).toHaveLength(1)
    expect(entry.reactions[0]?.reactors.map((r) => r.nodeId)).toEqual(['!alice', '!bob'])
  })

  it('keeps two different emojis as separate pills in chronological order', () => {
    const events: ChatEvent[] = [
      makeMessage({ id: 1, packet_id: 100 }),
      makeReaction({ id: 2, reaction_emoji: '❤️', reply_to_packet_id: 100, observed_at: '2026-05-10T12:00:10Z' }),
      makeReaction({ id: 3, reaction_emoji: '👍', reply_to_packet_id: 100, observed_at: '2026-05-10T12:00:20Z' })
    ]
    const out = groupChatEvents(events)
    const entry = out[0] as ChatMessageEntry
    expect(entry.reactions.map((r) => r.emoji)).toEqual(['❤️', '👍'])
  })

  it('orders reactors within a pill by observed_at ascending', () => {
    const events: ChatEvent[] = [
      makeMessage({ id: 1, packet_id: 100 }),
      makeReaction({ id: 2, node_id: '!late', observed_at: '2026-05-10T12:00:30Z', reply_to_packet_id: 100 }),
      makeReaction({ id: 3, node_id: '!early', observed_at: '2026-05-10T12:00:10Z', reply_to_packet_id: 100 })
    ]
    const out = groupChatEvents(events)
    const entry = out[0] as ChatMessageEntry
    expect(entry.reactions[0]?.reactors.map((r) => r.nodeId)).toEqual(['!early', '!late'])
  })

  it('treats self-reactions as ordinary reactor entries', () => {
    const events: ChatEvent[] = [
      makeMessage({ id: 1, node_id: '!alice', node_display_name: 'alice', packet_id: 100 }),
      makeReaction({ id: 2, node_id: '!alice', node_display_name: 'alice', reply_to_packet_id: 100 })
    ]
    const out = groupChatEvents(events)
    const entry = out[0] as ChatMessageEntry
    expect(entry.reactions[0]?.reactors).toEqual([
      { nodeId: '!alice', displayName: 'alice', observedAt: '2026-05-10T12:01:00Z' }
    ])
  })

  it('keeps a reaction whose target is not in scope as an orphaned entry', () => {
    const events: ChatEvent[] = [makeMessage({ id: 1, packet_id: 100 }), makeReaction({ id: 2, reply_to_packet_id: 999 })]
    const out = groupChatEvents(events)
    expect(out).toHaveLength(2)
    expect(out[0]?.kind).toBe('message')
    const orphan = out[1] as ChatReactionEntry
    expect(orphan.kind).toBe('reaction')
    expect(orphan.orphaned).toBe(true)
  })

  it('keeps a reaction with no reply_to_packet_id as orphaned', () => {
    const events: ChatEvent[] = [makeMessage({ id: 1, packet_id: 100 }), makeReaction({ id: 2, reply_to_packet_id: undefined })]
    const out = groupChatEvents(events)
    const orphan = out.find((e) => e.kind === 'reaction')
    expect(orphan?.kind).toBe('reaction')
    expect((orphan!).orphaned).toBe(true)
  })

  it('leaves messages without packet_id free of reactions even if reply_to_packet_id is set elsewhere', () => {
    const events: ChatEvent[] = [makeMessage({ id: 1, packet_id: undefined }), makeReaction({ id: 2, reply_to_packet_id: 1 })]
    const out = groupChatEvents(events)
    const message = out[0] as ChatMessageEntry
    expect(message.reactions).toEqual([])
    expect(out).toHaveLength(2)
  })

  it('preserves the order of input events for messages and orphaned reactions', () => {
    const events: ChatEvent[] = [
      makeMessage({ id: 1, message_text: 'a', packet_id: 100, observed_at: '2026-05-10T12:00:00Z' }),
      makeMessage({ id: 2, message_text: 'b', packet_id: 200, observed_at: '2026-05-10T12:00:10Z' }),
      makeReaction({ id: 3, reply_to_packet_id: 999, observed_at: '2026-05-10T12:00:20Z' })
    ]
    const out = groupChatEvents(events)
    expect(out.map((e) => (e.kind === 'reaction' ? `reaction:${e.event.id}` : `message:${e.event.id}`))).toEqual([
      'message:1',
      'message:2',
      'reaction:3'
    ])
  })
})
