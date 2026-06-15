// @vitest-environment jsdom

import { fireEvent, render, screen } from '@testing-library/preact'
import { describe, expect, it, vi } from 'vitest'

import { HtmlModal } from './HtmlModal'

describe('HtmlModal', () => {
  it('renders the supplied title and content', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<h1>Hello</h1>"
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByRole('dialog', { name: 'Test modal' })).toBeTruthy()
    expect(screen.getByText('Hello')).toBeTruthy()
    expect(document.querySelector('.info-modal-body')).toBeTruthy()
  })

  it('calls onClose when the close button is pressed', () => {
    const onClose = vi.fn()
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<p>Hello</p>"
        title="Test modal"
        onClose={onClose}
        onDismiss={() => undefined}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Close test' }))

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onDismiss from the primary action', () => {
    const onDismiss = vi.fn()
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<p>Hello</p>"
        title="Test modal"
        onClose={() => undefined}
        onDismiss={onDismiss}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Got it' }))

    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('shows the updated notice when requested', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<p>Hello</p>"
        showUpdatedNotice
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByText('This information was updated since you last dismissed it.')).toBeTruthy()
  })

  it('renders tabs when provided and exposes them in a tablist', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<p>Hello</p>"
        tabs={<>
          <button role="tab" aria-selected="true" type="button">One</button>
          <button role="tab" aria-selected="false" type="button">Two</button>
        </>}
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    const list = screen.getByRole('tablist', { name: 'Test modal sections' })
    expect(list).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'One' })).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'Two' })).toBeTruthy()
  })

  it('renders custom children inside the body', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<p>Hello</p>"
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      >
        <p data-testid="custom-child">child</p>
      </HtmlModal>
    )

    expect(screen.getByTestId('custom-child')).toBeTruthy()
  })

  it('uses a custom dismiss label when provided', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content="<p>Hello</p>"
        dismissLabel="Dismiss"
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeTruthy()
  })

  it('shows the loading state when loading', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content=""
        loading
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByText('Loading...')).toBeTruthy()
  })

  it('shows the error message when an error is supplied', () => {
    render(
      <HtmlModal
        ariaCloseLabel="Close test"
        content=""
        error="Boom"
        title="Test modal"
        onClose={() => undefined}
        onDismiss={() => undefined}
      />
    )

    expect(screen.getByRole('alert').textContent).toBe('Boom')
  })
})
