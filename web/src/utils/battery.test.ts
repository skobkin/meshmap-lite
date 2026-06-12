import { describe, expect, it } from 'vitest'

import { batteryQualityClass, formatBattery, formatBatteryLevel, formatVoltage } from './battery'

describe('formatVoltage', () => {
  it('formats a positive voltage to two decimals with the V unit', () => {
    expect(formatVoltage(4.03)).toBe('4.03 V')
    expect(formatVoltage(4)).toBe('4.00 V')
    expect(formatVoltage(3.311)).toBe('3.31 V')
  })

  it('treats 0 as valid data', () => {
    expect(formatVoltage(0)).toBe('0.00 V')
  })

  it('returns null for undefined, null, or NaN', () => {
    expect(formatVoltage(undefined)).toBeNull()
    expect(formatVoltage(null)).toBeNull()
    expect(formatVoltage(Number.NaN)).toBeNull()
  })
})

describe('formatBatteryLevel', () => {
  it('rounds to the nearest integer and appends a percent sign', () => {
    expect(formatBatteryLevel(95)).toBe('95%')
    expect(formatBatteryLevel(94.5)).toBe('95%')
    expect(formatBatteryLevel(94.4)).toBe('94%')
  })

  it('treats 0 as valid data', () => {
    expect(formatBatteryLevel(0)).toBe('0%')
  })

  it('returns null for undefined, null, or NaN', () => {
    expect(formatBatteryLevel(undefined)).toBeNull()
    expect(formatBatteryLevel(null)).toBeNull()
    expect(formatBatteryLevel(Number.NaN)).toBeNull()
  })
})

describe('batteryQualityClass', () => {
  it('returns the strong tier for 75..100', () => {
    expect(batteryQualityClass(100)).toBe('signal-strong')
    expect(batteryQualityClass(75)).toBe('signal-strong')
  })

  it('returns the good tier for 50..74', () => {
    expect(batteryQualityClass(74)).toBe('signal-good')
    expect(batteryQualityClass(50)).toBe('signal-good')
  })

  it('returns the weak tier for 25..49', () => {
    expect(batteryQualityClass(49)).toBe('signal-weak')
    expect(batteryQualityClass(25)).toBe('signal-weak')
  })

  it('returns the poor tier for 0..24', () => {
    expect(batteryQualityClass(24)).toBe('signal-poor')
    expect(batteryQualityClass(0)).toBe('signal-poor')
  })

  it('clamps values above 100% to the strong tier', () => {
    // Meshtastic occasionally reports >100% (overcharged LiPo, sensor error);
    // those readings are valid data and should still be coloured, not rendered
    // without a tier. "More than full" is at least as good as full.
    expect(batteryQualityClass(101)).toBe('signal-strong')
    expect(batteryQualityClass(120)).toBe('signal-strong')
    expect(batteryQualityClass(999)).toBe('signal-strong')
  })

  it('returns an empty string for missing or negative values', () => {
    expect(batteryQualityClass(undefined)).toBe('')
    expect(batteryQualityClass(null)).toBe('')
    expect(batteryQualityClass(Number.NaN)).toBe('')
    expect(batteryQualityClass(-1)).toBe('')
    expect(batteryQualityClass(-0.5)).toBe('')
  })
})

describe('formatBattery', () => {
  it('returns no data when both fields are missing', () => {
    expect(formatBattery(undefined)).toEqual({ voltage: null, level: null, qualityClass: '', hasData: false })
    expect(formatBattery(null)).toEqual({ voltage: null, level: null, qualityClass: '', hasData: false })
    expect(formatBattery({})).toEqual({ voltage: null, level: null, qualityClass: '', hasData: false })
  })

  it('formats voltage only and does not assign a quality class from voltage', () => {
    const result = formatBattery({ voltage: 4.03 })
    expect(result.voltage).toBe('4.03 V')
    expect(result.level).toBeNull()
    expect(result.qualityClass).toBe('')
    expect(result.hasData).toBe(true)
  })

  it('formats level only and assigns a quality class from the level', () => {
    const result = formatBattery({ battery_level: 12 })
    expect(result.voltage).toBeNull()
    expect(result.level).toBe('12%')
    expect(result.qualityClass).toBe('signal-poor')
    expect(result.hasData).toBe(true)
  })

  it('formats both fields and assigns a quality class from the level', () => {
    const result = formatBattery({ voltage: 4.03, battery_level: 95 })
    expect(result.voltage).toBe('4.03 V')
    expect(result.level).toBe('95%')
    expect(result.qualityClass).toBe('signal-strong')
    expect(result.hasData).toBe(true)
  })

  it('treats 0 as valid for both fields and colours a depleted battery as poor', () => {
    const result = formatBattery({ voltage: 0, battery_level: 0 })
    expect(result.voltage).toBe('0.00 V')
    expect(result.level).toBe('0%')
    expect(result.qualityClass).toBe('signal-poor')
    expect(result.hasData).toBe(true)
  })

  it('treats NaN and null as missing for the format helpers but not contaminates the other field', () => {
    const result = formatBattery({ voltage: Number.NaN, battery_level: 50 })
    expect(result.voltage).toBeNull()
    expect(result.level).toBe('50%')
    expect(result.qualityClass).toBe('signal-good')
    expect(result.hasData).toBe(true)
  })
})
