import { HtmlModal } from './HtmlModal'
import { UpdatesPanel } from './UpdatesPanel'

import type { SourceSummary } from '../api/types'
import type { JSX } from 'preact'

export interface AppModalTab {
  id: string
  label: string
  isInformation?: boolean
  source?: SourceSummary
}

interface Props {
  activeTabID: string
  infoContent: string
  infoError: string
  infoLoading: boolean
  infoShowUpdatedNotice: boolean
  tabs: AppModalTab[]
  updatesDismissedAt: Record<string, string>
  onClose: () => void
  onDismiss: () => void
  onDismissUpdates: (source: string, publishedAt: string) => void
  onSelectTab: (id: string) => void
}

export function AppModal({
  activeTabID,
  infoContent,
  infoError,
  infoLoading,
  infoShowUpdatedNotice,
  tabs,
  updatesDismissedAt,
  onClose,
  onDismiss,
  onDismissUpdates,
  onSelectTab
}: Props): JSX.Element {
  const activeTab = tabs.find((tab) => tab.id === activeTabID) ?? tabs[0]
  const isInformation = Boolean(activeTab?.isInformation)
  const source = activeTab?.source

  const tabsNav = (
    <>
      {tabs.map((tab) => {
        const selected = tab.id === activeTab?.id

        return (
          <button
            aria-selected={selected}
            className={selected ? 'app-modal-tab selected' : 'app-modal-tab outline'}
            key={tab.id}
            role="tab"
            type="button"
            onClick={() => onSelectTab(tab.id)}
          >
            {tab.label}
          </button>
        )
      })}
    </>
  )

  if (isInformation) {
    return (
      <HtmlModal
        ariaCloseLabel="Close site information"
        content={infoContent}
        error={infoError}
        loading={infoLoading}
        showUpdatedNotice={infoShowUpdatedNotice}
        tabs={tabsNav}
        title="Site information"
        onClose={onClose}
        onDismiss={onDismiss}
      />
    )
  }

  if (source) {
    return (
      <HtmlModal
        ariaCloseLabel={`Close ${source.label} updates`}
        content=""
        dismissLabel="Mark as read"
        tabs={tabsNav}
        title={source.label}
        onClose={onClose}
        onDismiss={() => {
          const newest = source.releases[0]
          if (newest) {
            onDismissUpdates(source.name, newest.published_at)
          }
        }}
      >
        <UpdatesPanel
          dismissedPublishedAt={updatesDismissedAt[source.name] ?? ''}
          source={source}
          onDismiss={(publishedAt) => onDismissUpdates(source.name, publishedAt)}
        />
      </HtmlModal>
    )
  }

  return (
    <HtmlModal
      ariaCloseLabel="Close"
      content=""
      tabs={tabsNav}
      title="Updates"
      onClose={onClose}
      onDismiss={onDismiss}
    />
  )
}
