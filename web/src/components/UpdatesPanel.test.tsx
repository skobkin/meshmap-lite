// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from '@testing-library/preact'
import { useSyncExternalStore } from 'preact/compat'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { UpdatesPanel as UpdatesPanelType } from './UpdatesPanel'
import type { SourceSummary, UpdateRelease, UpdatesResponse } from '../api/types'

interface UpdatesStoreState {
  bySource: Record<string, UpdatesResponse>
  loading: Record<string, boolean>
  errors: Record<string, string>
  setResponse: (source: string, response: UpdatesResponse) => void
  setLoading: (source: string, value: boolean) => void
  setError: (source: string, message: string) => void
  reset: () => void
}

type Selector<T, U> = (state: T) => U
interface StoreHook<T> {
  (): T
  <U>(selector: Selector<T, U>): U
  getState: () => T
}

function createStoreHook<T extends object>(
  creator: (set: (partial: Partial<T> | ((state: T) => Partial<T>)) => void, get: () => T) => T
): StoreHook<T> {
  const listeners = new Set<() => void>()
  let state: T
  const get = (): T => state
  const set = (partial: Partial<T> | ((state: T) => Partial<T>)): void => {
    const next = typeof partial === 'function' ? partial(state) : partial
    state = { ...state, ...next }
    listeners.forEach((listener) => listener())
  }
  state = creator(set, get)
  const subscribe = (listener: () => void): (() => void) => {
    listeners.add(listener)

    return () => listeners.delete(listener)
  }
  const hook = (<U,>(selector?: Selector<T, U>): U => {
    const snapshot = useSyncExternalStore(subscribe, get)

    return selector ? selector(snapshot) : (snapshot as unknown as U)
  }) as StoreHook<T>
  hook.getState = get

  return hook
}

let apiMock: { updates: ReturnType<typeof vi.fn> }
let updatesStore: StoreHook<UpdatesStoreState>
let UpdatesPanel: typeof UpdatesPanelType

async function loadModule(): Promise<void> {
  vi.resetModules()
  updatesStore = createStoreHook<UpdatesStoreState>((set) => ({
    bySource: {},
    loading: {},
    errors: {},
    setResponse: (source, response) => set((state) => ({ bySource: { ...state.bySource, [source]: response } })),
    setLoading: (source, value) => set((state) => ({ loading: { ...state.loading, [source]: value } })),
    setError: (source, message) => set((state) => ({ errors: { ...state.errors, [source]: message } })),
    reset: () => set({ bySource: {}, loading: {}, errors: {} })
  }))
  apiMock = { updates: vi.fn().mockResolvedValue({ format: 'html', source: 'meshmap-lite', source_hash: 'h', releases: [] }) }
  vi.doMock('../api/client', () => ({ api: apiMock }))
  vi.doMock('../stores/updates', () => ({ useUpdatesStore: updatesStore }))
  const mod = await import('./UpdatesPanel')
  UpdatesPanel = mod.UpdatesPanel
}

function buildSource(overrides: Partial<SourceSummary> = {}): SourceSummary {
  return {
    name: 'meshmap-lite',
    label: 'MeshMap Lite',
    releases: [],
    ...overrides
  }
}

function buildRelease(overrides: Partial<UpdateRelease> = {}): UpdateRelease {
  return {
    version: 'v1.0.0',
    published_at: '2026-06-15T10:00:00Z',
    html_url: 'https://example.test/release',
    body: '<p>Notes</p>',
    prerelease: false,
    ...overrides
  }
}

describe('UpdatesPanel', () => {
  beforeEach(async () => {
    await loadModule()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches updates on mount and renders the list with separator on the first item', async () => {
    const response: UpdatesResponse = {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash-1',
      releases: [
        buildRelease({ version: 'v2.0.0', published_at: '2026-06-15T12:00:00Z' }),
        buildRelease({ version: 'v1.9.0', published_at: '2026-06-14T12:00:00Z' })
      ]
    }
    apiMock.updates = vi.fn().mockResolvedValueOnce(response)

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt=""
        onDismiss={() => undefined}
      />
    )

    await waitFor(() => {
      expect(apiMock.updates).toHaveBeenCalledWith('meshmap-lite', 'html')
    })
    await waitFor(() => {
      expect(screen.getByText('v2.0.0')).toBeTruthy()
    })
    expect(screen.getByText('v1.9.0')).toBeTruthy()
    const firstItem = document.querySelector('.updates-release.updates-separator')
    expect(firstItem).toBeTruthy()
  })

  it('marks a release as NEW when its published_at is after the dismissed timestamp', () => {
    const response: UpdatesResponse = {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash-1',
      releases: [buildRelease({ version: 'v2.0.0', published_at: '2026-06-15T12:00:00Z' })]
    }
    updatesStore.getState().setResponse('meshmap-lite', response)

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt="2026-06-14T00:00:00Z"
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByText('NEW')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Mark as read' })).toBeTruthy()
  })

  it('labels prereleases separately from unread releases', () => {
    const response: UpdatesResponse = {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash-1',
      releases: [buildRelease({ version: 'v2.7.25.104df5f', prerelease: true })]
    }
    updatesStore.getState().setResponse('meshmap-lite', response)

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt=""
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByText('v2.7.25.104df5f')).toBeTruthy()
    expect(screen.getByText('PRE-RELEASE')).toBeTruthy()
    expect(screen.queryByText('NEW')).toBeNull()
  })

  it('does not show the NEW pill or mark-as-read button when nothing is newer than the dismissed timestamp', () => {
    const response: UpdatesResponse = {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash-1',
      releases: [buildRelease({ published_at: '2026-06-10T00:00:00Z' })]
    }
    updatesStore.getState().setResponse('meshmap-lite', response)

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt="2026-06-15T12:00:00Z"
        onDismiss={() => undefined}
      />
    )

    expect(screen.queryByText('NEW')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Mark as read' })).toBeNull()
  })

  it('invokes onDismiss with the newest published_at when Mark as read is pressed', () => {
    const newestPublished = '2026-06-15T12:00:00Z'
    const response: UpdatesResponse = {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash-1',
      releases: [buildRelease({ published_at: newestPublished })]
    }
    updatesStore.getState().setResponse('meshmap-lite', response)
    const onDismiss = vi.fn()

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt="2026-06-14T00:00:00Z"
        onDismiss={onDismiss}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Mark as read' }))

    expect(onDismiss).toHaveBeenCalledWith(newestPublished)
  })

  it('renders an error message when the fetch fails', async () => {
    apiMock.updates = vi.fn().mockRejectedValueOnce(new Error('Network down'))

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt=""
        onDismiss={() => undefined}
      />
    )

    await waitFor(() => {
      expect(screen.getByRole('alert').textContent).toBe('Network down')
    })
  })

  it('shows the loading state while waiting for the response', () => {
    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt=""
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByRole('status').textContent).toBe('Loading...')
  })

  it('shows a placeholder when the source has no releases', () => {
    const response: UpdatesResponse = {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash-1',
      releases: []
    }
    updatesStore.getState().setResponse('meshmap-lite', response)

    render(
      <UpdatesPanel
        source={buildSource()}
        dismissedPublishedAt=""
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByText('No releases to show.')).toBeTruthy()
  })
})
