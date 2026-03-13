import { create } from 'zustand'

import { type LogFilters, prependLiveLogItem } from './logState'

import type { LogEvent } from '../api/types'

interface LogState {
  items: LogEvent[]
  filters: LogFilters
  loadedOnce: boolean
  loadError: string
  setInitial: (items: LogEvent[]) => void
  appendOlder: (items: LogEvent[]) => void
  prependLive: (item: LogEvent) => void
  setFilters: (filters: LogFilters) => void
  setLoadError: (msg: string) => void
}

export const useLogStore = create<LogState>((set) => ({
  items: [],
  filters: { eventKinds: [], channel: '' },
  loadedOnce: false,
  loadError: '',
  setInitial: (items) => set({ items, loadedOnce: true, loadError: '' }),
  appendOlder: (items) => set((s) => ({ items: [...s.items, ...items] })),
  prependLive: (item) => set((s) => ({ items: prependLiveLogItem(s.items, s.filters, item) })),
  setFilters: (filters) => set({ filters }),
  setLoadError: (msg) => set({ loadError: msg })
}))
