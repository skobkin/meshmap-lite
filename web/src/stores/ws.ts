import { create } from 'zustand'
import type { WSState, WSStats } from '../api/types'

interface WSStore {
  state: WSState
  stats: WSStats | null
  setState: (state: WSState) => void
  setStats: (stats: WSStats) => void
}

export const useWSStore = create<WSStore>((set) => ({
  state: 'connecting',
  stats: null,
  setState: (state) => set({ state }),
  setStats: (stats) => set({ stats })
}))
