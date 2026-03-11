import { afterEach, describe, expect, it, vi } from 'vitest'
import { dayKey, dayLabel, relativeTime } from './time'

describe('time helpers', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('formats relative times across second, minute, hour, and day boundaries', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-11T12:00:00Z'))

    expect(relativeTime('2026-03-11T11:59:31Z')).toBe('29s ago')
    expect(relativeTime('2026-03-11T11:58:31Z')).toBe('1m ago')
    expect(relativeTime('2026-03-11T10:00:00Z')).toBe('2h ago')
    expect(relativeTime('2026-03-08T12:00:00Z')).toBe('3d ago')
    expect(relativeTime()).toBe('n/a')
  })

  it('builds stable day keys and labels', () => {
    expect(dayKey('2026-03-11T12:34:56Z')).toBe('2026-03-11')
    expect(dayLabel('2026-03-11T12:34:56Z')).toBe('11.03.2026')
  })
})
