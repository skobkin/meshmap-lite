// @vitest-environment jsdom

import { fireEvent, render, screen } from '@testing-library/preact'
import { describe, expect, it, vi } from 'vitest'

import { InfoModal } from './InfoModal'

describe('InfoModal', () => {
  it('renders supplied HTML in a scrollable content container', () => {
    render(
      <InfoModal
        content="<h1>Hello</h1><div class='callout callout-note'>Note</div>"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByRole('dialog', { name: 'Site information' })).toBeTruthy()
    expect(screen.getByText('Hello')).toBeTruthy()
    expect(document.querySelector('.info-modal-body')).toBeTruthy()
  })

  it('calls dismiss from the primary action', () => {
    const onDismiss = vi.fn()
    render(
      <InfoModal
        content="<p>Hello</p>"
        onClose={() => undefined}
        onDismiss={onDismiss}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Got it' }))

    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('shows the updated notice when requested', () => {
    render(
      <InfoModal
        content="<p>Hello</p>"
        showUpdatedNotice
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByText('This information was updated since you last dismissed it.')).toBeTruthy()
  })
})
