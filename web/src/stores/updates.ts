import { create } from 'zustand'

import type { UpdatesResponse } from '../api/types'

interface UpdatesState {
  bySource: Record<string, UpdatesResponse>
  loading: Record<string, boolean>
  errors: Record<string, string>
  setResponse: (source: string, response: UpdatesResponse) => void
  setLoading: (source: string, loading: boolean) => void
  setError: (source: string, error: string) => void
  reset: () => void
}

export const useUpdatesStore = create<UpdatesState>((set) => ({
  bySource: {},
  loading: {},
  errors: {},
  setResponse: (source, response) => set((state) => ({
    bySource: { ...state.bySource, [source]: response },
    errors: { ...state.errors, [source]: '' }
  })),
  setLoading: (source, loading) => set((state) => ({
    loading: { ...state.loading, [source]: loading }
  })),
  setError: (source, error) => set((state) => ({
    errors: { ...state.errors, [source]: error }
  })),
  reset: () => set({ bySource: {}, loading: {}, errors: {} })
}))
