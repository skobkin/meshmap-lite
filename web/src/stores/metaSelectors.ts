import type { SourceSummaryRelease } from '../api/types'

function parseTime(value: string): number {
  const time = new Date(value).getTime()

  return Number.isNaN(time) ? Number.NaN : time
}

export function countNewerReleases(
  releases: SourceSummaryRelease[],
  dismissedAt: string
): number {
  if (!dismissedAt) {return releases.length}

  const dismissed = parseTime(dismissedAt)
  if (Number.isNaN(dismissed)) {return 0}

  let count = 0
  for (const release of releases) {
    const published = parseTime(release.published_at)
    if (Number.isNaN(published)) {continue}
    if (published > dismissed) {count++}
  }

  return count
}
