import type { JSX } from 'preact'

interface Props {
  enabled: boolean
  loading: boolean
  count?: number
  truncated: boolean
  onToggle: (next: boolean) => void
}

export function MapTopologyToggle({ enabled, loading, count, truncated, onToggle }: Props): JSX.Element {
  const label = enabled
    ? (typeof count === 'number' ? `🕸️ (${count}${truncated ? '+' : ''})` : '🕸️')
    : '🕸️ Show all topology'

  const hint = loading
    ? 'Loading topology…'
    : (truncated && typeof count === 'number' ? `Capped at ${count}+` : '')

  return (
    <div className="map-topology-toggle" role="group" aria-label="Topology view">
      <label className="map-topology-toggle-label">
        <input
          type="checkbox"
          checked={enabled}
          disabled={loading}
          onChange={(e) => onToggle((e.currentTarget).checked)}
          aria-label="Show all topology"
        />
        <span aria-hidden={hint ? 'false' : 'true'}>{label}</span>
      </label>
      {hint && (
        <span className="map-topology-toggle-hint" role="status">{hint}</span>
      )}
    </div>
  )
}
