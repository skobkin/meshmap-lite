// @vitest-environment jsdom

import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { useSyncExternalStore } from 'preact/compat'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { chatStorageKey } from './stores/chatState'

import type { ActivityStats, ChannelItem, ChatEvent, InfoResponse, LogEvent, MQTTConnectionStatus, MapNode, Meta, NodeDetails, NodeSummary, WSState, WSStats } from './api/types'
import type { JSX } from 'preact'

type Selector<T, U> = (state: T) => U

interface StoreHook<T> {
  (): T
  <U>(selector: Selector<T, U>): U
  getState: () => T
  reset: () => void
}

interface RequestOptions {
  signal?: AbortSignal
}

function createStoreHook<T extends object>(
  creator: (set: (partial: Partial<T>) => void, get: () => T) => T
): StoreHook<T> {
  const listeners = new Set<() => void>()

  let state: T

  const get = (): T => state
  const set = (partial: Partial<T>): void => {
    state = { ...state, ...partial }
    listeners.forEach((listener) => listener())
  }
  const buildInitial = (): T => creator(set, get)

  state = buildInitial()

  const subscribe = (listener: () => void): (() => void) => {
    listeners.add(listener)

    return () => listeners.delete(listener)
  }

  const hook = (function <U>(selector?: Selector<T, U>) {
    const snapshot = useSyncExternalStore(subscribe, get)

    return selector ? selector(snapshot) : snapshot
  }) as StoreHook<T>

  hook.getState = get
  hook.reset = () => {
    state = buildInitial()
    listeners.forEach((listener) => listener())
  }

  return hook
}

function meta(overrides: Partial<Meta> = {}): Meta {
  return {
    app_name: 'MeshMap Lite',
    version: '1.2.3',
    websocket_path: '/api/v1/ws',
    default_chat_channel: 'mesh',
    show_recent_messages: 50,
    log_live_updates: true,
    log_page_size_default: 100,
    disconnected_threshold: '10m',
    info_available: false,
    info_source_hash: undefined,
    relevance: {
      telemetry_max_age: '24h',
      topology_evidence_max_age: '72h',
      map_position_max_age: '336h'
    },
    map: {
      clustering: true,
      topology_cache_ttl: '10m',
      precision_circles_mode: 'selected',
      default_view: {
        latitude: 64.5,
        longitude: 40.6,
        zoom: 12
      }
    },
    ...overrides
  }
}

function channel(name: string): ChannelItem {
  return {
    name,
    chat_enabled: true,
    is_primary: name === 'mesh'
  }
}

function chatMessage(id: number): ChatEvent {
  return {
    id,
    event_type: 'message',
    observed_at: '2026-03-11T12:00:00Z',
    message_text: `message-${id}`
  }
}

function logEvent(id: number): LogEvent {
  return {
    id,
    observed_at: '2026-03-11T12:00:00Z',
    event_kind_value: 1,
    event_kind_title: 'Map report',
    encrypted: false
  }
}

function nodeDetails(nodeID: string): NodeDetails {
  return {
    node: {
      node_id: nodeID,
      last_seen_any_event_at: '2026-03-11T12:00:00Z'
    },
    neighbors: []
  }
}

interface ChatStoreState {
  channel: string
  messages: ChatEvent[]
  setChannel: (channel: string) => void
  setMessages: (items: ChatEvent[]) => void
  appendOlder: (items: ChatEvent[]) => void
  pushMessage: (item: ChatEvent) => void
}

interface LogStoreState {
  items: LogEvent[]
  filters: { eventKinds: number[]; channel: string; nodeID: string }
  loadedOnce: boolean
  loadError: string
  setInitial: (items: LogEvent[]) => void
  appendOlder: (items: LogEvent[]) => void
  prependLive: (item: LogEvent) => void
  setFilters: (filters: { eventKinds: number[]; channel: string; nodeID: string }) => void
  setLoadError: (msg: string) => void
}

interface MetaStoreState {
  meta?: Meta
  setMeta: (meta: Meta) => void
}

interface NodeStoreState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
  setMapNodes: (items: MapNode[]) => void
  upsertMapNode: (item: MapNode) => void
  upsertNode: (_item: unknown) => void
  upsertPosition: (_item: unknown) => void
  setSummaries: (items: NodeSummary[]) => void
  setSelectedId: (id?: string) => void
  setDetails: (details?: NodeDetails) => void
}

interface WSStoreState {
  mqttStatus: MQTTConnectionStatus | null
  state: WSState
  stats: WSStats | null
  setState: (state: WSState) => void
  setMQTTStatus: (status: MQTTConnectionStatus) => void
  setStats: (stats: WSStats) => void
}

let chatStore: StoreHook<ChatStoreState>
let logStore: StoreHook<LogStoreState>
let metaStore: StoreHook<MetaStoreState>
let nodeStore: StoreHook<NodeStoreState>
let wsStore: StoreHook<WSStoreState>
let apiMock: {
  meta: ReturnType<typeof vi.fn>
  channels: ReturnType<typeof vi.fn>
  mapNodes: ReturnType<typeof vi.fn>
  chatMessages: ReturnType<typeof vi.fn>
  nodes: ReturnType<typeof vi.fn>
  node: ReturnType<typeof vi.fn>
  logEvents: ReturnType<typeof vi.fn>
  statsActivity: ReturnType<typeof vi.fn>
  info: ReturnType<typeof vi.fn>
}
let startWSMock: ReturnType<typeof vi.fn>

async function renderApp(): Promise<ReturnType<typeof render>> {
  const appModule = await import('./App')

  return render(<appModule.App />)
}

function setupModuleMocks(): void {
  chatStore = createStoreHook<ChatStoreState>((set, get) => ({
    channel: localStorage.getItem(chatStorageKey) ?? '',
    messages: [],
    setChannel: (channel) => {
      localStorage.setItem(chatStorageKey, channel)
      set({ channel })
    },
    setMessages: (items) => set({ messages: items }),
    appendOlder: (items) => set({ messages: [...get().messages, ...items] }),
    pushMessage: (item) => set({ messages: [item, ...get().messages].slice(0, 500) })
  }))

  logStore = createStoreHook<LogStoreState>((set, get) => ({
    items: [],
    filters: { eventKinds: [], channel: '', nodeID: '' },
    loadedOnce: false,
    loadError: '',
    setInitial: (items) => set({ items, loadedOnce: true, loadError: '' }),
    appendOlder: (items) => set({ items: [...get().items, ...items] }),
    prependLive: (item) => set({ items: [item, ...get().items].slice(0, 1000) }),
    setFilters: (filters) => set({ filters }),
    setLoadError: (msg) => set({ loadError: msg })
  }))

  metaStore = createStoreHook<MetaStoreState>((set) => ({
    meta: undefined,
    setMeta: (meta) => set({ meta })
  }))

  nodeStore = createStoreHook<NodeStoreState>((set, get) => ({
    mapNodes: [],
    summaries: [],
    selectedId: undefined,
    details: undefined,
    setMapNodes: (items) => set({ mapNodes: items }),
    upsertMapNode: (item) => set({ mapNodes: [item, ...get().mapNodes] }),
    upsertNode: () => undefined,
    upsertPosition: () => undefined,
    setSummaries: (items) => set({ summaries: items }),
    setSelectedId: (id) => set({ selectedId: id }),
    setDetails: (details) => set({ details })
  }))

  wsStore = createStoreHook<WSStoreState>((set, get) => ({
    mqttStatus: null,
    state: 'connecting',
    stats: null,
    setState: (state) => set({
      state,
      mqttStatus: state === 'connected' ? get().mqttStatus : null
    }),
    setMQTTStatus: (mqttStatus) => set({ mqttStatus }),
    setStats: (stats) => set({ stats })
  }))

  apiMock = {
    meta: vi.fn().mockResolvedValue(meta()),
    channels: vi.fn().mockResolvedValue([channel('mesh'), channel('ops')]),
    mapNodes: vi.fn().mockResolvedValue([] satisfies MapNode[]),
    chatMessages: vi.fn().mockResolvedValue([chatMessage(1)]),
    nodes: vi.fn().mockResolvedValue([] satisfies NodeSummary[]),
    node: vi.fn().mockResolvedValue(nodeDetails('!alpha')),
    logEvents: vi.fn().mockResolvedValue([logEvent(1)]),
    statsActivity: vi.fn().mockResolvedValue({ generated_at: '2026-05-04T12:00:00Z', periods: [] } satisfies ActivityStats),
    info: vi.fn().mockResolvedValue({
      format: 'html',
      source_hash: 'hash-1',
      content: '<h1>Site info</h1>'
    } satisfies InfoResponse)
  }
  startWSMock = vi.fn().mockReturnValue(vi.fn())

  vi.doMock('./api/client', () => ({
    api: apiMock
  }))
  vi.doMock('./api/ws', () => ({
    startWS: startWSMock
  }))
  vi.doMock('./stores/chat', () => ({
    useChatStore: chatStore
  }))
  vi.doMock('./stores/log', () => ({
    useLogStore: logStore
  }))
  vi.doMock('./stores/meta', () => ({
    useMetaStore: metaStore
  }))
  vi.doMock('./stores/nodes', () => ({
    useNodeStore: nodeStore
  }))
  vi.doMock('./stores/ws', () => ({
    useWSStore: wsStore
  }))
  vi.doMock('./pages/MapPage', () => ({
    MapPage: ({
      center,
      zoom,
      channels,
      focusNodeId,
      topologyNodeId,
      channel,
      chatPanel,
      onFocusNodeHandled,
      onChannelChange,
      onChatPanelChange,
      onLoadMoreChat,
      onSelectNode,
      onViewChange,
      chatHasMore,
      chatLoadingMore,
      chatLoadMoreError
    }: {
      center: [number, number]
      zoom: number
      channels: string[]
      focusNodeId?: string
      topologyNodeId?: string
      channel: string
      chatPanel: 'open' | 'collapsed'
      onFocusNodeHandled: () => void
      onChannelChange: (channel: string) => void
      onChatPanelChange: (state: 'open' | 'collapsed') => void
      onLoadMoreChat: () => void
      onSelectNode: (id?: string) => void
      onViewChange: (center: [number, number], zoom: number) => void
      chatHasMore: boolean
      chatLoadingMore: boolean
      chatLoadMoreError: string
    }): JSX.Element => (
      <section data-testid="map-page">
        <p>Map page</p>
        <p>Map center: {center.join(',')}</p>
        <p>Map zoom: {zoom}</p>
        <p>Focus node: {focusNodeId ?? ''}</p>
        <p>Channels: {channels.join(',')}</p>
        <p>Chat channel: {channel}</p>
        <p>Chat panel: {chatPanel}</p>
        <p>Chat messages: {chatStore.getState().messages.length}</p>
        <p>Topology node: {topologyNodeId ?? ''}</p>
        <p>Chat has more: {chatHasMore ? 'yes' : 'no'}</p>
        <p>Chat loading more: {chatLoadingMore ? 'yes' : 'no'}</p>
        <p>Chat load error: {chatLoadMoreError}</p>
        <button type="button" onClick={() => onFocusNodeHandled()}>Focus handled</button>
        <button type="button" onClick={() => onSelectNode('!alpha')}>Select marker alpha</button>
        <button type="button" onClick={() => onSelectNode(undefined)}>Close marker popup</button>
        <button type="button" onClick={() => onViewChange([65.1234567, 41.9876543], 9.876)}>Move map</button>
        <button type="button" onClick={() => onChannelChange('ops')}>Set chat channel ops</button>
        <button type="button" onClick={() => onChatPanelChange(chatPanel === 'open' ? 'collapsed' : 'open')}>Toggle chat panel</button>
        <button type="button" disabled={!chatHasMore || chatLoadingMore} onClick={onLoadMoreChat}>Load more chat</button>
      </section>
    )
  }))
  vi.doMock('./pages/NodesPage', () => ({
    NodesPage: ({
      items,
      selected,
      details,
      filter,
      loading,
      onFilter,
      onSelect
    }: {
      items: NodeSummary[]
      selected?: string
      details?: NodeDetails
      filter: string
      loading?: boolean
      onFilter: (filter: string) => void
      onSelect: (id: string) => void
    }): JSX.Element => (
      <section data-testid="nodes-page">
        <p>Nodes page</p>
        <p>Node summaries: {items.length}</p>
        <p>Selected node: {selected ?? ''}</p>
        <p>Node filter: {filter}</p>
        <p>Details node: {details?.node.node_id ?? ''}</p>
        <p>Loading details: {loading ? 'yes' : 'no'}</p>
        <button type="button" onClick={() => onSelect('!bravo')}>Select node bravo</button>
        <button type="button" onClick={() => onFilter('relay')}>Filter relay</button>
      </section>
    )
  }))
  vi.doMock('./pages/LogPage', () => ({
    LogPage: ({
      items,
      selectedKinds,
      selectedChannel,
      selectedNodeID,
      onChangeKinds,
      onChangeChannel,
      onChangeNodeID
    }: {
      items: LogEvent[]
      selectedKinds: number[]
      selectedChannel: string
      selectedNodeID?: string
      onChangeKinds: (kinds: number[]) => void
      onChangeChannel: (channel: string) => void
      onChangeNodeID: (nodeID: string) => void
    }): JSX.Element => (
      <section data-testid="log-page">
        <p>Log page</p>
        <p>Log items: {items.length}</p>
        <p>Log kinds: {selectedKinds.join(',')}</p>
        <p>Log channel: {selectedChannel}</p>
        <p>Log node: {selectedNodeID ?? ''}</p>
        <button type="button" onClick={() => onChangeKinds([7])}>Set log kind 7</button>
        <button type="button" onClick={() => onChangeChannel('ops')}>Set log channel ops</button>
        <button type="button" onClick={() => onChangeNodeID('!alpha')}>Set log node alpha</button>
      </section>
    )
  }))
  vi.doMock('./pages/StatsPage', () => ({
    StatsPage: (): JSX.Element => (
      <section data-testid="stats-page">
        <p>Stats page</p>
      </section>
    )
  }))
}

describe('App', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    localStorage.clear()
    document.cookie = 'meshmap-lite.info.dismissed_source_hash=; Max-Age=0; Path=/'
    window.location.hash = ''
    window.history.replaceState(null, '', '/')
    setupModuleMocks()
  })

  it('bootstraps initial data, canonicalizes the chat channel, and starts websocket updates', async () => {
    localStorage.setItem(chatStorageKey, 'MESH')
    setupModuleMocks()

    await renderApp()

    await screen.findByTestId('map-page')

    await waitFor(() => {
      expect(apiMock.chatMessages).toHaveBeenCalled()
      expect(apiMock.chatMessages.mock.calls[0]?.[0]).toEqual({ channel: 'mesh', limit: 50 })
      const options = apiMock.chatMessages.mock.calls[0]?.[1] as RequestOptions | undefined
      expect(options?.signal).toBeInstanceOf(AbortSignal)
    })

    expect(startWSMock).toHaveBeenCalledWith('/api/v1/ws')
    expect(screen.getByText('Channels: mesh,ops')).toBeTruthy()
    expect(screen.getByText('Chat channel: mesh')).toBeTruthy()
    expect(screen.getByText('Chat messages: 1')).toBeTruthy()
  })

  it('shows degraded mode when one bootstrap request fails but still loads the usable parts', async () => {
    apiMock.channels.mockRejectedValue(new Error('boom'))

    await renderApp()

    await screen.findByTestId('map-page')
    await waitFor(() => {
      expect(apiMock.chatMessages).toHaveBeenCalled()
      expect(apiMock.chatMessages.mock.calls[0]?.[0]).toEqual({ channel: 'mesh', limit: 50 })
      const options = apiMock.chatMessages.mock.calls[0]?.[1] as RequestOptions | undefined
      expect(options?.signal).toBeInstanceOf(AbortSignal)
    })

    expect(startWSMock).toHaveBeenCalledWith('/api/v1/ws')
    expect(screen.getByRole('alert').textContent).toContain('Failed to load channels list')
    expect(screen.getByText('Chat channel: mesh')).toBeTruthy()
    expect(screen.getByText('Chat messages: 1')).toBeTruthy()
  })

  it('loads the node list only once when the user first opens the nodes page', async () => {
    const user = userEvent.setup()

    await renderApp()
    await screen.findByTestId('map-page')

    await user.click(screen.getByRole('button', { name: 'Nodes' }))
    await screen.findByTestId('nodes-page')
    await waitFor(() => {
      expect(apiMock.nodes).toHaveBeenCalledTimes(1)
    })

    await user.click(screen.getByRole('button', { name: 'Map' }))
    await screen.findByTestId('map-page')
    await user.click(screen.getByRole('button', { name: 'Nodes' }))
    await screen.findByTestId('nodes-page')

    expect(apiMock.nodes).toHaveBeenCalledTimes(1)
  })

  it('opens the stats tab between nodes and log', async () => {
    const user = userEvent.setup()

    await renderApp()
    await screen.findByTestId('map-page')

    const buttons = screen.getAllByRole('button').map((button) => button.textContent)
    expect(buttons.slice(0, 4)).toEqual(['Map', 'Nodes', 'Stats', 'Log'])

    await user.click(screen.getByRole('button', { name: 'Stats' }))
    await screen.findByTestId('stats-page')
  })

  it('auto-opens site information when the current source hash is not dismissed', async () => {
    apiMock.meta.mockResolvedValue(meta({
      info_available: true,
      info_source_hash: 'hash-1'
    }))

    await renderApp()

    await screen.findByRole('dialog', { name: 'Site information' })
    await waitFor(() => {
      expect(apiMock.info).toHaveBeenCalled()
      const options = apiMock.info.mock.calls[0]?.[1] as RequestOptions | undefined
      expect(options?.signal).toBeInstanceOf(AbortSignal)
    })
    expect(screen.getByText('Site info')).toBeTruthy()
  })

  it('does not auto-open site information after a matching dismissal cookie', async () => {
    document.cookie = 'meshmap-lite.info.dismissed_source_hash=hash-1; Path=/; SameSite=Lax'
    apiMock.meta.mockResolvedValue(meta({
      info_available: true,
      info_source_hash: 'hash-1'
    }))

    await renderApp()
    await screen.findByTestId('map-page')

    await waitFor(() => {
      expect(apiMock.meta).toHaveBeenCalled()
    })
    expect(screen.queryByRole('dialog', { name: 'Site information' })).toBeNull()
    expect(apiMock.info).not.toHaveBeenCalled()
  })

  it('shows an update notice when the dismissed hash is stale and writes the new hash on dismiss', async () => {
    const user = userEvent.setup()
    document.cookie = 'meshmap-lite.info.dismissed_source_hash=old-hash; Path=/; SameSite=Lax'
    apiMock.meta.mockResolvedValue(meta({
      info_available: true,
      info_source_hash: 'hash-1'
    }))

    await renderApp()

    await screen.findByRole('dialog', { name: 'Site information' })
    expect(screen.getByText('This information was updated since you last dismissed it.')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Got it' }))

    expect(document.cookie).toContain('meshmap-lite.info.dismissed_source_hash=hash-1')
    expect(screen.queryByRole('dialog', { name: 'Site information' })).toBeNull()
  })

  it('opens site information from the header button', async () => {
    const user = userEvent.setup()
    document.cookie = 'meshmap-lite.info.dismissed_source_hash=hash-1; Path=/; SameSite=Lax'
    apiMock.meta.mockResolvedValue(meta({
      info_available: true,
      info_source_hash: 'hash-1'
    }))

    await renderApp()
    await screen.findByTestId('map-page')

    await user.click(screen.getByRole('button', { name: 'Site information' }))

    await screen.findByRole('dialog', { name: 'Site information' })
    await waitFor(() => {
      expect(apiMock.info).toHaveBeenCalledWith('html', expect.any(Object))
    })
  })

  it('hydrates map URL state and keeps marker selection in the fragment', async () => {
    const user = userEvent.setup()
    window.history.replaceState(null, '', '/#/map?lat=64.5&lng=40.6&z=12&node=%21alpha&chat=ops&chat_panel=collapsed')
    setupModuleMocks()

    await renderApp()
    await screen.findByTestId('map-page')

    expect(screen.getByText('Map center: 64.5,40.6')).toBeTruthy()
    expect(screen.getByText('Map zoom: 12')).toBeTruthy()
    expect(screen.getByText('Focus node: !alpha')).toBeTruthy()
    expect(screen.getByText('Topology node: !alpha')).toBeTruthy()
    expect(screen.getByText('Chat channel: ops')).toBeTruthy()
    expect(screen.getByText('Chat panel: collapsed')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Select marker alpha' }))
    expect(window.location.hash).toContain('node=%21alpha')

    await user.click(screen.getByRole('button', { name: 'Close marker popup' }))
    expect(window.location.hash).not.toContain('node=')
  })

  it('hydrates nodes URL state, preserves the filter, and fetches details', async () => {
    window.history.replaceState(null, '', '/#/nodes?node=%21alpha&q=relay')
    setupModuleMocks()

    await renderApp()
    await screen.findByTestId('nodes-page')

    expect(screen.getByText('Selected node: !alpha')).toBeTruthy()
    expect(screen.getByText('Node filter: relay')).toBeTruthy()
    await waitFor(() => {
      expect(apiMock.nodes).toHaveBeenCalledTimes(1)
      expect(apiMock.node).toHaveBeenCalledWith('!alpha', expect.anything())
    })
  })

  it('hydrates log URL filters and loads filtered log data', async () => {
    window.history.replaceState(null, '', '/#/log?event_kind=7&event_kind=4&channel=ops&node_id=%21alpha')
    setupModuleMocks()

    await renderApp()
    await screen.findByTestId('log-page')

    expect(screen.getByText('Log kinds: 7,4')).toBeTruthy()
    expect(screen.getByText('Log channel: ops')).toBeTruthy()
    expect(screen.getByText('Log node: !alpha')).toBeTruthy()
    await waitFor(() => {
      expect(apiMock.logEvents).toHaveBeenCalledWith({
        limit: 100,
        eventKinds: [7, 4],
        channel: 'ops',
        nodeID: '!alpha'
      }, expect.anything())
    })
  })

  it('rehydrates the previous route when browser back returns to nodes state', async () => {
    const user = userEvent.setup()
    window.history.replaceState(null, '', '/#/nodes?node=%21alpha&q=relay')
    setupModuleMocks()

    await renderApp()
    await screen.findByTestId('nodes-page')
    await user.click(screen.getByRole('button', { name: 'Log' }))
    await screen.findByTestId('log-page')

    window.history.back()

    await waitFor(() => {
      expect(screen.getByTestId('nodes-page')).toBeTruthy()
      expect(screen.getByText('Selected node: !alpha')).toBeTruthy()
      expect(screen.getByText('Node filter: relay')).toBeTruthy()
    })
  })

  it('replaces history instead of pushing when map movement updates the fragment', async () => {
    const user = userEvent.setup()
    const pushSpy = vi.spyOn(window.history, 'pushState')
    const replaceSpy = vi.spyOn(window.history, 'replaceState')

    await renderApp()
    await screen.findByTestId('map-page')
    pushSpy.mockClear()
    replaceSpy.mockClear()

    await user.click(screen.getByRole('button', { name: 'Move map' }))

    expect(pushSpy).not.toHaveBeenCalled()
    expect(replaceSpy).toHaveBeenCalled()
    expect(window.location.hash).toBe('#/map?lat=65.123457&lng=41.987654&z=9.88&chat=mesh&chat_panel=open')
    pushSpy.mockRestore()
    replaceSpy.mockRestore()
  })

  it('reuses cached node details before refreshing stale entries', async () => {
    const user = userEvent.setup()
    localStorage.setItem('meshmap-lite.node-details-cache.v1', JSON.stringify({
      '!cached': {
        fetchedAt: 0,
        details: nodeDetails('!cached')
      }
    }))
    setupModuleMocks()
    apiMock.node.mockResolvedValue(nodeDetails('!cached'))

    await renderApp()
    await screen.findByTestId('map-page')

    nodeStore.getState().setSelectedId('!cached')
    await user.click(screen.getByRole('button', { name: 'Nodes' }))

    await screen.findByTestId('nodes-page')
    await waitFor(() => {
      expect(screen.getByText('Details node: !cached')).toBeTruthy()
    })
    await waitFor(() => {
      expect(apiMock.node).toHaveBeenCalledWith('!cached', expect.anything())
    })
    await waitFor(() => {
      expect(apiMock.logEvents).toHaveBeenCalledWith({
        limit: 100,
        nodeID: '!cached'
      }, expect.anything())
    })
  })

  it('reloads log data when filters change and replaces the visible list', async () => {
    const user = userEvent.setup()
    apiMock.logEvents
      .mockResolvedValueOnce([logEvent(1), logEvent(2)])
      .mockResolvedValueOnce([logEvent(3)])

    await renderApp()
    await screen.findByTestId('map-page')

    await user.click(screen.getByRole('button', { name: 'Log' }))
    await screen.findByTestId('log-page')
    await waitFor(() => {
      expect(apiMock.logEvents).toHaveBeenNthCalledWith(1, {
        limit: 100,
        eventKinds: [],
        channel: '',
        nodeID: ''
      }, expect.anything())
      const options = apiMock.logEvents.mock.calls[0]?.[1] as RequestOptions | undefined
      expect(options?.signal).toBeInstanceOf(AbortSignal)
    })

    expect(screen.getByText('Log items: 2')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Set log kind 7' }))

    await waitFor(() => {
      expect(apiMock.logEvents).toHaveBeenNthCalledWith(2, {
        limit: 100,
        eventKinds: [7],
        channel: '',
        nodeID: ''
      }, expect.anything())
      const options = apiMock.logEvents.mock.calls[1]?.[1] as RequestOptions | undefined
      expect(options?.signal).toBeInstanceOf(AbortSignal)
    })

    expect(screen.getByText('Log items: 1')).toBeTruthy()
    expect(screen.getByText('Log kinds: 7')).toBeTruthy()
  })

  it('loads older chat messages from the oldest visible row and stops when the page is short', async () => {
    const user = userEvent.setup()
    apiMock.chatMessages
      .mockResolvedValueOnce([chatMessage(10), chatMessage(9)])
      .mockResolvedValueOnce([chatMessage(8)])
    apiMock.meta.mockResolvedValue(meta({ show_recent_messages: 2 }))

    await renderApp()
    await screen.findByTestId('map-page')

    await waitFor(() => {
      expect(screen.getByText('Chat messages: 2')).toBeTruthy()
      expect(screen.getByText('Chat has more: yes')).toBeTruthy()
    })

    await user.click(screen.getByRole('button', { name: 'Load more chat' }))

    await waitFor(() => {
      expect(apiMock.chatMessages).toHaveBeenNthCalledWith(2, {
        channel: 'mesh',
        limit: 2,
        before: 9
      })
      expect(screen.getByText('Chat messages: 3')).toBeTruthy()
      expect(screen.getByText('Chat has more: no')).toBeTruthy()
    })
  })
})
