// @vitest-environment jsdom

import { act, render, screen, waitFor } from '@testing-library/preact'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { StatsPage, nextBoundaryDelay, parseDurationMillis } from './StatsPage'

import type { ActivityStats, FirmwareHistory, FirmwareSnapshot } from '../api/types'

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
  rotate?: number
  size?: number
  space?: number
  splits?: MockAxisSplits
  stroke?: string
  ticks?: { stroke?: string }
  values?: MockAxisValues
}

interface MockSeries {
  fill?: string
  label?: string
  paths?: unknown
  stroke?: string
  width?: number
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
  scales?: {
    x?: { distr?: number; time?: boolean }
  }
  series?: MockSeries[]
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
    public static paths = {
      bars: vi.fn(() => (): unknown[] => [])
    }

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
  statsActivity: vi.fn(),
  firmwareSnapshot: vi.fn(),
  firmwareHistory: vi.fn()
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
      range_test: 0,
      traceroute: 0
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

function firmwareSnapshot(): FirmwareSnapshot {
  return {
    generated_at: '2026-05-04T12:00:00Z',
    total_nodes_with_version: 12,
    versions: [
      { version: '2.6.5', count: 7, last_seen_at: '2026-05-04T11:00:00Z' },
      { version: '2.7.10', count: 3, last_seen_at: '2026-05-04T11:30:00Z' },
      { version: '2.7.15', count: 2, last_seen_at: '2026-05-04T11:45:00Z' }
    ]
  }
}

function firmwareHistory(): FirmwareHistory {
  // 8 weeks, 3 top versions + "(other)". Pad zeros explicitly so the
  // chart has visible non-zero values at known indices.
  return {
    generated_at: '2026-05-04T12:00:00Z',
    weeks: 8,
    top: 3,
    versions: ['2.6.5', '2.7.10', '2.7.15', '(other)'],
    versions_by_week: [
      [10, 12, 15, 8, 5, 3, 2, 18],
      [12, 13, 8, 5, 1, 3, 18, 2],
      [12, 13, 8, 4, 4, 2, 12, 8],
      [0, 0, 0, 0, 0, 0, 0, 1]
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
    apiMock.firmwareSnapshot.mockResolvedValue(firmwareSnapshot())
    apiMock.firmwareHistory.mockResolvedValue(firmwareHistory())
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
    render(<StatsPage initialStats={stats()} initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={firmwareHistory()} />)

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
      'Others',
      'Text messages',
      'NodeInfo',
      'PKI',
      'Others',
      'Firmware versions',
      'Firmware adoption over time'
    ])
    expect(uplotMock.data[3]?.[1]).toEqual([3])
    expect(uplotMock.data[3]?.[2]).toEqual([0])
    expect(uplotMock.data[3]?.[3]).toEqual([0])
    expect(uplotMock.data[3]?.[4]).toEqual([0])
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
      'Range',
      'Traceroute'
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

describe('StatsPage Software section', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    uplotMock.data = []
    uplotMock.options = []
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-04T12:01:00Z'))
    apiMock.statsActivity.mockResolvedValue(stats())
    apiMock.firmwareSnapshot.mockResolvedValue(firmwareSnapshot())
    apiMock.firmwareHistory.mockResolvedValue(firmwareHistory())
  })

  it('fetches firmwareSnapshot and firmwareHistory on mount', async () => {
    render(<StatsPage />)

    await waitFor(() => {
      expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(1)
    })
    await waitFor(() => {
      expect(apiMock.firmwareHistory).toHaveBeenCalledTimes(1)
    })
  })

  it('renders the snapshot bar chart and history area chart with cursor sync key', async () => {
    render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })
    expect(screen.getByLabelText('Firmware version distribution')).toBeTruthy()
    expect(screen.getByLabelText('Firmware version history')).toBeTruthy()

    // The last two uPlot instances are the firmware charts (in order:
    // snapshot, then history).
    const chartsCount = uplotMock.options.length
    const snapshot = uplotMock.options[chartsCount - 2]
    const history = uplotMock.options[chartsCount - 1]

    // Snapshot: ordinal x axis (distr=4), bars path builder, sync key.
    expect(snapshot?.scales?.x?.distr).toBe(4)
    expect(snapshot?.scales?.x?.time).toBe(false)
    expect(snapshot?.cursor?.sync?.key).toBe('stats-firmware')
    expect(snapshot?.series?.[1]?.paths).toBeDefined()
    expect(snapshot?.axes?.[0]?.rotate).toBeGreaterThan(0)
    // X axis label callback renders the firmware version strings.
    const snapshotXValues = snapshot?.axes?.[0]?.values?.({}, [0, 1, 2], 0, 0, 0) ?? []
    expect(snapshotXValues.map(String)).toEqual(['2.6.5', '2.7.10', '2.7.15'])
    // Data shape: [x_indices..., count_series...]
    expect(snapshot && uplotMock.data[chartsCount - 2]?.[0]).toEqual([0, 1, 2])
    expect(uplotMock.data[chartsCount - 2]?.[1]).toEqual([7, 3, 2])

    // History: index-based x, multi-series, four columns + x.
    expect(history?.cursor?.sync?.key).toBe('stats-firmware')
    expect(history?.series?.slice(1).map((series) => series.label)).toEqual(['2.6.5', '2.7.10', '2.7.15', '(other)'])
    expect(history && uplotMock.data[chartsCount - 1]?.[0]).toEqual([0, 1, 2, 3, 4, 5, 6, 7])
    expect(uplotMock.data[chartsCount - 1]?.[1]).toEqual([10, 12, 15, 8, 5, 3, 2, 18])
    // Bottom series carries the area fill so the stacked effect is readable.
    expect(history?.series?.[1]?.fill).toBe('rgb(51 154 240 / 10%)')
    expect(history?.series?.[2]?.fill).toBeUndefined()
  })

  it('shows empty-state placeholder when snapshot has no versions', async () => {
    const emptySnapshot: FirmwareSnapshot = {
      generated_at: '2026-05-04T12:00:00Z',
      total_nodes_with_version: 0,
      versions: []
    }
    // The component re-loads on mount — match the mock to the empty
    // fixture so the polling effect can't overwrite it with the default.
    apiMock.firmwareSnapshot.mockResolvedValue(emptySnapshot)
    const before = uplotMock.options.length

    render(<StatsPage initialFirmwareSnapshot={emptySnapshot} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })
    expect(screen.getByText('No nodes have reported a firmware version yet.')).toBeTruthy()
    // The snapshot chart must NOT instantiate uPlot when there are no
    // versions — only the history chart adds a new plot.
    expect(uplotMock.options.length).toBe(before + 1)
  })

  it('shows empty-state placeholder when history has no versions', async () => {
    const emptyHistory: FirmwareHistory = {
      generated_at: '2026-05-04T12:00:00Z',
      weeks: 8,
      top: 3,
      versions: [],
      versions_by_week: []
    }
    apiMock.firmwareHistory.mockResolvedValue(emptyHistory)
    const before = uplotMock.options.length

    render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={emptyHistory} />)

    await screen.findByRole('heading', { name: 'Software' })
    expect(screen.getByText('No firmware history recorded yet.')).toBeTruthy()
    expect(uplotMock.options.length).toBe(before + 1)
  })

  it('refreshes the firmware snapshot every hour', async () => {
    render(<StatsPage />)

    await waitFor(() => {
      expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(1)
    })

    // Just before 1h — no refresh.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60 * 60 * 1000 - 1)
    })
    expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(1)

    // Past 1h — refresh fires.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000)
    })
    await waitFor(() => {
      expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(2)
    })
  })

  it('refreshes the firmware history every 24h', async () => {
    render(<StatsPage />)

    await waitFor(() => {
      expect(apiMock.firmwareHistory).toHaveBeenCalledTimes(1)
    })

    // Past 24h — refresh fires.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(24 * 60 * 60 * 1000 + 1_000)
    })
    await waitFor(() => {
      expect(apiMock.firmwareHistory).toHaveBeenCalledTimes(2)
    })
  })

  it('renders the "(other)" series last in the legend', async () => {
    render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })
    const legendItems = screen.getAllByText(/^2\.|\(other\)$/)
    const labels = legendItems.map((el) => el.textContent)

    expect(labels).toContain('(other)')
    // The legend is rendered in the order the server returned.
    expect(labels).toEqual(['2.6.5', '2.7.10', '2.7.15', '(other)'])
  })

  it('shows the hovered firmware snapshot count', async () => {
    render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })

    const chartsCount = uplotMock.options.length
    const snapshot = uplotMock.options[chartsCount - 2]

    await act(async () => {
      snapshot?.plugins?.[0]?.hooks?.setCursor?.({
        cursor: { idx: 1 },
        data: [[0, 1, 2], [7, 3, 2]]
      })
    })

    expect(screen.getByText(/2\.7\.10 · 3 nodes$/)).toBeTruthy()
  })
})