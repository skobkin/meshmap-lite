import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'
import uPlot from 'uplot'

import { api } from '../api/client'

import type { ActivityBucket, ActivityPeriod, ActivityStats } from '../api/types'
import type { JSX } from 'preact'

type ActivityMetricKey = 'text_messages' | 'pki' | 'node_info' | 'telemetry' | 'neighbor_info' | 'range_test'

interface ActivityMetric {
  key: ActivityMetricKey
  title: string
}

const chartHeight = 220
const fallbackChartWidth = 360
const metrics: ActivityMetric[] = [
  { key: 'text_messages', title: 'Text messages' },
  { key: 'pki', title: 'PKI' },
  { key: 'node_info', title: 'NodeInfo' },
  { key: 'telemetry', title: 'Telemetry' },
  { key: 'neighbor_info', title: 'Neighbor Info' },
  { key: 'range_test', title: 'Range test' }
]

interface Props {
  initialStats?: ActivityStats
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

export function parseDurationMillis(raw: string): number | null {
  const match = /^(\d+)(ms|s|m|h)$/.exec(raw.trim())
  if (!match) {return null}
  const value = Number(match[1])
  if (!Number.isFinite(value) || value <= 0) {return null}
  const unit = match[2] as 'ms' | 's' | 'm' | 'h'
  const multipliers = {
    ms: 1,
    s: 1000,
    m: 60 * 1000,
    h: 60 * 60 * 1000
  } satisfies Record<typeof unit, number>

  return value * multipliers[unit]
}

export function nextBoundaryDelay(nowMillis: number, bucketMillis: number): number {
  if (bucketMillis <= 0) {return 60_000}
  const next = Math.floor(nowMillis / bucketMillis) * bucketMillis + bucketMillis

  return Math.max(1000, next - nowMillis)
}

function nextRefreshDelay(stats: ActivityStats): number {
  const now = Date.now()
  const delays = stats.periods
    .map((period) => parseDurationMillis(period.bucket))
    .filter((value): value is number => value !== null)
    .map((bucketMillis) => nextBoundaryDelay(now, bucketMillis))

  return delays.length > 0 ? Math.min(...delays) : 60_000
}

function chartData(buckets: ActivityBucket[], metric: ActivityMetricKey): [number[], number[]] {
  return [
    buckets.map((bucket) => Math.floor(Date.parse(bucket.bucket_start) / 1000)),
    buckets.map((bucket) => bucket[metric])
  ]
}

function hasAnyCount(buckets: ActivityBucket[], metric: ActivityMetricKey): boolean {
  return buckets.some((bucket) => bucket[metric] > 0)
}

function chartWidth(root: HTMLDivElement): number {
  const width = Math.floor(root.clientWidth)

  return width > 0 ? width : fallbackChartWidth
}

function cssVar(root: Element, name: string, fallback: string): string {
  const value = getComputedStyle(root).getPropertyValue(name).trim()

  return value || fallback
}

function chartColors(root: Element): { axis: string; grid: string; line: string; fill: string } {
  return {
    axis: cssVar(root, '--pico-muted-color', '#8b9bb4'),
    grid: cssVar(root, '--surface-border', '#2b3442'),
    line: cssVar(root, '--pico-primary', '#339af0'),
    fill: 'rgb(51 154 240 / 14%)'
  }
}

function formatTick(sec: number, periodKey: string): string {
  const date = new Date(sec * 1000)

  if (periodKey === 'weekly') {
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric'
    })
  }

  return date.toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit'
  })
}

function integerSplits(max: number): number[] {
  const ceiling = Number.isFinite(max) ? Math.max(1, Math.ceil(max)) : 1
  if (ceiling <= 6) {
    return Array.from({ length: ceiling + 1 }, (_unused, index) => index)
  }

  const step = Math.ceil(ceiling / 5)
  const splits: number[] = []
  for (let value = 0; value < ceiling; value += step) {
    splits.push(value)
  }
  if (splits.at(-1) !== ceiling) {
    splits.push(ceiling)
  }

  return splits
}

function ActivityChart({ buckets, metric, periodKey }: { buckets: ActivityBucket[]; metric: ActivityMetric; periodKey: string }): JSX.Element {
  const rootRef = useRef<HTMLDivElement>(null)
  const plotRef = useRef<uPlot>()
  const data = useMemo(() => chartData(buckets, metric.key), [buckets, metric.key])
  const anyCount = hasAnyCount(buckets, metric.key)

  useEffect(() => {
    const root = rootRef.current
    if (!root) {return undefined}

    const colors = chartColors(root)
    const plot = new uPlot({
      width: chartWidth(root),
      height: chartHeight,
      legend: { show: false },
      cursor: {
        drag: { x: false, y: false },
        points: { show: false }
      },
      scales: {
        x: { time: true },
        y: { range: (_plot, _min, max) => [0, Math.max(1, Math.ceil(max))] }
      },
      axes: [
        {
          stroke: colors.axis,
          grid: { stroke: colors.grid, width: 1 },
          ticks: { stroke: colors.grid, width: 1 },
          border: { stroke: colors.grid, width: 1 },
          space: 92,
          size: 34,
          values: (_plot, ticks) => ticks.map((tick) => formatTick(tick, periodKey))
        },
        {
          stroke: colors.axis,
          label: 'Count',
          labelGap: 4,
          labelSize: 20,
          size: 42,
          grid: { stroke: colors.grid, width: 1 },
          ticks: { stroke: colors.grid, width: 1 },
          border: { stroke: colors.grid, width: 1 },
          splits: (_plot, _axisIndex, _scaleMin, scaleMax) => integerSplits(scaleMax),
          values: (_plot, ticks) => ticks.map((tick) => String(Math.round(tick)))
        }
      ],
      series: [
        {},
        {
          label: metric.title,
          stroke: colors.line,
          width: 2,
          points: { show: false },
          fill: colors.fill
        }
      ]
    }, data, root)
    plotRef.current = plot

    const resize = (): void => {
      plot.setSize({ width: chartWidth(root), height: chartHeight })
    }
    const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(resize) : undefined
    observer?.observe(root)

    return () => {
      observer?.disconnect()
      plot.destroy()
      plotRef.current = undefined
    }
  }, [data, metric.title, periodKey])

  useEffect(() => {
    plotRef.current?.setData(data)
  }, [data])

  return (
    <article className="activity-chart" data-period={periodKey}>
      <header>
        <h3>{metric.title}</h3>
      </header>
      <div className="activity-chart-canvas" ref={rootRef} aria-label={`${metric.title} packet counts`} />
      {!anyCount && <p className="activity-empty">No packets in this period.</p>}
    </article>
  )
}

function ActivitySection({ period }: { period: ActivityPeriod }): JSX.Element {
  return (
    <section className="stats-section" aria-labelledby={`stats-${period.key}`}>
      <div className="stats-section-heading">
        <h2 id={`stats-${period.key}`}>{period.title}</h2>
        <p>{period.bucket} buckets</p>
      </div>
      <div className="activity-grid">
        {metrics.map((metric) => (
          <ActivityChart key={metric.key} buckets={period.buckets} metric={metric} periodKey={period.key} />
        ))}
      </div>
    </section>
  )
}

export function StatsPage({ initialStats }: Props): JSX.Element {
  const [stats, setStats] = useState<ActivityStats | undefined>(initialStats)
  const [loadError, setLoadError] = useState('')

  const loadStats = useCallback((signal?: AbortSignal): Promise<void> => (
    api.statsActivity({ signal })
      .then((next) => {
        setStats(next)
        setLoadError('')
      })
  ), [])

  useEffect(() => {
    const controller = new AbortController()
    void loadStats(controller.signal)
      .catch((err) => {
        if (isAbortError(err)) {return}
        setLoadError('Failed to load activity stats.')
      })

    return () => controller.abort()
  }, [loadStats])

  useEffect(() => {
    if (!stats) {return undefined}
    const timeout = window.setTimeout(() => {
      void loadStats()
        .catch((err) => {
          if (isAbortError(err)) {return}
          setLoadError('Failed to refresh activity stats.')
        })
    }, nextRefreshDelay(stats))

    return () => window.clearTimeout(timeout)
  }, [loadStats, stats])

  return (
    <section className="stats-layout container-fluid">
      {loadError && <p className="load-error">{loadError}</p>}
      {stats ? (
        stats.periods.map((period) => <ActivitySection key={period.key} period={period} />)
      ) : (
        <p className="node-list-empty">Loading activity stats.</p>
      )}
    </section>
  )
}
