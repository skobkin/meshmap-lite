import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'
import uPlot from 'uplot'

import { api } from '../api/client'

import type { ActivityBucket, ActivityPeriod, ActivityStats } from '../api/types'
import type { JSX } from 'preact'

type ActivityMetricKey = 'text_messages' | 'pki' | 'node_info' | 'telemetry' | 'neighbor_info' | 'range_test'

interface ActivityMetric {
  series: {
    key: ActivityMetricKey
    label: string
  }[]
  title: string
}

interface ChartTooltip {
  time: string
  value: string
}

const chartHeight = 220
const fallbackChartWidth = 360
const metrics: ActivityMetric[] = [
  { series: [{ key: 'text_messages', label: 'Text messages' }], title: 'Text messages' },
  { series: [{ key: 'node_info', label: 'NodeInfo' }], title: 'NodeInfo' },
  { series: [{ key: 'pki', label: 'PKI' }], title: 'PKI' },
  {
    series: [
      { key: 'telemetry', label: 'Telemetry' },
      { key: 'neighbor_info', label: 'Neighbor' },
      { key: 'range_test', label: 'Range' }
    ],
    title: 'Telemetry / Neighbor / Range'
  }
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

function chartData(buckets: ActivityBucket[], metric: ActivityMetric): uPlot.AlignedData {
  return [
    buckets.map((bucket) => Math.floor(Date.parse(bucket.bucket_start) / 1000)),
    ...metric.series.map((series) => buckets.map((bucket) => bucket[series.key]))
  ]
}

function hasAnyCount(buckets: ActivityBucket[], metric: ActivityMetric): boolean {
  return buckets.some((bucket) => metric.series.some((series) => bucket[series.key] > 0))
}

function chartWidth(root: HTMLDivElement): number {
  const width = Math.floor(root.clientWidth)

  return width > 0 ? width : fallbackChartWidth
}

function cssVar(root: Element, name: string, fallback: string): string {
  const value = getComputedStyle(root).getPropertyValue(name).trim()

  return value || fallback
}

function chartColors(root: Element): { axis: string; grid: string; lines: string[]; fill: string } {
  return {
    axis: cssVar(root, '--pico-muted-color', '#8b9bb4'),
    grid: cssVar(root, '--surface-border', '#2b3442'),
    lines: [
      cssVar(root, '--pico-primary', '#339af0'),
      '#f59f00',
      '#51cf66'
    ],
    fill: 'rgb(51 154 240 / 10%)'
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

function formatTooltipTime(sec: number): string {
  return new Date(sec * 1000).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function formatPacketCount(value: number): string {
  return `${value} ${value === 1 ? 'packet' : 'packets'}`
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
  const [tooltip, setTooltip] = useState<ChartTooltip | null>(null)
  const data = useMemo(() => chartData(buckets, metric), [buckets, metric])
  const anyCount = hasAnyCount(buckets, metric)
  const cursorPlugin = useMemo<uPlot.Plugin>(() => ({
    hooks: {
      setCursor: (plot) => {
        const idx = plot.cursor.idx
        if (idx === null || idx === undefined || idx < 0) {
          setTooltip(null)

          return
        }

        const timestamps = plot.data[0]
        const timestamp = timestamps[idx]
        if (typeof timestamp !== 'number' || !Number.isFinite(timestamp)) {
          setTooltip(null)

          return
        }

        const values = metric.series.map((series, index) => {
          const value = plot.data[index + 1]?.[idx]

          return typeof value === 'number' && Number.isFinite(value)
            ? `${series.label}: ${formatPacketCount(value)}`
            : null
        }).filter((value): value is string => value !== null)
        if (values.length === 0) {
          setTooltip(null)

          return
        }

        setTooltip({
          time: formatTooltipTime(timestamp),
          value: values.join(' · ')
        })
      }
    }
  }), [metric])

  useEffect(() => {
    const root = rootRef.current
    if (!root) {return undefined}

    const colors = chartColors(root)
    const plot = new uPlot({
      width: chartWidth(root),
      height: chartHeight,
      legend: { show: false },
      cursor: {
        show: true,
        drag: { x: false, y: false },
        points: { show: true, size: 5 },
        sync: { key: `stats-activity-${periodKey}` }
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
          label: 'Packets',
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
        ...metric.series.map((series, index) => ({
          label: series.label,
          stroke: colors.lines[index % colors.lines.length],
          width: 1,
          points: { show: false },
          fill: metric.series.length === 1 ? colors.fill : undefined
        }))
      ],
      plugins: [cursorPlugin]
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
  }, [cursorPlugin, data, metric.series, metric.title, periodKey])

  useEffect(() => {
    plotRef.current?.setData(data)
  }, [data])

  return (
    <article className="activity-chart" data-period={periodKey}>
      <header>
        <h3>{metric.title}</h3>
        <span className={tooltip ? 'activity-tooltip' : 'activity-tooltip muted'}>
          {tooltip ? `${tooltip.time} · ${tooltip.value}` : 'Hover for values'}
        </span>
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
          <ActivityChart key={metric.title} buckets={period.buckets} metric={metric} periodKey={period.key} />
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
