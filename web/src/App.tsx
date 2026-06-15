import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'

import { api } from './api/client'
import { startWS } from './api/ws'
import { AppModal } from './components/AppModal'
import { Header } from './components/Header'
import { LogPage } from './pages/LogPage'
import { MapPage } from './pages/MapPage'
import { NodesPage } from './pages/NodesPage'
import { StatsPage } from './pages/StatsPage'
import { useChatStore } from './stores/chat'
import { useLogStore } from './stores/log'
import { useMetaStore } from './stores/meta'
import { useNodeStore } from './stores/nodes'
import { useTopologyAllStore } from './stores/topologyAll'
import { useWSStore } from './stores/ws'
import { parseDurationMs } from './utils/duration'
import { infoDismissedCookie, updatesDismissedCookie } from './utils/infoCookie'
import { isNodeDetailsCacheFresh, persistNodeDetailsCache, readNodeDetailsCache, upsertNodeDetailsCache } from './utils/nodeDetailsCache'
import { pruneMapNodesByRelevance, pruneNodeDetailsByRelevance, pruneNodeDetailsCacheByRelevance, pruneNodeSummariesByRelevance } from './utils/relevance'
import { isTopologyAllFresh } from './utils/topologyAllCache'
import { parseFragmentState, serializeFragmentState } from './utils/urlState'

import type { InfoResponse, LogEvent, NodeDetails } from './api/types'
import type { AppModalTab } from './components/AppModal'
import type { FragmentState, MapViewState } from './utils/urlState'
import type { JSX } from 'preact'

const informationTabID = 'information'

export type Page = FragmentState['page']

const mapViewKey = 'meshmap-lite.map.view'
const defaultAppName = 'MeshMap Lite'
const defaultAppVersion = 'dev'
const defaultTopologyCacheTTL = '10m'

type SavedMapView = MapViewState

function isSavedMapView(value: unknown): value is SavedMapView {
  if (typeof value !== 'object' || value === null) {return false}
  const { center, zoom } = value as { center?: unknown; zoom?: unknown }
  if (!Array.isArray(center) || center.length !== 2) {return false}
  if (typeof center[0] !== 'number' || typeof center[1] !== 'number') {return false}

  return typeof zoom === 'number'
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

function canonicalChannelName(channels: string[], value: string | undefined): string {
  const needle = value?.trim()
  if (!needle) {return ''}
  const exact = channels.find((c) => c === needle)
  if (exact) {return exact}
  const folded = channels.find((c) => c.toLowerCase() === needle.toLowerCase())

  return folded ?? needle
}

function writeFragment(state: FragmentState, mode: 'push' | 'replace'): void {
  const url = new URL(window.location.href)
  const next = `${url.pathname}${url.search}${serializeFragmentState(state)}`
  const current = `${url.pathname}${url.search}${url.hash}`
  if (next === current) {return}
  if (mode === 'push') {
    window.history.pushState(window.history.state, '', next)

    return
  }
  window.history.replaceState(window.history.state, '', next)
}

function readSavedMapView(): SavedMapView | null {
  const raw = localStorage.getItem(mapViewKey)
  if (!raw) {return null}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!isSavedMapView(parsed)) {return null}

    return parsed
  } catch {
    return null
  }
}

export function App(): JSX.Element {
  const initialURLState = useRef(parseFragmentState(window.location.hash))
  const [page, setPage] = useState<Page>(() => initialURLState.current.page)
  const [hoveredTopologyNodeId, setHoveredTopologyNodeId] = useState<string>()
  const [mapFocusNodeId, setMapFocusNodeId] = useState<string | undefined>(() => initialURLState.current.page === 'map' ? initialURLState.current.map?.node : undefined)
  const [bootstrapDone, setBootstrapDone] = useState(false)
  const [bootstrapErrors, setBootstrapErrors] = useState<string[]>([])
  const [nodesLoadedOnce, setNodesLoadedOnce] = useState(false)
  const [nodesLoadError, setNodesLoadError] = useState<string>('')
  const [logsLoading, setLogsLoading] = useState(false)
  const [chatLoadingMore, setChatLoadingMore] = useState(false)
  const [chatLoadMoreError, setChatLoadMoreError] = useState('')
  const [chatHasMore, setChatHasMore] = useState(false)
  const [infoRouteRequested, setInfoRouteRequested] = useState(() => Boolean(initialURLState.current.infoRequested))
  const [updatesRouteRequestedSource, setUpdatesRouteRequestedSource] = useState(() => initialURLState.current.updatesRequestedSource ?? '')
  const [appModalOpen, setAppModalOpen] = useState(false)
  const [appModalActiveTab, setAppModalActiveTab] = useState<string>(informationTabID)
  const [infoDismissedHash, setInfoDismissedHash] = useState(() => infoDismissedCookie.read())
  const [updatesDismissedAt, setUpdatesDismissedAt] = useState<string>(() => updatesDismissedCookie.read())
  const [infoContent, setInfoContent] = useState<InfoResponse>()
  const [infoLoading, setInfoLoading] = useState(false)
  const [infoError, setInfoError] = useState('')
  const [nodeLogItems, setNodeLogItems] = useState<LogEvent[]>([])
  const [nodeLogLoading, setNodeLogLoading] = useState(false)
  const [nodeLogError, setNodeLogError] = useState('')
  const [channels, setChannels] = useState<string[]>([])
  const [nodesFilter, setNodesFilter] = useState(() => initialURLState.current.page === 'nodes' ? initialURLState.current.nodes?.q ?? '' : '')
  const [selectedLogEventID, setSelectedLogEventID] = useState<number | undefined>(() => initialURLState.current.page === 'log' ? initialURLState.current.log?.eventID : undefined)
  const [chatPanel, setChatPanel] = useState<'open' | 'collapsed'>(() => initialURLState.current.page === 'map' ? initialURLState.current.map?.chatPanel ?? 'open' : 'open')
  const [detailsCache, setDetailsCache] = useState(() => readNodeDetailsCache(localStorage))
  const [mapView, setMapView] = useState<SavedMapView>(() => initialURLState.current.map?.view ?? readSavedMapView() ?? { center: [64.5, 40.6], zoom: 12 })
  const mqttStatus = useWSStore((s) => s.mqttStatus)
  const ws = useWSStore((s) => s.state)
  const wsStats = useWSStore((s) => s.stats)
  const meta = useMetaStore((s) => s.meta)
  const setMeta = useMetaStore((s) => s.setMeta)
  const channel = useChatStore((s) => s.channel)
  const setChannel = useChatStore((s) => s.setChannel)
  const setMessages = useChatStore((s) => s.setMessages)
  const appendOlderMessages = useChatStore((s) => s.appendOlder)
  const chatMessages = useChatStore((s) => s.messages)
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
  const activeNodeLogRequest = useRef(0)
  const inFlightNodeDetails = useRef(new Map<string, Promise<void>>())
  const initialChannelRef = useRef(initialURLState.current.page === 'map' ? initialURLState.current.map?.chatChannel ?? channel : channel)
  const topologyCacheTTL = meta?.map.topology_cache_ttl ?? defaultTopologyCacheTTL
  const topologyAllEnabled = useTopologyAllStore((s) => s.enabled)
  const topologyAllEdges = useTopologyAllStore((s) => s.edges)
  const topologyAllTruncated = useTopologyAllStore((s) => s.truncated)
  const topologyAllLoading = useTopologyAllStore((s) => s.loading)
  const topologyAllCount = topologyAllEdges.length
  const setTopologyAllEnabled = useTopologyAllStore((s) => s.setEnabled)
  const refreshTopologyAll = useTopologyAllStore((s) => s.refresh)
  const resetTopologyAll = useTopologyAllStore((s) => s.reset)

  const applyRelevance = useCallback((activeMeta: NonNullable<typeof meta>): void => {
    const state = useNodeStore.getState()
    state.setMapNodes(pruneMapNodesByRelevance(state.mapNodes, activeMeta))
    state.setSummaries(pruneNodeSummariesByRelevance(state.summaries, activeMeta))
    state.setDetails(pruneNodeDetailsByRelevance(state.details, activeMeta))
    setDetailsCache((current) => {
      const next = pruneNodeDetailsCacheByRelevance(current, activeMeta)
      persistNodeDetailsCache(next, localStorage)

      return next
    })
  }, [])

  const currentFragmentState = useCallback((nextPage = page): FragmentState => {
    switch (nextPage) {
      case 'map':
        return {
          page: 'map',
          map: {
            view: mapView,
            node: selectedId,
            chatChannel: channel,
            chatPanel
          }
        }
      case 'nodes':
        return {
          page: 'nodes',
          nodes: {
            node: selectedId,
            q: nodesFilter.trim() ? nodesFilter : undefined
          }
        }
      case 'log':
        return {
          page: 'log',
          log: {
            eventKinds: logFilters.eventKinds,
            channel: logFilters.channel,
            nodeID: logFilters.nodeID,
            eventID: page === 'log' ? selectedLogEventID : undefined
          }
        }
      case 'stats':
        return { page: 'stats' }
    }
  }, [channel, chatPanel, logFilters.channel, logFilters.eventKinds, logFilters.nodeID, mapView, nodesFilter, page, selectedId, selectedLogEventID])

  const applyFragmentState = useCallback((state: FragmentState): void => {
    setInfoRouteRequested(Boolean(state.infoRequested))
    setUpdatesRouteRequestedSource(state.updatesRequestedSource ?? '')
    setPage(state.page)
    if (state.page !== 'log') {
      setSelectedLogEventID(undefined)
    }
    if (state.page === 'map') {
      if (state.map?.view) {setMapView(state.map.view)}
      setSelectedId(state.map?.node)
      setMapFocusNodeId(state.map?.node)
      if (state.map?.chatChannel) {setChannel(state.map.chatChannel)}
      setChatPanel(state.map?.chatPanel ?? 'open')

      return
    }
    if (state.page === 'nodes') {
      setSelectedId(state.nodes?.node)
      setNodesFilter(state.nodes?.q ?? '')
      setMapFocusNodeId(undefined)

      return
    }
    if (state.page === 'log') {
      setLogFilters(state.log ?? { eventKinds: [], channel: '', nodeID: '' })
      setSelectedLogEventID(state.log?.eventID)
      setMapFocusNodeId(undefined)

      return
    }
    setSelectedLogEventID(undefined)
    setMapFocusNodeId(undefined)
  }, [setChannel, setLogFilters, setSelectedId])

  const updateURL = useCallback((state: FragmentState, mode: 'push' | 'replace'): void => {
    writeFragment(state, mode)
  }, [])

  useEffect(() => {
    applyFragmentState(initialURLState.current)
  }, [applyFragmentState])

  useEffect(() => {
    const handleURLChange = (): void => {
      applyFragmentState(parseFragmentState(window.location.hash))
    }
    window.addEventListener('hashchange', handleURLChange)
    window.addEventListener('popstate', handleURLChange)

    return () => {
      window.removeEventListener('hashchange', handleURLChange)
      window.removeEventListener('popstate', handleURLChange)
    }
  }, [applyFragmentState])

  const cacheNodeDetails = useCallback((item: NodeDetails): void => {
    setDetailsCache((current) => {
      const relevant = meta ? pruneNodeDetailsByRelevance(item, meta) ?? item : item
      const next = meta
        ? pruneNodeDetailsCacheByRelevance(upsertNodeDetailsCache(current, relevant), meta)
        : upsertNodeDetailsCache(current, relevant)
      persistNodeDetailsCache(next, localStorage)

      return next
    })
  }, [meta])

  const refreshNodeDetails = useCallback((nodeID: string, signal?: AbortSignal): Promise<void> => {
    const existing = inFlightNodeDetails.current.get(nodeID)
    if (existing) {
      return existing
    }

    const request = api.node(nodeID, { signal })
      .then((item) => {
        const relevant = meta ? pruneNodeDetailsByRelevance(item, meta) ?? item : item
        cacheNodeDetails(relevant)
        if (selectedId === nodeID) {
          setDetails(relevant)
        }
      })
      .finally(() => {
        inFlightNodeDetails.current.delete(nodeID)
      })

    inFlightNodeDetails.current.set(nodeID, request)

    return request
  }, [cacheNodeDetails, meta, selectedId, setDetails])

  useEffect(() => {
    if (!meta) {return}
    applyRelevance(meta)
    const timer = window.setInterval(() => applyRelevance(meta), 60000)

    return () => window.clearInterval(timer)
  }, [applyRelevance, meta])

  // The "show all topology" toggle is intentionally in-memory only. When the
  // user navigates away from the map page or unmounts, we drop the snapshot.
  useEffect(() => () => resetTopologyAll(), [resetTopologyAll])

  // Reuse the cached snapshot while the meta-supplied TTL is fresh. On expiry
  // we refetch in the background; the user can also force a refresh by
  // toggling the switch off and on again.
  useEffect(() => {
    if (!topologyAllEnabled) {return}
    if (!meta) {return}
    const ttlMs = parseDurationMs(topologyCacheTTL) ?? 10 * 60 * 1000
    const state = useTopologyAllStore.getState()
    if (isTopologyAllFresh(state.lastFetchedAt, ttlMs)) {
      return
    }
    void refreshTopologyAll()
  }, [meta, refreshTopologyAll, topologyAllEnabled, topologyCacheTTL])

  useEffect(() => {
    let stopWS: (() => void) | undefined
    const controller = new AbortController()

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
        if (!controller.signal.aborted) {
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
        if (!controller.signal.aborted) {setChannels(nextChannels)}
      } else if (!isAbortError(channelsResult.reason)) {
        errors.push('Failed to load channels list. Stored/default channel will be used.')
      }

      if (mapNodesResult.status === 'fulfilled') {
        if (!controller.signal.aborted) {
          setMapNodes(nextMeta ? pruneMapNodesByRelevance(mapNodesResult.value, nextMeta) : mapNodesResult.value)
        }
      } else if (!isAbortError(mapNodesResult.reason)) {
        errors.push('Failed to load map nodes snapshot.')
      }

      if (controller.signal.aborted) {return}

      const preferredChannel = initialChannelRef.current.trim().length > 0
        ? initialChannelRef.current
        : nextMeta?.default_chat_channel ?? nextChannels[0]
      const selected = canonicalChannelName(nextChannels, preferredChannel)
      if (selected) {
        setChannel(selected)
      }

      setBootstrapErrors(errors)
      setBootstrapDone(true)
    })()

    return () => {
      controller.abort()
      stopWS?.()
    }
  }, [setChannel, setMapNodes, setMeta])

  useEffect(() => {
    if (!bootstrapDone) {return}
    if (!channel) {return}
    if (loadedMessagesFor.current === channel) {return}
    const controller = new AbortController()
    const limit = meta?.show_recent_messages ?? 50
    setChatLoadMoreError('')
    setChatHasMore(false)
    void api.chatMessages({ channel, limit }, { signal: controller.signal })
      .then((items) => {
        setMessages(items)
        setChatHasMore(items.length >= limit)
        loadedMessagesFor.current = channel
      })
      .catch((err) => {
        if (isAbortError(err)) {return}
        setBootstrapErrors((prev) => [...prev, `Failed to load chat history for channel "${channel}".`])
      })

    return () => controller.abort()
  }, [bootstrapDone, channel, meta?.show_recent_messages, setMessages])

  useEffect(() => {
    if (page !== 'nodes') {return}
    if (nodesLoadedOnce) {return}
    const controller = new AbortController()
    void api.nodes({ signal: controller.signal })
      .then((items) => {
        setSummaries(meta ? pruneNodeSummariesByRelevance(items, meta) : items)
        setNodesLoadedOnce(true)
        setNodesLoadError('')
      })
      .catch((err) => {
        if (isAbortError(err)) {return}
        setNodesLoadError('Failed to load node list.')
      })

    return () => controller.abort()
  }, [meta, page, nodesLoadedOnce, setSummaries])

  useEffect(() => {
    if (!selectedId) {
      setDetails(undefined)

      return
    }

    const cached = detailsCache[selectedId]
    if (cached) {
      setDetails(meta ? pruneNodeDetailsByRelevance(cached.details, meta) : cached.details)
      if (isNodeDetailsCacheFresh(cached, topologyCacheTTL)) {
        return
      }
    } else {
      setDetails(undefined)
    }

    const controller = new AbortController()
    void refreshNodeDetails(selectedId, controller.signal)
      .catch((err) => {
        if (isAbortError(err)) {return}
        setBootstrapErrors((prev) => [...prev, `Failed to load details for node "${selectedId}".`])
      })

    return () => controller.abort()
  }, [detailsCache, meta, refreshNodeDetails, selectedId, setDetails, topologyCacheTTL])

  useEffect(() => {
    if (!hoveredTopologyNodeId) {return}

    const cached = detailsCache[hoveredTopologyNodeId]
    if (isNodeDetailsCacheFresh(cached, topologyCacheTTL)) {
      return
    }

    const controller = new AbortController()
    void refreshNodeDetails(hoveredTopologyNodeId, controller.signal)
      .catch((err) => {
        if (isAbortError(err)) {return}
        setBootstrapErrors((prev) => [...prev, `Failed to load topology for node "${hoveredTopologyNodeId}".`])
      })

    return () => controller.abort()
  }, [detailsCache, hoveredTopologyNodeId, refreshNodeDetails, topologyCacheTTL])

  useEffect(() => {
    if (page !== 'log') {return}
    if (!bootstrapDone) {return}

    const selectedEventBeforeID = selectedLogEventID ? selectedLogEventID + 1 : undefined
    const requestKey = JSON.stringify({
      limit: meta?.log_page_size_default ?? 100,
      before: selectedEventBeforeID,
      eventKinds: logFilters.eventKinds,
      channel: logFilters.channel,
      nodeID: logFilters.nodeID
    })
    if (logLoadedOnce && lastLoadedLogKey.current === requestKey) {return}

    const requestID = activeLogRequest.current + 1
    activeLogRequest.current = requestID
    const controller = new AbortController()
    setLogsLoading(true)
    void api.logEvents({
      limit: meta?.log_page_size_default ?? 100,
      before: selectedEventBeforeID,
      eventKinds: logFilters.eventKinds,
      channel: logFilters.channel,
      nodeID: logFilters.nodeID
    }, { signal: controller.signal })
      .then((items) => {
        if (activeLogRequest.current !== requestID) {return}
        lastLoadedLogKey.current = requestKey
        setLogInitial(items)
        setLogLoadError('')
      })
      .catch((err) => {
        if (activeLogRequest.current !== requestID) {return}
        if (isAbortError(err)) {return}
        setLogLoadError('Failed to load log events.')
      })
      .finally(() => {
        if (activeLogRequest.current === requestID) {
          setLogsLoading(false)
        }
      })

    return () => controller.abort()
  }, [page, bootstrapDone, logLoadedOnce, logFilters.eventKinds, logFilters.channel, logFilters.nodeID, meta?.log_page_size_default, selectedLogEventID, setLogInitial, setLogLoadError])

  useEffect(() => {
    if (!selectedId) {
      setNodeLogItems([])
      setNodeLogError('')
      setNodeLogLoading(false)

      return
    }
    if (!bootstrapDone) {return}

    const requestID = activeNodeLogRequest.current + 1
    activeNodeLogRequest.current = requestID
    const controller = new AbortController()
    setNodeLogLoading(true)
    setNodeLogError('')

    void api.logEvents({
      limit: meta?.log_page_size_default ?? 100,
      nodeID: selectedId
    }, { signal: controller.signal })
      .then((items) => {
        if (activeNodeLogRequest.current !== requestID) {return}
        setNodeLogItems(items)
      })
      .catch((err) => {
        if (activeNodeLogRequest.current !== requestID) {return}
        if (isAbortError(err)) {return}
        setNodeLogItems([])
        setNodeLogError('Failed to load recent events.')
      })
      .finally(() => {
        if (activeNodeLogRequest.current === requestID) {
          setNodeLogLoading(false)
        }
      })

    return () => controller.abort()
  }, [bootstrapDone, meta?.log_page_size_default, selectedId])

  useEffect(() => {
    if (!channels.length || !channel) {return}
    const canonical = canonicalChannelName(channels, channel)
    if (canonical !== channel) {
      setChannel(canonical)
      if (page === 'map') {
        updateURL({
          page: 'map',
          map: {
            view: mapView,
            node: selectedId,
            chatChannel: canonical,
            chatPanel
          }
        }, 'replace')
      }
    }
  }, [channels, channel, chatPanel, mapView, page, selectedId, setChannel, updateURL])

  useEffect(() => {
    if (!meta) {return}
    if (initialURLState.current.map?.view) {return}
    if (readSavedMapView()) {return}
    setMapView({
      center: [meta.map.default_view.latitude, meta.map.default_view.longitude],
      zoom: meta.map.default_view.zoom
    })
  }, [meta])

  useEffect(() => {
    if (!meta?.info_available || !meta.info_source_hash) {return}
    if (infoDismissedHash === meta.info_source_hash) {return}
    setAppModalOpen(true)
    setAppModalActiveTab(informationTabID)
  }, [infoDismissedHash, meta])

  useEffect(() => {
    if (!infoRouteRequested) {return}
    if (!meta?.info_available || !meta.info_source_hash) {return}
    setAppModalOpen(true)
    setAppModalActiveTab(informationTabID)
  }, [infoRouteRequested, meta])

  useEffect(() => {
    if (!updatesRouteRequestedSource) {return}
    setAppModalOpen(true)
    setAppModalActiveTab(updatesRouteRequestedSource)
  }, [updatesRouteRequestedSource])

  useEffect(() => {
    if (!appModalOpen) {return}
    if (appModalActiveTab !== informationTabID) {return}
    if (!meta?.info_available || !meta.info_source_hash) {return}
    if (infoContent?.source_hash === meta.info_source_hash && infoContent.format === 'html') {return}

    const controller = new AbortController()
    setInfoLoading(true)
    setInfoError('')
    void api.info('html', { signal: controller.signal })
      .then((item) => {
        setInfoContent(item)
        setInfoError('')
      })
      .catch((err) => {
        if (isAbortError(err)) {return}
        setInfoError('Failed to load site information.')
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setInfoLoading(false)
        }
      })

    return () => controller.abort()
  }, [appModalActiveTab, appModalOpen, infoContent?.format, infoContent?.source_hash, meta])

  const onMapViewChange = useCallback((center: [number, number], zoom: number): void => {
    const next = { center, zoom }
    setMapView(next)
    localStorage.setItem(mapViewKey, JSON.stringify(next))
    updateURL({
      page: 'map',
      map: {
        view: next,
        node: selectedId,
        chatChannel: channel,
        chatPanel
      }
    }, 'replace')
  }, [channel, chatPanel, selectedId, updateURL])

  const navigateToPage = useCallback((nextPage: Page): void => {
    setPage(nextPage)
    if (nextPage !== 'map') {
      setMapFocusNodeId(undefined)
    }
    if (nextPage !== 'log') {
      setSelectedLogEventID(undefined)
    }
    updateURL(currentFragmentState(nextPage), 'push')
  }, [currentFragmentState, updateURL])

  const openNodeDetails = useCallback((id: string): void => {
    setPage('nodes')
    setMapFocusNodeId(undefined)
    setSelectedLogEventID(undefined)
    setSelectedId(id)
    updateURL({
      page: 'nodes',
      nodes: {
        node: id,
        q: nodesFilter.trim() ? nodesFilter : undefined
      }
    }, 'push')
  }, [nodesFilter, setSelectedId, updateURL])

  const openNodeOnMap = useCallback((id: string): void => {
    setSelectedId(id)
    setMapFocusNodeId(id)
    setPage('map')
    updateURL({
      page: 'map',
      map: {
        view: mapView,
        node: id,
        chatChannel: channel,
        chatPanel
      }
    }, 'push')
  }, [channel, chatPanel, mapView, setSelectedId, updateURL])

  const selectMapNode = useCallback((id?: string): void => {
    setSelectedId(id)
    updateURL({
      page: 'map',
      map: {
        view: mapView,
        node: id,
        chatChannel: channel,
        chatPanel
      }
    }, id ? 'push' : 'replace')
  }, [channel, chatPanel, mapView, setSelectedId, updateURL])

  const changeChatPanel = useCallback((next: 'open' | 'collapsed'): void => {
    setChatPanel(next)
    updateURL({
      page: 'map',
      map: {
        view: mapView,
        node: selectedId,
        chatChannel: channel,
        chatPanel: next
      }
    }, 'replace')
  }, [channel, mapView, selectedId, updateURL])

  const changeChatChannel = useCallback((next: string): void => {
    setChannel(next)
    updateURL({
      page: 'map',
      map: {
        view: mapView,
        node: selectedId,
        chatChannel: next,
        chatPanel
      }
    }, 'replace')
  }, [chatPanel, mapView, selectedId, setChannel, updateURL])

  const toggleTopologyAll = useCallback((next: boolean): void => {
    setTopologyAllEnabled(next)
  }, [setTopologyAllEnabled])

  const changeNodesFilter = useCallback((q: string): void => {
    setNodesFilter(q)
    updateURL({
      page: 'nodes',
      nodes: {
        node: selectedId,
        q: q.trim() ? q : undefined
      }
    }, 'replace')
  }, [selectedId, updateURL])

  const selectNodeOnNodesPage = useCallback((id: string): void => {
    setSelectedId(id)
    updateURL({
      page: 'nodes',
      nodes: {
        node: id,
        q: nodesFilter.trim() ? nodesFilter : undefined
      }
    }, 'push')
  }, [nodesFilter, setSelectedId, updateURL])

  const changeLogFilters = useCallback((filters: typeof logFilters): void => {
    setLogFilters(filters)
    setSelectedLogEventID(undefined)
    updateURL({
      page: 'log',
      log: {
        eventKinds: filters.eventKinds,
        channel: filters.channel,
        nodeID: filters.nodeID
      }
    }, 'replace')
  }, [setLogFilters, updateURL])

  const selectLogEvent = useCallback((eventID: number): void => {
    setSelectedLogEventID(eventID)
    updateURL({
      page: 'log',
      log: {
        eventKinds: logFilters.eventKinds,
        channel: logFilters.channel,
        nodeID: logFilters.nodeID,
        eventID
      }
    }, 'push')
  }, [logFilters.channel, logFilters.eventKinds, logFilters.nodeID, updateURL])

  const closeLogEvent = useCallback((): void => {
    setSelectedLogEventID(undefined)
    updateURL({
      page: 'log',
      log: {
        eventKinds: logFilters.eventKinds,
        channel: logFilters.channel,
        nodeID: logFilters.nodeID
      }
    }, 'replace')
  }, [logFilters.channel, logFilters.eventKinds, logFilters.nodeID, updateURL])

  const loadMoreLogs = useCallback(() => {
    if (logsLoading) {return}
    const before = logItems[logItems.length - 1]?.id
    if (!before) {return}
    setLogsLoading(true)
    void api.logEvents({
      limit: meta?.log_page_size_default ?? 100,
      before,
      eventKinds: logFilters.eventKinds,
      channel: logFilters.channel,
      nodeID: logFilters.nodeID
    })
      .then((items) => {
        appendOlderLogs(items)
        setLogLoadError('')
      })
      .catch((err) => {
        if (isAbortError(err)) {return}
        setLogLoadError('Failed to load older log events.')
      })
      .finally(() => setLogsLoading(false))
  }, [appendOlderLogs, logFilters.channel, logFilters.eventKinds, logFilters.nodeID, logItems, logsLoading, meta?.log_page_size_default, setLogLoadError])

  const loadMoreChat = useCallback(() => {
    if (chatLoadingMore) {return}
    if (!chatHasMore) {return}
    if (!channel) {return}
    const before = chatMessages[chatMessages.length - 1]?.id
    if (!before) {return}
    const limit = meta?.show_recent_messages ?? 50
    setChatLoadingMore(true)
    void api.chatMessages({ channel, limit, before })
      .then((items) => {
        appendOlderMessages(items)
        setChatHasMore(items.length >= limit)
        setChatLoadMoreError('')
      })
      .catch((err) => {
        if (isAbortError(err)) {return}
        setChatLoadMoreError('Failed to load older chat messages.')
      })
      .finally(() => setChatLoadingMore(false))
  }, [appendOlderMessages, channel, chatHasMore, chatLoadingMore, chatMessages, meta?.show_recent_messages])

  const openInfoModal = useCallback(() => {
    if (!meta?.info_available) {return}
    setAppModalOpen(true)
    setAppModalActiveTab(informationTabID)
  }, [meta?.info_available])

  const dismissInfoModal = useCallback(() => {
    const hash = meta?.info_source_hash
    if (hash) {
      infoDismissedCookie.write(hash)
      setInfoDismissedHash(hash)
    }
    setAppModalOpen(false)
  }, [meta?.info_source_hash])

  const closeAppModal = useCallback((): void => {
    setAppModalOpen(false)
  }, [])

  const selectAppModalTab = useCallback((id: string): void => {
    setAppModalActiveTab(id)
  }, [])

  const dismissUpdatesForSource = useCallback((_source: string, publishedAt: string): void => {
    setUpdatesDismissedAt(publishedAt)
    updatesDismissedCookie.write(publishedAt)
  }, [])

  const center = useMemo<[number, number]>(() => mapView.center, [mapView.center])
  const zoom = mapView.zoom
  const topologyNodeId = hoveredTopologyNodeId ?? selectedId
  const topologyDetails = topologyNodeId ? detailsCache[topologyNodeId]?.details : undefined

  const bannerText = bootstrapErrors.length > 0
    ? `Degraded mode: ${bootstrapErrors[bootstrapErrors.length - 1]}`
    : ''
  const visibleInfoContent = infoContent && infoContent.source_hash === meta?.info_source_hash
    ? infoContent.content
    : ''

  const updateSources = meta?.update_check_sources ?? []
  const showAppModal = appModalOpen && (Boolean(meta?.info_available) || updateSources.length > 0)
  const modalTabs: AppModalTab[] = []
  if (meta?.info_available) {
    modalTabs.push({ id: informationTabID, label: 'Information', isInformation: true })
  }
  for (const source of updateSources) {
    modalTabs.push({ id: source.name, label: source.label, source })
  }
  const updatesUnread = updateSources.reduce((sum, source) => {
    if (!updatesDismissedAt) {return sum + source.releases.length}
    const dismissed = new Date(updatesDismissedAt).getTime()
    if (Number.isNaN(dismissed)) {return sum}
    let count = 0
    for (const release of source.releases) {
      const published = new Date(release.published_at).getTime()
      if (Number.isNaN(published)) {continue}
      if (published > dismissed) {count++}
    }

    return sum + count
  }, 0)

  const mainClass = page === 'map'
    ? `app-shell map-page${bannerText ? ' has-banner' : ''}`
    : page === 'nodes'
      ? `app-shell nodes-page${bannerText ? ' has-banner' : ''}`
      : 'app-shell'

  return (
    <main className={mainClass}>
      <Header
        appName={meta?.app_name ?? defaultAppName}
        infoAvailable={Boolean(meta?.info_available)}
        mqttStatus={mqttStatus}
        page={page}
        unreadCount={updatesUnread}
        version={meta?.version ?? defaultAppVersion}
        ws={ws}
        wsStats={wsStats}
        onOpenInfo={openInfoModal}
        onPage={navigateToPage}
      />
      {showAppModal && (
        <AppModal
          activeTabID={appModalActiveTab}
          infoContent={visibleInfoContent}
          infoError={infoError}
          infoLoading={infoLoading}
          infoShowUpdatedNotice={Boolean(infoDismissedHash && meta?.info_source_hash && infoDismissedHash !== meta.info_source_hash)}
          tabs={modalTabs}
          updatesDismissedAt={updatesDismissedAt}
          onClose={closeAppModal}
          onDismiss={dismissInfoModal}
          onDismissUpdates={dismissUpdatesForSource}
          onSelectTab={selectAppModalTab}
        />
      )}
      {bannerText && <p className="banner warning" role="alert">{bannerText}</p>}
      {page === 'map' && (
        <MapPage
          center={center}
          zoom={zoom}
          clustering={meta?.map.clustering ?? true}
          precisionCirclesMode={meta?.map.precision_circles_mode ?? 'selected'}
          channels={channels}
          disconnectedThreshold={meta?.disconnected_threshold}
          focusNodeId={mapFocusNodeId}
          topologyDetails={topologyDetails}
          topologyNodeId={topologyNodeId}
          topologyAllEnabled={topologyAllEnabled}
          topologyAllLoading={topologyAllLoading}
          topologyAllCount={topologyAllCount}
          topologyAllTruncated={topologyAllTruncated}
          topologyAllEdges={topologyAllEdges}
          chatPanel={chatPanel}
          channel={channel}
          onFocusNodeHandled={() => setMapFocusNodeId(undefined)}
          onChatPanelChange={changeChatPanel}
          onChannelChange={changeChatChannel}
          onHoverTopologyNode={setHoveredTopologyNodeId}
          onLoadMoreChat={loadMoreChat}
          onOpenNodeDetails={openNodeDetails}
          onSelectNode={selectMapNode}
          onToggleTopologyAll={toggleTopologyAll}
          onViewChange={onMapViewChange}
          chatHasMore={chatHasMore}
          chatLoadingMore={chatLoadingMore}
          chatLoadMoreError={chatLoadMoreError}
        />
      )}
      {page === 'nodes' && (
        <NodesPage
          items={nodes}
          selected={selectedId}
          details={details}
          filter={nodesFilter}
          loading={Boolean(selectedId && details?.node.node_id !== selectedId)}
          loadError={nodesLoadError}
          recentEvents={nodeLogItems}
          recentEventsLoading={nodeLogLoading}
          recentEventsError={nodeLogError}
          onOpenMap={openNodeOnMap}
          onOpenNodeDetails={openNodeDetails}
          onFilter={changeNodesFilter}
          onSelect={selectNodeOnNodesPage}
        />
      )}
      {page === 'stats' && <StatsPage />}
      {page === 'log' && (
        <LogPage
          channels={channels}
          items={logItems}
          loadError={logLoadError}
          selectedKinds={logFilters.eventKinds}
          selectedChannel={logFilters.channel}
          selectedNodeID={logFilters.nodeID}
          selectedEventID={selectedLogEventID}
          onChangeKinds={(eventKinds) => {
            changeLogFilters({ ...logFilters, eventKinds })
          }}
          onChangeChannel={(filterChannel) => {
            changeLogFilters({ ...logFilters, channel: filterChannel })
          }}
          onChangeNodeID={(nodeID) => {
            changeLogFilters({ ...logFilters, nodeID })
          }}
          onSelectEvent={selectLogEvent}
          onCloseEventDetails={closeLogEvent}
          onOpenNodeDetails={openNodeDetails}
          onLoadMore={loadMoreLogs}
        />
      )}
    </main>
  )
}
