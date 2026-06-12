// Shared helpers for rendering the "power" portion of a node's telemetry:
// voltage and battery level, plus a battery-level -> signal-quality colour
// classification that re-uses the universal `.signal-strong` / `.signal-good` /
// `.signal-weak` / `.signal-poor` CSS classes (same palette as `signal.ts`'s
// hop classifier and the link-quality tiers set in #38 / #83). View-agnostic:
// no HTML, no JSX, no emoji — callers compose their own layout.

export type BatteryQualityClass = 'signal-strong' | 'signal-good' | 'signal-weak' | 'signal-poor' | ''

export interface BatteryFormat {
  /** "4.03 V" or null when no usable voltage was reported. */
  voltage: string | null
  /** "95%" (rounded integer) or null when no usable level was reported. */
  level: string | null
  /** Signal-quality CSS class derived from the battery level (empty when unknown). */
  qualityClass: BatteryQualityClass
  /** True when at least one of the two fields was present and finite. */
  hasData: boolean
}

export interface BatteryPower {
  voltage?: number
  battery_level?: number
}

/**
 * Format a voltage reading for display. Returns `"4.03 V"` (two decimals, locale-
 * independent, trailing space + unit) or `null` for `undefined` / `null` / `NaN`.
 * Treats `0` as valid data — Meshtastic reports `0.00 V` on some devices during
 * brownouts, and a true-zero reading should still be visible to the operator.
 */
export function formatVoltage(v: number | undefined | null): string | null {
  if (typeof v !== 'number' || !Number.isFinite(v)) {return null}

  return `${v.toFixed(2)} V`
}

/**
 * Format a battery level for display. Returns `"95%"` (rounded to the nearest
 * integer via `Math.round`) or `null` for `undefined` / `null` / `NaN`. Treats
 * `0` as valid data — a node that has actually reported `0%` (battery depleted)
 * should not be hidden.
 */
export function formatBatteryLevel(l: number | undefined | null): string | null {
  if (typeof l !== 'number' || !Number.isFinite(l)) {return null}

  return `${Math.round(l)}%`
}

/**
 * Map a battery level (0-100, integer) to a signal-quality CSS class so the
 * caller can colour the rendered value the same way the rest of the UI
 * colours "how good is this number" (hop count, link quality, etc.).
 *
 * Tiers:
 *  - 75..100: strong (clamped: anything ≥ 75, including >100% readings,
 *             is treated as "fully charged or better")
 *  - 50..74:  good
 *  - 25..49:  weak
 *  - 0..24:   poor
 *
 * Returns `''` for `null` / `undefined` / `NaN` / negative values. Meshtastic
 * nodes occasionally report >100% (overcharged LiPo, sensor error); those
 * readings are valid data and are coloured as `signal-strong` rather than
 * rendered without a tier. The caller is expected to have already null-
 * checked the underlying field; the remaining guard is defensive so the
 * helper is safe to call unconditionally from the composite `formatBattery()`.
 */
export function batteryQualityClass(level: number | null | undefined): BatteryQualityClass {
  if (typeof level !== 'number' || !Number.isFinite(level)) {return ''}
  if (level < 0) {return ''}
  if (level >= 75) {return 'signal-strong'}
  if (level >= 50) {return 'signal-good'}
  if (level >= 25) {return 'signal-weak'}

  return 'signal-poor'
}

/**
 * Composite helper for callers that want both fields plus the quality class in
 * a single call. `qualityClass` is populated only when `battery_level` is a
 * finite number — we don't colour from voltage alone, because a healthy
 * voltage on a node that never reports percent isn't the same signal as a
 * low battery reading, and we don't want a 4.03 V reading to render in red.
 */
export function formatBattery(power: BatteryPower | null | undefined): BatteryFormat {
  const voltage = formatVoltage(power?.voltage)
  const rawLevel = power?.battery_level
  const level = formatBatteryLevel(rawLevel)
  const qualityClass = level === null ? '' : batteryQualityClass(rawLevel)
  const hasData = voltage !== null || level !== null

  return { voltage, level, qualityClass, hasData }
}
