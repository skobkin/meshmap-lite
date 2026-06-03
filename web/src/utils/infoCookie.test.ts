// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'

import { infoDismissedCookieName, readInfoDismissedSourceHash } from './infoCookie'

describe('info dismissed cookie helpers', () => {
  it('reads the dismissed source hash from a cookie string', () => {
    expect(readInfoDismissedSourceHash(`theme=dark; ${infoDismissedCookieName}=abc%20123`)).toBe('abc 123')
  })

  it('returns an empty string when the cookie is absent', () => {
    expect(readInfoDismissedSourceHash('theme=dark')).toBe('')
  })
})
