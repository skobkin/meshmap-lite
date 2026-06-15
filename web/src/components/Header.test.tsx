// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import { describe, expect, it } from 'vitest'

import { Header } from './Header'

function renderHeader(unreadCount: number): void {
  render(
    <Header
      appName="MeshMap Lite"
      infoAvailable
      mqttStatus="connected"
      page="map"
      unreadCount={unreadCount}
      version="test"
      ws="connected"
      wsStats={null}
      onPage={() => undefined}
    />
  )
}

describe('Header', () => {
  it.each([
    [1, 'Site information (1 update)', '1'],
    [12, 'Site information (12 updates)', '12']
  ])('renders the %s unread badge inside the information link', (unreadCount, label, badgeText) => {
    renderHeader(unreadCount)

    const link = screen.getByRole('link', { name: label })
    const badge = link.querySelector('.header-badge')

    expect(badge?.textContent).toBe(badgeText)
  })
})
