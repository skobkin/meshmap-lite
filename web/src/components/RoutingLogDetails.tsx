import { JsonDetailsView } from './JsonDetailsView'
import { ResolvedNodeData } from './ResolvedNodeData'

import type { LogDetailsRenderer } from './LogDetailsModal'
import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

const knownKeys = [
  'variant',
  'request_id',
  'from',
  'to',
  'route',
  'route_back',
  'error_reason',
  'traceroute_ref',
  'traceroute_status'
] as const

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return undefined
  }

  return value as Record<string, unknown>
}

function scalar(value: unknown): string | undefined {
  if (typeof value === 'string') {
    const trimmed = value.trim()

    return trimmed === '' ? undefined : trimmed
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }

  return undefined
}

function nodeIdList(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined
  }

  const items = value
    .map((item) => scalar(item))
    .filter((item): item is string => Boolean(item))

  return items.length > 0 ? items : undefined
}

interface ScalarRow {
  key: typeof knownKeys[number]
  label: string
  value: string
}

interface PathRow {
  key: 'route' | 'route_back'
  label: string
  value: string[]
}

function scalarRows(details: Record<string, unknown>): ScalarRow[] {
  const rows: ScalarRow[] = []
  const add = (label: string, key: ScalarRow['key']): void => {
    const value = scalar(details[key])
    if (value) {
      rows.push({ key, label, value })
    }
  }

  add('Variant', 'variant')
  add('Request ID', 'request_id')
  add('From', 'from')
  add('To', 'to')
  add('Error reason', 'error_reason')
  add('Traceroute ref', 'traceroute_ref')
  add('Traceroute status', 'traceroute_status')

  return rows
}

function pathRows(details: Record<string, unknown>): PathRow[] {
  const rows: PathRow[] = []
  const add = (label: string, key: PathRow['key']): void => {
    const value = nodeIdList(details[key])
    if (value) {
      rows.push({ key, label, value })
    }
  }

  add('Route', 'route')
  add('Return route', 'route_back')

  return rows
}

function extraDetails(details: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(details)) {
    if (!knownKeys.includes(key as typeof knownKeys[number])) {
      out[key] = value
    }
  }

  return out
}

function NodeReference({
  nodeId,
  onOpenNodeDetails
}: {
  nodeId: string
  onOpenNodeDetails?: (id: string) => void
}): JSX.Element {
  return (
    <ResolvedNodeData nodeId={nodeId}>
      {({ label, title }) => onOpenNodeDetails
        ? (
          <button
            type="button"
            className="chat-node-link log-details-node-link"
            title={title}
            onClick={() => onOpenNodeDetails(nodeId)}
          >
            <code>{label}</code>
          </button>
        )
        : (
          <span className="log-details-node-link" title={title}>
            <code>{label}</code>
          </span>
        )}
    </ResolvedNodeData>
  )
}

function RoutingLogDetailsView({
  event,
  onOpenNodeDetails
}: {
  event: LogEvent
  onOpenNodeDetails?: (id: string) => void
}): JSX.Element {
  const details = asRecord(event.details)
  if (!details) {
    return <JsonDetailsView value={event.details ?? {}} />
  }

  const rows = scalarRows(details)
  const paths = pathRows(details)
  const extra = extraDetails(details)

  if (rows.length === 0 && paths.length === 0 && Object.keys(extra).length === 0) {
    return <JsonDetailsView value={details} />
  }

  return (
    <div>
      {rows.length > 0 || paths.length > 0 ? (
        <dl className="log-details-grid">
          {rows.map((row) => (
            <div key={row.label} className="log-details-row">
              <dt className="log-details-label">{row.label}</dt>
              <dd className="log-details-value">
                {row.key === 'from' || row.key === 'to'
                  ? <NodeReference nodeId={row.value} onOpenNodeDetails={onOpenNodeDetails} />
                  : row.value}
              </dd>
            </div>
          ))}
          {paths.map((row) => (
            <div key={row.label} className="log-details-row">
              <dt className="log-details-label">{row.label}</dt>
              <dd className="log-details-value log-details-path">
                {row.value.map((nodeId, index) => (
                  <span key={`${row.key}-${nodeId}-${index}`}>
                    {index > 0 ? <span className="log-details-path-separator">{'->'}</span> : null}
                    <NodeReference nodeId={nodeId} onOpenNodeDetails={onOpenNodeDetails} />
                  </span>
                ))}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}
      {Object.keys(extra).length > 0 ? <JsonDetailsView value={extra} /> : null}
    </div>
  )
}

export const routingLogDetailsRenderer: LogDetailsRenderer = {
  id: 'routing',
  match: (event) => event.event_kind_value === 7,
  render: (event, context) => (
    <RoutingLogDetailsView event={event} onOpenNodeDetails={context.onOpenNodeDetails} />
  )
}
