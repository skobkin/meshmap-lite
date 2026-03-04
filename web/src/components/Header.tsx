import type { WSState, WSStats } from '../api/types'
import { ConnectionStatus } from './ConnectionStatus'

interface Props {
  appName: string
  page: 'map' | 'nodes' | 'log'
  version: string
  ws: WSState
  wsStats: WSStats | null
  onPage: (p: 'map' | 'nodes' | 'log') => void
}

export function Header({ appName, page, version, ws, wsStats, onPage }: Props) {
  const brandTitle = `${appName} ${version}`

  return (
    <header className="topbar container-fluid">
      <a className="brand-logo" href="/" aria-label={appName} title={brandTitle}>
        <img src="/static/icons/favicon.svg" alt={appName} />
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
        <ConnectionStatus ws={ws} wsStats={wsStats} />
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
