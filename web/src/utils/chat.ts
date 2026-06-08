import type { ChatEvent } from '../api/types'

export interface ChatReaction {
  emoji: string
  reactors: ChatReactor[]
}

export interface ChatReactor {
  nodeId: string
  displayName: string
  observedAt: string
}

export interface ChatMessageEntry {
  kind: 'message' | 'system'
  event: ChatEvent
  reactions: ChatReaction[]
  orphanedReactions: boolean
}

export interface ChatReactionEntry {
  kind: 'reaction'
  event: ChatEvent
  orphaned: boolean
}

export type ChatTimelineEntry = ChatMessageEntry | ChatReactionEntry

function reactorFor(event: ChatEvent): ChatReactor {
  const nodeId = event.node_id ?? ''

  return {
    nodeId,
    displayName: event.node_display_name ?? (nodeId !== '' ? nodeId : 'unknown'),
    observedAt: event.observed_at
  }
}

// Reactions carry the same emoji verbatim from the wire payload, but two
// distinct reactions from the same node on the same target packet are still
// separate events in the DB. Group by emoji only, aggregating all reactors
// (a self-reaction still appears under the same emoji pill).
function aggregateReactions(reactions: ChatEvent[]): ChatReaction[] {
  if (reactions.length === 0) {
    return []
  }
  const sorted = [...reactions].sort((a, b) => a.observed_at.localeCompare(b.observed_at))
  const order: string[] = []
  const byEmoji = new Map<string, ChatReactor[]>()
  for (const r of sorted) {
    const emoji = r.reaction_emoji ?? ''
    let list = byEmoji.get(emoji)
    if (!list) {
      list = []
      byEmoji.set(emoji, list)
      order.push(emoji)
    }
    list.push(reactorFor(r))
  }

  return order.map((emoji) => ({ emoji, reactors: byEmoji.get(emoji) ?? [] }))
}

// Compose a flat chat timeline into a list of entries the UI can render
// without further grouping. System and message rows are always preserved as
// their own entries. Reactions are folded into the message they target when
// that message is in scope; otherwise they surface as standalone orphaned
// entries so the UI can hint at the missing target.
export function groupChatEvents(events: ChatEvent[]): ChatTimelineEntry[] {
  const byPacketID = new Map<number, ChatEvent[]>()
  const messageOrder: number[] = []
  const out: ChatTimelineEntry[] = []

  // First pass: walk the timeline in the order it was provided (caller is
  // responsible for any sort, e.g. newest-first) and seed the entry list
  // with messages / system rows plus orphaned reactions.
  for (const event of events) {
    if (event.event_type === 'reaction') {
      const target = event.reply_to_packet_id
      if (target === undefined) {
        out.push({ kind: 'reaction', event, orphaned: true })
        continue
      }
      let bucket = byPacketID.get(target)
      if (!bucket) {
        bucket = []
        byPacketID.set(target, bucket)
      }
      bucket.push(event)
      continue
    }
    out.push({
      kind: event.event_type === 'system' ? 'system' : 'message',
      event,
      reactions: [],
      orphanedReactions: false
    })
    messageOrder.push(event.id)
  }

  // Second pass: attach reactions to their message. The store keeps a single
  // map of packet_id -> reactions; for a given message entry, look up the
  // packet_id (PacketID is the join key) and fold in any reactions that were
  // buffered. Messages without a packet id (e.g. system rows) never match.
  for (const entry of out) {
    if (entry.kind === 'reaction') {
      continue
    }
    const packetID = entry.event.packet_id
    if (packetID === undefined) {
      continue
    }
    const reactions = byPacketID.get(packetID)
    if (!reactions || reactions.length === 0) {
      continue
    }
    entry.reactions = aggregateReactions(reactions)
    byPacketID.delete(packetID)
  }

  // Third pass: any leftover reactions target packet ids that are not in the
  // current window. Surface them as orphaned entries appended at the end so
  // the user still sees the reaction (with an "earlier message" hint).
  for (const [, leftover] of byPacketID) {
    for (const r of leftover) {
      out.push({ kind: 'reaction', event: r, orphaned: true })
    }
  }
  // Touch messageOrder so the variable is read even when no reactions remain;
  // this keeps the function shape stable for future per-message lookups.
  void messageOrder

  return out
}
