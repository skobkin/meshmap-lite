import { Fragment } from 'preact'
import { useEffect, useState } from 'preact/hooks'

import { dayKey, dayLabel, fullDateTime, hhmmss } from '../utils/time'

import { LogDetailsModal, hasLogDetails } from './LogDetailsModal'
import { pkiLogDetailsRenderer } from './PKILogDetails'
import { ResolvedNodeData } from './ResolvedNodeData'
import { routingLogDetailsRenderer } from './RoutingLogDetails'

import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  items: LogEvent[]
  showNodeColumn: boolean
  compact?: boolean
  maxBodyRows?: number
  emptyText?: string
  onOpenNodeDetails: (id: string) => void
}

const mobileLogMediaQuery = '(max-width: 768px)'

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

export function LogEventList({
  items,
  showNodeColumn,
  compact,
  maxBodyRows,
  emptyText = 'No log events.',
  onOpenNodeDetails
}: Props): JSX.Element {
  const [selectedEvent, setSelectedEvent] = useState<LogEvent>()
  const mobileLayout = useMobileLogLayout()
  const dayGroups = groupLogItemsByDay(items)
  const firstRowIDs = new Set(dayGroups.map((group) => group.items[0]?.id).filter((id): id is number => typeof id === 'number'))
  const colSpan = showNodeColumn ? 7 : 6
  const scrollStyle = maxBodyRows && maxBodyRows > 0
    ? { maxHeight: `${maxBodyRows * 2.45}rem`, overflowY: 'auto' }
    : undefined

  return (
    <>
      <div
        className={`log-event-list${compact ? ' compact' : ''}${maxBodyRows ? ' scrollable' : ''}`}
        style={scrollStyle}
      >
        {items.length === 0 ? (
          <p className="node-list-empty">{emptyText}</p>
        ) : mobileLayout ? (
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
                        {showNodeColumn && (
                          <LogNodeLabel
                            nodeId={nodeId}
                            fallbackLabel={row.node_display_name}
                            onOpenNodeDetails={onOpenNodeDetails}
                          />
                        )}
                        <strong className="log-card-type">{row.event_kind_title}</strong>
                      </div>
                      <dl className="log-card-meta">
                        <div>
                          <dt>Channel</dt>
                          <dd>{row.channel_name ?? '-'}</dd>
                        </div>
                        <div>
                          <dt>Gateway</dt>
                          <dd>
                            <LogNodeLabel
                              nodeId={row.mqtt_uploader_node_id}
                              fallbackLabel={row.mqtt_uploader_display_name}
                              onOpenNodeDetails={onOpenNodeDetails}
                            />
                          </dd>
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
                {showNodeColumn && <th>Node</th>}
                <th>Type</th>
                <th>Encrypted</th>
                <th>Channel</th>
                <th>Gateway</th>
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
                        <td colSpan={colSpan}>{dayLabel(row.observed_at)}</td>
                      </tr>
                    )}
                    <tr>
                      <td className="log-time-cell" title={fullTimestamp} aria-label={fullTimestamp}>
                        <span className="log-time-value">{hhmmss(row.observed_at)}</span>
                      </td>
                      {showNodeColumn && (
                        <td>
                          <LogNodeLabel
                            nodeId={nodeId}
                            fallbackLabel={row.node_display_name}
                            onOpenNodeDetails={onOpenNodeDetails}
                          />
                        </td>
                      )}
                      <td>{row.event_kind_title}</td>
                      <td>{row.encrypted ? 'yes' : 'no'}</td>
                      <td>{row.channel_name ?? '-'}</td>
                      <td>
                        <LogNodeLabel
                          nodeId={row.mqtt_uploader_node_id}
                          fallbackLabel={row.mqtt_uploader_display_name}
                          onOpenNodeDetails={onOpenNodeDetails}
                        />
                      </td>
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
      </div>
      <LogDetailsModal
        event={selectedEvent}
        onClose={() => setSelectedEvent(undefined)}
        onOpenNodeDetails={onOpenNodeDetails}
        renderers={[pkiLogDetailsRenderer, routingLogDetailsRenderer]}
      />
    </>
  )
}
