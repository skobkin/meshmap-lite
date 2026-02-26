import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'
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

function canonicalChannelName(channels: string[], value: string | undefined): string {
  const needle = value?.trim()
  if (!needle) return ''
  const exact = channels.find((c) => c === needle)
  if (exact) return exact
  const folded = channels.find((c) => c.toLowerCase() === needle.toLowerCase())
  return folded ?? needle
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
  const [bootstrapDone, setBootstrapDone] = useState(false)
  const [bootstrapErrors, setBootstrapErrors] = useState<string[]>([])
  const [nodesLoadedOnce, setNodesLoadedOnce] = useState(false)
  const [nodesLoadError, setNodesLoadError] = useState<string>('')
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
  const loadedMessagesFor = useRef('')

  useEffect(() => {
    let stopWS: (() => void) | undefined
    let cancelled = false

    void (async () => {
      const errors: string[] = []
      let nextMeta: typeof meta
      let nextChannels: string[] = []

      try {
        nextMeta = await api.meta()
        if (!cancelled) setMeta(nextMeta)
      } catch {
        errors.push('Failed to load app metadata. Live updates are unavailable until reload.')
      }

      try {
        const channelItems = await api.channels()
        nextChannels = channelItems.map((x) => x.name)
        if (!cancelled) setChannels(nextChannels)
      } catch {
        errors.push('Failed to load channels list. Stored/default channel will be used.')
      }

      try {
        const mapNodes = await api.mapNodes()
        if (!cancelled) setMapNodes(mapNodes)
      } catch {
        errors.push('Failed to load map nodes snapshot.')
      }

      const selected = canonicalChannelName(nextChannels, channel || nextMeta?.default_chat_channel || nextChannels[0])
      if (selected && !cancelled) {
        setChannel(selected)
      }

      if (selected) {
        try {
          const messages = await api.chatMessages(selected, nextMeta?.show_recent_messages ?? 50)
          if (!cancelled) {
            setMessages(messages)
            loadedMessagesFor.current = selected
          }
        } catch {
          errors.push(`Failed to load chat history for channel "${selected}".`)
        }
      }

      if (nextMeta?.websocket_path && !cancelled) {
        stopWS = startWS(nextMeta.websocket_path)
      }

      if (!cancelled) {
        setBootstrapErrors(errors)
        setBootstrapDone(true)
      }
    })()

    return () => {
      cancelled = true
      stopWS?.()
    }
  }, [])

  useEffect(() => {
    if (!bootstrapDone) return
    if (!channel) return
    if (loadedMessagesFor.current === channel) return
    void api.chatMessages(channel, meta?.show_recent_messages ?? 50)
      .then((items) => {
        setMessages(items)
        loadedMessagesFor.current = channel
      })
      .catch(() => {
        setBootstrapErrors((prev) => [...prev, `Failed to load chat history for channel "${channel}".`])
      })
  }, [bootstrapDone, channel, meta?.show_recent_messages])

  useEffect(() => {
    if (page !== 'nodes') return
    if (nodesLoadedOnce) return
    void api.nodes()
      .then((items) => {
        setSummaries(items)
        setNodesLoadedOnce(true)
        setNodesLoadError('')
      })
      .catch(() => {
        setNodesLoadError('Failed to load node list.')
      })
  }, [page, nodesLoadedOnce])

  useEffect(() => {
    if (!selectedId) return
    void api.node(selectedId)
      .then(setDetails)
      .catch(() => {
        setBootstrapErrors((prev) => [...prev, `Failed to load details for node "${selectedId}".`])
      })
  }, [selectedId])

  useEffect(() => {
    if (!channels.length || !channel) return
    const canonical = canonicalChannelName(channels, channel)
    if (canonical !== channel) setChannel(canonical)
  }, [channels, channel])

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

  const bannerText = ws === 'reconnecting'
    ? 'Restoring connection to the server...'
    : (bootstrapErrors.length > 0 ? `Degraded mode: ${bootstrapErrors[bootstrapErrors.length - 1]}` : '')

  const mainClass = page === 'map'
    ? `app-shell map-page${bannerText ? ' has-banner' : ''}`
    : 'app-shell'

  return (
    <main className={mainClass}>
      <Header page={page} ws={ws} wsStats={wsStats} onPage={setPage} />
      {bannerText && <p className={`banner${ws === 'reconnecting' ? '' : ' warning'}`} role="alert">{bannerText}</p>}
      {page === 'map'
        ? <MapPage center={center} zoom={zoom} channels={channels} disconnectedThreshold={meta?.disconnected_threshold} onViewChange={onMapViewChange} />
        : <NodesPage items={nodes} selected={selectedId} details={details} loadError={nodesLoadError} onSelect={setSelectedId} />
      }
    </main>
  )
}
