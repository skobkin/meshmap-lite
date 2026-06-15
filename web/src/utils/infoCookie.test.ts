// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest'

import { infoDismissedCookie, updatesDismissedCookie } from './infoCookie'

describe('infoDismissedCookie', () => {
  afterEach(() => {
    document.cookie = 'meshmap-lite.info.dismissed_source_hash=; Max-Age=0; Path=/'
    vi.restoreAllMocks()
  })

  it('returns an empty string when no cookie is set', () => {
    document.cookie = 'meshmap-lite.info.dismissed_source_hash=; Max-Age=0; Path=/'
    expect(infoDismissedCookie.read()).toBe('')
  })

  it('round-trips a value via write/read', () => {
    infoDismissedCookie.write('hash-123')
    expect(infoDismissedCookie.read()).toBe('hash-123')
  })

  it('uses Max-Age=10y, Path=/ and SameSite=Lax', () => {
    let captured = ''
    const setter = vi.spyOn(document, 'cookie', 'set').mockImplementation((value: string) => {
      captured = value

      return value
    })
    infoDismissedCookie.write('hash-123')
    setter.mockRestore()
    expect(captured).toContain('meshmap-lite.info.dismissed_source_hash=hash-123')
    expect(captured).toContain('Max-Age=315360000')
    expect(captured).toContain('Path=/')
    expect(captured).toContain('SameSite=Lax')
  })
})

describe('updatesDismissedCookie', () => {
  afterEach(() => {
    document.cookie = 'meshmap-lite.updates.dismissed_published_at=; Max-Age=0; Path=/'
    vi.restoreAllMocks()
  })

  it('returns an empty string when no cookie is set', () => {
    document.cookie = 'meshmap-lite.updates.dismissed_published_at=; Max-Age=0; Path=/'
    expect(updatesDismissedCookie.read()).toBe('')
  })

  it('round-trips a value via write/read', () => {
    updatesDismissedCookie.write('2026-06-15T12:00:00Z')
    expect(updatesDismissedCookie.read()).toBe('2026-06-15T12:00:00Z')
  })

  it('uses Max-Age=10y, Path=/ and SameSite=Lax', () => {
    let captured = ''
    const setter = vi.spyOn(document, 'cookie', 'set').mockImplementation((value: string) => {
      captured = value

      return value
    })
    updatesDismissedCookie.write('2026-06-15T12:00:00Z')
    setter.mockRestore()
    expect(captured).toContain('meshmap-lite.updates.dismissed_published_at=2026-06-15T12%3A00%3A00Z')
    expect(captured).toContain('Max-Age=315360000')
    expect(captured).toContain('Path=/')
    expect(captured).toContain('SameSite=Lax')
  })
})
