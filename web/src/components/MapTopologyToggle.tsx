import type { JSX } from 'preact'

interface Props {
  enabled: boolean
  loading: boolean
  count?: number
  truncated: boolean
  onToggle: (next: boolean) => void
}

export function MapTopologyToggle({ enabled, loading, count, truncated, onToggle }: Props): JSX.Element {
  const showCount = enabled && typeof count === 'number'
  const countText = showCount ? ` (${count}${truncated ? '+' : ''})` : ''

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
        <svg className="map-topology-toggle-icon" aria-hidden="true" viewBox="0 0 16 16" focusable="false">
          <circle cx="8" cy="8" r="1.2" fill="currentColor" />
          <path d="M8 1.5v5.3M8 9.2v5.3M1.5 8h5.3M9.2 8h5.3M3.4 3.4l3.7 3.7M8.9 8.9l3.7 3.7M3.4 12.6l3.7-3.7M8.9 7.1l3.7-3.7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" fill="none" />
        </svg>
        <span
          className={`map-topology-toggle-count${showCount ? '' : ' map-topology-toggle-count-hidden'}`}
          aria-hidden={hint ? 'false' : 'true'}
        >
          {countText || ' '}
        </span>
      </label>
      {hint && (
        <span className="map-topology-toggle-hint" role="status">{hint}</span>
      )}
    </div>
  )
}
