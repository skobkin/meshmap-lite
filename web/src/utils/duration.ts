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
