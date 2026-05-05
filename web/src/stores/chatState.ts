import type { ChatEvent } from '../api/types'

export const chatStorageKey = 'meshmap-lite.chat.channel'

export function readStoredChatChannel(storage: Pick<Storage, 'getItem'>): string {
  return storage.getItem(chatStorageKey) ?? ''
}

export function pushChatMessage(messages: ChatEvent[], item: ChatEvent): ChatEvent[] {
  return [item, ...messages].slice(0, 500)
}

export function appendOlderChatMessages(messages: ChatEvent[], older: ChatEvent[]): ChatEvent[] {
  if (older.length === 0) {return messages}

  const seen = new Set(messages.map((item) => item.id))
  const uniqueOlder = older.filter((item) => !seen.has(item.id))

  return [...messages, ...uniqueOlder]
}
