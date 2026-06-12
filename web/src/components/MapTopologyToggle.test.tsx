// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { MapTopologyToggle } from './MapTopologyToggle'

describe('MapTopologyToggle', () => {
  it('renders the off-state label and a checkbox that fires onToggle', async () => {
    const onToggle = vi.fn()
    const user = userEvent.setup()

    render(
      <MapTopologyToggle
        enabled={false}
        loading={false}
        truncated={false}
        onToggle={onToggle}
      />
    )

    expect(screen.getByLabelText('Show all topology')).toBeTruthy()
    expect(screen.getByLabelText('Show all topology').closest('label')?.querySelector('.map-topology-toggle-icon')).toBeTruthy()
    expect(screen.queryByText(/Capped at/)).toBeNull()

    await user.click(screen.getByLabelText('Show all topology'))
    expect(onToggle).toHaveBeenCalledWith(true)
  })

  it('shows the edge count and a truncation hint when enabled', () => {
    render(
      <MapTopologyToggle
        enabled={true}
        loading={false}
        count={123}
        truncated={true}
        onToggle={() => undefined}
      />
    )

    expect(screen.getByText(/\(123\+\)/)).toBeTruthy()
    expect(screen.getByText('Capped at 123+')).toBeTruthy()
  })

  it('disables the checkbox while loading', () => {
    render(
      <MapTopologyToggle
        enabled={true}
        loading={true}
        count={50}
        truncated={false}
        onToggle={() => undefined}
      />
    )

    const checkbox = screen.getByLabelText<HTMLInputElement>('Show all topology')
    expect(checkbox.disabled).toBe(true)
  })

  it('reserves space for the count text when disabled so the block does not resize', () => {
    const { container } = render(
      <MapTopologyToggle
        enabled={false}
        loading={false}
        truncated={false}
        onToggle={() => undefined}
      />
    )

    const count = container.querySelector('.map-topology-toggle-count')
    expect(count).toBeTruthy()
    expect(count?.className).toContain('map-topology-toggle-count-hidden')
  })
})
