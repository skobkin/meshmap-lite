// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'

import { resolveLogDetailsRenderer } from './LogDetailsModal'

import type { LogEvent } from '../api/types'

function event(overrides: Partial<LogEvent> = {}): LogEvent {
  return {
    id: 7,
    observed_at: '2026-03-11T12:00:00Z',
    event_kind_value: 4,
    event_kind_title: 'Telemetry',
    encrypted: false,
    details: {
      metric: 'voltage'
    },
    ...overrides
  }
}

describe('LogDetailsModal renderer resolution', () => {
  it('returns the first matching custom renderer', () => {
    const telemetryRenderer = {
      id: 'telemetry',
      match: (item: LogEvent) => item.event_kind_value === 4,
      render: () => 'telemetry'
    }

    const resolved = resolveLogDetailsRenderer(event(), [
      {
        id: 'never',
        match: () => false,
        render: () => 'never'
      },
      telemetryRenderer
    ])

    expect(resolved).toBe(telemetryRenderer)
  })

  it('falls back to undefined when no renderer matches', () => {
    const resolved = resolveLogDetailsRenderer(event(), [
      {
        id: 'map-report',
        match: (item) => item.event_kind_value === 1,
        render: () => 'map'
      }
    ])

    expect(resolved).toBeUndefined()
  })
})
