import { LogEventList } from '../components/LogEventList'

import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  channels: string[]
  items: LogEvent[]
  loadError: string
  selectedKinds: number[]
  selectedChannel: string
  selectedNodeID?: string
  selectedEventID?: number
  onChangeKinds: (kinds: number[]) => void
  onChangeChannel: (channel: string) => void
  onChangeNodeID?: (nodeID: string) => void
  onSelectEvent?: (id: number) => void
  onCloseEventDetails?: () => void
  onOpenNodeDetails: (id: string) => void
  onLoadMore: () => void
}

const eventKinds = [
  { value: 1, label: 'Map report' },
  { value: 2, label: 'Node info' },
  { value: 3, label: 'Position' },
  { value: 4, label: 'Telemetry' },
  { value: 5, label: 'Traceroute' },
  { value: 6, label: 'Neighbor info' },
  { value: 7, label: 'Routing' },
  { value: 8, label: 'Other app packet' },
  { value: 9, label: 'Encrypted (undecryptable)' },
  { value: 10, label: 'Range test' },
  { value: 11, label: 'PKI' }
]

function summaryForEventKinds(selectedKinds: number[]): string {
  const labels = eventKinds
    .filter((item) => selectedKinds.includes(item.value))
    .map((item) => item.label)

  if (labels.length === 0) {return 'All event types'}
  if (labels.length <= 2) {return labels.join(', ')}

  return `${labels.length} event types`
}

export function LogPage({
  channels,
  items,
  loadError,
  selectedKinds,
  selectedChannel,
  selectedNodeID = '',
  selectedEventID,
  onChangeKinds,
  onChangeChannel,
  onChangeNodeID = () => undefined,
  onSelectEvent,
  onCloseEventDetails,
  onOpenNodeDetails,
  onLoadMore
}: Props): JSX.Element {
  const selectedKindSet = new Set(selectedKinds)

  const toggleEventKind = (value: number): void => {
    const nextKinds = selectedKindSet.has(value)
      ? selectedKinds.filter((kind) => kind !== value)
      : eventKinds.filter((item) => selectedKindSet.has(item.value) || item.value === value).map((item) => item.value)
    onChangeKinds(nextKinds)
  }

  return (
    <section className="log-layout container-fluid">
      <details className="log-filters">
        <summary>Filters</summary>
        <div className="log-filters-content">
          <div className="log-filter-field">
            <span className="log-filter-label">Event type</span>
            <details className="dropdown log-filter-dropdown">
              <summary aria-label="Event type filter">{summaryForEventKinds(selectedKinds)}</summary>
              <ul className="log-filter-options">
                {eventKinds.map((item) => (
                  <li key={item.value}>
                    <label>
                      <input
                        type="checkbox"
                        checked={selectedKindSet.has(item.value)}
                        onChange={() => toggleEventKind(item.value)}
                      />
                      {item.label}
                    </label>
                  </li>
                ))}
              </ul>
            </details>
          </div>
          {channels.length > 1 && (
            <div className="log-filter-field">
              <label htmlFor="log-channel-filter">Channel</label>
              <select
                id="log-channel-filter"
                aria-label="Channel filter"
                value={selectedChannel}
                onChange={(e) => onChangeChannel((e.target as HTMLSelectElement).value)}
              >
                <option value="">All channels</option>
                {channels.map((c) => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
          )}
          <div className="log-filter-field compact">
            <label htmlFor="log-node-filter">Node ID</label>
            <input
              id="log-node-filter"
              type="search"
              aria-label="Node ID filter"
              placeholder="Exact node ID"
              value={selectedNodeID}
              onInput={(e) => onChangeNodeID((e.currentTarget).value)}
            />
          </div>
        </div>
      </details>
      <article className="log-table-wrap">
        {loadError && <p className="load-error">{loadError}</p>}
        <LogEventList
          items={items}
          showNodeColumn
          selectedEventID={selectedEventID}
          onSelectEvent={onSelectEvent}
          onCloseEventDetails={onCloseEventDetails}
          onOpenNodeDetails={onOpenNodeDetails}
        />
        <button type="button" className="secondary" onClick={onLoadMore}>Load more</button>
      </article>
    </section>
  )
}
