import { Fragment } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import { LeafletMapAdapter } from '../maps/leafletMap'
import { useNodeStore } from '../stores/nodes'
import { useChatStore } from '../stores/chat'
import { dayKey, dayLabel, hhmm } from '../utils/time'
import type { ChatEvent, MapPrecisionCirclesMode } from '../api/types'

interface Props {
  center: [number, number]
  zoom: number
  clustering: boolean
  precisionCirclesMode: MapPrecisionCirclesMode
  channels: string[]
  disconnectedThreshold?: string
  onOpenNodeDetails: (id: string) => void
  onViewChange: (center: [number, number], zoom: number) => void
}

const sidebarStateKey = 'meshmap-lite.map.chat.collapsed'

function readSidebarState(): boolean {
  return localStorage.getItem(sidebarStateKey) === '1'
}

interface ChatTimelineOptions {
  nodeNameByID: Map<string, string>
  onSelectNode: (id: string) => void
  systemText: (code?: string) => string
}

function renderChatTimeline(messages: ChatEvent[], { nodeNameByID, onSelectNode, systemText }: ChatTimelineOptions) {
  let previousDay = ''

  return messages.map((m) => {
    const currentDay = dayKey(m.observed_at)
    const needsSeparator = currentDay !== previousDay
    previousDay = currentDay
    const isNodeClickable = typeof m.node_id === 'string'
    const nodeLabel = m.node_id ? (nodeNameByID.get(m.node_id) ?? m.node_id) : 'system'

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
            <button type="button" className="chat-node-link" onClick={() => onSelectNode(m.node_id!)}>
              <mark>{nodeLabel}</mark>
            </button>
          ) : (
            <mark>{nodeLabel}</mark>
          )}{' '}
          {m.event_type === 'system' ? systemText(m.system_code) : (m.message_text ?? '')}
        </p>
      </Fragment>
    )
  })
}

export function MapPage({ center, zoom, clustering, precisionCirclesMode, channels, disconnectedThreshold, onOpenNodeDetails, onViewChange }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const adapterRef = useRef<LeafletMapAdapter | null>(null)
  const nodes = useNodeStore((s) => s.mapNodes)
  const selectedId = useNodeStore((s) => s.selectedId)
  const setSelectedId = useNodeStore((s) => s.setSelectedId)
  const chat = useChatStore((s) => s.messages)
  const channel = useChatStore((s) => s.channel)
  const setChannel = useChatStore((s) => s.setChannel)
  const [collapsed, setCollapsed] = useState<boolean>(() => readSidebarState())

  const toggleCollapsed = () => {
    const next = !collapsed
    setCollapsed(next)
    localStorage.setItem(sidebarStateKey, next ? '1' : '0')
  }

  useEffect(() => {
    if (!ref.current) return
    adapterRef.current = new LeafletMapAdapter(ref.current, center, zoom, {
      clustering,
      precisionCirclesMode,
      onOpenNodeDetails,
      onViewChange,
      onSelectNode: setSelectedId
    })
    return () => {
      adapterRef.current?.destroy()
      adapterRef.current = null
    }
  }, [clustering, onOpenNodeDetails, onViewChange, precisionCirclesMode, setSelectedId])

  useEffect(() => {
    adapterRef.current?.setView(center, zoom)
  }, [center[0], center[1], zoom])

  useEffect(() => {
    adapterRef.current?.render(nodes, disconnectedThreshold)
  }, [nodes, disconnectedThreshold])

  useEffect(() => {
    adapterRef.current?.setSelectedNode(selectedId)
  }, [selectedId])

  const focusNodeFromChat = (id: string) => {
    const mapNode = nodes.find((item) => item.node.node_id === id)
    if (mapNode?.position) {
      setSelectedId(id)
      adapterRef.current?.focusNode(id)
      return
    }
    onOpenNodeDetails(id)
  }

  const nodeNameByID = new Map<string, string>()
  for (const item of nodes) {
    const node = item.node
    nodeNameByID.set(node.node_id, node.long_name || node.short_name || node.node_id)
  }

  const systemText = (code?: string): string => {
    switch (code) {
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
              nodeNameByID,
              onSelectNode: focusNodeFromChat,
              systemText
            })}
          </div>
        </aside>
      )}
    </section>
  )
}
