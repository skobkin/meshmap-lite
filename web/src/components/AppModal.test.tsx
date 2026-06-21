// @vitest-environment jsdom

import { fireEvent, render, screen } from '@testing-library/preact'
import { useSyncExternalStore } from 'preact/compat'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AppModal as AppModalType } from './AppModal'
import type { SourceSummary, UpdatesResponse } from '../api/types'

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

let updatesStore: StoreHook<UpdatesStoreState>
let AppModal: typeof AppModalType

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
  vi.doMock('../stores/updates', () => ({ useUpdatesStore: updatesStore }))
  const mod = await import('./AppModal')
  AppModal = mod.AppModal
}

function buildSource(overrides: Partial<SourceSummary> = {}): SourceSummary {
  return {
    name: 'meshmap-lite',
    label: 'MeshMap Lite',
    releases: [
      { version: 'v2.0.0', published_at: '2026-06-15T12:00:00Z', prerelease: false },
      { version: 'v1.9.0', published_at: '2026-06-14T12:00:00Z', prerelease: false }
    ],
    ...overrides
  }
}

describe('AppModal', () => {
  beforeEach(async () => {
    await loadModule()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders the information tab by default with provided content', () => {
    render(
      <AppModal
        activeTabID="info"
        infoContent="<h1>Welcome</h1>"
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[{ id: 'info', label: 'Information', isInformation: true }]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    expect(screen.getByRole('dialog', { name: 'Site information' })).toBeTruthy()
    expect(screen.getByText('Welcome')).toBeTruthy()
  })

  it('renders the tab list with one button per tab', () => {
    render(
      <AppModal
        activeTabID="info"
        infoContent="<p>Info</p>"
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource() }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    const tablist = screen.getByRole('tablist', { name: 'Site information sections' })

    expect(tablist).toBeTruthy()
    expect(tablist.parentElement?.classList.contains('info-modal-header')).toBe(true)
    expect(screen.queryByRole('heading', { name: 'Site information' })).toBeNull()
    expect(screen.getByRole('tab', { name: 'Information' })).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'MeshMap Lite' })).toBeTruthy()
  })

  it('marks the active tab as selected and others as not selected', () => {
    render(
      <AppModal
        activeTabID="source:meshmap-lite"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource() }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    expect(screen.getByRole('tab', { name: 'Information' }).getAttribute('aria-selected')).toBe('false')
    expect(screen.getByRole('tab', { name: 'MeshMap Lite' }).getAttribute('aria-selected')).toBe('true')
  })

  it('invokes onSelectTab when a tab is clicked', () => {
    const onSelectTab = vi.fn()
    render(
      <AppModal
        activeTabID="info"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource() }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={onSelectTab}
      />
    )

    fireEvent.click(screen.getByRole('tab', { name: 'MeshMap Lite' }))

    expect(onSelectTab).toHaveBeenCalledWith('source:meshmap-lite')
  })

  it('renders UpdatesPanel when a source tab is active', () => {
    updatesStore.getState().setResponse('meshmap-lite', {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash',
      releases: [
        {
          version: 'v2.0.0',
          published_at: '2026-06-15T12:00:00Z',
          prerelease: false,
          html_url: 'https://example.test/v2',
          body: '<p>v2 body</p>'
        }
      ]
    })

    render(
      <AppModal
        activeTabID="source:meshmap-lite"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource() }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    expect(screen.getByText('v2.0.0')).toBeTruthy()
  })

  it('invokes onDismissUpdates with the newest release when the panel footer button is pressed', () => {
    updatesStore.getState().setResponse('meshmap-lite', {
      format: 'html',
      source: 'meshmap-lite',
      source_hash: 'hash',
      releases: [
        {
          version: 'v2.0.0',
          published_at: '2026-06-15T12:00:00Z',
          prerelease: false,
          html_url: 'https://example.test/v2',
          body: '<p>v2 body</p>'
        }
      ]
    })
    const onDismissUpdates = vi.fn()

    render(
      <AppModal
        activeTabID="source:meshmap-lite"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource() }
        ]}
        updatesDismissedAt={{ 'meshmap-lite': '2026-06-14T00:00:00Z' }}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={onDismissUpdates}
        onSelectTab={() => undefined}
      />
    )

    const buttons = screen.getAllByRole('button', { name: 'Mark as read' })
    const panelButton = buttons.find((b) => b.classList.contains('updates-panel-footer') || b.closest('.updates-panel-footer'))

    expect(panelButton).toBeTruthy()
    fireEvent.click(panelButton!)

    expect(onDismissUpdates).toHaveBeenCalledWith('meshmap-lite', '2026-06-15T12:00:00Z')
  })

  it('invokes onClose when the close button is pressed', () => {
    const onClose = vi.fn()
    render(
      <AppModal
        activeTabID="info"
        infoContent="<p>Info</p>"
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[{ id: 'info', label: 'Information', isInformation: true }]}
        updatesDismissedAt={{}}
        onClose={onClose}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Close site information' }))

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('invokes onDismiss when the primary action is pressed on the information tab', () => {
    const onDismiss = vi.fn()
    render(
      <AppModal
        activeTabID="info"
        infoContent="<p>Info</p>"
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[{ id: 'info', label: 'Information', isInformation: true }]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={onDismiss}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Got it' }))

    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('renders the update dot on a source tab when hasUnread is true', () => {
    render(
      <AppModal
        activeTabID="info"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource(), hasUnread: true }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    const sourceTab = screen.getByRole('tab', { name: 'MeshMap Lite' })

    expect(sourceTab.querySelector('.app-modal-tab-dot')).toBeTruthy()
  })

  it('does not render the update dot on a source tab when hasUnread is false or absent', () => {
    render(
      <AppModal
        activeTabID="info"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:a', label: 'Source A', source: buildSource({ name: 'a', label: 'Source A' }), hasUnread: false },
          { id: 'source:b', label: 'Source B', source: buildSource({ name: 'b', label: 'Source B' }) }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    expect(screen.getByRole('tab', { name: 'Source A' }).querySelector('.app-modal-tab-dot')).toBeNull()
    expect(screen.getByRole('tab', { name: 'Source B' }).querySelector('.app-modal-tab-dot')).toBeNull()
  })

  it('never renders the update dot on the information tab even if hasUnread is set', () => {
    render(
      <AppModal
        activeTabID="info"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true, hasUnread: true }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    expect(screen.getByRole('tab', { name: 'Information' }).querySelector('.app-modal-tab-dot')).toBeNull()
  })

  it('renders the update dot on the selected source tab as well as unselected ones', () => {
    render(
      <AppModal
        activeTabID="source:meshmap-lite"
        infoContent=""
        infoError=""
        infoLoading={false}
        infoShowUpdatedNotice={false}
        tabs={[
          { id: 'info', label: 'Information', isInformation: true },
          { id: 'source:meshmap-lite', label: 'MeshMap Lite', source: buildSource(), hasUnread: true }
        ]}
        updatesDismissedAt={{}}
        onClose={() => undefined}
        onDismiss={() => undefined}
        onDismissUpdates={() => undefined}
        onSelectTab={() => undefined}
      />
    )

    const selectedTab = screen.getByRole('tab', { name: 'MeshMap Lite' })

    expect(selectedTab.getAttribute('aria-selected')).toBe('true')
    expect(selectedTab.querySelector('.app-modal-tab-dot')).toBeTruthy()
  })
})
