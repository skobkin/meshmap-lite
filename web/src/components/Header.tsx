import type { WSState, WSStats } from '../api/types'

interface Props {
  page: 'map' | 'nodes'
  ws: WSState
  wsStats: WSStats | null
  onPage: (p: 'map' | 'nodes') => void
}

function formatTime(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

function wsStateMeta(state: WSState, wsStats: WSStats | null): { title: string; tone: 'connected' | 'connecting' | 'disconnected' } {
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
    return { title: lines.join('\n'), tone: 'connecting' }
  }
  if (state === 'connected') {
    return { title: lines.join('\n'), tone: 'connected' }
  }
  return { title: lines.join('\n'), tone: 'disconnected' }
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
        </ul>
      </nav>
      <div className="header-icons">
        <span className={`ws-dot ${status.tone}`} title={status.title} aria-label={status.title}>
          <svg viewBox="0 0 12 12" aria-hidden="true">
            <circle cx="6" cy="6" r="5" />
          </svg>
        </span>
        <a className="repo-link" href="https://git.skobk.in/skobkin/meshmap-lite" target="_blank" rel="noreferrer" title="Source repository" aria-label="Source repository">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="5" cy="6" r="2" />
            <circle cx="5" cy="18" r="2" />
            <circle cx="19" cy="12" r="2" />
            <path d="M7 6h5a3 3 0 0 1 3 3v1" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            <path d="M7 18h5a3 3 0 0 0 3-3v-1" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
        </a>
        <a href="https://meshtastic.org" target="_blank" rel="noreferrer" title="Powered by Meshtastic">
          <img className="meshtastic-logo" src="/static/icons/meshtastic-powered.svg" alt="Powered by Meshtastic" />
        </a>
      </div>
    </header>
  )
}
