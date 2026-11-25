import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'
import NormPanel from '../NormPanel.vue'
import { fmtKm } from '@/utils/timeOverviewFormat'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

const row = (month, o = {}) => ({
  month, norm_days: 20, norm_minutes: 9600, worked_minutes: 9000, travel_minutes: 600,
  travel_declarable_minutes: 300, distance: 85.5,
  sick_minutes: 0, vacation_minutes: 480, booked_minutes: 9480, diff_minutes: -120,
  booked_days: 19, unbooked_norm_days: 1, past_unbooked_norm_days: 1, ...o,
})

const fixture = {
  year: 2026, hours_per_week: 40, norm_minutes_per_day: 480, weekdays: 261,
  holiday_weekdays: ['2026-12-25'],
  months: Array.from({ length: 12 }, (_, i) => row(i + 1, i === 0 ? { diff_minutes: 90, booked_minutes: 9690 } : {})),
  total: row(0, { norm_days: 240, norm_minutes: 115200, booked_minutes: 113760, diff_minutes: -1440, booked_days: 200, unbooked_norm_days: 12, past_unbooked_norm_days: 3 }),
  norm_to_date_minutes: 60000, booked_to_date_minutes: 59000, diff_to_date_minutes: -1000, planned_minutes: 480,
  total_days: 198,
}

beforeEach(() => setActivePinia(createPinia()))

describe('NormPanel', () => {
  const mountIt = () => mount(NormPanel, { props: { data: fixture }, global: { plugins: [i18n] } })

  it('renders a row per month plus a totals row', () => {
    const w = mountIt()
    expect(w.findAll('tbody tr')).toHaveLength(12)
    expect(w.get('tbody tr').text()).toContain('January')
    expect(w.get('tfoot').text()).toContain('1896:00') // 113760 min
  })

  it('shows signed, coloured differences', () => {
    const w = mountIt()
    const first = w.get('tbody tr').findAll('td')[9]
    expect(first.text()).toBe('+1:30')
    expect(first.classes()).toContain('diff-pos')
    const second = w.findAll('tbody tr')[1].findAll('td')[9]
    expect(second.text()).toBe('-2:00')
    expect(second.classes()).toContain('diff-neg')
  })

  it('uses the API unique day total, not a sum of rows', () => {
    const w = mountIt()
    const cells = w.get('tfoot').findAll('td')
    expect(cells[10].text()).toBe('198')
    expect(w.text()).toContain('198')
  })

  it('shows the number of vacation days in the norm line', () => {
    // total vacation 480 min at 480 min a day = 1 day
    expect(mountIt().text()).toContain('Norm based on 40 hours per week (8:00 per day). Vacation days: 1.')
  })

  it('counts part days when the norm day is shorter', () => {
    const data = { ...fixture, hours_per_week: 36, norm_minutes_per_day: 432, total: { ...fixture.total, vacation_minutes: 648 } }
    const w = mount(NormPanel, { props: { data }, global: { plugins: [i18n] } })
    expect(w.text()).toContain('Vacation days: 1.5.')
  })

  it('shows the declarable travel time and the kilometres', () => {
    const cells = mountIt().get('tbody tr').findAll('td')
    expect(cells[3].text()).toBe('10:00')
    expect(cells[4].text()).toBe('5:00')
    expect(cells[5].text()).toBe(fmtKm(85.5))
  })

  it('lists the holiday weekdays', () => {
    expect(mountIt().text()).toContain('2026-12-25')
  })
})
