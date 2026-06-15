import { create } from 'zustand'

import { countNewerReleases } from './metaSelectors'

import type { Meta } from '../api/types'

interface MetaState {
  meta?: Meta
  setMeta: (meta: Meta) => void
}

export const useMetaStore = create<MetaState>((set) => ({
  meta: undefined,
  setMeta: (meta) => set({ meta })
}))

function getState(): MetaState {
  return useMetaStore.getState()
}

export function unreadCountBySource(dismissedAt: Record<string, string>): Record<string, number> {
  const meta = getState().meta
  if (!meta?.update_check_sources) {return {}}

  const counts: Record<string, number> = {}
  for (const source of meta.update_check_sources) {
    counts[source.name] = countNewerReleases(source.releases, dismissedAt[source.name] ?? '')
  }

  return counts
}

export function totalUnreadCount(dismissedAt: Record<string, string>): number {
  const counts = unreadCountBySource(dismissedAt)

  return Object.values(counts).reduce((sum, value) => sum + value, 0)
}
