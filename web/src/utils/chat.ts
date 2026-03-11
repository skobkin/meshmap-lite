import type { ChatEvent } from '../api/types'

export function chatNodeLabel(message: ChatEvent): string {
  return message.node_display_name ?? message.node_id ?? 'system'
}
