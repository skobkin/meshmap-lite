export function parseDurationMs(raw?: string): number | undefined {
  if (!raw) {return undefined}
  const token = /([0-9]+(?:\.[0-9]+)?)(ns|us|µs|ms|s|m|h)/g
  let total = 0
  let found = false
  for (const match of raw.matchAll(token)) {
    found = true
    const n = Number(match[1])
    const unit = match[2]
    if (!Number.isFinite(n)) {continue}
    if (unit === 'h') {total += n * 3600000}
    if (unit === 'm') {total += n * 60000}
    if (unit === 's') {total += n * 1000}
    if (unit === 'ms') {total += n}
    if (unit === 'us' || unit === 'µs') {total += n / 1000}
    if (unit === 'ns') {total += n / 1000000}
  }
  if (!found) {return undefined}

  return Math.max(0, Math.floor(total))
}

// formatDurationSeconds renders a whole-second duration as a compact
// human-readable string (e.g. "1d 02h 03m", "02h 03m", "03m", "0m").
// Returns null for null/undefined/NaN/negative input so callers can fall back
// to a placeholder like "—".
//
// See https://git.skobk.in/skobkin/meshmap-lite/issues/103
// — candidate for the centralized duration formatter refactor.
export function formatDurationSeconds(totalSeconds: number | null | undefined): string | null {
  if (totalSeconds === null || totalSeconds === undefined) {return null}
  if (!Number.isFinite(totalSeconds) || totalSeconds < 0) {return null}
  const s = Math.floor(totalSeconds)
  const days = Math.floor(s / 86400)
  const hours = Math.floor((s % 86400) / 3600)
  const minutes = Math.floor((s % 3600) / 60)
  if (days > 0) {
    return `${days}d ${String(hours).padStart(2, '0')}h ${String(minutes).padStart(2, '0')}m`
  }
  if (hours > 0) {
    return `${String(hours).padStart(2, '0')}h ${String(minutes).padStart(2, '0')}m`
  }
  if (minutes > 0) {
    return `${minutes}m`
  }

  return '0m'
}
