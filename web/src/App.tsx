import { useCallback, useEffect, useMemo, useState } from 'preact/hooks'
import { api } from './api/client'
import { startWS } from './api/ws'
import { Header } from './components/Header'
import { MapPage } from './pages/MapPage'
import { NodesPage } from './pages/NodesPage'
import { useMetaStore } from './stores/meta'
import { useChatStore } from './stores/chat'
import { useNodeStore } from './stores/nodes'
import { useWSStore } from './stores/ws'

const mapViewKey = 'meshmap-lite.map.view'

interface SavedMapView {
  center: [number, number]
  zoom: number
}

function readSavedMapView(): SavedMapView | null {
  const raw = localStorage.getItem(mapViewKey)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as { center?: [number, number]; zoom?: number }
    const center = parsed.center
    const zoom = parsed.zoom
    if (!Array.isArray(center) || center.length !== 2 || typeof center[0] !== 'number' || typeof center[1] !== 'number' || typeof zoom !== 'number') {
      return null
    }
    return { center, zoom }
  } catch {
    return null
  }
}

export function App() {
  const [page, setPage] = useState<'map' | 'nodes'>('map')
  const [channels, setChannels] = useState<string[]>([])
  const [mapView, setMapView] = useState<SavedMapView>(() => readSavedMapView() ?? { center: [64.5, 40.6], zoom: 12 })
  const ws = useWSStore((s) => s.state)
  const wsStats = useWSStore((s) => s.stats)
  const meta = useMetaStore((s) => s.meta)
  const setMeta = useMetaStore((s) => s.setMeta)
  const channel = useChatStore((s) => s.channel)
  const setChannel = useChatStore((s) => s.setChannel)
  const setMessages = useChatStore((s) => s.setMessages)
  const nodes = useNodeStore((s) => s.summaries)
  const details = useNodeStore((s) => s.details)
  const selectedId = useNodeStore((s) => s.selectedId)
  const setSelectedId = useNodeStore((s) => s.setSelectedId)
  const setDetails = useNodeStore((s) => s.setDetails)
  const setMapNodes = useNodeStore((s) => s.setMapNodes)
  const setSummaries = useNodeStore((s) => s.setSummaries)

  useEffect(() => {
    void (async () => {
      const [m, c, mapNodes] = await Promise.all([api.meta(), api.channels(), api.mapNodes()])
      setMeta(m)
      setMapNodes(mapNodes)
      const names = c.map((x) => x.name)
      setChannels(names)
      const selected = channel || m.default_chat_channel || names[0]
      if (selected) setChannel(selected)
    })()
  }, [])

  useEffect(() => {
    if (!channel) return
    void api.chatMessages(channel, meta?.show_recent_messages ?? 50).then(setMessages)
  }, [channel, meta?.show_recent_messages])

  useEffect(() => {
    if (!meta) return
    const stop = startWS(meta.websocket_path)
    return () => stop()
  }, [meta?.websocket_path])

  useEffect(() => {
    if (page !== 'nodes') return
    void api.nodes().then(setSummaries)
  }, [page])

  useEffect(() => {
    if (!selectedId) return
    void api.node(selectedId).then(setDetails)
  }, [selectedId])

  useEffect(() => {
    if (!meta) return
    if (readSavedMapView()) return
    setMapView({
      center: [meta.map.default_view.latitude, meta.map.default_view.longitude],
      zoom: meta.map.default_view.zoom
    })
  }, [meta])

  const onMapViewChange = useCallback((center: [number, number], zoom: number) => {
    const next = { center, zoom }
    setMapView(next)
    localStorage.setItem(mapViewKey, JSON.stringify(next))
  }, [])

  const center = useMemo<[number, number]>(() => mapView.center, [mapView.center])
  const zoom = mapView.zoom

  const mainClass = page === 'map'
    ? `app-shell map-page${ws === 'reconnecting' ? ' has-banner' : ''}`
    : 'app-shell'

  return (
    <main className={mainClass}>
      <Header page={page} ws={ws} wsStats={wsStats} onPage={setPage} />
      {ws === 'reconnecting' && <p className="banner">Restoring connection to the server...</p>}
      {page === 'map'
        ? <MapPage center={center} zoom={zoom} channels={channels} onViewChange={onMapViewChange} />
        : <NodesPage items={nodes} selected={selectedId} details={details} onSelect={setSelectedId} />
      }
    </main>
  )
}
