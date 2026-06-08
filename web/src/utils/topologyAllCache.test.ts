import { describe, expect, it } from 'vitest'

import { isTopologyAllFresh } from './topologyAllCache'

describe('isTopologyAllFresh', () => {
  it('returns false when the snapshot has never been fetched', () => {
    expect(isTopologyAllFresh(0, 60_000)).toBe(false)
  })

  it('returns false when the TTL is non-positive', () => {
    expect(isTopologyAllFresh(Date.now() - 1000, 0)).toBe(false)
    expect(isTopologyAllFresh(Date.now() - 1000, -1)).toBe(false)
  })

  it('returns true within the TTL window', () => {
    const now = 1_700_000_000_000
    expect(isTopologyAllFresh(now - 5_000, 10_000, now)).toBe(true)
  })

  it('returns false once the TTL has elapsed', () => {
    const now = 1_700_000_000_000
    expect(isTopologyAllFresh(now - 30_000, 10_000, now)).toBe(false)
  })
})
