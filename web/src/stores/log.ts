import { create } from 'zustand'
import type { LogEvent } from '../api/types'

interface LogFilters {
  eventKinds: number[]
  channel: string
}

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
  prependLive: (item) => set((s) => {
    if (s.filters.channel && (item.channel_name ?? '') !== s.filters.channel) {
      return s
    }
    if (s.filters.eventKinds.length > 0 && !s.filters.eventKinds.includes(item.event_kind_value)) {
      return s
    }
    return { items: [item, ...s.items].slice(0, 1000) }
  }),
  setFilters: (filters) => set({ filters }),
  setLoadError: (msg) => set({ loadError: msg })
}))
