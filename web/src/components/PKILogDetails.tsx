import { type RenderedScalar, formatScalar, renderScalar } from '../utils/logValueRender'

import { JsonDetailsView } from './JsonDetailsView'
import { ResolvedNodeData } from './ResolvedNodeData'

import type { LogDetailsRenderer } from './LogDetailsModal'
import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

const knownKeys = [
  'sender_node_id',
  'destination_node_id',
  'gateway_id',
  'topic_channel',
  'envelope_channel_id',
  'packet_id',
  'encrypted',
  'decrypted',
  'pki_encrypted',
  'payload_size_bytes',
  'hop_start',
  'hop_limit',
  'priority'
] as const

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return undefined
  }

  return value as Record<string, unknown>
}

interface KnownRow {
  key: typeof knownKeys[number]
  label: string
  value: RenderedScalar
}

function knownRows(details: Record<string, unknown>): KnownRow[] {
  const rows: KnownRow[] = []
  const add = (label: string, key: typeof knownKeys[number]): void => {
    const value = formatScalar(details[key])
    if (value) {
      rows.push({ key, label, value })
    }
  }

  add('Sender', 'sender_node_id')
  add('Destination', 'destination_node_id')
  add('Gateway', 'gateway_id')
  add('Topic channel', 'topic_channel')
  add('Envelope channel', 'envelope_channel_id')
  add('Packet ID', 'packet_id')
  add('Encrypted', 'encrypted')
  add('Decrypted', 'decrypted')
  add('PKI encrypted', 'pki_encrypted')
  add('Payload size', 'payload_size_bytes')
  add('Hop start', 'hop_start')
  add('Hop limit', 'hop_limit')
  add('Priority', 'priority')

  return rows
}

function isNodeReferenceKey(key: KnownRow['key']): boolean {
  return key === 'sender_node_id' || key === 'destination_node_id' || key === 'gateway_id'
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

function PKILogDetailsView({
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

  const rows = knownRows(details)
  const extra = extraDetails(details)

  if (rows.length === 0 && Object.keys(extra).length === 0) {
    return <JsonDetailsView value={details} />
  }

  return (
    <div>
      {rows.length > 0 ? (
        <dl className="log-details-grid">
          {rows.map((row) => (
            <div key={row.label} className="log-details-row">
              <dt className="log-details-label">{row.label}</dt>
              <dd className="log-details-value">
                {isNodeReferenceKey(row.key) ? (
                  <ResolvedNodeData nodeId={row.value as string}>
                    {({ label, title, nodeId }) => onOpenNodeDetails
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
                ) : renderScalar(row.value)}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}
      {Object.keys(extra).length > 0 ? <JsonDetailsView value={extra} /> : null}
    </div>
  )
}

export const pkiLogDetailsRenderer: LogDetailsRenderer = {
  id: 'pki',
  match: (event) => event.event_kind_value === 11,
  render: (event, context) => (
    <PKILogDetailsView event={event} onOpenNodeDetails={context.onOpenNodeDetails} />
  )
}
