// @vitest-environment jsdom

import { act, render, screen, waitFor } from '@testing-library/preact'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { StatsPage, nextBoundaryDelay, parseDurationMillis } from './StatsPage'

import type { ActivityStats } from '../api/types'

type MockAxisValues = (plot: unknown, ticks: number[], axisIndex: number, foundSpace: number, foundIncr: number) => (number | string | null)[]
type MockAxisSplits = (plot: unknown, axisIndex: number, scaleMin: number, scaleMax: number, foundIncr: number, foundSpace: number) => number[]
type MockSetCursorHook = (plot: MockPlot) => void

interface MockPlot {
  cursor: { idx?: number | null }
  data: number[][]
}

interface MockAxis {
  border?: { stroke?: string }
  grid?: { stroke?: string }
  size?: number
  space?: number
  splits?: MockAxisSplits
  stroke?: string
  ticks?: { stroke?: string }
  values?: MockAxisValues
}

interface MockOptions {
  axes?: MockAxis[]
  cursor?: {
    points?: { show?: boolean; size?: number }
    show?: boolean
    sync?: { key?: string }
  }
  plugins?: {
    hooks?: {
      setCursor?: MockSetCursorHook
    }
  }[]
  series?: {
    fill?: string
    label?: string
    stroke?: string
    width?: number
  }[]
}

const uplotMock = vi.hoisted(() => ({
  created: vi.fn(),
  data: [] as number[][][],
  options: [] as MockOptions[],
  setData: vi.fn(),
  setSize: vi.fn(),
  destroy: vi.fn()
}))

vi.mock('uplot', () => ({
  default: class UPlotMock {
    public constructor(options: MockOptions, data: number[][]) {
      uplotMock.options.push(options)
      uplotMock.data.push(data)
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
    uplotMock.data = []
    uplotMock.options = []
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-04T12:01:00Z'))
    apiMock.statsActivity.mockResolvedValue(stats())
  })

  it('parses durations and calculates next bucket boundaries', () => {
    expect(parseDurationMillis('5m')).toBe(300_000)
    expect(parseDurationMillis('1h')).toBe(3_600_000)
    expect(parseDurationMillis('30s')).toBe(30_000)
    expect(parseDurationMillis('1m30s')).toBe(90_000)
    expect(parseDurationMillis('1h30m')).toBe(5_400_000)
    expect(parseDurationMillis('bad')).toBeNull()
    expect(parseDurationMillis('')).toBeNull()
    expect(nextBoundaryDelay(Date.parse('2026-05-04T12:01:00Z'), 300_000)).toBe(240_000)
  })

  it('renders both activity sections and four charts per section in display order', async () => {
    render(<StatsPage initialStats={stats()} />)

    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByRole('heading', { name: '24 hours' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: '7 days' })).toBeTruthy()
    expect(screen.getAllByLabelText(/packet counts$/)).toHaveLength(8)
    expect(screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent)).toEqual([
      'Text messages',
      'NodeInfo',
      'PKI',
      'Telemetry / Neighbor / Range',
      'Text messages',
      'NodeInfo',
      'PKI',
      'Telemetry / Neighbor / Range'
    ])
    expect(uplotMock.data[3]?.[1]).toEqual([3])
    expect(uplotMock.data[3]?.[2]).toEqual([0])
    expect(uplotMock.data[3]?.[3]).toEqual([0])
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

  it('configures readable chart axes for dark surfaces and dense time buckets', async () => {
    render(<StatsPage initialStats={stats()} />)

    await screen.findByRole('heading', { name: '24 hours' })

    const dailyXAxis = uplotMock.options[0]?.axes?.[0]
    const dailyYAxis = uplotMock.options[0]?.axes?.[1]
    const weeklyXAxis = uplotMock.options[4]?.axes?.[0]
    const dailyValues = dailyXAxis?.values?.({}, [Date.parse('2026-05-04T12:00:00Z') / 1000], 0, 0, 0) ?? []
    const weeklyValues = weeklyXAxis?.values?.({}, [Date.parse('2026-05-04T12:00:00Z') / 1000], 0, 0, 0) ?? []
    const dailyLabel = String(dailyValues[0])
    const weeklyLabel = String(weeklyValues[0])

    expect(dailyXAxis?.stroke).toBe('#8b9bb4')
    expect(dailyXAxis?.grid?.stroke).toBe('#2b3442')
    expect(dailyXAxis?.space).toBeGreaterThanOrEqual(90)
    expect(uplotMock.options[0]?.cursor?.show).toBe(true)
    expect(uplotMock.options[0]?.cursor?.points?.show).toBe(true)
    expect(uplotMock.options[0]?.cursor?.sync?.key).toBe('stats-activity-daily')
    expect(uplotMock.options[0]?.series?.[1]?.width).toBe(1)
    expect(uplotMock.options[0]?.series?.[1]?.fill).toBe('rgb(51 154 240 / 10%)')
    expect(uplotMock.options[3]?.series?.slice(1).map((series) => series.label)).toEqual([
      'Telemetry',
      'Neighbor',
      'Range'
    ])
    expect(uplotMock.options[3]?.series?.[1]?.fill).toBeUndefined()
    expect(dailyLabel).not.toContain('May')
    expect(weeklyLabel).not.toContain(':')
    expect(dailyYAxis?.splits?.({}, 1, 0, 1.4, 0, 0)).toEqual([0, 1, 2])
    expect(dailyYAxis?.splits?.({}, 1, 0, 8.2, 0, 0)).toEqual([0, 2, 4, 6, 8, 9])
  })

  it('shows the hovered chart point time and packet count', async () => {
    render(<StatsPage initialStats={stats()} />)

    await screen.findByRole('heading', { name: '24 hours' })
    expect(screen.getAllByText('Hover for values')).toHaveLength(8)

    await act(async () => {
      uplotMock.options[0]?.plugins?.[0]?.hooks?.setCursor?.({
        cursor: { idx: 0 },
        data: [[Date.parse('2026-05-04T11:50:00Z') / 1000], [2]]
      })
    })

    expect(screen.getByText(/2 packets$/)).toBeTruthy()
  })
})
