import { create } from 'zustand'

import { appendOlderChatMessages, chatStorageKey, pushChatMessage, readStoredChatChannel } from './chatState'

import type { ChatEvent } from '../api/types'

interface ChatState {
  channel: string
  messages: ChatEvent[]
  setChannel: (channel: string) => void
  setMessages: (items: ChatEvent[]) => void
  appendOlder: (items: ChatEvent[]) => void
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
  appendOlder: (items) => set((s) => ({ messages: appendOlderChatMessages(s.messages, items) })),
  pushMessage: (item) => set((s) => ({ messages: pushChatMessage(s.messages, item) }))
}))
