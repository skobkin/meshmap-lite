import { Fragment } from 'preact'
import { useEffect, useState } from 'preact/hooks'

import { LogDetailsModal, hasLogDetails } from '../components/LogDetailsModal'
import { pkiLogDetailsRenderer } from '../components/PKILogDetails'
import { ResolvedNodeData } from '../components/ResolvedNodeData'
import { routingLogDetailsRenderer } from '../components/RoutingLogDetails'
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
  { value: 9, label: 'Encrypted (undecryptable)' },
  { value: 10, label: 'Range test' },
  { value: 11, label: 'PKI' }
]

const mobileLogMediaQuery = '(max-width: 768px)'

function summaryForEventKinds(selectedKinds: number[]): string {
  const labels = eventKinds
    .filter((item) => selectedKinds.includes(item.value))
    .map((item) => item.label)

  if (labels.length === 0) {return 'All event types'}
  if (labels.length <= 2) {return labels.join(', ')}

  return `${labels.length} event types`
}

function isMobileLogLayout(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }

  return window.matchMedia(mobileLogMediaQuery).matches
}

function useMobileLogLayout(): boolean {
  const [mobile, setMobile] = useState<boolean>(() => isMobileLogLayout())

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return undefined
    }

    const mediaQuery = window.matchMedia(mobileLogMediaQuery)
    const onChange = (event: MediaQueryListEvent): void => {
      setMobile(event.matches)
    }

    setMobile(mediaQuery.matches)

    if (typeof mediaQuery.addEventListener === 'function') {
      mediaQuery.addEventListener('change', onChange)

      return () => mediaQuery.removeEventListener('change', onChange)
    }

    const legacyMediaQuery = mediaQuery as MediaQueryList & {
      addListener: (listener: (event: MediaQueryListEvent) => void) => void
      removeListener: (listener: (event: MediaQueryListEvent) => void) => void
    }

    legacyMediaQuery.addListener(onChange)

    return () => legacyMediaQuery.removeListener(onChange)
  }, [])

  return mobile
}

interface LogDayGroup {
  key: string
  label: string
  items: LogEvent[]
}

function groupLogItemsByDay(items: LogEvent[]): LogDayGroup[] {
  const groups: LogDayGroup[] = []

  for (const item of items) {
    const key = dayKey(item.observed_at)
    const label = dayLabel(item.observed_at)
    const previous = groups[groups.length - 1]

    if (previous?.key !== key) {
      groups.push({ key, label, items: [item] })
      continue
    }

    previous.items.push(item)
  }

  return groups
}

function LogNodeLabel({
  nodeId,
  fallbackLabel,
  onOpenNodeDetails
}: {
  nodeId?: string
  fallbackLabel?: string
  onOpenNodeDetails: (id: string) => void
}): JSX.Element {
  if (!nodeId) {
    return <code>{fallbackLabel ?? '-'}</code>
  }

  return (
    <ResolvedNodeData nodeId={nodeId} fallbackLabel={fallbackLabel}>
      {({ label, title }) => (
        <button
          type="button"
          className="chat-node-link"
          title={title}
          onClick={() => onOpenNodeDetails(nodeId)}
        >
          <code>{label}</code>
        </button>
      )}
    </ResolvedNodeData>
  )
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
  const mobileLayout = useMobileLogLayout()
  const selectedKindSet = new Set(selectedKinds)
  const dayGroups = groupLogItemsByDay(items)
  const firstRowIDs = new Set(dayGroups.map((group) => group.items[0]?.id).filter((id): id is number => typeof id === 'number'))

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
        </div>
      </details>
      <article className="log-table-wrap">
        {loadError && <p className="load-error">{loadError}</p>}
        {mobileLayout ? (
          <div className="log-mobile-list" data-layout="mobile">
            {dayGroups.map((group) => (
              <section key={group.key} className="log-day-group" aria-labelledby={`log-day-${group.key}`}>
                <h2 id={`log-day-${group.key}`} className="log-day-heading">{group.label}</h2>
                {group.items.map((row) => {
                  const nodeId = row.node_id
                  const fullTimestamp = fullDateTime(row.observed_at)

                  return (
                    <article key={row.id} className="log-card">
                      <div className="log-card-head">
                        <LogNodeLabel
                          nodeId={nodeId}
                          fallbackLabel={row.node_display_name}
                          onOpenNodeDetails={onOpenNodeDetails}
                        />
                        <strong className="log-card-type">{row.event_kind_title}</strong>
                      </div>
                      <dl className="log-card-meta">
                        <div>
                          <dt>Channel</dt>
                          <dd>{row.channel_name ?? '-'}</dd>
                        </div>
                        <div>
                          <dt>Encrypted</dt>
                          <dd>{row.encrypted ? 'yes' : 'no'}</dd>
                        </div>
                      </dl>
                      <div className="log-card-actions">
                        {hasLogDetails(row.details) ? (
                          <button
                            type="button"
                            className="secondary log-details-trigger"
                            aria-label={`View details for ${row.event_kind_title}`}
                            onClick={() => setSelectedEvent(row)}
                          >
                            View details
                          </button>
                        ) : (
                          <span className="log-card-no-details">No details</span>
                        )}
                        <time className="log-time-value" dateTime={row.observed_at} title={fullTimestamp}>
                          {hhmmss(row.observed_at)}
                        </time>
                      </div>
                    </article>
                  )
                })}
              </section>
            ))}
          </div>
        ) : (
          <table className="log-table" data-layout="desktop">
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
                const fullTimestamp = fullDateTime(row.observed_at)
                const needsSeparator = firstRowIDs.has(row.id)

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
                        <LogNodeLabel
                          nodeId={nodeId}
                          fallbackLabel={row.node_display_name}
                          onOpenNodeDetails={onOpenNodeDetails}
                        />
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
        )}
        <button type="button" className="secondary" onClick={onLoadMore}>Load more</button>
        <LogDetailsModal
          event={selectedEvent}
          onClose={() => setSelectedEvent(undefined)}
          onOpenNodeDetails={onOpenNodeDetails}
          renderers={[pkiLogDetailsRenderer, routingLogDetailsRenderer]}
        />
      </article>
    </section>
  )
}
