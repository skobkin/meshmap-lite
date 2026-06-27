// @vitest-environment jsdom

import { act, render, screen, waitFor } from '@testing-library/preact'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { StatsPage, nextBoundaryDelay, parseDurationMillis, shortVersionLabel } from './StatsPage'

import type { ActivityStats, FirmwareHistory, FirmwareSnapshot } from '../api/types'

type MockAxisValues = (plot: unknown, ticks: number[], axisIndex: number, foundSpace: number, foundIncr: number) => (number | string | null)[]
type MockAxisSplits = (plot: unknown, axisIndex: number, scaleMin: number, scaleMax: number, foundIncr: number, foundSpace: number) => number[]
type MockAxisFilter = (plot: unknown, ticks: number[], axisIndex: number, foundSpace: number, foundIncr: number) => (number | null)[]
type MockSetCursorHook = (plot: MockPlot) => void

interface MockPlot {
  cursor: { idx?: number | null }
  data: number[][]
}

interface MockAxis {
  border?: { stroke?: string }
  filter?: MockAxisFilter
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
    x?: {
      distr?: number
      range?: (plot: unknown, min: number, max: number) => [number, number]
      time?: boolean
    }
    // y is only declared in the snapshot chart; the area chart uses the
    // uPlot auto-fit default. We only need the bits the tests assert on.
    y?: {
      range?: (plot: unknown, min: number, max: number | undefined) => [number, number]
    }
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
    cache_ttl_seconds: 3600,
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
  // chart has visible non-zero values at known indices. week_starts
  // mirrors the inner versions_by_week axis in oldest-first order —
  // Monday-anchored RFC3339Nano strings so the chart's tooltip label
  // source matches what a real server would emit. The last entry
  // (2026-05-25) is three weeks ahead of the test's frozen clock
  // (2026-05-04), so the fixture intentionally exercises the case
  // where the cached response is from a later week than the
  // browser's wall-clock — the chart must use week_starts, not
  // Date.now(), to render labels that match the data.
  const weekStarts = [
    '2026-04-06T00:00:00Z',
    '2026-04-13T00:00:00Z',
    '2026-04-20T00:00:00Z',
    '2026-04-27T00:00:00Z',
    '2026-05-04T00:00:00Z',
    '2026-05-11T00:00:00Z',
    '2026-05-18T00:00:00Z',
    '2026-05-25T00:00:00Z'
  ]

  return {
    generated_at: '2026-05-04T12:00:00Z',
    cache_ttl_seconds: 86400,
    weeks: 8,
    top: 3,
    versions: ['2.6.5', '2.7.10', '2.7.15', '(other)'],
    versions_by_week: [
      [10, 12, 15, 8, 5, 3, 2, 18],
      [12, 13, 8, 5, 1, 3, 18, 2],
      [12, 13, 8, 4, 4, 2, 12, 8],
      [0, 0, 0, 0, 0, 0, 0, 1]
    ],
    week_starts: weekStarts
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

describe('shortVersionLabel', () => {
  it('returns short versions unchanged', () => {
    expect(shortVersionLabel('2.6.5')).toBe('2.6.5')
    expect(shortVersionLabel('2.7.10')).toBe('2.7.10')
    expect(shortVersionLabel('2.7.15')).toBe('2.7.15')
  })

  it('strips the trailing commit hash from modern Meshtastic versions', () => {
    expect(shortVersionLabel('2.7.23.b246bcd')).toBe('2.7.23')
    expect(shortVersionLabel('2.7.10.abcdef0')).toBe('2.7.10')
    expect(shortVersionLabel('2.6.5.deadbeef')).toBe('2.6.5')
  })

  it('truncates and ellipsizes legacy or unusually long versions', () => {
    // Hash-stripped prefixes that are still too long get ellipsized.
    expect(shortVersionLabel('1.2.3.4.5.6.7.8.9.0')).toBe('1.2.3.4.5.6…')
    // Trailing separators get trimmed so the ellipsis attaches to the
    // last digit, not dangle after a dot.
    expect(shortVersionLabel('1.2.3.4.5.6.7')).toBe('1.2.3.4.5.6…')
    // "7.16.a35972230" hash-strips to "7.16" — short enough to keep as-is.
    expect(shortVersionLabel('7.16.a35972230')).toBe('7.16')
  })

  it('returns an empty string for missing input', () => {
    expect(shortVersionLabel(undefined)).toBe('')
    expect(shortVersionLabel('')).toBe('')
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

  it('renders the snapshot bar chart and history area chart without syncing cursors', async () => {
    render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })
    expect(screen.getByLabelText('Firmware version distribution')).toBeTruthy()
    expect(screen.getByLabelText('Firmware version history')).toBeTruthy()

    // The last two uPlot instances are the firmware charts (in order:
    // snapshot, then history).
    const chartsCount = uplotMock.options.length
    const snapshot = uplotMock.options[chartsCount - 2]
    const history = uplotMock.options[chartsCount - 1]

    // Snapshot: ordinal x axis (distr=2), bars path builder, no sync key.
    // uPlot uses distr=4 for arcsinh; that warps the version index spacing
    // and makes bars render as wide, uneven blocks instead of columns.
    expect(snapshot?.scales?.x?.distr).toBe(2)
    expect(snapshot?.scales?.x?.time).toBe(false)
    expect(snapshot?.cursor?.sync).toBeUndefined()
    expect(snapshot?.series?.[1]?.paths).toBeDefined()
    // No rotation: shortVersionLabel caps labels at ~12 chars, so they
    // fit horizontally and rotation was causing the rightmost labels
    // to drop / render off-canvas in production.
    expect(snapshot?.axes?.[0]?.rotate ?? 0).toBe(0)
    // Compact x axis row (horizontal labels only need ~28 px).
    expect(snapshot?.axes?.[0]?.size).toBeLessThanOrEqual(32)
    // X axis label callback renders the firmware version strings.
    const snapshotXValues = snapshot?.axes?.[0]?.values?.({}, [0, 1, 2], 0, 0, 0) ?? []
    expect(snapshotXValues.map(String)).toEqual(['2.6.5', '2.7.10', '2.7.15'])
    // Data shape: [x_indices..., count_series...]
    expect(snapshot && uplotMock.data[chartsCount - 2]?.[0]).toEqual([0, 1, 2])
    expect(uplotMock.data[chartsCount - 2]?.[1]).toEqual([7, 3, 2])

    // History: index-based x, multi-series, four columns + x.
    expect(history?.cursor?.sync).toBeUndefined()
    expect(history?.series?.slice(1).map((series) => series.label)).toEqual(['2.6.5', '2.7.10', '2.7.15', '(other)'])
    expect(history && uplotMock.data[chartsCount - 1]?.[0]).toEqual([0, 1, 2, 3, 4, 5, 6, 7])
    expect(uplotMock.data[chartsCount - 1]?.[1]).toEqual([10, 12, 15, 8, 5, 3, 2, 18])
    // Bottom series carries the area fill so the stacked effect is readable.
    expect(history?.series?.[1]?.fill).toBe('rgb(51 154 240 / 10%)')
    expect(history?.series?.[2]?.fill).toBeUndefined()
  })

  it('renders every version label and a sane y range for production-shape data (4 versions with hash suffixes)', async () => {
    // Mirrors the live fleet shape the screenshot regression came from:
    // 4 versions, two of them carrying modern Meshtastic commit hashes.
    const productionSnapshot: FirmwareSnapshot = {
      generated_at: '2026-05-04T12:00:00Z',
      cache_ttl_seconds: 3600,
      total_nodes_with_version: 6,
      versions: [
        { version: '2.7.16.abcdef0', count: 2, last_seen_at: '2026-05-04T11:00:00Z' },
        { version: '2.7.23.b246bcd', count: 2, last_seen_at: '2026-05-04T11:30:00Z' },
        { version: '2.7.10.deadbee', count: 1, last_seen_at: '2026-05-04T11:45:00Z' },
        { version: '2.7.05.1234567', count: 1, last_seen_at: '2026-05-04T11:50:00Z' }
      ]
    }

    render(<StatsPage initialFirmwareSnapshot={productionSnapshot} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })

    const chartsCount = uplotMock.options.length
    const snapshot = uplotMock.options[chartsCount - 2]
    const snapshotData = uplotMock.data[chartsCount - 2]
    const snapshotXLabels = snapshot?.axes?.[0]?.values?.({}, [0, 1, 2, 3], 0, 0, 0) ?? []

    // All 4 versions must produce a label (the screenshot regression was
    // that bars 3 and 4 had no visible axis label under them).
    expect(snapshotXLabels.map(String)).toEqual(['2.7.16', '2.7.23', '2.7.10', '2.7.05'])
    // Data indices reach the last bar.
    expect(snapshotData?.[0]).toEqual([0, 1, 2, 3])
    expect(snapshotData?.[1]).toEqual([2, 2, 1, 1])
    // y range: max data is 2, so the upper bound should be exactly 2 —
    // not 3+ (which would mean the chart is silently squashing the bars
    // because Math.ceil(undefined) leaked through on first render).
    const yRange = snapshot?.scales?.y?.range?.({}, 0, 2) ?? []
    expect(yRange).toEqual([0, 2])
    // x range pads the first and last ordinal slots so centered bars
    // are fully visible instead of half-clipped by the plot border.
    const xRange = snapshot?.scales?.x?.range?.({}, 0, 3) ?? []
    expect(xRange).toEqual([-0.5, 3.5])
    const xSplits = snapshot?.axes?.[0]?.splits?.({}, 0, -0.5, 3.5, 1, 60) ?? []
    expect(xSplits).toEqual([0, 1, 2, 3])
    expect(snapshot?.axes?.[0]?.filter?.({}, xSplits, 0, 60, 1)).toEqual([0, 1, 2, 3])
    // And on the first-render call where max is undefined, the chart
    // must NOT return NaN (which uPlot would interpret as no upper
    // bound and silently pad the data).
    const firstRenderRange = snapshot?.scales?.y?.range?.({}, 0, undefined) ?? []
    expect(Number.isFinite(firstRenderRange[1])).toBe(true)
    expect(firstRenderRange[1]).toBeGreaterThanOrEqual(1)
  })

  it('shows empty-state placeholder when snapshot has no versions', async () => {
    const emptySnapshot: FirmwareSnapshot = {
      generated_at: '2026-05-04T12:00:00Z',
      cache_ttl_seconds: 3600,
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
      cache_ttl_seconds: 86400,
      weeks: 8,
      top: 3,
      versions: [],
      versions_by_week: [],
      week_starts: []
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

  it('honors a custom snapshot cache_ttl_seconds instead of the hardcoded 1h', async () => {
    // The fixture advertises a 5-minute cache TTL. The polling
    // cadence must follow the server's resolved value, not the
    // historical 1h hardcoded interval — that's the CodeX review
    // point: an operator shortening snapshot_cache_ttl expects the
    // UI to pick up the change.
    const shortTtlSnapshot: FirmwareSnapshot = {
      ...firmwareSnapshot(),
      cache_ttl_seconds: 300
    }
    apiMock.firmwareSnapshot.mockResolvedValue(shortTtlSnapshot)

    render(<StatsPage />)

    await waitFor(() => {
      expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(1)
    })

    // 4 minutes: still inside the configured 5-minute window, no
    // refresh yet.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4 * 60 * 1000)
    })
    expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(1)

    // Past 5 minutes: refresh fires.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000)
    })
    await waitFor(() => {
      expect(apiMock.firmwareSnapshot).toHaveBeenCalledTimes(2)
    })
  })

  it('anchors the history x-axis tick labels to week_starts, not the browser clock', async () => {
    // The frozen browser clock (set in beforeEach) is 2026-05-04, but
    // the fixture's last week_start is 2026-05-25. The old
    // `Date.now() - offset*7d` math would label tick 7 (the newest
    // column) as approximately "Apr 13" — three weeks before the
    // browser clock. Anchoring to week_starts must label it as a
    // date near the end of May instead, matching the cursor
    // tooltip. The exact day depends on the jsdom timezone so we
    // assert on the month, not the day.
    render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={firmwareHistory()} />)

    await screen.findByRole('heading', { name: 'Software' })

    const chartsCount = uplotMock.options.length
    const history = uplotMock.options[chartsCount - 1]
    const tickLabels = (history?.axes?.[0]?.values?.({}, [0, 7], 0, 0, 0) ?? []).map(String)

    // Newest tick must be late May (the last week_start), not April
    // — which is what the old `Date.now() - offset*7d` math would
    // produce given the frozen 2026-05-04 system time and an 8-week
    // offset.
    expect(tickLabels[1]).toContain('May')
    expect(tickLabels[1]).not.toContain('Apr')
    // And the oldest tick must still be April (the first week_start).
    expect(tickLabels[0]).toContain('Apr')
  })

  it('formats UTC history week labels without shifting them into the local timezone', async () => {
    vi.stubEnv('TZ', 'America/New_York')

    try {
      const history = firmwareHistory()
      history.week_starts = history.week_starts.map((weekStart, index) => (
        index === 7 ? '2026-06-01T00:00:00Z' : weekStart
      ))

      render(<StatsPage initialFirmwareSnapshot={firmwareSnapshot()} initialFirmwareHistory={history} />)

      await screen.findByRole('heading', { name: 'Software' })

      const historyIndex = uplotMock.options.length - 1
      const chart = uplotMock.options[historyIndex]
      const tickLabels = (chart?.axes?.[0]?.values?.({}, [7], 0, 0, 0) ?? []).map(String)

      expect(tickLabels[0]).toContain('Jun')
      expect(tickLabels[0]).not.toContain('May')

      await act(async () => {
        chart?.plugins?.[0]?.hooks?.setCursor?.({
          cursor: { idx: 7 },
          data: uplotMock.data[historyIndex] ?? []
        })
      })

      expect(screen.getByText((text) => text.startsWith('Week of Jun 1, 2026 ·'))).toBeTruthy()
      expect(screen.queryByText((text) => text.startsWith('Week of May 31, 2026 ·'))).toBeNull()
    } finally {
      vi.unstubAllEnvs()
    }
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
