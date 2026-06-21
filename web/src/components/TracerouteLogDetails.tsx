import { useState } from 'preact/hooks'

import { type RenderedScalar, formatScalar, renderScalar } from '../utils/logValueRender'

import { JsonDetailsView } from './JsonDetailsView'
import { ResolvedNodeData } from './ResolvedNodeData'

import type { LogDetailsRenderer } from './LogDetailsModal'
import type { LogEvent } from '../api/types'
import type { JSX } from 'preact'

const scalarMetadataKeys = [
  'from',
  'to',
  'hop_start',
  'hop_limit'
] as const

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return undefined
  }

  return value as Record<string, unknown>
}

function nodeIdList(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined
  }

  const items = value
    .map((item) => formatScalar(item))
    .filter((item): item is string => typeof item === 'string' && item.length > 0)

  return items.length > 0 ? items : undefined
}

function snrList(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .map((item) => formatScalar(item))
    .filter((item): item is string => typeof item === 'string' && item.length > 0)
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString()
}

function metadataLabel(key: typeof scalarMetadataKeys[number]): string {
  switch (key) {
    case 'from':
      return 'From'
    case 'to':
      return 'To'
    case 'hop_start':
      return 'Hop start'
    case 'hop_limit':
      return 'Hop limit'
  }
}

interface MetadataRow {
  key: typeof scalarMetadataKeys[number]
  label: string
  value: RenderedScalar
}

interface RouteSection {
  key: 'forward' | 'return'
  title: string
  nodeIds: string[]
  snr: string[]
}

interface StepRow {
  type: string
  observedAt?: string
  reportedAt?: string
  packetId?: string
}

function metadataRows(details: Record<string, unknown>): MetadataRow[] {
  const rows: MetadataRow[] = []

  for (const key of scalarMetadataKeys) {
    const value = formatScalar(details[key])
    if (!value) {
      continue
    }

    rows.push({
      key,
      label: metadataLabel(key),
      value
    })
  }

  return rows
}

function routeSections(details: Record<string, unknown>): RouteSection[] {
  const forward = nodeIdList(details.forward_path) ?? nodeIdList(details.route)
  const routeBack = nodeIdList(details.return_path) ?? nodeIdList(details.route_back)
  const sections: RouteSection[] = []

  if (forward) {
    sections.push({
      key: 'forward',
      title: 'Route traced toward destination:',
      nodeIds: forward,
      snr: snrList(details.forward_snr)
    })
  }

  if (routeBack) {
    sections.push({
      key: 'return',
      title: 'Route traced back:',
      nodeIds: routeBack,
      snr: snrList(details.return_snr)
    })
  }

  return sections
}

function stepRows(details: Record<string, unknown>): StepRow[] {
  if (!Array.isArray(details.steps)) {
    return []
  }

  return details.steps.flatMap((item) => {
    const record = asRecord(item)
    if (!record) {
      return []
    }

    const type = formatScalar(record.type)
    if (typeof type !== 'string' || type.length === 0) {
      return []
    }

    return [{
      type,
      observedAt: optionalString(formatScalar(record.observed_at)),
      reportedAt: optionalString(formatScalar(record.reported_at)),
      packetId: optionalString(formatScalar(record.packet_id))
    }]
  })
}

function optionalString(value: RenderedScalar | undefined): string | undefined {
  return typeof value === 'string' && value.length > 0 ? value : undefined
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

function TracerouteRouteSection({
  section,
  onOpenNodeDetails
}: {
  section: RouteSection
  onOpenNodeDetails?: (id: string) => void
}): JSX.Element {
  return (
    <section className="traceroute-route-section" aria-labelledby={`traceroute-${section.key}-route`}>
      <h4 id={`traceroute-${section.key}-route`}>{section.title}</h4>
      <ol className="traceroute-hop-list">
        {section.nodeIds.map((nodeId, index) => (
          <li key={`${section.key}-${nodeId}-${index}`} className="traceroute-hop">
            <span className="traceroute-hop-index">{index + 1}</span>
            <span className="traceroute-hop-node">
              <NodeReference nodeId={nodeId} onOpenNodeDetails={onOpenNodeDetails} />
              {section.snr[index] ? (
                <span className="traceroute-hop-snr">SNR: {section.snr[index]} dB</span>
              ) : null}
            </span>
          </li>
        ))}
      </ol>
    </section>
  )
}

function TracerouteSteps({ steps }: { steps: StepRow[] }): JSX.Element {
  return (
    <section className="traceroute-steps" aria-labelledby="traceroute-steps-heading">
      <h4 id="traceroute-steps-heading">Steps</h4>
      <ol className="traceroute-step-list">
        {steps.map((step, index) => (
          <li key={`${step.type}-${step.packetId ?? 'no-packet'}-${index}`} className="traceroute-step">
            <span className="traceroute-step-type">{step.type}</span>
            <span className="traceroute-step-meta">
              {step.packetId ? `packet ${step.packetId}` : null}
              {step.packetId && step.observedAt ? ' · ' : null}
              {step.observedAt ? formatTime(step.observedAt) : null}
              {step.reportedAt ? ` · reported ${formatTime(step.reportedAt)}` : null}
            </span>
          </li>
        ))}
      </ol>
    </section>
  )
}

function TracerouteLogDetailsView({
  event,
  onOpenNodeDetails
}: {
  event: LogEvent
  onOpenNodeDetails?: (id: string) => void
}): JSX.Element {
  const [activeView, setActiveView] = useState<'route' | 'json'>('route')
  const details = asRecord(event.details)
  if (!details) {
    return <JsonDetailsView value={event.details ?? {}} />
  }

  const rows = metadataRows(details)
  const sections = routeSections(details)
  const steps = stepRows(details)

  if (rows.length === 0 && sections.length === 0 && steps.length === 0) {
    return <JsonDetailsView value={details} />
  }

  return (
    <div className="traceroute-log-details">
      <nav className="view-switch traceroute-view-switch" aria-label="Traceroute detail view">
        <ul role="tablist">
          <li>
            <button
              type="button"
              role="tab"
              aria-selected={activeView === 'route'}
              className={activeView === 'route' ? undefined : 'outline'}
              onClick={() => setActiveView('route')}
            >
              Route
            </button>
          </li>
          <li>
            <button
              type="button"
              role="tab"
              aria-selected={activeView === 'json'}
              className={activeView === 'json' ? undefined : 'outline'}
              onClick={() => setActiveView('json')}
            >
              Raw
            </button>
          </li>
        </ul>
      </nav>

      {activeView === 'route' ? (
        <div className="traceroute-summary" role="tabpanel">
          {sections.map((section) => (
            <TracerouteRouteSection
              key={section.key}
              section={section}
              onOpenNodeDetails={onOpenNodeDetails}
            />
          ))}
          {rows.length > 0 ? (
            <dl className="log-details-grid traceroute-metadata">
              {rows.map((row) => (
                <div key={row.key} className="log-details-row">
                  <dt className="log-details-label">{row.label}</dt>
                  <dd className="log-details-value">
                    {row.key === 'from' || row.key === 'to'
                      ? <NodeReference nodeId={row.value as string} onOpenNodeDetails={onOpenNodeDetails} />
                      : renderScalar(row.value)}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
          {steps.length > 0 ? <TracerouteSteps steps={steps} /> : null}
        </div>
      ) : (
        <div role="tabpanel">
          <JsonDetailsView value={details} />
        </div>
      )}
    </div>
  )
}

export const tracerouteLogDetailsRenderer: LogDetailsRenderer = {
  id: 'traceroute',
  match: (event) => event.event_kind_value === 5,
  render: (event, context) => (
    <TracerouteLogDetailsView event={event} onOpenNodeDetails={context.onOpenNodeDetails} />
  )
}
