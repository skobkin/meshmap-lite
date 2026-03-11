// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'

import { chatNodeLabel } from '../utils/chat'

describe('chatNodeLabel', () => {
  it('prefers node_display_name over node_id', () => {
    expect(chatNodeLabel({
      id: 7,
      event_type: 'message',
      node_id: '!deadbeef',
      node_display_name: 'test-node-alpha',
      observed_at: '2026-03-11T17:58:00Z'
    })).toBe('test-node-alpha')
  })

  it('falls back to node_id and then system', () => {
    expect(chatNodeLabel({
      id: 8,
      event_type: 'message',
      node_id: '!deadbeef',
      observed_at: '2026-03-11T17:58:00Z'
    })).toBe('!deadbeef')

    expect(chatNodeLabel({
      id: 9,
      event_type: 'system',
      observed_at: '2026-03-11T17:58:00Z'
    })).toBe('system')
  })
})
