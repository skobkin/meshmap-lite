import { Fragment } from 'preact'
import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'

import { ResolvedNodeData } from '../components/ResolvedNodeData'
import { LeafletMapAdapter } from '../maps/leafletMap'
import { useChatStore } from '../stores/chat'
import { useNodeStore } from '../stores/nodes'
import { dayKey, dayLabel, hhmm } from '../utils/time'
import { TOPOLOGY_COLOR, sortedNeighbors } from '../utils/topology'

import type { ChatEvent, MapPrecisionCirclesMode, NodeDetails } from '../api/types'
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
  onFocusNodeHandled: () => void
  onHoverTopologyNode: (id?: string) => void
  onLoadMoreChat: () => void
  onOpenNodeDetails: (id: string) => void
  onViewChange: (center: [number, number], zoom: number) => void
  chatHasMore: boolean
  chatLoadingMore: boolean
  chatLoadMoreError: string
}

const sidebarStateKey = 'meshmap-lite.map.chat.collapsed'

function readSidebarState(): boolean {
  return localStorage.getItem(sidebarStateKey) === '1'
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

function renderChatTimeline(messages: ChatEvent[], { onOpenNodeDetails, onSelectNode, systemText }: ChatTimelineOptions): JSX.Element[] {
  let previousDay = ''

  return messages.map((m) => {
    const currentDay = dayKey(m.observed_at)
    const needsSeparator = currentDay !== previousDay
    previousDay = currentDay
    const isNodeClickable = typeof m.node_id === 'string'
    const showUploader = typeof m.mqtt_uploader_node_id === 'string' &&
      m.mqtt_uploader_node_id.length > 0 &&
      m.mqtt_uploader_node_id !== m.node_id

    return (
      <Fragment key={m.id}>
        {needsSeparator && (
          <div className="chat-day-separator" role="separator" aria-label={dayLabel(m.observed_at)}>
            <span>{dayLabel(m.observed_at)}</span>
          </div>
        )}
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
          {m.event_type === 'system' ? systemText(m.system_code) : (m.message_text ?? '')}
        </p>
      </Fragment>
    )
  })
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
  onFocusNodeHandled,
  onHoverTopologyNode,
  onLoadMoreChat,
  onOpenNodeDetails,
  onViewChange,
  chatHasMore,
  chatLoadingMore,
  chatLoadMoreError
}: Props): JSX.Element {
  const ref = useRef<HTMLDivElement>(null)
  const adapterRef = useRef<LeafletMapAdapter | null>(null)
  const initialCenterRef = useRef(center)
  const initialZoomRef = useRef(zoom)
  const hoverTimerRef = useRef<number>()
  const hoverEnabledRef = useRef(hoverTopologyEnabled())
  const nodes = useNodeStore((s) => s.mapNodes)
  const selectedId = useNodeStore((s) => s.selectedId)
  const setSelectedId = useNodeStore((s) => s.setSelectedId)
  const chat = useChatStore((s) => s.messages)
  const channel = useChatStore((s) => s.channel)
  const setChannel = useChatStore((s) => s.setChannel)
  const [collapsed, setCollapsed] = useState<boolean>(() => readSidebarState())
  const activeNeighbors = useMemo(() => sortedNeighbors(topologyDetails), [topologyDetails])

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
    const next = !collapsed
    setCollapsed(next)
    localStorage.setItem(sidebarStateKey, next ? '1' : '0')
  }

  useEffect(() => {
    if (!ref.current) {return}
    adapterRef.current = new LeafletMapAdapter(ref.current, initialCenterRef.current, initialZoomRef.current, {
      clustering,
      onHoverNode: handleHoverNode,
      precisionCirclesMode,
      onOpenNodeDetails,
      onViewChange,
      onSelectNode: setSelectedId
    })

    return () => {
      clearHoverTimer()
      onHoverTopologyNode(undefined)
      adapterRef.current?.destroy()
      adapterRef.current = null
    }
  }, [clustering, handleHoverNode, onHoverTopologyNode, onOpenNodeDetails, onViewChange, precisionCirclesMode, setSelectedId])

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
    adapterRef.current?.setSelectedNode(selectedId)
  }, [selectedId])

  useEffect(() => {
    if (!focusNodeId) {return}
    adapterRef.current?.focusNode(focusNodeId)
    onFocusNodeHandled()
  }, [focusNodeId, onFocusNodeHandled])

  const focusNodeFromChat = (id: string): void => {
    const mapNode = nodes.find((item) => item.node.node_id === id)
    if (mapNode?.position) {
      setSelectedId(id)
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
        {topologyNodeId && activeNeighbors.some((item) => item.has_position) && (
          <aside className="map-topology-legend" aria-label="Topology legend">
            <strong>Topology</strong>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.inferred }} /> Inferred</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.mqttDirect }} /> MQTT direct</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.noSNR }} /> Neighbor info</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.poor }} /> Poor SNR</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.fair }} /> Fair SNR</span>
            <span><i style={{ backgroundColor: TOPOLOGY_COLOR.good }} /> Good SNR</span>
          </aside>
        )}
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
            <select aria-label="Channel" value={channel} onChange={(e) => setChannel((e.target as HTMLSelectElement).value)}>
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
