import { create } from 'zustand'

import { api } from '../api/client'

import type { TopologyEdge } from '../api/types'

interface TopologyAllState {
  // The mode flag intentionally does NOT survive page navigation or
  // reload — keeping it in useState-only matches the project rule that
  // "show all topology" should be a transient, opt-in view.
  enabled: boolean
  edges: TopologyEdge[]
  truncated: boolean
  loading: boolean
  error: string
  lastFetchedAt: number
  setEnabled: (next: boolean) => void
  refresh: (signal?: AbortSignal) => Promise<void>
  reset: () => void
}

const initialState = {
  enabled: false,
  edges: [] as TopologyEdge[],
  truncated: false,
  loading: false,
  error: '',
  lastFetchedAt: 0
}

export const useTopologyAllStore = create<TopologyAllState>((set, get) => ({
  ...initialState,
  setEnabled: (next) => {
    if (next === get().enabled) {
      return
    }
    set({ enabled: next })
    if (!next) {
      return
    }
    // Fire-and-forget: errors land in the store; callers can read error/loading.
    void get().refresh()
  },
  refresh: async (signal) => {
    set({ loading: true, error: '' })
    try {
      const response = await api.topologyEdges({}, { signal })
      if (signal?.aborted) {
        return
      }
      set({
        edges: response.items,
        truncated: response.truncated,
        loading: false,
        error: '',
        lastFetchedAt: Date.now()
      })
    } catch (err) {
      if (signal?.aborted) {
        return
      }
      const message = err instanceof Error ? err.message : String(err)
      set({ loading: false, error: message })
    }
  },
  reset: () => set({ ...initialState })
}))
