import { create } from 'zustand'
import type { Meta } from '../api/types'

interface MetaState {
  meta?: Meta
  setMeta: (meta: Meta) => void
}

export const useMetaStore = create<MetaState>((set) => ({
  meta: undefined,
  setMeta: (meta) => set({ meta })
}))
