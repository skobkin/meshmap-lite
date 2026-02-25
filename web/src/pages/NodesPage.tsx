import type { NodeDetails } from '../api/types'
import { relativeTime } from '../utils/time'

interface Props {
  items: Array<{ node_id: string; display_name: string; role?: string }>
  selected?: string
  details?: NodeDetails
  onSelect: (id: string) => void
}

export function NodesPage({ items, selected, details, onSelect }: Props) {
  return (
    <section className="nodes-layout container-fluid">
      <article className="node-list">
        {items.map((n) => (
          <a key={n.node_id} href="#" className={selected === n.node_id ? 'active' : ''} onClick={(e) => { e.preventDefault(); onSelect(n.node_id) }}>
            <strong>{n.display_name}</strong>
          </a>
        ))}
      </article>
      <article>
        {details ? (
          <>
            <h3>{details.node.long_name ?? details.node.short_name ?? details.node.node_id}</h3>
            <p>ID: <code>{details.node.node_id}</code></p>
            <p>Role: {details.node.role ?? 'n/a'}</p>
            <p>Board: {details.node.board_model ?? 'n/a'}</p>
            <p>Firmware: {details.node.firmware_version ?? 'n/a'}</p>
            <p>Last update: {relativeTime(details.node.last_seen_any_event_at)}</p>
            <pre>{JSON.stringify(details.telemetry ?? {}, null, 2)}</pre>
          </>
        ) : <p>Select node</p>}
      </article>
    </section>
  )
}
