import type { WSState, WSStats } from '../api/types'

interface Props {
  page: 'map' | 'nodes' | 'log'
  ws: WSState
  wsStats: WSStats | null
  onPage: (p: 'map' | 'nodes' | 'log') => void
}

function formatTime(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

function wsStateMeta(
  state: WSState,
  wsStats: WSStats | null
): { title: string; tone: 'connected' | 'connecting' | 'disconnected'; label: string } {
  const lines = [] as string[]
  if (state === 'connected') {
    lines.push('WebSocket: connected')
  } else if (state === 'connecting' || state === 'reconnecting') {
    lines.push(`WebSocket: ${state}`)
  } else {
    lines.push('WebSocket: disconnected')
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
    return { title: lines.join('\n'), tone: 'connected', label: '' }
  }
  return { title: lines.join('\n'), tone: 'disconnected', label: 'Disconnected' }
}

export function Header({ page, ws, wsStats, onPage }: Props) {
  const status = wsStateMeta(ws, wsStats)

  return (
    <header className="topbar container-fluid">
      <a className="brand-logo" href="/" aria-label="MeshMap Lite">
        <img src="/static/icons/favicon.svg" alt="MeshMap Lite" />
      </a>
      <nav className="view-switch" aria-label="View">
        <ul>
          <li>
            <button
              type="button"
              className={page === 'map' ? '' : 'outline'}
              aria-current={page === 'map' ? 'page' : undefined}
              onClick={() => onPage('map')}
            >
              Map
            </button>
          </li>
          <li>
            <button
              type="button"
              className={page === 'nodes' ? '' : 'outline'}
              aria-current={page === 'nodes' ? 'page' : undefined}
              onClick={() => onPage('nodes')}
            >
              Nodes
            </button>
          </li>
          <li>
            <button
              type="button"
              className={page === 'log' ? '' : 'outline'}
              aria-current={page === 'log' ? 'page' : undefined}
              onClick={() => onPage('log')}
            >
              Log
            </button>
          </li>
        </ul>
      </nav>
      <div className="header-icons">
        <span className={`ws-status ${status.tone}`} title={status.title} aria-label={status.title}>
          <span id="ws-status-icon" className="ws-status-icon" aria-hidden="true" />
          {status.label && <span className="ws-status-label">{status.label}</span>}
        </span>
        <a className="repo-link" href="https://git.skobk.in/skobkin/meshmap-lite" target="_blank" rel="noreferrer" title="Source repository" aria-label="Source repository">
          <img src="/static/icons/repo-graph.svg" alt="" aria-hidden="true" />
        </a>
        <a href="https://meshtastic.org" target="_blank" rel="noreferrer" title="Powered by Meshtastic">
          <img className="meshtastic-logo" src="/static/icons/meshtastic-powered.svg" alt="Powered by Meshtastic" />
        </a>
      </div>
    </header>
  )
}
