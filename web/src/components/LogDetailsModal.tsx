import { useEffect, useId } from 'preact/hooks'

import { JsonDetailsView } from './JsonDetailsView'

import type { LogEvent } from '../api/types'
import type { ComponentChildren } from 'preact'

export interface LogDetailsRenderer {
  id: string
  match: (event: LogEvent) => boolean
  render: (event: LogEvent) => ComponentChildren
}

const registeredLogDetailsRenderers: LogDetailsRenderer[] = []

export function registerLogDetailsRenderer(renderer: LogDetailsRenderer): () => void {
  registeredLogDetailsRenderers.push(renderer)

  return () => {
    const index = registeredLogDetailsRenderers.indexOf(renderer)
    if (index >= 0) {
      registeredLogDetailsRenderers.splice(index, 1)
    }
  }
}

export function resolveLogDetailsRenderer(
  event: LogEvent,
  renderers: LogDetailsRenderer[] = registeredLogDetailsRenderers
): LogDetailsRenderer | undefined {
  return renderers.find((renderer) => renderer.match(event))
}

export function hasLogDetails(details?: Record<string, unknown>): boolean {
  return Boolean(details && Object.keys(details).length > 0)
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  return date.toLocaleString()
}

function logTitle(event: LogEvent): string {
  const parts = [event.event_kind_title]

  if (event.node_display_name ?? event.node_id) {
    parts.push(event.node_display_name ?? event.node_id ?? '')
  }

  parts.push(formatTime(event.observed_at))

  return parts.join(' · ')
}

interface Props {
  event?: LogEvent
  onClose: () => void
  renderers?: LogDetailsRenderer[]
}

export function LogDetailsModal({ event, onClose, renderers }: Props) {
  const titleId = useId()

  useEffect(() => {
    if (!event) return undefined

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('keydown', onKeyDown)

    return () => document.removeEventListener('keydown', onKeyDown)
  }, [event, onClose])

  if (!event) return null

  const renderer = resolveLogDetailsRenderer(event, renderers)
  const content = renderer
    ? renderer.render(event)
    : <JsonDetailsView value={event.details ?? {}} />

  return (
    <div
      className="log-details-modal-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose()
        }
      }}
    >
      <section
        aria-labelledby={titleId}
        aria-modal="true"
        className="log-details-modal"
        role="dialog"
      >
        <header className="log-details-modal-header">
          <div>
            <p className="log-details-modal-kicker">Event details</p>
            <h3 id={titleId}>{logTitle(event)}</h3>
          </div>
          <button
            aria-label="Close details"
            className="secondary"
            type="button"
            onClick={onClose}
          >
            Close
          </button>
        </header>
        <div className="log-details-modal-body">
          {content}
        </div>
      </section>
    </div>
  )
}
