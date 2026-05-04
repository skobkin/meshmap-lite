// @vitest-environment jsdom

import { act, render, screen, waitFor } from '@testing-library/preact'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { StatsPage, nextBoundaryDelay, parseDurationMillis } from './StatsPage'

import type { ActivityStats } from '../api/types'

const uplotMock = vi.hoisted(() => ({
  created: vi.fn(),
  setData: vi.fn(),
  setSize: vi.fn(),
  destroy: vi.fn()
}))

vi.mock('uplot', () => ({
  default: class UPlotMock {
    public constructor() {
      uplotMock.created()
    }

    public setData(...args: unknown[]): void {
      uplotMock.setData(...args)
    }

    public setSize(...args: unknown[]): void {
      uplotMock.setSize(...args)
    }

    public destroy(): void {
      uplotMock.destroy()
    }
  }
}))

const apiMock = vi.hoisted(() => ({
  statsActivity: vi.fn()
}))

vi.mock('../api/client', () => ({
  api: apiMock
}))

function stats(): ActivityStats {
  const dailyBuckets = [
    {
      bucket_start: '2026-05-04T11:50:00Z',
      text_messages: 2,
      pki: 1,
      node_info: 0,
      telemetry: 3,
      neighbor_info: 0,
      range_test: 0
    }
  ]

  return {
    generated_at: '2026-05-04T12:00:00Z',
    periods: [
      {
        key: 'daily',
        title: '24 hours',
        window: '24h',
        bucket: '5m',
        buckets: dailyBuckets
      },
      {
        key: 'weekly',
        title: '7 days',
        window: '168h',
        bucket: '1h',
        buckets: []
      }
    ]
  }
}

describe('StatsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-04T12:01:00Z'))
    apiMock.statsActivity.mockResolvedValue(stats())
  })

  it('parses durations and calculates next bucket boundaries', () => {
    expect(parseDurationMillis('5m')).toBe(300_000)
    expect(parseDurationMillis('1h')).toBe(3_600_000)
    expect(parseDurationMillis('bad')).toBeNull()
    expect(nextBoundaryDelay(Date.parse('2026-05-04T12:01:00Z'), 300_000)).toBe(240_000)
  })

  it('renders both activity sections and six charts per section', async () => {
    render(<StatsPage />)

    await screen.findByRole('heading', { name: '24 hours' })
    expect(screen.getByRole('heading', { name: '7 days' })).toBeTruthy()
    expect(screen.getAllByLabelText(/packet counts$/)).toHaveLength(12)
  })

  it('refreshes on the next returned bucket boundary', async () => {
    render(<StatsPage />)

    await waitFor(() => {
      expect(apiMock.statsActivity).toHaveBeenCalledTimes(1)
    })

    await act(async () => {
      await vi.advanceTimersByTimeAsync(240_000)
    })

    await waitFor(() => {
      expect(apiMock.statsActivity).toHaveBeenCalledTimes(2)
    })
  })
})
