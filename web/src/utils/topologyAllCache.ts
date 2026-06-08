// isTopologyAllFresh checks whether the in-memory "show all topology" snapshot
// is still within the meta-supplied TTL. It lives outside the zustand store
// so it can be unit-tested without pulling in the React adapter.
export function isTopologyAllFresh(lastFetchedAt: number, ttlMs: number, now: number = Date.now()): boolean {
  if (lastFetchedAt === 0 || ttlMs <= 0) {
    return false
  }

  return now - lastFetchedAt < ttlMs
}
