import { describe, it, expect } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'
import en from '@/i18n/en.json'
import TimeTrackingCalendarWeekGrid from '../TimeTrackingCalendarWeekGrid.vue'
import TimeTrackingCalendarDay from '../TimeTrackingCalendarDay.vue'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

const weekDays = ['2026-09-21', '2026-09-22', '2026-09-23', '2026-09-24', '2026-09-25', '2026-09-26', '2026-09-27']
  .map(iso => ({ iso, mmdd: iso.slice(5), abbr: '', isToday: false }))

function blocksByDay(entries) {
  setActivePinia(createPinia())
  const w = shallowMount(TimeTrackingCalendarWeekGrid, {
    props: {
      weekDays, entries, pxPerHour: 60,
      customerName: () => '', projectName: () => '', entryColor: () => '#888888',
    },
    global: { plugins: [i18n], stubs: { TimeTrackingCalendarDay: true } },
  })
  const out = {}
  w.findAllComponents(TimeTrackingCalendarDay).forEach((day, i) => {
    out[weekDays[i].iso] = day.props('entries').map(b => ({ id: b.entry.id, segment: b.segment, top: b.top, height: b.height }))
  })
  return out
}

describe('TimeTrackingCalendarWeekGrid overnight layout', () => {
  it('draws an entry ending exactly at midnight on its own day only', () => {
    const days = blocksByDay([
      { id: 1, date: '2026-09-25T00:00:00Z', start_time: '23:00', end_time: '00:00' },
      { id: 2, date: '2026-09-26T00:00:00Z', start_time: '00:00', end_time: '00:30' },
    ])
    expect(days['2026-09-25']).toEqual([{ id: 1, segment: 'start', top: 23 * 60, height: 60 }])
    // Only the real second half on the next day, no stray continuation of entry 1.
    expect(days['2026-09-26'].map(b => b.id)).toEqual([2])
  })

  it('still splits a genuine overnight entry across both days', () => {
    const days = blocksByDay([{ id: 3, date: '2026-09-25T00:00:00Z', start_time: '22:00', end_time: '02:00' }])
    expect(days['2026-09-25'].map(b => b.segment)).toEqual(['start'])
    expect(days['2026-09-26']).toEqual([{ id: 3, segment: 'continuation', top: 0, height: 120 }])
  })
})
