import { useNodeStore } from '../stores/nodes'

import type { MapNode, NodeDetails, NodeSummary } from '../api/types'
import type { ComponentChildren, JSX } from 'preact'

export interface ResolvedNodeDataValue {
  nodeId: string
  label: string
  title?: string
  resolved: boolean
  details?: NodeDetails
  mapNode?: MapNode
  summary?: NodeSummary
}

interface Props {
  nodeId: string
  children: (value: ResolvedNodeDataValue) => ComponentChildren
}

function bestNodeLabel(longName?: string, shortName?: string, fallback?: string): string | undefined {
  const longLabel = longName?.trim()
  if (longLabel) {
    return longLabel
  }
  const shortLabel = shortName?.trim()
  if (shortLabel) {
    return shortLabel
  }

  return fallback?.trim() ?? undefined
}

export function resolveNodeDataValue(
  nodeId: string,
  details?: NodeDetails,
  mapNode?: MapNode,
  summary?: NodeSummary
): ResolvedNodeDataValue {
  const detailsLabel = details?.node.node_id === nodeId
    ? bestNodeLabel(details.node.long_name, details.node.short_name)
    : undefined
  const mapNodeLabel = mapNode?.node.node_id === nodeId
    ? bestNodeLabel(mapNode.node.long_name, mapNode.node.short_name)
    : undefined
  const summaryLabel = summary?.node_id === nodeId
    ? bestNodeLabel(summary.long_name, summary.short_name, summary.display_name)
    : undefined
  const label = detailsLabel ?? mapNodeLabel ?? summaryLabel ?? nodeId
  const resolved = label !== nodeId

  return {
    nodeId,
    label,
    title: resolved ? nodeId : undefined,
    resolved,
    details: details?.node.node_id === nodeId ? details : undefined,
    mapNode: mapNode?.node.node_id === nodeId ? mapNode : undefined,
    summary: summary?.node_id === nodeId ? summary : undefined
  }
}

export function ResolvedNodeData({ nodeId, children }: Props): JSX.Element {
  const details = useNodeStore((state) => (
    state.details?.node.node_id === nodeId
      ? state.details
      : undefined
  ))
  const mapNode = useNodeStore((state) => state.mapNodes.find((item) => item.node.node_id === nodeId))
  const summary = useNodeStore((state) => state.summaries.find((item) => item.node_id === nodeId))

  return <>{children(resolveNodeDataValue(nodeId, details, mapNode, summary))}</>
}
