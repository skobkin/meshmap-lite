import type { LogEvent } from '../api/types'

interface Props {
  channels: string[]
  items: LogEvent[]
  loadError: string
  selectedKinds: number[]
  selectedChannel: string
  onChangeKinds: (kinds: number[]) => void
  onChangeChannel: (channel: string) => void
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

function formatTime(value: string): string {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

function detailsText(details?: Record<string, unknown>): string {
  if (!details || Object.keys(details).length === 0) return '-'
  return JSON.stringify(details)
}

export function LogPage({
  channels,
  items,
  loadError,
  selectedKinds,
  selectedChannel,
  onChangeKinds,
  onChangeChannel,
  onLoadMore
}: Props) {
  return (
    <section className="log-layout container-fluid">
      <details className="log-filters">
        <summary>Filters</summary>
        <div className="log-filters-content">
          <label>
            Event type
            <select
              aria-label="Event type filter"
              multiple
              onChange={(e) => {
                const options = Array.from((e.target as HTMLSelectElement).selectedOptions)
                onChangeKinds(options.map((o) => Number(o.value)).filter((v) => Number.isFinite(v)))
              }}
            >
              {eventKinds.map((item) => <option key={item.value} value={item.value} selected={selectedKinds.includes(item.value)}>{item.label}</option>)}
            </select>
          </label>
          <label>
            Channel
            <select
              aria-label="Channel filter"
              value={selectedChannel}
              onChange={(e) => onChangeChannel((e.target as HTMLSelectElement).value)}
            >
              <option value="">All channels</option>
              {channels.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          </label>
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
            {items.map((row) => (
              <tr key={row.id}>
                <td>{formatTime(row.observed_at)}</td>
                <td><code>{row.node_display_name || row.node_id || '-'}</code></td>
                <td>{row.event_kind_title}</td>
                <td>{row.encrypted ? 'yes' : 'no'}</td>
                <td>{row.channel_name || '-'}</td>
                <td><small>{detailsText(row.details)}</small></td>
              </tr>
            ))}
          </tbody>
        </table>
        <button type="button" className="secondary" onClick={onLoadMore}>Load more</button>
      </article>
    </section>
  )
}
