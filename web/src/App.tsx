import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'
import { api } from './api/client'
import { startWS } from './api/ws'
import { Header } from './components/Header'
import { LogPage } from './pages/LogPage'
import { MapPage } from './pages/MapPage'
import { NodesPage } from './pages/NodesPage'
import { useMetaStore } from './stores/meta'
import { useChatStore } from './stores/chat'
import { useLogStore } from './stores/log'
import { useNodeStore } from './stores/nodes'
import { useWSStore } from './stores/ws'

const mapViewKey = 'meshmap-lite.map.view'
const mapHashLatParam = 'lat'
const mapHashLngParam = 'lng'
const mapHashZoomParam = 'z'
const defaultAppName = 'MeshMap Lite'
const defaultAppVersion = 'dev'

interface SavedMapView {
  center: [number, number]
  zoom: number
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

function parseURLNumber(raw: string | null): number | null {
  if (raw === null) return null
  const n = Number(raw)
  if (!Number.isFinite(n)) return null
  return n
}

function readHashMapView(): SavedMapView | null {
  const rawHash = window.location.hash
  if (!rawHash || rawHash.length < 2) return null
  const hash = rawHash.startsWith('#') ? rawHash.slice(1) : rawHash
  const params = new URLSearchParams(hash)
  const lat = parseURLNumber(params.get(mapHashLatParam))
  const lng = parseURLNumber(params.get(mapHashLngParam))
  const zoom = parseURLNumber(params.get(mapHashZoomParam))
  if (lat === null || lng === null || zoom === null) return null
  if (lat < -90 || lat > 90) return null
  if (lng < -180 || lng > 180) return null
  if (zoom < 0 || zoom > 24) return null
  return { center: [lat, lng], zoom }
}

function writeHashMapView(view: SavedMapView): void {
  const url = new URL(window.location.href)
  const hash = url.hash.startsWith('#') ? url.hash.slice(1) : url.hash
  const params = new URLSearchParams(hash)
  const lat = Number(view.center[0].toFixed(6)).toString()
  const lng = Number(view.center[1].toFixed(6)).toString()
  const zoom = Number(view.zoom.toFixed(2)).toString()
  if (
    params.get(mapHashLatParam) === lat &&
    params.get(mapHashLngParam) === lng &&
    params.get(mapHashZoomParam) === zoom
  ) {
    return
  }
  params.set(mapHashLatParam, lat)
  params.set(mapHashLngParam, lng)
  params.set(mapHashZoomParam, zoom)
  window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${params.toString() ? `#${params.toString()}` : ''}`)
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
  const [page, setPage] = useState<'map' | 'nodes' | 'log'>('map')
  const [bootstrapDone, setBootstrapDone] = useState(false)
  const [bootstrapErrors, setBootstrapErrors] = useState<string[]>([])
  const [nodesLoadedOnce, setNodesLoadedOnce] = useState(false)
  const [nodesLoadError, setNodesLoadError] = useState<string>('')
  const [logsLoading, setLogsLoading] = useState(false)
  const [channels, setChannels] = useState<string[]>([])
  const [mapView, setMapView] = useState<SavedMapView>(() => readHashMapView() ?? readSavedMapView() ?? { center: [64.5, 40.6], zoom: 12 })
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
  const logItems = useLogStore((s) => s.items)
  const logFilters = useLogStore((s) => s.filters)
  const logLoadedOnce = useLogStore((s) => s.loadedOnce)
  const logLoadError = useLogStore((s) => s.loadError)
  const setLogInitial = useLogStore((s) => s.setInitial)
  const appendOlderLogs = useLogStore((s) => s.appendOlder)
  const setLogFilters = useLogStore((s) => s.setFilters)
  const setLogLoadError = useLogStore((s) => s.setLoadError)
  const loadedMessagesFor = useRef('')
  const lastLoadedLogKey = useRef('')
  const activeLogRequest = useRef(0)

  useEffect(() => {
    let stopWS: (() => void) | undefined
    const controller = new AbortController()
    let mounted = true

    void (async () => {
      const errors: string[] = []
      const [metaResult, channelsResult, mapNodesResult] = await Promise.allSettled([
        api.meta({ signal: controller.signal }),
        api.channels({ signal: controller.signal }),
        api.mapNodes({ signal: controller.signal })
      ])

      let nextMeta: typeof meta
      let nextChannels: string[] = []

      if (metaResult.status === 'fulfilled') {
        nextMeta = metaResult.value
        if (mounted) {
          setMeta(nextMeta)
          if (nextMeta.websocket_path) {
            stopWS = startWS(nextMeta.websocket_path)
          }
        }
      } else if (!isAbortError(metaResult.reason)) {
        errors.push('Failed to load app metadata. Live updates are unavailable until reload.')
      }

      if (channelsResult.status === 'fulfilled') {
        nextChannels = channelsResult.value.map((x) => x.name)
        if (mounted) setChannels(nextChannels)
      } else if (!isAbortError(channelsResult.reason)) {
        errors.push('Failed to load channels list. Stored/default channel will be used.')
      }

      if (mapNodesResult.status === 'fulfilled') {
        if (mounted) setMapNodes(mapNodesResult.value)
      } else if (!isAbortError(mapNodesResult.reason)) {
        errors.push('Failed to load map nodes snapshot.')
      }

      if (!mounted) return

      const selected = canonicalChannelName(nextChannels, channel || nextMeta?.default_chat_channel || nextChannels[0])
      if (selected) {
        setChannel(selected)
      }

      setBootstrapErrors(errors)
      setBootstrapDone(true)
    })()

    return () => {
      mounted = false
      controller.abort()
      stopWS?.()
    }
  }, [])

  useEffect(() => {
    if (!bootstrapDone) return
    if (!channel) return
    if (loadedMessagesFor.current === channel) return
    const controller = new AbortController()
    void api.chatMessages(channel, meta?.show_recent_messages ?? 50, { signal: controller.signal })
      .then((items) => {
        setMessages(items)
        loadedMessagesFor.current = channel
      })
      .catch((err) => {
        if (isAbortError(err)) return
        setBootstrapErrors((prev) => [...prev, `Failed to load chat history for channel "${channel}".`])
      })

    return () => controller.abort()
  }, [bootstrapDone, channel, meta?.show_recent_messages])

  useEffect(() => {
    if (page !== 'nodes') return
    if (nodesLoadedOnce) return
    const controller = new AbortController()
    void api.nodes({ signal: controller.signal })
      .then((items) => {
        setSummaries(items)
        setNodesLoadedOnce(true)
        setNodesLoadError('')
      })
      .catch((err) => {
        if (isAbortError(err)) return
        setNodesLoadError('Failed to load node list.')
      })

    return () => controller.abort()
  }, [page, nodesLoadedOnce])

  useEffect(() => {
    if (!selectedId) return
    const controller = new AbortController()
    void api.node(selectedId, { signal: controller.signal })
      .then(setDetails)
      .catch((err) => {
        if (isAbortError(err)) return
        setBootstrapErrors((prev) => [...prev, `Failed to load details for node "${selectedId}".`])
      })

    return () => controller.abort()
  }, [selectedId])

  useEffect(() => {
    if (page !== 'log') return
    if (!bootstrapDone) return

    const requestKey = JSON.stringify({
      limit: meta?.log_page_size_default ?? 100,
      eventKinds: logFilters.eventKinds,
      channel: logFilters.channel
    })
    if (logLoadedOnce && lastLoadedLogKey.current === requestKey) return

    const requestID = activeLogRequest.current + 1
    activeLogRequest.current = requestID
    const controller = new AbortController()
    setLogsLoading(true)
    void api.logEvents({
      limit: meta?.log_page_size_default ?? 100,
      eventKinds: logFilters.eventKinds,
      channel: logFilters.channel
    }, { signal: controller.signal })
      .then((items) => {
        if (activeLogRequest.current !== requestID) return
        lastLoadedLogKey.current = requestKey
        setLogInitial(items)
        setLogLoadError('')
      })
      .catch((err) => {
        if (activeLogRequest.current !== requestID) return
        if (isAbortError(err)) return
        setLogLoadError('Failed to load log events.')
      })
      .finally(() => {
        if (activeLogRequest.current === requestID) {
          setLogsLoading(false)
        }
      })

    return () => controller.abort()
  }, [page, bootstrapDone, logLoadedOnce, logFilters.eventKinds, logFilters.channel, meta?.log_page_size_default, setLogInitial, setLogLoadError])

  useEffect(() => {
    if (!channels.length || !channel) return
    const canonical = canonicalChannelName(channels, channel)
    if (canonical !== channel) setChannel(canonical)
  }, [channels, channel])

  useEffect(() => {
    if (!meta) return
    if (readHashMapView()) return
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

  useEffect(() => {
    writeHashMapView(mapView)
  }, [mapView.center[0], mapView.center[1], mapView.zoom])

  const loadMoreLogs = useCallback(() => {
    if (logsLoading) return
    const before = logItems[logItems.length - 1]?.id
    if (!before) return
    setLogsLoading(true)
    void api.logEvents({
      limit: meta?.log_page_size_default ?? 100,
      before,
      eventKinds: logFilters.eventKinds,
      channel: logFilters.channel
    })
      .then((items) => {
        appendOlderLogs(items)
        setLogLoadError('')
      })
      .catch((err) => {
        if (isAbortError(err)) return
        setLogLoadError('Failed to load older log events.')
      })
      .finally(() => setLogsLoading(false))
  }, [appendOlderLogs, logFilters.channel, logFilters.eventKinds, logItems, logsLoading, meta?.log_page_size_default, setLogLoadError])

  const center = useMemo<[number, number]>(() => mapView.center, [mapView.center])
  const zoom = mapView.zoom

  const bannerText = bootstrapErrors.length > 0
    ? `Degraded mode: ${bootstrapErrors[bootstrapErrors.length - 1]}`
    : ''

  const mainClass = page === 'map'
    ? `app-shell map-page${bannerText ? ' has-banner' : ''}`
    : page === 'nodes'
      ? `app-shell nodes-page${bannerText ? ' has-banner' : ''}`
      : 'app-shell'

  return (
    <main className={mainClass}>
      <Header
        appName={meta?.app_name ?? defaultAppName}
        page={page}
        version={meta?.version ?? defaultAppVersion}
        ws={ws}
        wsStats={wsStats}
        onPage={setPage}
      />
      {bannerText && <p className="banner warning" role="alert">{bannerText}</p>}
      {page === 'map' && (
        <MapPage
          center={center}
          zoom={zoom}
          clustering={meta?.map.clustering ?? true}
          channels={channels}
          disconnectedThreshold={meta?.disconnected_threshold}
          onViewChange={onMapViewChange}
        />
      )}
      {page === 'nodes' && (
        <NodesPage items={nodes} selected={selectedId} details={details} loadError={nodesLoadError} onSelect={setSelectedId} />
      )}
      {page === 'log' && (
        <LogPage
          channels={channels}
          items={logItems}
          loadError={logLoadError}
          selectedKinds={logFilters.eventKinds}
          selectedChannel={logFilters.channel}
          onChangeKinds={(eventKinds) => {
            setLogFilters({ ...logFilters, eventKinds })
          }}
          onChangeChannel={(filterChannel) => {
            setLogFilters({ ...logFilters, channel: filterChannel })
          }}
          onLoadMore={loadMoreLogs}
        />
      )}
    </main>
  )
}
