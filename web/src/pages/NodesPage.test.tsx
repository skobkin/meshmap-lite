// @vitest-environment jsdom

import { fireEvent, render, screen } from '@testing-library/preact'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { NodesPage } from './NodesPage'

import type { NodeDetails, NodeSummary } from '../api/types'

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
      last_seen_any_event_at: '2026-03-11T12:00:00Z'
    },
    position: {
      node_id: '!zero',
      latitude: 0,
      longitude: 0,
      altitude_m: 0,
      source_kind: 'telemetry',
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
      observed_at: '2026-03-11T12:00:00Z',
      updated_at: '2026-03-11T12:00:00Z'
    },
    neighbors: [],
    ...overrides
  }
}

describe('NodesPage', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-11T12:10:00Z'))
  })

  it('filters nodes by id, short name, and long name case-insensitively', async () => {
    render(
      <NodesPage
        items={[
          summary('!alpha', { display_name: 'Alpha Node', short_name: 'A1', long_name: 'Field Router' }),
          summary('!bravo', { display_name: 'Bravo Node', short_name: 'B2', long_name: 'Relay Station' })
        ]}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
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
    expect(screen.getByText('Online local nodes: 0')).toBeTruthy()
    expect(screen.getByText('Latitude: 0')).toBeTruthy()
    expect(screen.getByText('Voltage: 0')).toBeTruthy()
    expect(screen.getByText('Battery level: 0')).toBeTruthy()
    expect(screen.getByText('Temperature (C): 0')).toBeTruthy()
  })

  it('renders neighbors ordered by evidence quality and exposes map actions for positioned peers', () => {
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
              last_observed_at: '2026-03-11T12:00:00Z'
            },
            {
              node_id: '!snr',
              display_name: 'Strong Link',
              has_position: true,
              evidence_kind: 'neighbor_info',
              snr: 12.4,
              last_observed_at: '2026-03-11T12:05:00Z'
            }
          ]
        })}
        onOpenMap={() => undefined}
        onSelect={() => undefined}
      />
    )

    expect(screen.getByText('Strong Link')).toBeTruthy()
    expect(screen.getByText('Signal: SNR 12.4 dB')).toBeTruthy()
    expect(screen.getByText('Evidence: Inferred')).toBeTruthy()
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
})
