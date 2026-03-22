import { create } from 'zustand'

import { upsertMapNode, upsertNode, upsertPosition, upsertTelemetry } from './nodeState'

import type { MapNode, Node, NodeDetails, NodePosition, NodeSummary, NodeTelemetry } from '../api/types'

interface NodeState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
  setMapNodes: (items: MapNode[]) => void
  upsertMapNode: (item: MapNode) => void
  upsertNode: (node: Node) => void
  upsertPosition: (position: NodePosition) => void
  upsertTelemetry: (telemetry: NodeTelemetry) => void
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
  upsertMapNode: (item) => set((s) => ({ mapNodes: upsertMapNode(s.mapNodes, item) })),
  upsertNode: (node) => set((s) => ({ mapNodes: upsertNode(s.mapNodes, node) })),
  upsertPosition: (position) => set((s) => ({ mapNodes: upsertPosition(s.mapNodes, position) })),
  upsertTelemetry: (telemetry) => set((s) => ({ mapNodes: upsertTelemetry(s.mapNodes, telemetry) })),

  setSummaries: (items) => set({ summaries: items }),
  setSelectedId: (id) => set({ selectedId: id }),
  setDetails: (d) => set({ details: d })
}))
