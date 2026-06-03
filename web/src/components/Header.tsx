import { ConnectionStatus } from './ConnectionStatus'

import type { Page } from '../App'
import type { MQTTConnectionStatus, WSState, WSStats } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  appName: string
  mqttStatus: MQTTConnectionStatus | null
  page: Page
  version: string
  ws: WSState
  wsStats: WSStats | null
  infoAvailable?: boolean
  onPage: (p: Page) => void
  onOpenInfo?: () => void
}

export function Header({ appName, mqttStatus, page, version, ws, wsStats, infoAvailable = false, onPage, onOpenInfo }: Props): JSX.Element {
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
              className={page === 'stats' ? '' : 'outline'}
              aria-current={page === 'stats' ? 'page' : undefined}
              onClick={() => onPage('stats')}
            >
              Stats
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
        {infoAvailable && (
          <a
            aria-label="Site information"
            className="header-icon-button"
            href="#/info"
            title="Site information"
            onClick={onOpenInfo}
          >
            <span aria-hidden="true">i</span>
          </a>
        )}
        <ConnectionStatus mqttStatus={mqttStatus} ws={ws} wsStats={wsStats} />
        <a className="repo-link" href="https://git.skobk.in/skobkin/meshmap-lite" target="_blank" rel="noreferrer" title="Source" aria-label="Source code">
          <span className="repo-link-icon" aria-hidden="true" />
        </a>
        <a href="https://meshtastic.org" target="_blank" rel="noreferrer" title="Powered by Meshtastic">
          <img className="meshtastic-logo" src="/static/icons/meshtastic-powered.svg" alt="Powered by Meshtastic" />
        </a>
      </div>
    </header>
  )
}
