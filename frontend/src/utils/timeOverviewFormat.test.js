import { describe, it, expect } from 'vitest'
import { fmtHM, fmtSignedHM, minutesToHours, hoursToMinutes, diffClass, fmtKm } from './timeOverviewFormat'

describe('timeOverviewFormat', () => {
  it('formats minutes as h:mm', () => {
    expect(fmtHM(0)).toBe('0:00')
    expect(fmtHM(90)).toBe('1:30')
    expect(fmtHM(65)).toBe('1:05')
    expect(fmtHM(-45)).toBe('-0:45')
    expect(fmtHM(12000)).toBe('200:00')
    expect(fmtHM(undefined)).toBe('0:00')
  })
  it('always signs differences', () => {
    expect(fmtSignedHM(90)).toBe('+1:30')
    expect(fmtSignedHM(-90)).toBe('-1:30')
    expect(fmtSignedHM(0)).toBe('0:00')
  })
  it('converts to decimal hours and back', () => {
    expect(minutesToHours(90)).toBe(1.5)
    expect(minutesToHours(100)).toBe(1.67)
    expect(hoursToMinutes('1,5')).toBe(90)
    expect(hoursToMinutes(8)).toBe(480)
    expect(hoursToMinutes('x')).toBe(0)
  })
  it('classifies differences', () => {
    expect(diffClass(5)).toBe('diff-pos')
    expect(diffClass(-5)).toBe('diff-neg')
    expect(diffClass(0)).toBe('diff-zero')
  })
  it('rounds kilometres', () => {
    expect(fmtKm(12.345)).toBe('12.3')
    expect(fmtKm(null)).toBe('0')
  })
})
