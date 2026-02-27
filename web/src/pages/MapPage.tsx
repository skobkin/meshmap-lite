import { useEffect, useRef, useState } from 'preact/hooks'
import { LeafletMapAdapter } from '../maps/leafletMap'
import { useNodeStore } from '../stores/nodes'
import { useChatStore } from '../stores/chat'
import { hhmm } from '../utils/time'

interface Props {
  center: [number, number]
  zoom: number
  clustering: boolean
  channels: string[]
  disconnectedThreshold?: string
  onViewChange: (center: [number, number], zoom: number) => void
}

const sidebarStateKey = 'meshmap-lite.map.chat.collapsed'

function readSidebarState(): boolean {
  return localStorage.getItem(sidebarStateKey) === '1'
}

export function MapPage({ center, zoom, clustering, channels, disconnectedThreshold, onViewChange }: Props) {
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
      onViewChange,
      onSelectNode: setSelectedId
    })
    return () => {
      adapterRef.current?.destroy()
      adapterRef.current = null
    }
  }, [clustering, onViewChange, setSelectedId])

  useEffect(() => {
    adapterRef.current?.setView(center, zoom)
  }, [center[0], center[1], zoom])

  useEffect(() => {
    adapterRef.current?.render(nodes, disconnectedThreshold)
  }, [nodes, disconnectedThreshold])

  useEffect(() => {
    adapterRef.current?.setSelectedNode(selectedId)
  }, [selectedId])

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
            {chat.map((m) => (
              <p key={m.id} className={m.event_type === 'system' ? 'system' : ''}>
                <code>{hhmm(m.observed_at)}</code> <mark>{m.node_id ? (nodeNameByID.get(m.node_id) ?? m.node_id) : 'system'}</mark> {m.event_type === 'system' ? systemText(m.system_code) : (m.message_text ?? '')}
              </p>
            ))}
          </div>
        </aside>
      )}
    </section>
  )
}
