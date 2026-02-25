import { create } from 'zustand'
import type { MapNode, Node, NodeDetails, NodePosition, NodeSummary } from '../api/types'

interface NodeState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
  setMapNodes: (items: MapNode[]) => void
  upsertMapNode: (item: MapNode) => void
  upsertNode: (node: Node) => void
  upsertPosition: (position: NodePosition) => void
  setSummaries: (items: NodeSummary[]) => void
  setSelectedId: (id?: string) => void
  setDetails: (d?: NodeDetails) => void
}

export const useNodeStore = create<NodeState>((set) => ({
  mapNodes: [],
  summaries: [],
  selectedId: undefined,
  details: undefined,
  setMapNodes: (items) => set({ mapNodes: items }),
  upsertMapNode: (item) => set((s) => {
    const idx = s.mapNodes.findIndex((n) => n.node.node_id === item.node.node_id)
    if (idx < 0) {
      return { mapNodes: [item, ...s.mapNodes] }
    }
    const clone = s.mapNodes.slice()
    clone[idx] = item
    return { mapNodes: clone }
  }),
  upsertNode: (node) => set((s) => {
    const idx = s.mapNodes.findIndex((n) => n.node.node_id === node.node_id)
    if (idx < 0) {
      return { mapNodes: [{ node }, ...s.mapNodes] }
    }
    const clone = s.mapNodes.slice()
    clone[idx] = { ...clone[idx], node }
    return { mapNodes: clone }
  }),
  upsertPosition: (position) => set((s) => {
    const idx = s.mapNodes.findIndex((n) => n.node.node_id === position.node_id)
    if (idx < 0) {
      const stubNode: Node = {
        node_id: position.node_id,
        last_seen_any_event_at: position.observed_at
      }
      return { mapNodes: [{ node: stubNode, position }, ...s.mapNodes] }
    }
    const clone = s.mapNodes.slice()
    clone[idx] = { ...clone[idx], position }
    return { mapNodes: clone }
  }),
  setSummaries: (items) => set({ summaries: items }),
  setSelectedId: (id) => set({ selectedId: id }),
  setDetails: (d) => set({ details: d })
}))
