// @vitest-environment jsdom

import { fireEvent, render, screen } from '@testing-library/preact'
import { useState } from 'preact/hooks'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { NodesPage } from './NodesPage'

import type { LogEvent, MapNode, NodeDetails, NodeSummary } from '../api/types'
import type { JSX } from 'preact'

interface NodeStoreState {
  mapNodes: MapNode[]
  summaries: NodeSummary[]
  selectedId?: string
  details?: NodeDetails
}

const { useNodeStore } = vi.hoisted(() => {
  const state: NodeStoreState = {
    mapNodes: [],
    summaries: [],
    selectedId: undefined,
    details: undefined
  }
  const store = ((selector?: (value: NodeStoreState) => unknown) => (
    selector ? selector(state) : state
  )) as (selector?: (value: NodeStoreState) => unknown) => unknown

  return { useNodeStore: store }
})

vi.mock('../stores/nodes', () => ({
  useNodeStore
}))

function summary(nodeId: string, overrides: Partial<NodeSummary> = {}): NodeSummary {
  return {
    node_id: nodeId,
    display_name: overrides.display_name ?? nodeId,
    last_seen_any_event_at: '2026-03-11T12:00:00Z',
    has_position: true,
    ...overrides
  }
}

function details(overrides: Partial<NodeDetails> = {}): NodeDetails {
  return {
    node: {
      node_id: '!zero',
      long_name: 'Zero Node',
      mqtt_gateway_capable: false,
      neighbor_nodes_count: 0,
      last_mqtt_uploader_node_id: '!gateway',
      last_mqtt_uploader_display_name: 'Gateway',
      last_mqtt_uploader_at: '2026-03-11T12:00:00Z',
      last_seen_any_event_at: '2026-03-11T12:00:00Z'
    },
    position: {
      node_id: '!zero',
      latitude: 0,
      longitude: 0,
      altitude_m: 0,
      source_kind: 'telemetry',
      mqtt_uploader_node_id: '!gateway',
      mqtt_uploader_display_name: 'Gateway',
      observed_at: '2026-03-11T12:00:00Z'
    },
    telemetry: {
      node_id: '!zero',
      power: {
        voltage: 0,
        battery_level: 0
      },
      environment: {
        temperature_c: 0,
        humidity: 0,
        pressure_hpa: 0
      },
      air_quality: {
        pm25: 0,
        pm10: 0,
        co2: 0,
        iaq: 0
      },
      mqtt_uploader_node_id: '!gateway',
      mqtt_uploader_display_name: 'Gateway',
      observed_at: '2026-03-11T12:00:00Z',
      updated_at: '2026-03-11T12:00:00Z'
    },
    neighbors: [],
    ...overrides
  }
}

function logEvent(id: number, overrides: Partial<LogEvent> = {}): LogEvent {
  return {
    id,
    observed_at: '2026-03-11T12:00:00Z',
    node_id: '!zero',
    node_display_name: 'Zero Node',
    event_kind_value: 4,
    event_kind_title: 'Telemetry',
    encrypted: false,
    channel_name: 'mesh',
    details: { telemetry: 'ok' },
    ...overrides
  }
}

function hasTextPrefix(prefix: string): (_content: string, element: Element | null) => boolean {
  return (_content, element) => {
    if (!element) {
      return false
    }

    return element.textContent.startsWith(prefix)
  }
}

describe('NodesPage', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-11T12:10:00Z'))
  })

  it('filters nodes by id, short name, and long name case-insensitively', async () => {
    function ControlledNodesPage(): JSX.Element {
      const [filter, setFilter] = useState('')

      return (
        <NodesPage
          items={[
            summary('!alpha', { display_name: 'Alpha Node', short_name: 'A1', long_name: 'Field Router' }),
            summary('!bravo', { display_name: 'Bravo Node', short_name: 'B2', long_name: 'Relay Station' })
          ]}
          filter={filter}
          onFilter={setFilter}
          onOpenMap={() => undefined}
          onSelect={() => undefined}
        />
      )
    }

    render(
      <ControlledNodesPage />
    )

    const filter = screen.getByRole('searchbox', { name: 'Filter nodes' })

    fireEvent.input(filter, { target: { value: 'relay' } })
    expect(screen.queryByText('Alpha Node')).toBeNull()
    expect(screen.getByText('Bravo Node')).toBeTruthy()

    fireEvent.input(filter, { target: { value: 'a1' } })
    expect(screen.getByText('Alpha Node')).toBeTruthy()
    expect(screen.queryByText('Bravo Node')).toBeNull()

    fireEvent.input(filter, { target: { value: '!BRAVO' } })
    expect(screen.getByText('Bravo Node')).toBeTruthy()
  })

  it('renders zero and false-like detail values instead of hiding them', () => {
    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details()}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('MQTT gateway capable: no')).toBeTruthy()
    expect(screen.getByText(hasTextPrefix('Last MQTT via:')).textContent).toContain('Gateway')
    expect(screen.getByText(hasTextPrefix('MQTT via:')).textContent).toContain('Gateway')
    expect(screen.getByText(hasTextPrefix('Telemetry MQTT via:')).textContent).toContain('Gateway')
    expect(screen.getByText('Online local nodes: 0')).toBeTruthy()
    expect(screen.getByText('Latitude: 0')).toBeTruthy()
    expect(screen.getByText('Voltage: 0')).toBeTruthy()
    expect(screen.getByText('Battery level: 0')).toBeTruthy()
    expect(screen.getByText('Temperature (C): 0')).toBeTruthy()
  })

  it('renders previous node names when history exists', () => {
    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details({
          previous_names: [
            {
              previous_long_name: 'Old Zero',
              previous_short_name: 'OZ',
              new_long_name: 'Zero Node',
              new_short_name: 'ZN',
              changed_at: '2026-03-11T12:00:00Z'
            }
          ]
        })}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('Previously known as')).toBeTruthy()
    expect(screen.getByText(/Long: Old Zero \/ Short: OZ/)).toBeTruthy()
    expect(screen.queryByText(/Long: Zero Node/)).toBeNull()
  })

  it('hides previous node names when history is empty', () => {
    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details({ previous_names: [] })}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.queryByText('Previously known as')).toBeNull()
  })

  it('renders partial previous node names cleanly', () => {
    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details({
          previous_names: [
            {
              previous_long_name: 'Long Only',
              new_long_name: 'Zero Node',
              changed_at: '2026-03-11T12:00:00Z'
            },
            {
              previous_short_name: 'SO',
              new_short_name: 'ZN',
              changed_at: '2026-03-11T11:00:00Z'
            }
          ]
        })}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText(/Long: Long Only/)).toBeTruthy()
    expect(screen.getByText(/Short: SO/)).toBeTruthy()
    expect(screen.queryByText(/undefined/)).toBeNull()
  })

  it('renders neighbors ordered by evidence quality and exposes map actions for positioned peers', () => {
    const openNodeDetails = vi.fn()

    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details({
          neighbors: [
            {
              node_id: '!route',
              display_name: 'Route Only',
              has_position: false,
              evidence_kind: 'inferred',
              last_observed_at: '2026-03-11T12:00:00Z',
              updated_at: '2026-03-11T12:00:00Z'
            },
            {
              node_id: '!mqtt',
              display_name: 'MQTT Direct',
              has_position: false,
              evidence_kind: 'mqtt_direct',
              last_observed_at: '2026-03-11T12:04:00Z',
              updated_at: '2026-03-11T12:04:00Z'
            },
            {
              node_id: '!snr',
              display_name: 'Strong Link',
              has_position: true,
              evidence_kind: 'neighbor_info',
              snr: 12.4,
              last_observed_at: '2026-03-11T12:05:00Z',
              updated_at: '2026-03-11T12:05:00Z'
            }
          ]
        })}
        onOpenMap={() => undefined}
        onOpenNodeDetails={openNodeDetails}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('Strong Link')).toBeTruthy()
    expect(screen.queryByText('!snr')).toBeNull()
    expect(screen.getByText('Signal: SNR 12.4 dB')).toBeTruthy()
    expect(screen.getByText('Evidence: MQTT direct')).toBeTruthy()
    expect(screen.getByText('Signal: Direct upload')).toBeTruthy()
    expect(screen.getByText('Evidence: Inferred')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Strong Link' }))
    expect(openNodeDetails).toHaveBeenCalledWith('!snr')
    expect(screen.getByRole('button', { name: 'Open Strong Link on map' })).toBeTruthy()
  })

  it('shows a loading message while the selected node details are pending', () => {
    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        loading
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('Loading node details...')).toBeTruthy()
  })

  it('renders recent events without the node column and keeps details actions', () => {
    render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details()}
        recentEvents={[logEvent(1)]}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('Recent events')).toBeTruthy()
    expect(screen.queryByRole('columnheader', { name: 'Node' })).toBeNull()
    expect(screen.getByRole('columnheader', { name: 'Gateway' })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: 'Details' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'View details for Telemetry' })).toBeTruthy()
  })

  it('renders recent event loading, error, and empty states', () => {
    const { rerender } = render(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details()}
        recentEventsLoading
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('Loading recent events...')).toBeTruthy()

    rerender(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details()}
        recentEventsError="Failed to load recent events."
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )
    expect(screen.getByText('Failed to load recent events.')).toBeTruthy()

    rerender(
      <NodesPage
        items={[summary('!zero', { display_name: 'Zero Node' })]}
        selected="!zero"
        details={details()}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )
    expect(screen.getByText('No recent events.')).toBeTruthy()
  })
})
