import { create } from 'zustand'
import type { ChatEvent } from '../api/types'

const key = 'meshmap-lite.chat.channel'

interface ChatState {
  channel: string
  messages: ChatEvent[]
  setChannel: (channel: string) => void
  setMessages: (items: ChatEvent[]) => void
  pushMessage: (item: ChatEvent) => void
}

export const useChatStore = create<ChatState>((set) => ({
  channel: localStorage.getItem(key) ?? '',
  messages: [],
  setChannel: (channel) => {
    localStorage.setItem(key, channel)
    set({ channel })
  },
  setMessages: (messages) => set({ messages }),
  pushMessage: (item) => set((s) => ({ messages: [item, ...s.messages].slice(0, 500) }))
}))
