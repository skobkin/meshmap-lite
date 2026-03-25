// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import { describe, expect, it } from 'vitest'

import { ConnectionStatus } from './ConnectionStatus'

describe('ConnectionStatus', () => {
  it('shows reconnecting status text while keeping stats in the tooltip', () => {
    render(
      <ConnectionStatus
        mqttStatus={null}
        ws="reconnecting"
        wsStats={{
          known_nodes_count: 10,
          online_nodes_count: 4,
          ws_clients_count: 2,
          last_ingest_at: '2026-03-11T12:00:00Z'
        }}
      />
    )

    expect(screen.getByText('Reconnecting...')).toBeTruthy()
    expect(screen.getByLabelText(/WebSocket: reconnecting/).getAttribute('title')).toContain('Known nodes: 10')
  })

  it('shows a disconnected label when the websocket is down', () => {
    render(<ConnectionStatus mqttStatus={null} ws="disconnected" wsStats={null} />)

    expect(screen.getByText('Disconnected')).toBeTruthy()
    expect(screen.getByLabelText('WebSocket: disconnected')).toBeTruthy()
  })

  it('shows a warning label when websocket is up but MQTT is disconnected', () => {
    render(
      <ConnectionStatus
        mqttStatus="disconnected"
        ws="connected"
        wsStats={null}
      />
    )

    expect(screen.getByText('MQTT disconnected')).toBeTruthy()
    expect(screen.getByLabelText(/MQTT: disconnected/)).toBeTruthy()
  })
})
