import { useEffect, useId } from 'preact/hooks'

import type { JSX } from 'preact'

interface Props {
  content: string
  error?: string
  loading?: boolean
  showUpdatedNotice?: boolean
  onClose: () => void
  onDismiss: () => void
}

export function InfoModal({ content, error, loading = false, showUpdatedNotice = false, onClose, onDismiss }: Props): JSX.Element {
  const titleId = useId()

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('keydown', onKeyDown)

    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  return (
    <div
      className="info-modal-backdrop"
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
        className="info-modal"
        role="dialog"
      >
        <header className="info-modal-header">
          <h2 id={titleId}>Site information</h2>
          <button
            aria-label="Close site information"
            className="secondary info-modal-close"
            type="button"
            onClick={onClose}
          >
            x
          </button>
        </header>
        <div className="info-modal-body">
          {showUpdatedNotice && (
            <p className="info-modal-notice" role="status">
              This information was updated since you last dismissed it.
            </p>
          )}
          {loading && <p className="info-modal-state">Loading...</p>}
          {error && <p className="info-modal-error" role="alert">{error}</p>}
          {!loading && !error && (
            <article
              className="info-markdown"
              dangerouslySetInnerHTML={{ __html: content }}
            />
          )}
        </div>
        <footer className="info-modal-footer">
          <button type="button" onClick={onDismiss}>Got it</button>
        </footer>
      </section>
    </div>
  )
}
