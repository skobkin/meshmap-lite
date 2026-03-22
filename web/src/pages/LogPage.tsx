import { Fragment } from 'preact'
import { useState } from 'preact/hooks'

import { LogDetailsModal, hasLogDetails } from '../components/LogDetailsModal'
import { dayKey, dayLabel, fullDateTime, hhmmss } from '../utils/time'

import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  channels: string[]
  items: LogEvent[]
  loadError: string
  selectedKinds: number[]
  selectedChannel: string
  onChangeKinds: (kinds: number[]) => void
  onChangeChannel: (channel: string) => void
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
  { value: 9, label: 'Encrypted (undecryptable)' }
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
  onChangeKinds,
  onChangeChannel,
  onOpenNodeDetails,
  onLoadMore
}: Props): JSX.Element {
  const [selectedEvent, setSelectedEvent] = useState<LogEvent>()
  const selectedKindSet = new Set(selectedKinds)
  let previousDay = ''

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
        </div>
      </details>
      <article className="log-table-wrap">
        {loadError && <p className="load-error">{loadError}</p>}
        <table className="log-table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Node</th>
              <th>Type</th>
              <th>Encrypted</th>
              <th>Channel</th>
              <th>Details</th>
            </tr>
          </thead>
          <tbody>
            {items.map((row) => {
              const nodeId = row.node_id
              const currentDay = dayKey(row.observed_at)
              const needsSeparator = currentDay !== previousDay
              previousDay = currentDay
              const fullTimestamp = fullDateTime(row.observed_at)

              return (
                <Fragment key={row.id}>
                  {needsSeparator && (
                    <tr className="log-day-separator" aria-label={dayLabel(row.observed_at)}>
                      <td colSpan={6}>{dayLabel(row.observed_at)}</td>
                    </tr>
                  )}
                  <tr>
                    <td className="log-time-cell" title={fullTimestamp} aria-label={fullTimestamp}>
                      <span className="log-time-value">{hhmmss(row.observed_at)}</span>
                    </td>
                    <td>
                      {nodeId ? (
                        <button
                          type="button"
                          className="chat-node-link"
                          onClick={() => onOpenNodeDetails(nodeId)}
                        >
                          <code>{row.node_display_name ?? nodeId}</code>
                        </button>
                      ) : (
                        <code>{row.node_display_name ?? '-'}</code>
                      )}
                    </td>
                    <td>{row.event_kind_title}</td>
                    <td>{row.encrypted ? 'yes' : 'no'}</td>
                    <td>{row.channel_name ?? '-'}</td>
                    <td>
                      {hasLogDetails(row.details) ? (
                        <button
                          type="button"
                          className="secondary log-details-trigger"
                          aria-label={`View details for ${row.event_kind_title}`}
                          onClick={() => setSelectedEvent(row)}
                        >
                          View
                        </button>
                      ) : '-'}
                    </td>
                  </tr>
                </Fragment>
              )
            })}
          </tbody>
        </table>
        <button type="button" className="secondary" onClick={onLoadMore}>Load more</button>
        <LogDetailsModal event={selectedEvent} onClose={() => setSelectedEvent(undefined)} />
      </article>
    </section>
  )
}
