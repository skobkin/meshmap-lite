import { Fragment } from 'preact'
import { useCallback, useEffect, useMemo, useRef } from 'preact/hooks'

import { MapTopologyToggle } from '../components/MapTopologyToggle'
import { ResolvedNodeData } from '../components/ResolvedNodeData'
import { LeafletMapAdapter } from '../maps/leafletMap'
import { useChatStore } from '../stores/chat'
import { useNodeStore } from '../stores/nodes'
import { type ChatMessageEntry, type ChatReaction, type ChatReactionEntry, groupChatEvents } from '../utils/chat'
import { classifyHops } from '../utils/signal'
import { dayKey, dayLabel, hhmm } from '../utils/time'
import { TOPOLOGY_COLOR, sortedNeighbors } from '../utils/topology'

import type { ChatEvent, MapPrecisionCirclesMode, NodeDetails, TopologyEdge } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  center: [number, number]
  zoom: number
  clustering: boolean
  precisionCirclesMode: MapPrecisionCirclesMode
  channels: string[]
  disconnectedThreshold?: string
  focusNodeId?: string
  topologyDetails?: NodeDetails
  topologyNodeId?: string
  topologyAllEnabled: boolean
  topologyAllLoading: boolean
  topologyAllCount?: number
  topologyAllTruncated: boolean
  topologyAllEdges: TopologyEdge[]
  chatPanel?: 'open' | 'collapsed'
  channel?: string
  onFocusNodeHandled: () => void
  onChatPanelChange?: (state: 'open' | 'collapsed') => void
  onChannelChange?: (channel: string) => void
  onHoverTopologyNode: (id?: string) => void
  onLoadMoreChat: () => void
  onOpenNodeDetails: (id: string) => void
  onSelectNode?: (id?: string) => void
  onToggleTopologyAll: (next: boolean) => void
  onViewChange: (center: [number, number], zoom: number) => void
  chatHasMore: boolean
  chatLoadingMore: boolean
  chatLoadMoreError: string
}

function hoverTopologyEnabled(): boolean {
  if (typeof window.matchMedia !== 'function') {
    return true
  }

  return window.matchMedia('(hover: hover) and (pointer: fine)').matches
}

interface ChatTimelineOptions {
  onSelectNode: (id: string) => void
  onOpenNodeDetails: (id: string) => void
  systemText: (code?: string) => string
}

function ChatGatewayLink({
  nodeId,
  fallbackLabel,
  onOpenNodeDetails
}: {
  nodeId: string
  fallbackLabel?: string
  onOpenNodeDetails: (id: string) => void
}): JSX.Element {
  return (
    <ResolvedNodeData nodeId={nodeId} fallbackLabel={fallbackLabel}>
      {({ label, title }) => {
        const tooltip = `Gateway: ${title ? `${label} (${title})` : label}`

        return (
          <button
            type="button"
            className="chat-gateway-link"
            title={tooltip}
            aria-label={tooltip}
            onClick={() => onOpenNodeDetails(nodeId)}
          >
            <svg aria-hidden="true" viewBox="0 0 16 16" focusable="false">
              <path d="M8 9.2 5.8 14h4.4L8 9.2Z" />
              <path d="M8 2.2v7" />
              <path d="M5.5 4.2a3.5 3.5 0 0 0 0 4.9M10.5 4.2a3.5 3.5 0 0 1 0 4.9" />
              <path d="M3.5 2.4a6.2 6.2 0 0 0 0 8.6M12.5 2.4a6.2 6.2 0 0 1 0 8.6" />
            </svg>
          </button>
        )
      }}
    </ResolvedNodeData>
  )
}

function ChatReactionPill({
  reaction,
  onSelectNode
}: {
  reaction: ChatReaction
  onSelectNode: (id: string) => void
}): JSX.Element {
  const reactors = reaction.reactors
  const count = reactors.length

  // PicoCSS <details>/<summary> provides accessible keyboard toggle; we also
  // open on focus/blur to mirror mouse hover. No JS state is needed.
  return (
    <details className="chat-reaction-pill">
      <summary
        aria-label={`${reaction.emoji} ${count} ${count === 1 ? 'reaction' : 'reactions'}`}
        title={reactors.map((r) => r.displayName).join(', ')}
      >
        <span className="chat-reaction-emoji" aria-hidden="true">{reaction.emoji}</span>
        <span className="chat-reaction-count">{count}</span>
      </summary>
      <ul className="chat-reaction-reactors">
        {reactors.map((r) => (
          <li key={`${r.nodeId}-${r.observedAt}`}>
            {r.nodeId ? (
              <button type="button" className="chat-reaction-reactor" onClick={() => onSelectNode(r.nodeId)}>
                {r.displayName}
              </button>
            ) : (
              <span className="chat-reaction-reactor">{r.displayName}</span>
            )}
          </li>
        ))}
      </ul>
    </details>
  )
}

function renderChatTimeline(
  messages: ChatEvent[],
  { onOpenNodeDetails, onSelectNode, systemText }: ChatTimelineOptions
): JSX.Element[] {
  const entries = groupChatEvents(messages)
  const out: JSX.Element[] = []
  let previousDay = ''

  for (const entry of entries) {
    if (entry.kind === 'reaction') {
      out.push(renderOrphanedReaction(entry, previousDay))
      // An orphaned reaction is just a hint pill; do not advance the day
      // separator (keep the day of the last real message), so previousDay
      // is intentionally left untouched.
      continue
    }
    const m = entry.event
    const currentDay = dayKey(m.observed_at)
    const needsSeparator = currentDay !== previousDay
    previousDay = currentDay
    out.push(renderMessageEntry(entry, needsSeparator, { onOpenNodeDetails, onSelectNode, systemText }))
  }

  return out
}

function renderOrphanedReaction(entry: ChatReactionEntry, _previousDay: string): JSX.Element {
  // Renders a compact hint that a reaction exists whose target message is
  // not in the current 500-row window. Hovering/focusing reveals the
  // reactor.
  return (
    <div className="chat-message chat-message-orphan" key={entry.event.id}>
      <p className="chat-orphan">
        <code>{hhmm(entry.event.observed_at)}</code>{' '}
        <span className="chat-orphan-emoji" aria-hidden="true">{entry.event.reaction_emoji}</span>{' '}
        <span className="muted">{entry.event.node_display_name ?? entry.event.node_id ?? 'unknown'}</span>
        <span className="muted"> {'\u00b7 reacting to an earlier message'}</span>
      </p>
    </div>
  )
}

function renderMessageEntry(
  entry: ChatMessageEntry,
  needsSeparator: boolean,
  { onOpenNodeDetails, onSelectNode, systemText }: ChatTimelineOptions
): JSX.Element {
  const m = entry.event
  const isNodeClickable = typeof m.node_id === 'string'
  const showUploader = typeof m.mqtt_uploader_node_id === 'string' &&
    m.mqtt_uploader_node_id.length > 0 &&
    m.mqtt_uploader_node_id !== m.node_id
  const hopsInfo = classifyHops(m.hop_start, m.hop_limit)
  const showHops = hopsInfo.traversed !== undefined && hopsInfo.traversed > 0

  return (
    <Fragment key={m.id}>
      {needsSeparator && (
        <div className="chat-day-separator" role="separator" aria-label={dayLabel(m.observed_at)}>
          <span>{dayLabel(m.observed_at)}</span>
        </div>
      )}
      <div className="chat-message">
        <p className={m.event_type === 'system' ? 'system' : ''}>
          <code>{hhmm(m.observed_at)}</code>{' '}
          {isNodeClickable && m.node_id ? (
            <ResolvedNodeData nodeId={m.node_id} fallbackLabel={m.node_display_name}>
              {({ label, title }) => (
                <button
                  type="button"
                  className="chat-node-link"
                  title={title}
                  onClick={() => onSelectNode(m.node_id!)}
                >
                  <mark>{label}</mark>
                </button>
              )}
            </ResolvedNodeData>
          ) : (
            <mark>{m.node_display_name ?? 'system'}</mark>
          )}
          {showUploader && (
            <ChatGatewayLink
              nodeId={m.mqtt_uploader_node_id!}
              fallbackLabel={m.mqtt_uploader_display_name}
              onOpenNodeDetails={onOpenNodeDetails}
            />
          )}{' '}
          {showHops && (
            <span
              className={`chat-hop-badge ${hopsInfo.qualityClass}`.trim()}
              title={hopsInfo.title}
            >
              ↓{hopsInfo.traversed}
            </span>
          )}
          {m.event_type === 'system' ? systemText(m.system_code) : (m.message_text ?? '')}
        </p>
        {entry.reactions.length > 0 && (
          <div className="chat-reactions" role="list">
            {entry.reactions.map((r) => (
              <ChatReactionPill key={`${m.id}-${r.emoji}`} reaction={r} onSelectNode={onSelectNode} />
            ))}
          </div>
        )}
      </div>
    </Fragment>
  )
}

export function MapPage({
  center,
  zoom,
  clustering,
  precisionCirclesMode,
  channels,
  disconnectedThreshold,
  focusNodeId,
  topologyDetails,
  topologyNodeId,
  topologyAllEnabled,
  topologyAllLoading,
  topologyAllCount,
  topologyAllTruncated,
  topologyAllEdges,
  chatPanel = 'open',
  channel = '',
  onFocusNodeHandled,
  onChatPanelChange = () => undefined,
  onChannelChange = () => undefined,
  onHoverTopologyNode,
  onLoadMoreChat,
  onOpenNodeDetails,
  onSelectNode = () => undefined,
  onToggleTopologyAll,
  onViewChange,
  chatHasMore,
  chatLoadingMore,
  chatLoadMoreError
}: Props): JSX.Element {
  const ref = useRef<HTMLDivElement>(null)
  const adapterRef = useRef<LeafletMapAdapter | null>(null)
  const onOpenNodeDetailsRef = useRef(onOpenNodeDetails)
  const onSelectNodeRef = useRef(onSelectNode)
  const onViewChangeRef = useRef(onViewChange)
  const initialCenterRef = useRef(center)
  const initialZoomRef = useRef(zoom)
  const hoverTimerRef = useRef<number>()
  const hoverEnabledRef = useRef(hoverTopologyEnabled())
  const nodes = useNodeStore((s) => s.mapNodes)
  const selectedId = useNodeStore((s) => s.selectedId)
  const chat = useChatStore((s) => s.messages)
  const collapsed = chatPanel === 'collapsed'
  const activeNeighbors = useMemo(() => sortedNeighbors(topologyDetails), [topologyDetails])

  useEffect(() => {
    onOpenNodeDetailsRef.current = onOpenNodeDetails
    onSelectNodeRef.current = onSelectNode
    onViewChangeRef.current = onViewChange
  }, [onOpenNodeDetails, onSelectNode, onViewChange])

  const clearHoverTimer = (): void => {
    if (hoverTimerRef.current) {
      window.clearTimeout(hoverTimerRef.current)
      hoverTimerRef.current = undefined
    }
  }

  const handleHoverNode = useCallback((id?: string): void => {
    if (!hoverEnabledRef.current) {
      return
    }

    clearHoverTimer()
    if (!id) {
      onHoverTopologyNode(undefined)

      return
    }

    hoverTimerRef.current = window.setTimeout(() => {
      onHoverTopologyNode(id)
      hoverTimerRef.current = undefined
    }, 200)
  }, [onHoverTopologyNode])

  const toggleCollapsed = (): void => {
    onChatPanelChange(collapsed ? 'open' : 'collapsed')
  }

  useEffect(() => {
    if (!ref.current) {return}
    adapterRef.current = new LeafletMapAdapter(ref.current, initialCenterRef.current, initialZoomRef.current, {
      clustering,
      onHoverNode: handleHoverNode,
      precisionCirclesMode,
      onOpenNodeDetails: (id) => onOpenNodeDetailsRef.current(id),
      onViewChange: (nextCenter, nextZoom) => onViewChangeRef.current(nextCenter, nextZoom),
      onSelectNode: (id) => onSelectNodeRef.current(id)
    })

    return () => {
      clearHoverTimer()
      onHoverTopologyNode(undefined)
      adapterRef.current?.destroy()
      adapterRef.current = null
    }
  }, [clustering, handleHoverNode, onHoverTopologyNode, precisionCirclesMode])

  useEffect(() => {
    adapterRef.current?.setView(center, zoom)
  }, [center, zoom])

  useEffect(() => {
    adapterRef.current?.render(nodes, disconnectedThreshold)
  }, [nodes, disconnectedThreshold])

  useEffect(() => {
    adapterRef.current?.renderTopology(topologyNodeId, activeNeighbors)
  }, [activeNeighbors, topologyNodeId])

  useEffect(() => {
    adapterRef.current?.renderAllTopology(topologyAllEnabled ? topologyAllEdges : [])
  }, [topologyAllEdges, topologyAllEnabled])

  useEffect(() => {
    adapterRef.current?.setSelectedNode(selectedId)
  }, [selectedId])

  useEffect(() => {
    if (!focusNodeId) {return}
    if (!nodes.some((item) => item.node.node_id === focusNodeId && item.position)) {return}
    adapterRef.current?.focusNode(focusNodeId)
    onFocusNodeHandled()
  }, [focusNodeId, nodes, onFocusNodeHandled])

  const focusNodeFromChat = (id: string): void => {
    const mapNode = nodes.find((item) => item.node.node_id === id)
    if (mapNode?.position) {
      onSelectNode(id)
      adapterRef.current?.focusNode(id)

      return
    }
    onOpenNodeDetails(id)
  }

  const systemText = (code?: string): string => {
    switch (code) {
      case undefined:
        return 'System event'
      case 'node_discovered':
        return 'New node discovered'
      default:
        return 'System event'
    }
  }

  return (
    <section className={`map-layout${collapsed ? ' chat-collapsed' : ''}`}>
      <div className="map-stage">
        <div className="map-canvas" ref={ref} />
        {(topologyNodeId && activeNeighbors.some((item) => item.has_position)) || topologyAllEnabled ? (
          <aside className="map-topology-legend" aria-label="Topology legend">
            <strong>Topology</strong>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.inferred }} /> Inferred</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.mqttDirect }} /> MQTT direct</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.noSNR }} /> Neighbor info</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.poor }} /> Poor SNR</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.fair }} /> Fair SNR</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.good }} /> Good SNR</span>
          </aside>
        ) : null}
        <MapTopologyToggle
          enabled={topologyAllEnabled}
          loading={topologyAllLoading}
          {...(typeof topologyAllCount === 'number' ? { count: topologyAllCount } : {})}
          truncated={topologyAllTruncated}
          onToggle={onToggleTopologyAll}
        />
        {collapsed && (
          <button
            type="button"
            className="secondary outline collapse-toggle map-chat-toggle"
            onClick={toggleCollapsed}
            aria-label="Expand chat sidebar"
            title="Expand chat sidebar"
          >
            <span aria-hidden="true">{'<'}</span>
          </button>
        )}
      </div>
      {!collapsed && (
        <aside className="chat-panel">
          <div className="chat-panel-head">
            <select aria-label="Channel" value={channel} onChange={(e) => onChannelChange((e.target as HTMLSelectElement).value)}>
              {channels.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
            <button
              type="button"
              className="secondary outline collapse-toggle"
              onClick={toggleCollapsed}
              aria-label="Collapse chat sidebar"
              title="Collapse chat sidebar"
            >
              <span aria-hidden="true">{'>'}</span>
            </button>
          </div>
          <div className="chat-list">
            {renderChatTimeline(chat, {
              onOpenNodeDetails,
              onSelectNode: focusNodeFromChat,
              systemText
            })}
            {chatLoadMoreError && <p className="chat-load-error" role="alert">{chatLoadMoreError}</p>}
            {chatHasMore && (
              <button
                type="button"
                className="secondary chat-load-more"
                disabled={chatLoadingMore}
                onClick={onLoadMoreChat}
              >
                {chatLoadingMore ? 'Loading...' : 'Load more'}
              </button>
            )}
          </div>
        </aside>
      )}
    </section>
  )
}
