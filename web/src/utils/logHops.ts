import { classifyHops } from './signal'

import type { LogEvent } from '../api/types'

export interface LogHopRange {
  min: number
  max: number
}

export const logHopsMin = 0
export const logHopsMax = 7
export const defaultLogHopRange: LogHopRange = { min: logHopsMin, max: logHopsMax }

export function normalizeLogHopRange(range?: Partial<LogHopRange>): LogHopRange {
  let min = clampHopValue(range?.min ?? defaultLogHopRange.min)
  let max = clampHopValue(range?.max ?? defaultLogHopRange.max)
  if (min > max) {
    const nextMin = max
    max = min
    min = nextMin
  }

  return { min, max }
}

export function isDefaultLogHopRange(range: LogHopRange): boolean {
  return range.min === defaultLogHopRange.min && range.max === defaultLogHopRange.max
}

export function logHopRangeLabel(range: LogHopRange): string {
  if (isDefaultLogHopRange(range)) {return 'All hops'}
  if (range.min === range.max) {
    return range.min === logHopsMax ? `${range.min}+ hops` : `${range.min} ${range.min === 1 ? 'hop' : 'hops'}`
  }
  if (range.max === logHopsMax) {return `${range.min}+ hops`}

  return `${range.min}-${range.max} hops`
}

export function eventMatchesLogHopRange(event: LogEvent, range: LogHopRange): boolean {
  if (isDefaultLogHopRange(range)) {return true}
  if (
    typeof event.node_id === 'string' &&
    event.node_id.length > 0 &&
    event.node_id === event.mqtt_uploader_node_id
  ) {
    return false
  }

  const result = classifyHops(event.hop_start, event.hop_limit)
  if (result.traversed === undefined) {return false}

  return result.traversed >= range.min && result.traversed <= range.max
}

function clampHopValue(value: number): number {
  if (!Number.isFinite(value)) {return logHopsMin}

  return Math.min(logHopsMax, Math.max(logHopsMin, Math.trunc(value)))
}
