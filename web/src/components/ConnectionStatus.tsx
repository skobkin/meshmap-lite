import type { MQTTConnectionStatus, WSState, WSStats } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  mqttStatus: MQTTConnectionStatus | null
  ws: WSState
  wsStats: WSStats | null
}

function formatTime(value?: string): string {
  if (!value) {return '-'}
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {return value}

  return d.toLocaleString()
}

function wsStateMeta(
  mqttStatus: MQTTConnectionStatus | null,
  state: WSState,
  wsStats: WSStats | null
): { title: string; tone: 'connected' | 'connecting' | 'disconnected' | 'warning'; label: string } {
  const lines = [] as string[]
  if (state === 'connected') {
    lines.push('WebSocket: connected')
  } else if (state === 'connecting' || state === 'reconnecting') {
    lines.push(`WebSocket: ${state}`)
  } else {
    lines.push('WebSocket: disconnected')
  }
  if (mqttStatus) {
    lines.push(`MQTT: ${mqttStatus}`)
  }

  if (wsStats) {
    lines.push(`Known nodes: ${wsStats.known_nodes_count}`)
    lines.push(`Online nodes: ${wsStats.online_nodes_count}`)
    lines.push(`WS clients: ${wsStats.ws_clients_count}`)
    lines.push(`Last ingest: ${formatTime(wsStats.last_ingest_at)}`)
  }

  if (state === 'connecting' || state === 'reconnecting') {
    return {
      title: lines.join('\n'),
      tone: 'connecting',
      label: state === 'reconnecting' ? 'Reconnecting...' : ''
    }
  }

  if (state === 'connected') {
    if (mqttStatus === 'disconnected') {
      return { title: lines.join('\n'), tone: 'warning', label: 'MQTT disconnected' }
    }

    return { title: lines.join('\n'), tone: 'connected', label: '' }
  }

  return { title: lines.join('\n'), tone: 'disconnected', label: 'Disconnected' }
}

export function ConnectionStatus({ mqttStatus, ws, wsStats }: Props): JSX.Element {
  const status = wsStateMeta(mqttStatus, ws, wsStats)

  return (
    <span className={`ws-status ${status.tone}`} title={status.title} aria-label={status.title}>
      <span id="ws-status-icon" className="ws-status-icon" aria-hidden="true" />
      {status.label && <span className="ws-status-label">{status.label}</span>}
    </span>
  )
}
