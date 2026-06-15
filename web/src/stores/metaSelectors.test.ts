// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'

import { countNewerReleases } from './metaSelectors'

import type { SourceSummaryRelease } from '../api/types'

function buildRelease(overrides: Partial<SourceSummaryRelease> = {}): SourceSummaryRelease {
  return {
    version: 'v1.0.0',
    published_at: '2026-06-10T00:00:00Z',
    ...overrides
  }
}

describe('countNewerReleases', () => {
  it('treats an empty dismissed timestamp as "all releases are unread"', () => {
    const releases = [
      buildRelease({ version: 'v2.0.0' }),
      buildRelease({ version: 'v1.0.0' })
    ]
    expect(countNewerReleases(releases, '')).toBe(2)
  })

  it('returns 0 when the dismissed timestamp itself is invalid', () => {
    const releases = [buildRelease({ version: 'v2.0.0', published_at: '2026-06-15T12:00:00Z' })]
    expect(countNewerReleases(releases, 'not-a-date')).toBe(0)
  })

  it('counts only releases strictly newer than the dismissed timestamp', () => {
    const releases = [
      buildRelease({ version: 'v3.0.0', published_at: '2026-06-15T12:00:00Z' }),
      buildRelease({ version: 'v2.0.0', published_at: '2026-06-10T12:00:00Z' }),
      buildRelease({ version: 'v1.0.0', published_at: '2026-06-05T12:00:00Z' })
    ]
    expect(countNewerReleases(releases, '2026-06-10T18:00:00Z')).toBe(1)
  })

  it('skips releases with an invalid published_at timestamp', () => {
    const releases = [
      buildRelease({ version: 'v3.0.0', published_at: 'not-a-date' }),
      buildRelease({ version: 'v2.0.0', published_at: '2026-06-15T12:00:00Z' })
    ]
    expect(countNewerReleases(releases, '2026-06-10T00:00:00Z')).toBe(1)
  })

  it('returns 0 when the dismissed timestamp equals the newest published_at', () => {
    const releases = [
      buildRelease({ version: 'v2.0.0', published_at: '2026-06-15T12:00:00Z' })
    ]
    expect(countNewerReleases(releases, '2026-06-15T12:00:00Z')).toBe(0)
  })

  it('returns 0 for an empty release list', () => {
    expect(countNewerReleases([], '2026-06-15T12:00:00Z')).toBe(0)
  })
})
