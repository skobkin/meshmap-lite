import { describe, expect, it } from 'vitest'

import { classifyHops } from './signal'

describe('classifyHops', () => {
  it('returns unknown classification when hop values are missing', () => {
    expect(classifyHops(undefined, undefined).qualityClass).toBe('')
    expect(classifyHops(0, 0).qualityClass).toBe('')
    expect(classifyHops(7, undefined).qualityClass).toBe('')
  })

  it('returns a neutral ↓0 badge for direct transmission to the uploader (no relay)', () => {
    // hop_start == hop_limit means the packet reached the uploader with
    // zero rebroadcasts between originator and uploader — the most common
    // case being a direct LoRa transmission to a node that is itself an
    // MQTT gateway. The classifier returns traversed: 0 (not undefined)
    // so the log view can render a ↓0 badge; the chat view hides it
    // (that decision lives in the renderer, not here).
    const result = classifyHops(7, 7)
    expect(result.traversed).toBe(0)
    expect(result.qualityClass).toBe('')
    expect(result.title).toBe('Hops traversed: 0 (direct transmission to uploader)')
    expect(result.exhausted).toBe(false)
  })

  it('classifies 1-2 hops as good', () => {
    expect(classifyHops(7, 6).qualityClass).toBe('signal-good')
    expect(classifyHops(7, 5).qualityClass).toBe('signal-good')
    expect(classifyHops(7, 5).traversed).toBe(2)
  })

  it('classifies mid-range hops as warn', () => {
    expect(classifyHops(7, 4).qualityClass).toBe('signal-warn')
    expect(classifyHops(7, 4).traversed).toBe(3)
  })

  it('classifies the last hop of the budget as bad', () => {
    expect(classifyHops(7, 1).qualityClass).toBe('signal-bad')
  })

  it('marks the hop_limit=0 case as exhausted and bad', () => {
    const result = classifyHops(7, 0)
    expect(result.exhausted).toBe(true)
    expect(result.qualityClass).toContain('signal-bad')
    expect(result.qualityClass).toContain('signal-exhausted')
    expect(result.traversed).toBe(7)
    expect(result.title).toContain('exhausted')
  })
})
