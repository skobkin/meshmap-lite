import { useEffect } from 'preact/hooks'

import { api } from '../api/client'
import { useUpdatesStore } from '../stores/updates'

import type { SourceSummary, UpdateRelease, UpdatesResponse } from '../api/types'
import type { JSX } from 'preact'

interface Props {
  source: SourceSummary
  dismissedPublishedAt: string
  onDismiss: (publishedAt: string) => void
}

function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {return value}

  return date.toLocaleString()
}

function isNewer(release: UpdateRelease, dismissed: string): boolean {
  if (!dismissed) {return false}

  const releaseTime = new Date(release.published_at).getTime()
  const dismissedTime = new Date(dismissed).getTime()
  if (Number.isNaN(releaseTime) || Number.isNaN(dismissedTime)) {return false}

  return releaseTime > dismissedTime
}

export function UpdatesPanel({ source, dismissedPublishedAt, onDismiss }: Props): JSX.Element {
  const cached = useUpdatesStore((s) => s.bySource[source.name])
  const loading = useUpdatesStore((s) => s.loading[source.name] ?? false)
  const error = useUpdatesStore((s) => s.errors[source.name] ?? '')
  const setResponse = useUpdatesStore((s) => s.setResponse)
  const setLoading = useUpdatesStore((s) => s.setLoading)
  const setError = useUpdatesStore((s) => s.setError)

  useEffect(() => {
    if (cached) {return}
    let cancelled = false
    setLoading(source.name, true)
    setError(source.name, '')
    void api.updates(source.name, 'html')
      .then((response: UpdatesResponse) => {
        if (cancelled) {return}
        setResponse(source.name, response)
      })
      .catch((err: unknown) => {
        if (cancelled) {return}
        setError(source.name, err instanceof Error ? err.message : 'request failed')
      })
      .finally(() => {
        if (cancelled) {return}
        setLoading(source.name, false)
      })

    return () => {
      cancelled = true
    }
  }, [cached, setError, setLoading, setResponse, source.name])

  const releases = cached?.releases ?? []
  const lastRelease = releases[0]
  const lastReleaseIsNew = lastRelease ? isNewer(lastRelease, dismissedPublishedAt) : false

  return (
    <div className="updates-panel" data-source={source.name}>
      <header className="updates-panel-header">
        <h3>{source.label}</h3>
        {source.releases_page_url && (
          <a href={source.releases_page_url} rel="noreferrer" target="_blank">
            View all releases
          </a>
        )}
      </header>
      {loading && <p className="info-modal-state" role="status">Loading...</p>}
      {error && <p className="info-modal-error" role="alert">{error}</p>}
      {!loading && !error && releases.length === 0 && (
        <p className="info-modal-state">No releases to show.</p>
      )}
      {!loading && !error && releases.length > 0 && (
        <ol className="updates-release-list">
          {releases.map((release, index) => {
            const newRelease = isNewer(release, dismissedPublishedAt)

            return (
              <li
                className={`updates-release${index === 0 ? ' updates-separator' : ''}`}
                key={release.html_url || `${release.version}-${release.published_at}`}
              >
                <header className="updates-release-header">
                  <h4>
                    {release.html_url ? (
                      <a href={release.html_url} rel="noreferrer" target="_blank">
                        {release.version}
                      </a>
                    ) : (
                      release.version
                    )}
                    {newRelease && <span className="updates-new-pill" role="status">NEW</span>}
                  </h4>
                  <time dateTime={release.published_at}>{formatDate(release.published_at)}</time>
                </header>
                <article
                  className="updates-release-body"
                  dangerouslySetInnerHTML={{ __html: release.body }}
                />
              </li>
            )
          })}
        </ol>
      )}
      {lastReleaseIsNew && lastRelease && (
        <footer className="updates-panel-footer">
          <button
            type="button"
            onClick={() => onDismiss(lastRelease.published_at)}
          >
            Mark as read
          </button>
        </footer>
      )}
    </div>
  )
}
