import {describe, expect, it} from 'vitest'

import {formatDurationSeconds} from './duration'

describe('formatDurationSeconds', () => {
  it('returns null for null', () => {
    expect(formatDurationSeconds(null)).toBeNull()
  })

  it('returns null for undefined', () => {
    expect(formatDurationSeconds(undefined)).toBeNull()
  })

  it('returns null for NaN', () => {
    expect(formatDurationSeconds(NaN)).toBeNull()
  })

  it('returns null for negative numbers', () => {
    expect(formatDurationSeconds(-1)).toBeNull()
  })

  it('returns null for Infinity', () => {
    expect(formatDurationSeconds(Infinity)).toBeNull()
  })

  it('formats zero as "0m"', () => {
    expect(formatDurationSeconds(0)).toBe('0m')
  })

  it('formats sub-minute as "0m"', () => {
    expect(formatDurationSeconds(45)).toBe('0m')
  })

  it('formats minutes without leading zeros when only minutes present', () => {
    expect(formatDurationSeconds(3 * 60 + 15)).toBe('3m')
  })

  it('formats minutes-only with two-digit count when needed', () => {
    expect(formatDurationSeconds(15 * 60)).toBe('15m')
  })

  it('formats hours and minutes', () => {
    expect(formatDurationSeconds(2 * 3600 + 3 * 60 + 15)).toBe('02h 03m')
  })

  it('formats days, hours, and minutes', () => {
    expect(formatDurationSeconds(86400 + 2 * 3600 + 3 * 60)).toBe('1d 02h 03m')
  })

  it('drops zero days but keeps hour padding', () => {
    expect(formatDurationSeconds(0 * 86400 + 5 * 3600 + 7 * 60)).toBe('05h 07m')
  })

  it('drops zero hours and minutes when days present', () => {
    expect(formatDurationSeconds(2 * 86400)).toBe('2d 00h 00m')
  })

  it('floors fractional seconds', () => {
    expect(formatDurationSeconds(125.9)).toBe('2m')
  })
})
