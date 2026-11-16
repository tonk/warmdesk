import { describe, it, expect } from 'vitest'
import { columnDurations, formatSpan } from '../cardColumnTime'

const T0 = Date.parse('2026-11-02T09:00:00Z')
const at = (hours) => new Date(T0 + hours * 3600000).toISOString()
const move = (id, hours, from, to, extra = {}) => ({
  id, created_at: at(hours), event_type: 'column_move',
  from_column_id: from, to_column_id: to,
  from_column: { name: `col${from}` }, to_column: { name: `col${to}` }, ...extra,
})

describe('columnDurations', () => {
  const columns = [{ id: 1, name: 'Todo' }, { id: 2, name: 'In Progress' }, { id: 3, name: 'Review' }]

  it('a card that never moved sat in its current column since creation', () => {
    const out = columnDurations([], { created_at: at(0), column_id: 1 }, columns, T0 + 2 * 3600000)
    expect(out).toEqual([{ column_id: 1, name: 'Todo', minutes: 120, current: true }])
  })

  it('splits time at each move, sums revisits, and keeps board order', () => {
    const history = [
      // newest first, as the API returns it
      move(4, 30, 2, 3),
      move(3, 20, 1, 2),
      move(2, 10, 2, 1),
      move(1, 2, 1, 2),
      { id: 9, created_at: at(5), event_type: 'title_changed' },
    ]
    const out = columnDurations(history, { created_at: at(0), column_id: 3 }, columns, T0 + 54 * 3600000)
    expect(out).toEqual([
      { column_id: 1, name: 'Todo', minutes: (2 + 10) * 60, current: false },
      { column_id: 2, name: 'In Progress', minutes: (8 + 10) * 60, current: false },
      { column_id: 3, name: 'Review', minutes: 24 * 60, current: true },
    ])
  })

  it('names columns that are no longer on the board from the history', () => {
    const out = columnDurations([move(1, 1, 7, 1)], { created_at: at(0), column_id: 1 }, columns, T0 + 2 * 3600000)
    expect(out.map(r => r.name)).toEqual(['Todo', 'col7'])
  })

  it('returns nothing without a creation time', () => {
    expect(columnDurations([], {}, columns)).toEqual([])
  })
})

describe('formatSpan', () => {
  it.each([
    [0, '0m'], [12, '12m'], [60, '1h'], [320, '5h 20m'], [1440, '1d'], [1440 * 3 + 240, '3d 4h'],
  ])('%i minutes → %s', (m, want) => {
    expect(formatSpan(m)).toBe(want)
  })
})
