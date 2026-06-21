import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'
import uPlot from 'uplot'

import { api } from '../api/client'

import type { ActivityBucket, ActivityPeriod, ActivityStats, FirmwareHistory, FirmwareSnapshot, FirmwareVersionCount } from '../api/types'
import type { JSX } from 'preact'

type ActivityMetricKey = 'text_messages' | 'pki' | 'node_info' | 'telemetry' | 'neighbor_info' | 'range_test' | 'traceroute'

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
// Default polling cadences for the firmware endpoints, used when the
// first response hasn't arrived yet. Once the server replies, its
// `cache_ttl_seconds` field drives the cadence — that lets an
// operator shorten the server-side TTL and have the UI honor it
// without a code change here. Defaults mirror the server's
// internal/config/defaults.go values for the period before the
// first poll completes.
const firmwareSnapshotDefaultTTL = 60 * 60 * 1000 // 1h
const firmwareHistoryDefaultTTL = 24 * 60 * 60 * 1000 // 24h

function pollDelaySeconds(ttlSeconds: number): number {
  // Clamp to a sane floor so a misconfigured TTL of 0 doesn't
  // busy-poll the API. We poll at the cache's exact expiry rather
  // than slightly before: uPlot's data swap is cheap, and polling
  // at TTL keeps the cadence in lockstep with whatever the
  // operator configured without smuggling in a hidden factor.
  if (!Number.isFinite(ttlSeconds) || ttlSeconds <= 0) {
    return 60
  }

  return Math.max(60, Math.floor(ttlSeconds))
}

const metrics: ActivityMetric[] = [
  { series: [{ key: 'text_messages', label: 'Text messages' }], title: 'Text messages' },
  { series: [{ key: 'node_info', label: 'NodeInfo' }], title: 'NodeInfo' },
  { series: [{ key: 'pki', label: 'PKI' }], title: 'PKI' },
  {
    series: [
      { key: 'telemetry', label: 'Telemetry' },
      { key: 'neighbor_info', label: 'Neighbor' },
      { key: 'range_test', label: 'Range' },
      { key: 'traceroute', label: 'Traceroute' }
    ],
    title: 'Others'
  }
]

interface Props {
  initialStats?: ActivityStats
  initialFirmwareSnapshot?: FirmwareSnapshot
  initialFirmwareHistory?: FirmwareHistory
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

export function parseDurationMillis(raw: string): number | null {
  const token = /([0-9]+(?:\.[0-9]+)?)(ms|s|m|h)/g
  let total = 0
  let found = false
  for (const match of raw.trim().matchAll(token)) {
    found = true
    const n = Number(match[1])
    if (!Number.isFinite(n) || n <= 0) {return null}
    const unit = match[2]
    if (unit === 'h') {total += n * 3600000}
    else if (unit === 'm') {total += n * 60000}
    else if (unit === 's') {total += n * 1000}
    else if (unit === 'ms') {total += n}
  }
  if (!found) {return null}

  return total
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
    lines: lineColors(root),
    fill: 'rgb(51 154 240 / 10%)'
  }
}

function lineColors(root?: Element): string[] {
  const el = root ?? document.documentElement

  return [
    cssVar(el, '--pico-primary', '#339af0'),
    '#f59f00',
    '#51cf66',
    '#e64980'
  ]
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

function formatNodeCount(value: number): string {
  return `${value} ${value === 1 ? 'node' : 'nodes'}`
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
      {metric.series.length > 1 && (
        <div className="chart-legend">
          {metric.series.map((series, index) => (
            <span key={series.key} className="chart-legend-item">
              <span className="chart-legend-swatch" style={{ background: lineColors()[index % lineColors().length] }} />
              {series.label}
            </span>
          ))}
        </div>
      )}
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

// FirmwareSnapshotChart renders the current fleet distribution as a bar
// chart. The x axis is ordinal: each bar is one firmware version, ordered
// the same way the server returned them (count DESC, then version string
// ASC).
function FirmwareSnapshotChart({ versions }: { versions: FirmwareVersionCount[] }): JSX.Element {
  const rootRef = useRef<HTMLDivElement>(null)
  const plotRef = useRef<uPlot>()
  const [tooltip, setTooltip] = useState<ChartTooltip | null>(null)
  const data = useMemo<uPlot.AlignedData>(() => {
    if (versions.length === 0) {return [[], []]}

    return [versions.map((_, index) => index), versions.map((entry) => entry.count)]
  }, [versions])
  const total = useMemo(() => versions.reduce((sum, entry) => sum + entry.count, 0), [versions])

  const cursorPlugin = useMemo<uPlot.Plugin>(() => ({
    hooks: {
      setCursor: (plot) => {
        const idx = plot.cursor.idx
        if (idx === null || idx === undefined || idx < 0 || idx >= versions.length) {
          setTooltip(null)

          return
        }

        const counts = plot.data[1]
        const value = counts?.[idx]
        const entry = versions[idx]
        if (typeof value !== 'number' || !entry) {
          setTooltip(null)

          return
        }

        setTooltip({
          time: entry.version,
          value: formatNodeCount(value)
        })
      }
    }
  }), [versions])

  useEffect(() => {
    const root = rootRef.current
    if (!root || versions.length === 0) {return undefined}

    const colors = chartColors(root)
    // Rotate bar colors so adjacent versions are visually distinct when
    // there are several of them. (Bars default to the primary line color
    // otherwise.)
    const barColor = colors.lines[0]
    const plot = new uPlot({
      width: chartWidth(root),
      height: chartHeight,
      legend: { show: false },
      cursor: {
        show: true,
        drag: { x: false, y: false },
        points: { show: true, size: 5 },
        sync: { key: 'stats-firmware' }
      },
      scales: {
        // distr: 4 is the ordinal x scale in uPlot — bars sit on integer
        // x indices instead of a continuous time axis.
        x: { distr: 4, time: false },
        y: { range: (_plot, _min, max) => [0, Math.max(1, Math.ceil(max))] }
      },
      axes: [
        {
          stroke: colors.axis,
          grid: { stroke: colors.grid, width: 1 },
          ticks: { stroke: colors.grid, width: 1 },
          border: { stroke: colors.grid, width: 1 },
          size: 64,
          // Tilted labels keep long firmware version strings readable
          // when there are many of them.
          values: (_plot, ticks) => ticks.map((tick) => versions[tick]?.version ?? ''),
          rotate: 35
        },
        {
          stroke: colors.axis,
          label: 'Nodes',
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
          label: 'Nodes',
          stroke: barColor,
          fill: colors.fill,
          width: 1,
          points: { show: false },
          // bars() is in uPlot's static API but its TypeScript types mark
          // it optional — optional-chain so the call typechecks, and the
          // resulting `undefined` is a legal `paths` value (uPlot falls
          // back to its linear default).
          paths: uPlot.paths.bars?.({
            size: [0.6, Infinity, 1],
            gap: 8,
            align: -1
          })
        }
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
  }, [cursorPlugin, data, versions])

  useEffect(() => {
    plotRef.current?.setData(data)
  }, [data])

  return (
    <article className="firmware-chart" data-chart="firmware-snapshot">
      <header>
        <h3>Firmware versions</h3>
        <span className={tooltip ? 'activity-tooltip' : 'activity-tooltip muted'}>
          {tooltip ? `${tooltip.time} · ${tooltip.value}` : versions.length === 0 ? 'No firmware versions reported' : 'Hover for details'}
        </span>
      </header>
      <div className="firmware-chart-canvas" ref={rootRef} aria-label="Firmware version distribution" />
      {versions.length === 0
        ? <p className="activity-empty">No nodes have reported a firmware version yet.</p>
        : <p className="firmware-summary">{formatNodeCount(total)} reporting a firmware version.</p>}
    </article>
  )
}

// FirmwareHistoryChart renders the weekly time series as a stacked area
// chart. One series per top-N version (the last slot is "(other)" when
// the server reports more than top-N versions).
function FirmwareHistoryChart({ history }: { history: FirmwareHistory }): JSX.Element {
  const rootRef = useRef<HTMLDivElement>(null)
  const plotRef = useRef<uPlot>()
  const [tooltip, setTooltip] = useState<ChartTooltip | null>(null)

  const data = useMemo<uPlot.AlignedData>(() => {
    const xs = Array.from({ length: history.weeks }, (_unused, i) => i)
    const series = history.versions_by_week
    const padded = series.map((row) => {
      // The server pads missing weeks with zero rows already, but guard
      // against a row shorter than `weeks` (older API responses) so the
      // chart never crashes on a malformed payload.
      if (row.length >= history.weeks) {return row.slice(0, history.weeks)}

      return [...row, ...Array.from({ length: history.weeks - row.length }, () => 0)]
    })

    return [xs, ...padded]
  }, [history])

  const cursorPlugin = useMemo<uPlot.Plugin>(() => ({
    hooks: {
      setCursor: (plot) => {
        const idx = plot.cursor.idx
        if (idx === null || idx === undefined || idx < 0 || idx >= history.weeks) {
          setTooltip(null)

          return
        }
        const xs = plot.data[0]
        const x = xs[idx]
        if (typeof x !== 'number') {
          setTooltip(null)

          return
        }
        const lines = history.versions.map((version, seriesIdx) => {
          const value = plot.data[seriesIdx + 1]?.[idx]

          return typeof value === 'number' && value > 0
            ? `${version}: ${formatNodeCount(value)}`
            : null
        }).filter((value): value is string => value !== null)
        if (lines.length === 0) {
          setTooltip(null)

          return
        }

        // The x axis is week index (0 = oldest). Use the server-supplied
        // week_starts so the label is consistent with the SQL store's
        // Monday math and survives cache drift across the week boundary
        // (a cached response from Saturday stays valid through Monday —
        // the browser's clock would mislabel the new week as the
        // previous one). Falls back to the old computation only if a
        // rolled-back API version is mid-deploy and didn't ship
        // week_starts; same end result for the user.
        const weekStart = history.week_starts[x]
        const weekDate = typeof weekStart === 'string'
          ? new Date(weekStart)
          : new Date(Date.now() - (history.weeks - 1 - x) * 7 * 24 * 60 * 60 * 1000)
        const weekLabel = weekDate.toLocaleDateString(undefined, {
          month: 'short',
          day: 'numeric',
          year: 'numeric'
        })

        setTooltip({
          time: `Week of ${weekLabel}`,
          value: lines.join(' · ')
        })
      }
    }
  }), [history])

  useEffect(() => {
    const root = rootRef.current
    if (!root || history.versions.length === 0 || history.weeks === 0) {return undefined}

    const colors = chartColors(root)
    const plot = new uPlot({
      width: chartWidth(root),
      height: chartHeight,
      legend: { show: false },
      cursor: {
        show: true,
        drag: { x: false, y: false },
        points: { show: true, size: 4 },
        sync: { key: 'stats-firmware' }
      },
      scales: {
        x: { time: false },
        y: { range: (_plot, _min, max) => [0, Math.max(1, Math.ceil(max))] }
      },
      axes: [
        {
          stroke: colors.axis,
          grid: { stroke: colors.grid, width: 1 },
          ticks: { stroke: colors.grid, width: 1 },
          border: { stroke: colors.grid, width: 1 },
          // 54 weeks is dense — show only ~8 ticks across the axis.
          space: 64,
          values: (_plot, ticks) => ticks.map((tick) => {
            // Mirror the cursor-plugin date math so tick labels and
            // tooltip labels agree on what each index means. With a
            // 24h cache TTL a response generated Monday 00:05 UTC can
            // still be served Tuesday — week_starts is the only
            // source of truth that stays in lockstep with the SQL
            // store's Monday math, so the axis must read from it
            // (not from Date.now(), which would label the prior
            // month on a Sunday response of a Monday-generated
            // cached payload).
            const weekStart = history.week_starts[tick]
            const date = typeof weekStart === 'string'
              ? new Date(weekStart)
              : new Date(Date.now() - (history.weeks - 1 - tick) * 7 * 24 * 60 * 60 * 1000)

            return date.toLocaleDateString(undefined, { month: 'short', year: '2-digit' })
          })
        },
        {
          stroke: colors.axis,
          label: 'Nodes',
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
        ...history.versions.map((version, index) => ({
          label: version,
          stroke: colors.lines[index % colors.lines.length],
          // Bottom series (largest by convention) gets the area fill at
          // low alpha; remaining series are lines for readability.
          fill: index === 0 ? colors.fill : undefined,
          width: 1,
          points: { show: false }
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
  }, [cursorPlugin, data, history])

  useEffect(() => {
    plotRef.current?.setData(data)
  }, [data])

  return (
    <article className="firmware-chart" data-chart="firmware-history">
      <header>
        <h3>Firmware adoption over time</h3>
        <span className={tooltip ? 'activity-tooltip' : 'activity-tooltip muted'}>
          {tooltip ? `${tooltip.time} · ${tooltip.value}` : 'Hover for details'}
        </span>
      </header>
      <div className="firmware-chart-canvas" ref={rootRef} aria-label="Firmware version history" />
      {history.versions.length === 0
        ? <p className="activity-empty">No firmware history recorded yet.</p>
        : (
          <div className="chart-legend">
            {history.versions.map((version, index) => (
              <span key={version} className="chart-legend-item">
                <span className="chart-legend-swatch" style={{ background: lineColors()[index % lineColors().length] }} />
                {version}
              </span>
            ))}
          </div>
        )}
    </article>
  )
}

function SoftwareSection({ snapshot, history }: { snapshot?: FirmwareSnapshot; history?: FirmwareHistory }): JSX.Element {
  return (
    <section className="stats-section stats-section-software" aria-labelledby="stats-software">
      <div className="stats-section-heading">
        <h2 id="stats-software">Software</h2>
        <p>Firmware version distribution</p>
      </div>
      <div className="firmware-grid">
        <FirmwareSnapshotChart versions={snapshot?.versions ?? []} />
        <FirmwareHistoryChart history={history ?? { generated_at: '', cache_ttl_seconds: 0, weeks: 0, top: 0, versions: [], versions_by_week: [], week_starts: [] }} />
      </div>
    </section>
  )
}

export function StatsPage({ initialStats, initialFirmwareSnapshot, initialFirmwareHistory }: Props): JSX.Element {
  const [stats, setStats] = useState<ActivityStats | undefined>(initialStats)
  const [loadError, setLoadError] = useState('')
  const [firmwareSnapshot, setFirmwareSnapshot] = useState<FirmwareSnapshot | undefined>(initialFirmwareSnapshot)
  const [firmwareHistory, setFirmwareHistory] = useState<FirmwareHistory | undefined>(initialFirmwareHistory)

  const loadStats = useCallback((signal?: AbortSignal): Promise<void> => (
    api.statsActivity({ signal })
      .then((next) => {
        setStats(next)
        setLoadError('')
      })
  ), [])

  const loadFirmwareSnapshot = useCallback((signal?: AbortSignal): Promise<void> => (
    api.firmwareSnapshot({ signal })
      .then((next) => {
        setFirmwareSnapshot(next)
      })
  ), [])

  const loadFirmwareHistory = useCallback((signal?: AbortSignal): Promise<void> => (
    api.firmwareHistory({ signal })
      .then((next) => {
        setFirmwareHistory(next)
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
    const controller = new AbortController()
    void loadFirmwareSnapshot(controller.signal)
      .catch((err) => {
        if (isAbortError(err)) {return}
        // Surface as a generic stats load error — the snapshot endpoint
        // shares its failure mode with the activity endpoint (network
        // / server down).
        setLoadError('Failed to load firmware snapshot.')
      })

    return () => controller.abort()
  }, [loadFirmwareSnapshot])

  useEffect(() => {
    const controller = new AbortController()
    void loadFirmwareHistory(controller.signal)
      .catch((err) => {
        if (isAbortError(err)) {return}
        setLoadError('Failed to load firmware history.')
      })

    return () => controller.abort()
  }, [loadFirmwareHistory])

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

  useEffect(() => {
    // Honour the server's resolved TTL (echoed in `cache_ttl_seconds`)
    // so an operator shortening web.stats.software.snapshot_cache_ttl
    // actually picks up fresher data instead of the UI staying stale
    // on a hard-coded 1h interval. Default to the server's default
    // for the period before the first response arrives.
    const ttlMs = firmwareSnapshot && firmwareSnapshot.cache_ttl_seconds > 0
      ? pollDelaySeconds(firmwareSnapshot.cache_ttl_seconds) * 1000
      : firmwareSnapshotDefaultTTL
    const timeout = window.setTimeout(() => {
      void loadFirmwareSnapshot()
        .catch((err) => {
          if (isAbortError(err)) {return}
          setLoadError('Failed to refresh firmware snapshot.')
        })
    }, ttlMs)

    return () => window.clearTimeout(timeout)
  }, [firmwareSnapshot, loadFirmwareSnapshot])

  useEffect(() => {
    // Same TTL-driven cadence as the snapshot endpoint — see the
    // comment above. Operators tune one knob
    // (web.stats.software.history_cache_ttl) and the UI follows.
    const ttlMs = firmwareHistory && firmwareHistory.cache_ttl_seconds > 0
      ? pollDelaySeconds(firmwareHistory.cache_ttl_seconds) * 1000
      : firmwareHistoryDefaultTTL
    const timeout = window.setTimeout(() => {
      void loadFirmwareHistory()
        .catch((err) => {
          if (isAbortError(err)) {return}
          setLoadError('Failed to refresh firmware history.')
        })
    }, ttlMs)

    return () => window.clearTimeout(timeout)
  }, [firmwareHistory, loadFirmwareHistory])

  const retry = (): void => {
    setLoadError('')
    void loadStats().catch((err) => {
      if (!isAbortError(err)) {setLoadError('Failed to load activity stats.')}
    })
    void loadFirmwareSnapshot().catch((err) => {
      if (!isAbortError(err)) {setLoadError('Failed to load firmware snapshot.')}
    })
    void loadFirmwareHistory().catch((err) => {
      if (!isAbortError(err)) {setLoadError('Failed to load firmware history.')}
    })
  }

  return (
    <section className="stats-layout container-fluid">
      {loadError && <p className="load-error">{loadError} <button className="outline secondary" onClick={retry}>Retry</button></p>}
      {stats
        ? stats.periods.map((period) => <ActivitySection key={period.key} period={period} />)
        : <p className="node-list-empty">{loadError ? 'Failed to load activity stats.' : 'Loading activity stats.'}</p>}
      <SoftwareSection snapshot={firmwareSnapshot} history={firmwareHistory} />
    </section>
  )
}