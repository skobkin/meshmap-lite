// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { useState } from 'preact/hooks'
import { describe, expect, it, vi } from 'vitest'

import { LogPage } from './LogPage'

import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

function mockViewport(matches: boolean): void {
  const listeners = new Set<(event: MediaQueryListEvent) => void>()

  vi.stubGlobal('window', {
    ...window,
    matchMedia: vi.fn().mockImplementation(() => ({
      matches,
      media: '(max-width: 768px)',
      onchange: null,
      addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
        listeners.add(listener)
      },
      removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener)
      },
      addListener: (listener: (event: MediaQueryListEvent) => void) => {
        listeners.add(listener)
      },
      removeListener: (listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener)
      },
      dispatchEvent: vi.fn()
    }))
  })
}

function event(id: number, overrides: Partial<LogEvent> = {}): LogEvent {
  return {
    id,
    observed_at: '2026-03-11T12:00:00Z',
    node_id: '!abc',
    node_display_name: 'Alpha Node',
    event_kind_value: 1,
    event_kind_title: 'Map report',
    encrypted: false,
    channel_name: 'mesh',
    details: {
      foo: 'bar',
      nested: {
        count: 2
      }
    },
    ...overrides
  }
}

function renderPage(items: LogEvent[], overrides: Partial<Parameters<typeof LogPage>[0]> = {}): ReturnType<typeof render> {
  return render(
    <LogPage
      channels={['mesh']}
      items={items}
      loadError=""
      selectedKinds={[]}
      selectedChannel=""
      onChangeKinds={() => undefined}
      onChangeChannel={() => undefined}
      onOpenNodeDetails={() => undefined}
      onLoadMore={() => undefined}
      {...overrides}
    />
  )
}

function eventTypeSummary(): HTMLElement {
  const element = document.querySelector('.log-filter-dropdown > summary')
  if (element?.tagName !== 'SUMMARY') {
    throw new Error('Event type filter summary not found')
  }

  return element as HTMLElement
}

function channelFilterValue(): string {
  const element = document.querySelector('#log-channel-filter')
  if (!(element instanceof HTMLSelectElement)) {
    throw new Error('Channel filter not found')
  }

  return element.value
}

describe('LogPage', () => {
  it('renders day separators and time-only cells for grouped events', () => {
    renderPage([
      event(1, { observed_at: '2026-03-11T12:00:00' }),
      event(2, { observed_at: '2026-03-11T12:05:10' }),
      event(3, { observed_at: '2026-03-12T09:15:20' })
    ])

    expect(screen.getByText('11.03.2026')).toBeTruthy()
    expect(screen.getByText('12.03.2026')).toBeTruthy()
    expect(screen.getAllByText('12:00:00')).toHaveLength(1)
    expect(screen.getAllByText('12:05:10')).toHaveLength(1)
    expect(screen.getAllByText('09:15:20')).toHaveLength(1)
    expect(screen.queryByText('11.03.2026, 12:00:00')).toBeNull()
  })

  it('renders grouped article cards on mobile without the desktop table', () => {
    mockViewport(true)

    renderPage([
      event(1, { observed_at: '2026-03-11T12:00:00', event_kind_title: 'Position', encrypted: true }),
      event(2, { observed_at: '2026-03-12T09:15:20', event_kind_title: 'Telemetry', channel_name: null, details: undefined })
    ])

    expect(document.querySelector('.log-mobile-list')).toBeTruthy()
    expect(document.querySelector('.log-table')).toBeNull()
    expect(screen.getByRole('heading', { name: '11.03.2026' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: '12.03.2026' })).toBeTruthy()
    const cards = Array.from(document.querySelectorAll('.log-card'))
    expect(cards).toHaveLength(2)
    expect(cards[0]?.querySelector('.log-card-type')?.textContent).toBe('Position')
    expect(cards[1]?.querySelector('.log-card-type')?.textContent).toBe('Telemetry')
    expect(cards[0]?.querySelector('.log-card-meta')?.textContent).toContain('Encrypted')
    expect(cards[0]?.querySelector('.log-card-meta')?.textContent).toContain('yes')
    expect(cards[1]?.textContent).toContain('No details')
  })

  it('falls back to the raw timestamp string when parsing fails', () => {
    renderPage([
      event(1, { observed_at: 'not-a-time' })
    ])

    expect(screen.getAllByText('not-a-time')).toHaveLength(2)
  })

  it('shows a View button for rows with details and a placeholder for rows without them', () => {
    renderPage([
      event(1),
      event(2, { details: undefined }),
      event(3, { details: {} })
    ])

    const viewButtons = screen.getAllByRole('button', { name: 'View details for Map report' })
    expect(viewButtons).toHaveLength(1)
    expect(screen.getAllByText('-')).toHaveLength(2)
  })

  it('opens a modal with prettified JSON for the selected row and closes it again', async () => {
    const user = userEvent.setup()

    renderPage([
      event(1),
      event(2, {
        node_display_name: 'Bravo Node',
        details: {
          answer: 42
        }
      })
    ])

    const viewButtons = screen.getAllByRole('button', { name: 'View details for Map report' })
    expect(viewButtons).toHaveLength(2)

    await user.click(viewButtons[1]!)

    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(screen.getByRole('heading', { name: /Map report · Bravo Node/i })).toBeTruthy()
    expect(screen.getByText('"answer"').classList.contains('json-key')).toBe(true)
    expect(screen.getByText('42').classList.contains('json-number')).toBe(true)

    await user.click(screen.getByRole('button', { name: 'Close details' }))

    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('keeps node navigation and details actions working in the mobile layout', async () => {
    mockViewport(true)

    const user = userEvent.setup()
    const onOpenNodeDetails = vi.fn()

    render(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={onOpenNodeDetails}
        onLoadMore={() => undefined}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Alpha Node' }))
    expect(onOpenNodeDetails).toHaveBeenCalledWith('!abc')

    await user.click(screen.getByRole('button', { name: 'View details for Map report' }))
    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('renders the node cell as an in-app navigation control when a node id is available', async () => {
    const user = userEvent.setup()
    const onOpenNodeDetails = vi.fn()

    render(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={onOpenNodeDetails}
        onLoadMore={() => undefined}
      />
    )

    expect(screen.getByRole('button', { name: 'Alpha Node' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Alpha Node' }))

    expect(onOpenNodeDetails).toHaveBeenCalledWith('!abc')
  })

  it('shows a compact Event type summary for the selected kinds', () => {
    const view = render(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={() => undefined}
        onLoadMore={() => undefined}
      />
    )

    expect(eventTypeSummary().textContent).toBe('All event types')

    view.rerender(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[7]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={() => undefined}
        onLoadMore={() => undefined}
      />
    )
    expect(eventTypeSummary().textContent).toBe('Routing')

    view.rerender(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[4, 7]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={() => undefined}
        onLoadMore={() => undefined}
      />
    )
    expect(eventTypeSummary().textContent).toBe('Telemetry, Routing')

    view.rerender(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[1, 4, 7]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={() => undefined}
        onLoadMore={() => undefined}
      />
    )
    expect(eventTypeSummary().textContent).toBe('3 event types')
  })

  it('toggles event kinds without resetting other selected kinds and clears back to all when the last one is unchecked', async () => {
    const user = userEvent.setup()
    const onChangeKinds = vi.fn()

    function Harness(): JSX.Element {
      const [selectedKinds, setSelectedKinds] = useState<number[]>([])

      return (
        <LogPage
          channels={['mesh', 'ops']}
          items={[event(1)]}
          loadError=""
          selectedKinds={selectedKinds}
          selectedChannel="mesh"
          onChangeKinds={(kinds) => {
            onChangeKinds(kinds)
            setSelectedKinds(kinds)
          }}
          onChangeChannel={() => undefined}
          onOpenNodeDetails={() => undefined}
          onLoadMore={() => undefined}
        />
      )
    }

    render(<Harness />)

    await user.click(screen.getByRole('checkbox', { name: 'Routing' }))
    expect(onChangeKinds).toHaveBeenLastCalledWith([7])
    expect(eventTypeSummary().textContent).toBe('Routing')
    expect(channelFilterValue()).toBe('mesh')

    await user.click(screen.getByRole('checkbox', { name: 'Telemetry' }))
    expect(onChangeKinds).toHaveBeenLastCalledWith([4, 7])
    expect(eventTypeSummary().textContent).toBe('Telemetry, Routing')
    expect(channelFilterValue()).toBe('mesh')

    await user.click(screen.getByRole('checkbox', { name: 'Routing' }))
    expect(onChangeKinds).toHaveBeenLastCalledWith([4])
    expect(eventTypeSummary().textContent).toBe('Telemetry')
    expect(channelFilterValue()).toBe('mesh')

    await user.click(screen.getByRole('checkbox', { name: 'Telemetry' }))
    expect(onChangeKinds).toHaveBeenLastCalledWith([])
    expect(eventTypeSummary().textContent).toBe('All event types')
    expect(channelFilterValue()).toBe('mesh')
  })

  it('hides the Channel filter when only one channel is available', () => {
    render(
      <LogPage
        channels={['mesh']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={() => undefined}
        onLoadMore={() => undefined}
      />
    )

    expect(screen.queryByLabelText('Channel filter')).toBeNull()
    expect(document.querySelector('label[for="log-channel-filter"]')).toBeNull()
  })

  it('shows the Channel filter when multiple channels are available', () => {
    render(
      <LogPage
        channels={['mesh', 'ops']}
        items={[event(1)]}
        loadError=""
        selectedKinds={[]}
        selectedChannel=""
        onChangeKinds={() => undefined}
        onChangeChannel={() => undefined}
        onOpenNodeDetails={() => undefined}
        onLoadMore={() => undefined}
      />
    )

    expect(screen.getByLabelText('Channel filter')).toBeTruthy()
    expect(document.querySelector('label[for="log-channel-filter"]')?.textContent).toBe('Channel')
  })
})
