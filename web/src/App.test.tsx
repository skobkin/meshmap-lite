// @vitest-environment jsdom

import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { useSyncExternalStore } from 'preact/compat'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { chatStorageKey } from './stores/chatState'

import type { ChannelItem, ChatEvent, LogEvent, MapNode, Meta, NodeDetails, NodeSummary, WSState, WSStats } from './api/types'

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
    map: {
      clustering: true,
      hide_position_after: '30m',
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

interface ChatStoreState {
  channel: string
  messages: ChatEvent[]
  setChannel: (channel: string) => void
  setMessages: (items: ChatEvent[]) => void
  pushMessage: (item: ChatEvent) => void
}

interface LogStoreState {
  items: LogEvent[]
  filters: { eventKinds: number[]; channel: string }
  loadedOnce: boolean
  loadError: string
  setInitial: (items: LogEvent[]) => void
  appendOlder: (items: LogEvent[]) => void
  prependLive: (item: LogEvent) => void
  setFilters: (filters: { eventKinds: number[]; channel: string }) => void
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
  state: WSState
  stats: WSStats | null
  setState: (state: WSState) => void
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
}
let startWSMock: ReturnType<typeof vi.fn>

async function renderApp() {
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
    pushMessage: (item) => set({ messages: [item, ...get().messages].slice(0, 500) })
  }))

  logStore = createStoreHook<LogStoreState>((set, get) => ({
    items: [],
    filters: { eventKinds: [], channel: '' },
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

  wsStore = createStoreHook<WSStoreState>((set) => ({
    state: 'connecting',
    stats: null,
    setState: (state) => set({ state }),
    setStats: (stats) => set({ stats })
  }))

  apiMock = {
    meta: vi.fn().mockResolvedValue(meta()),
    channels: vi.fn().mockResolvedValue([channel('mesh'), channel('ops')]),
    mapNodes: vi.fn().mockResolvedValue([] satisfies MapNode[]),
    chatMessages: vi.fn().mockResolvedValue([chatMessage(1)]),
    nodes: vi.fn().mockResolvedValue([] satisfies NodeSummary[]),
    node: vi.fn().mockResolvedValue(undefined),
    logEvents: vi.fn().mockResolvedValue([logEvent(1)])
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
    MapPage: ({ channels }: { channels: string[] }) => (
      <section data-testid="map-page">
        <p>Map page</p>
        <p>Channels: {channels.join(',')}</p>
        <p>Chat channel: {chatStore.getState().channel}</p>
        <p>Chat messages: {chatStore.getState().messages.length}</p>
      </section>
    )
  }))
  vi.doMock('./pages/NodesPage', () => ({
    NodesPage: ({ items }: { items: NodeSummary[] }) => (
      <section data-testid="nodes-page">
        <p>Nodes page</p>
        <p>Node summaries: {items.length}</p>
      </section>
    )
  }))
  vi.doMock('./pages/LogPage', () => ({
    LogPage: ({
      items,
      selectedKinds,
      selectedChannel,
      onChangeKinds,
      onChangeChannel
    }: {
      items: LogEvent[]
      selectedKinds: number[]
      selectedChannel: string
      onChangeKinds: (kinds: number[]) => void
      onChangeChannel: (channel: string) => void
    }) => (
      <section data-testid="log-page">
        <p>Log page</p>
        <p>Log items: {items.length}</p>
        <p>Log kinds: {selectedKinds.join(',')}</p>
        <p>Log channel: {selectedChannel}</p>
        <button type="button" onClick={() => onChangeKinds([7])}>Set log kind 7</button>
        <button type="button" onClick={() => onChangeChannel('ops')}>Set log channel ops</button>
      </section>
    )
  }))
}

describe('App', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    localStorage.clear()
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
      const options = apiMock.chatMessages.mock.calls[0]?.[2] as RequestOptions | undefined
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
      const options = apiMock.chatMessages.mock.calls[0]?.[2] as RequestOptions | undefined
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
        channel: ''
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
        channel: ''
      }, expect.anything())
      const options = apiMock.logEvents.mock.calls[1]?.[1] as RequestOptions | undefined
      expect(options?.signal).toBeInstanceOf(AbortSignal)
    })

    expect(screen.getByText('Log items: 1')).toBeTruthy()
    expect(screen.getByText('Log kinds: 7')).toBeTruthy()
  })
})
