import { describe, it, expect } from 'vitest'
import { clipboardAnchor, buildPastePayloads } from '../calendarClipboard'

const entry = (date, start_time, end_time, minutes, extra = {}) => ({
  id: Math.random(), customer_id: 1, project_id: 2, description: 'x', date: `${date}T00:00:00Z`,
  start_time, end_time, minutes, ...extra,
})

describe('clipboardAnchor', () => {
  it('picks the earliest date and start time', () => {
    expect(clipboardAnchor([
      entry('2026-10-06', '13:00', '17:00', 240),
      entry('2026-10-05', '09:00', '12:00', 180),
      entry('2026-10-05', '08:00', '08:30', 30),
    ])).toEqual({ date: '2026-10-05', start: 480 })
  })
})

describe('buildPastePayloads', () => {
  const day = [
    entry('2026-10-05', '09:00', '12:00', 180),
    entry('2026-10-05', '13:00', '17:00', 240),
  ]

  it('keeps original times when no target time is given', () => {
    const out = buildPastePayloads(day, '2026-10-07')
    expect(out.map(p => [p.date, p.start_time, p.end_time, p.minutes])).toEqual([
      ['2026-10-07', '09:00', '12:00', 180],
      ['2026-10-07', '13:00', '17:00', 240],
    ])
    expect(out[0].id).toBeUndefined()
    expect(out[0].customer_id).toBe(1)
  })

  it('shifts the group so the earliest entry starts at the target time', () => {
    const out = buildPastePayloads(day, '2026-10-08', '10:00')
    expect(out.map(p => [p.date, p.start_time, p.end_time])).toEqual([
      ['2026-10-08', '10:00', '13:00'],
      ['2026-10-08', '14:00', '18:00'],
    ])
  })

  it('keeps day offsets for a multi-day selection', () => {
    const out = buildPastePayloads([
      entry('2026-10-05', '09:00', '10:00', 60),
      entry('2026-10-06', '09:00', '10:00', 60),
    ], '2026-10-12')
    expect(out.map(p => p.date)).toEqual(['2026-10-12', '2026-10-13'])
  })

  it('clamps a same-day entry pushed past midnight and rolls later ones to the next day', () => {
    const out = buildPastePayloads(day, '2026-10-05', '20:00')
    expect(out.map(p => [p.date, p.start_time, p.end_time, p.minutes])).toEqual([
      ['2026-10-05', '20:00', '23:00', 180],
      ['2026-10-06', '00:00', '04:00', 240],
    ])
    const single = buildPastePayloads([day[1]], '2026-10-05', '22:00')
    expect(single[0]).toMatchObject({ start_time: '22:00', end_time: '23:59', minutes: 119 })
  })

  it('keeps overnight entries overnight', () => {
    const out = buildPastePayloads([entry('2026-10-05', '19:00', '07:00', 720)], '2026-10-09', '20:00')
    expect(out[0]).toMatchObject({ date: '2026-10-09', start_time: '20:00', end_time: '08:00', minutes: 720 })
  })

  it('moves unscheduled and whole-day entries by whole days only', () => {
    const out = buildPastePayloads([
      entry('2026-10-05', '09:00', '10:00', 60),
      entry('2026-10-05', '', '', 120),
      entry('2026-10-06', '00:00', '00:00', 480, { is_holiday: true }),
    ], '2026-10-12', '14:00')
    expect(out.map(p => [p.date, p.start_time, p.end_time, p.minutes])).toEqual([
      ['2026-10-12', '14:00', '15:00', 60],
      ['2026-10-12', '', '', 120],
      ['2026-10-13', '00:00', '00:00', 480],
    ])
    expect(out[2].is_holiday).toBe(true)
  })
})
