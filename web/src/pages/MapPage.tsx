import { useEffect, useRef } from 'preact/hooks'
import { LeafletMapAdapter } from '../maps/leafletMap'
import { useNodeStore } from '../stores/nodes'
import { useChatStore } from '../stores/chat'
import { hhmm } from '../utils/time'

interface Props {
  center: [number, number]
  zoom: number
  channels: string[]
  onViewChange: (center: [number, number], zoom: number) => void
}

export function MapPage({ center, zoom, channels, onViewChange }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const adapterRef = useRef<LeafletMapAdapter | null>(null)
  const nodes = useNodeStore((s) => s.mapNodes)
  const chat = useChatStore((s) => s.messages)
  const channel = useChatStore((s) => s.channel)
  const setChannel = useChatStore((s) => s.setChannel)

  useEffect(() => {
    if (!ref.current) return
    adapterRef.current = new LeafletMapAdapter(ref.current, center, zoom, onViewChange)
    return () => {
      adapterRef.current?.destroy()
      adapterRef.current = null
    }
  }, [onViewChange])

  useEffect(() => {
    adapterRef.current?.setView(center, zoom)
  }, [center[0], center[1], zoom])

  useEffect(() => {
    adapterRef.current?.render(nodes)
  }, [nodes])

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
    <section className="map-layout">
      <div className="map-canvas" ref={ref} />
      <aside className="chat-panel">
        <select aria-label="Channel" value={channel} onChange={(e) => setChannel((e.target as HTMLSelectElement).value)}>
          {channels.map((c) => <option key={c} value={c}>{c}</option>)}
        </select>
        <div className="chat-list">
          {chat.map((m) => (
            <p key={m.id} className={m.event_type === 'system' ? 'system' : ''}>
              <code>{hhmm(m.observed_at)}</code> <mark>{m.node_id ? (nodeNameByID.get(m.node_id) ?? m.node_id) : 'system'}</mark> {m.event_type === 'system' ? systemText(m.system_code) : (m.message_text ?? '')}
            </p>
          ))}
        </div>
      </aside>
    </section>
  )
}
