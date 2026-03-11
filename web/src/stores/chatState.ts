import type { ChatEvent } from '../api/types'

export const chatStorageKey = 'meshmap-lite.chat.channel'

export function readStoredChatChannel(storage: Pick<Storage, 'getItem'>): string {
  return storage.getItem(chatStorageKey) ?? ''
}

export function pushChatMessage(messages: ChatEvent[], item: ChatEvent): ChatEvent[] {
  return [item, ...messages].slice(0, 500)
}
