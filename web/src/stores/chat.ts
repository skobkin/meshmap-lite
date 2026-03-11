import { create } from 'zustand'

import { chatStorageKey, pushChatMessage, readStoredChatChannel } from './chatState'

import type { ChatEvent } from '../api/types'

interface ChatState {
  channel: string
  messages: ChatEvent[]
  setChannel: (channel: string) => void
  setMessages: (items: ChatEvent[]) => void
  pushMessage: (item: ChatEvent) => void
}

export const useChatStore = create<ChatState>((set) => ({
  channel: readStoredChatChannel(localStorage),
  messages: [],
  setChannel: (channel) => {
    localStorage.setItem(chatStorageKey, channel)
    set({ channel })
  },
  setMessages: (messages) => set({ messages }),
  pushMessage: (item) => set((s) => ({ messages: pushChatMessage(s.messages, item) }))
}))
