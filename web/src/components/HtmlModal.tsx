import { useEffect, useId } from 'preact/hooks'

import type { ComponentChildren, JSX } from 'preact'

interface Props {
  ariaCloseLabel: string
  children?: ComponentChildren
  content: string
  error?: string
  loading?: boolean
  showUpdatedNotice?: boolean
  tabs?: ComponentChildren
  title: string
  onClose: () => void
  onDismiss: () => void
  dismissLabel?: string
}

export function HtmlModal({
  ariaCloseLabel,
  children,
  content,
  error,
  loading = false,
  showUpdatedNotice = false,
  tabs,
  title,
  onClose,
  onDismiss,
  dismissLabel = 'Got it'
}: Props): JSX.Element {
  const titleId = useId()
  const tabsId = useId()

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
          <h2 id={titleId}>{title}</h2>
          <button
            aria-label={ariaCloseLabel}
            className="secondary info-modal-close"
            type="button"
            onClick={onClose}
          >
            x
          </button>
        </header>
        {tabs && (
          <div id={tabsId} className="app-modal-tabs" role="tablist" aria-label={`${title} sections`}>
            {tabs}
          </div>
        )}
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
          {children}
        </div>
        <footer className="info-modal-footer">
          <button type="button" onClick={onDismiss}>{dismissLabel}</button>
        </footer>
      </section>
    </div>
  )
}
