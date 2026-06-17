import { useState } from 'preact/hooks'

import { JsonDetailsView } from './JsonDetailsView'
import { ResolvedNodeData } from './ResolvedNodeData'

import type { LogDetailsRenderer } from './LogDetailsModal'
import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

// `rr` is rendered explicitly (with a human-readable label) above
// this list, so it is intentionally NOT in scalarKeys — keeping it
// here would also render a duplicate "Request/Response" row showing
// the raw enum code, which is unhelpful on its own.
const scalarKeys = [
  'from',
  'to'
] as const

const subPayloadKeys = ['stats', 'history', 'heartbeat'] as const
const textBytesKey = 'text_bytes'

// RR is stored as the numeric value of the meshtastic
// StoreAndForward_RequestResponse enum; this map turns that back into
// a human-readable label for the renderer. -1 is the Go-side sentinel
// for an RR value the pinned proto did not recognise (a newer firmware
// shipped a code we have not seen). For those we surface a distinct
// "UNKNOWN" label so the renderer can also show the preserved raw
// string.
const RR_UNKNOWN = -1
const RR_LABELS: Record<number, string> = {
  0: 'UNSET',
  1: 'ROUTER_ERROR',
  2: 'ROUTER_HEARTBEAT',
  3: 'ROUTER_PING',
  4: 'ROUTER_PONG',
  5: 'ROUTER_BUSY',
  6: 'ROUTER_HISTORY',
  7: 'ROUTER_STATS',
  8: 'ROUTER_TEXT_DIRECT',
  9: 'ROUTER_TEXT_BROADCAST',
  64: 'CLIENT_ERROR',
  65: 'CLIENT_HISTORY',
  66: 'CLIENT_STATS',
  67: 'CLIENT_PING',
  68: 'CLIENT_PONG',
  106: 'CLIENT_ABORT'
}

function rrLabel(rr: number): string {
  if (rr === RR_UNKNOWN) {
    return 'UNKNOWN'
  }

  return RR_LABELS[rr] ?? `RR ${rr}`
}

type Role = 'router' | 'client' | 'unknown'

function roleFromRR(rr: number): Role {
  if (rr === RR_UNKNOWN) {
    return 'unknown'
  }

  return rr >= 64 ? 'client' : 'router'
}

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

  // RR is the only piece we need to synthesize other fields from — role
  // is not stored, only derived, so we hand-roll the two extra rows
  // rather than try to fold them into the generic scalarRows() loop.
  const rrRaw = details.rr
  const rrNumber = typeof rrRaw === 'number'
    ? rrRaw
    : (typeof rrRaw === 'string' && rrRaw.trim() !== '' && !Number.isNaN(Number(rrRaw))
        ? Number(rrRaw)
        : undefined)

  const rrValue = rrNumber !== undefined ? rrLabel(rrNumber) : undefined
  const roleValue = rrNumber !== undefined ? roleFromRR(rrNumber) : undefined

  // raw_rr / raw_role are populated by the Go side when a publisher
  // shipped an enum value the pinned proto did not know. We surface
  // them so the operator can see exactly what came over the wire —
  // useful when a new firmware version introduces a new RR code.
  const rawRR = typeof details.raw_rr === 'string' && details.raw_rr.trim() !== ''
    ? details.raw_rr
    : undefined
  const rawRole = typeof details.raw_role === 'string' && details.raw_role.trim() !== ''
    ? details.raw_role
    : undefined

  // The text body is intentionally not persisted; we only retain its
  // byte count, so the renderer surfaces the size instead.
  const textBytesRaw = details[textBytesKey]
  const textBytes = typeof textBytesRaw === 'number' ? textBytesRaw : undefined

  const hasStructured = rows.length > 0
    || subPayloads.length > 0
    || rrValue !== undefined
    || textBytes !== undefined
    || rawRR !== undefined
    || rawRole !== undefined

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
          <dl className="log-details-grid store-forward-scalar">
            {rrValue !== undefined ? (
              <div className="log-details-row">
                <dt className="log-details-label">Request/Response</dt>
                <dd className="log-details-value">
                  <code>{rrValue}</code>
                </dd>
              </div>
            ) : null}
            {roleValue !== undefined ? (
              <div className="log-details-row">
                <dt className="log-details-label">Role</dt>
                <dd className="log-details-value">
                  <code>{roleValue}</code>
                </dd>
              </div>
            ) : null}
            {rawRR !== undefined ? (
              <div className="log-details-row">
                <dt className="log-details-label">Request/Response (raw)</dt>
                <dd className="log-details-value">
                  <code>{rawRR}</code>
                </dd>
              </div>
            ) : null}
            {rawRole !== undefined ? (
              <div className="log-details-row">
                <dt className="log-details-label">Role (raw)</dt>
                <dd className="log-details-value">
                  <code>{rawRole}</code>
                </dd>
              </div>
            ) : null}
            {rows.map((row) => (
              <div key={row.key} className="log-details-row">
                <dt className="log-details-label">{row.label}</dt>
                <dd className="log-details-value">
                  <NodeReferenceOrBroadcast nodeId={row.value} onOpenNodeDetails={onOpenNodeDetails} />
                </dd>
              </div>
            ))}
          </dl>
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
          {textBytes !== undefined ? (
            <section className="store-forward-text" aria-labelledby="store-forward-text-heading">
              <h4 id="store-forward-text-heading">Replayed text</h4>
              <p className="store-forward-text-block">
                {textBytes} {textBytes === 1 ? 'byte' : 'bytes'} (body not stored)
              </p>
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
