import { create } from 'zustand'

import type { MQTTConnectionStatus, WSState, WSStats } from '../api/types'

interface WSStore {
  state: WSState
  mqttStatus: MQTTConnectionStatus | null
  stats: WSStats | null
  setState: (state: WSState) => void
  setMQTTStatus: (status: MQTTConnectionStatus) => void
  setStats: (stats: WSStats) => void
}

export const useWSStore = create<WSStore>((set) => ({
  state: 'connecting',
  mqttStatus: null,
  stats: null,
  setState: (state) => set((current) => ({
    state,
    mqttStatus: state === 'connected' ? current.mqttStatus : null
  })),
  setMQTTStatus: (mqttStatus) => set({ mqttStatus }),
  setStats: (stats) => set({ stats })
}))
