// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { LogPage } from './LogPage'

import type { LogEvent } from '../api/types'

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

function renderPage(items: LogEvent[]): ReturnType<typeof render> {
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
    />
  )
}

describe('LogPage', () => {
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

    const control = screen.getByRole('button', { name: 'Alpha Node' })
    expect(control).toBeTruthy()

    await user.click(control)

    expect(onOpenNodeDetails).toHaveBeenCalledWith('!abc')
  })
})
