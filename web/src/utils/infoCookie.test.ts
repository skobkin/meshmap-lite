// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest'

import { infoDismissedCookie, readUpdatesDismissedAt, updatesDismissedCookie } from './infoCookie'

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
    document.cookie = 'meshmap-lite.updates.meshmap-lite.dismissed_published_at=; Max-Age=0; Path=/'
    document.cookie = 'meshmap-lite.updates.firmware.dismissed_published_at=; Max-Age=0; Path=/'
    vi.restoreAllMocks()
  })

  it('returns an empty string when no cookie is set', () => {
    expect(updatesDismissedCookie('meshmap-lite').read()).toBe('')
  })

  it('round-trips a value via write/read', () => {
    updatesDismissedCookie('meshmap-lite').write('2026-06-15T12:00:00Z')
    expect(updatesDismissedCookie('meshmap-lite').read()).toBe('2026-06-15T12:00:00Z')
    expect(updatesDismissedCookie('firmware').read()).toBe('')
  })

  it('reads dismissal timestamps independently for configured sources', () => {
    const cookie = [
      'meshmap-lite.updates.meshmap-lite.dismissed_published_at=2026-06-15T12%3A00%3A00Z',
      'meshmap-lite.updates.firmware.dismissed_published_at=2026-06-14T12%3A00%3A00Z'
    ].join('; ')

    expect(readUpdatesDismissedAt(['meshmap-lite', 'firmware'], cookie)).toEqual({
      'meshmap-lite': '2026-06-15T12:00:00Z',
      firmware: '2026-06-14T12:00:00Z'
    })
  })

  it('uses Max-Age=10y, Path=/ and SameSite=Lax', () => {
    let captured = ''
    const setter = vi.spyOn(document, 'cookie', 'set').mockImplementation((value: string) => {
      captured = value

      return value
    })
    updatesDismissedCookie('meshmap-lite').write('2026-06-15T12:00:00Z')
    setter.mockRestore()
    expect(captured).toContain('meshmap-lite.updates.meshmap-lite.dismissed_published_at=2026-06-15T12%3A00%3A00Z')
    expect(captured).toContain('Max-Age=315360000')
    expect(captured).toContain('Path=/')
    expect(captured).toContain('SameSite=Lax')
  })
})
