import { useState } from 'preact/hooks'

import { JsonDetailsView } from './JsonDetailsView'
import { ResolvedNodeData } from './ResolvedNodeData'

import type { LogDetailsRenderer } from './LogDetailsModal'
import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

const scalarKeys = [
  'rr',
  'role',
  'from',
  'to'
] as const

const subPayloadKeys = ['stats', 'history', 'heartbeat'] as const
const textKey = 'text'

// Meshtastic broadcast node ID. When the S&F packet is a broadcast
// announce, "to" carries this value rather than a real node ID; linking
// to it would navigate to a non-existent node page, so we render the
// plain word "broadcast" instead.
const BROADCAST_NODE_ID = '!ffffffff'

const statsLabels: Record<string, string> = {
  messages_total: 'Messages total',
  messages_saved: 'Messages saved',
  messages_max: 'Messages max',
  up_time: 'Up time (s)',
  requests: 'Requests',
  requests_history: 'History requests',
  heartbeat: 'Heartbeat enabled',
  return_max: 'Return max',
  return_window: 'Return window (s)'
}

const historyLabels: Record<string, string> = {
  history_messages: 'History messages',
  window: 'Window (min)',
  last_request: 'Last request'
}

const heartbeatLabels: Record<string, string> = {
  period: 'Period (s)',
  secondary: 'Secondary'
}

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

function isNodeReferenceKey(key: typeof scalarKeys[number]): boolean {
  return key === 'from' || key === 'to'
}

function isBroadcastNodeID(nodeId: string): boolean {
  return nodeId.toLowerCase() === BROADCAST_NODE_ID
}

interface ScalarRow {
  key: typeof scalarKeys[number]
  label: string
  value: string
}

function scalarRows(details: Record<string, unknown>): ScalarRow[] {
  const rows: ScalarRow[] = []
  for (const key of scalarKeys) {
    const value = scalar(details[key])
    if (!value) {
      continue
    }
    rows.push({ key, label: labelForKey(key), value })
  }

  return rows
}

function labelForKey(key: typeof scalarKeys[number]): string {
  switch (key) {
    case 'rr':
      return 'Request/Response'
    case 'role':
      return 'Role'
    case 'from':
      return 'From'
    case 'to':
      return 'To'
  }
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

function BroadcastLabel(): JSX.Element {
  return <span className="log-details-broadcast">broadcast</span>
}

function NodeReferenceOrBroadcast({
  nodeId,
  onOpenNodeDetails
}: {
  nodeId: string
  onOpenNodeDetails?: (id: string) => void
}): JSX.Element {
  if (isBroadcastNodeID(nodeId)) {
    return <BroadcastLabel />
  }

  return <NodeReference nodeId={nodeId} onOpenNodeDetails={onOpenNodeDetails} />
}

function labelForField(key: string, labels: Record<string, string>): string {
  return labels[key] ?? key
}

function subPayloadLabel(key: string): string {
  switch (key) {
    case 'stats':
      return 'Router stats'
    case 'history':
      return 'Router history'
    case 'heartbeat':
      return 'Router heartbeat'
    default:
      return key
  }
}

function SubPayloadGrid({
  title,
  payload,
  labels
}: {
  title: string
  payload: Record<string, unknown>
  labels: Record<string, string>
}): JSX.Element {
  const entries = Object.entries(payload)

  return (
    <section className="store-forward-subpayload" aria-labelledby={`store-forward-${title}`}>
      <h4 id={`store-forward-${title}`}>{title}</h4>
      <dl className="log-details-grid">
        {entries.map(([key, value]) => {
          const rendered = scalar(value)
          if (!rendered) {
            return null
          }

          return (
            <div key={key} className="log-details-row">
              <dt className="log-details-label">{labelForField(key, labels)}</dt>
              <dd className="log-details-value">{rendered}</dd>
            </div>
          )
        })}
      </dl>
    </section>
  )
}

function StoreForwardLogDetailsView({
  event,
  onOpenNodeDetails
}: {
  event: LogEvent
  onOpenNodeDetails?: (id: string) => void
}): JSX.Element {
  const [activeView, setActiveView] = useState<'details' | 'raw'>('details')
  const details = asRecord(event.details)
  if (!details) {
    return <JsonDetailsView value={event.details ?? {}} />
  }

  const rows = scalarRows(details)
  const subPayloads = subPayloadKeys
    .map((key: string) => {
      const payload = asRecord(details[key])

      return payload ? { key, payload } : null
    })
    .filter((entry): entry is { key: string, payload: Record<string, unknown> } => entry !== null)
  const text = scalar(details[textKey])

  const hasStructured = rows.length > 0 || subPayloads.length > 0 || Boolean(text)

  if (!hasStructured) {
    return <JsonDetailsView value={details} />
  }

  return (
    <div className="store-forward-log-details">
      <nav className="view-switch store-forward-view-switch" aria-label="Store & Forward detail view">
        <ul role="tablist">
          <li>
            <button
              type="button"
              role="tab"
              aria-selected={activeView === 'details'}
              className={activeView === 'details' ? undefined : 'outline'}
              onClick={() => setActiveView('details')}
            >
              Details
            </button>
          </li>
          <li>
            <button
              type="button"
              role="tab"
              aria-selected={activeView === 'raw'}
              className={activeView === 'raw' ? 'outline' : undefined}
              onClick={() => setActiveView('raw')}
            >
              Raw
            </button>
          </li>
        </ul>
      </nav>

      {activeView === 'details' ? (
        <div role="tabpanel" className="store-forward-summary">
          {rows.length > 0 ? (
            <dl className="log-details-grid store-forward-scalar">
              {rows.map((row) => (
                <div key={row.key} className="log-details-row">
                  <dt className="log-details-label">{row.label}</dt>
                  <dd className="log-details-value">
                    {isNodeReferenceKey(row.key)
                      ? <NodeReferenceOrBroadcast nodeId={row.value} onOpenNodeDetails={onOpenNodeDetails} />
                      : row.value}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
          {subPayloads.map(({ key, payload }: { key: string, payload: Record<string, unknown> }) => {
            const labels: Record<string, string> = key === 'stats'
              ? statsLabels
              : key === 'history'
                ? historyLabels
                : heartbeatLabels

            return <SubPayloadGrid
              key={key}
              title={subPayloadLabel(key)}
              payload={payload}
              labels={labels}
            />
          })}
          {text ? (
            <section className="store-forward-text" aria-labelledby="store-forward-text-heading">
              <h4 id="store-forward-text-heading">Replayed text</h4>
              <pre className="store-forward-text-block">{text}</pre>
            </section>
          ) : null}
        </div>
      ) : (
        <div role="tabpanel">
          <JsonDetailsView value={details} />
        </div>
      )}
    </div>
  )
}

export const storeForwardLogDetailsRenderer: LogDetailsRenderer = {
  id: 'store_forward',
  match: (event) => event.event_kind_value === 12,
  render: (event, context) => (
    <StoreForwardLogDetailsView event={event} onOpenNodeDetails={context.onOpenNodeDetails} />
  )
}
