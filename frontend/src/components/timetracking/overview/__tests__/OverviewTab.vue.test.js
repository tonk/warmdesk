import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'

vi.mock('@/api/timeEntries', () => ({
  timeEntriesApi: {
    overviewNorm: vi.fn(),
    overviewChecks: vi.fn(),
    overviewTravel: vi.fn(),
    overviewVacation: vi.fn(),
    overviewNormPDF: vi.fn(),
    overviewTravelPDF: vi.fn(),
  },
}))
vi.mock('@/api/client', () => ({ triggerDownload: vi.fn() }))

import { timeEntriesApi } from '@/api/timeEntries'
import { triggerDownload } from '@/api/client'
import OverviewTab from '../OverviewTab.vue'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

const row = (month) => ({
  month, norm_days: 20, norm_minutes: 9600, worked_minutes: 0, travel_minutes: 0,
  travel_declarable_minutes: 0, distance: 0, sick_minutes: 0, vacation_minutes: 0,
  booked_minutes: 0, diff_minutes: -9600, booked_days: 0, unbooked_norm_days: 20, past_unbooked_norm_days: 0,
})
const norm = {
  year: new Date().getFullYear(), hours_per_week: 40, norm_minutes_per_day: 480, weekdays: 261,
  holiday_weekdays: [], holiday_names: {},
  months: Array.from({ length: 12 }, (_, i) => row(i + 1)), total: row(0),
  norm_to_date_minutes: 0, booked_to_date_minutes: 0, diff_to_date_minutes: 0, planned_minutes: 0, total_days: 0,
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  timeEntriesApi.overviewNorm.mockResolvedValue({ data: norm })
  const acc = { entries: 0, days: 0, total_minutes: 0, own_minutes: 0, declarable_minutes: 0, distance: 0 }
  timeEntriesApi.overviewTravel.mockResolvedValue({ data: {
    year: 2026, months: Array.from({ length: 12 }, (_, i) => ({ month: i + 1, ...acc })),
    total: acc, out: acc, back: acc, no_direction: acc, by_customer: [], by_route: [],
    entries_without_distance: 0, total_days: 0, travel_project_count: 1,
  } })
  timeEntriesApi.overviewVacation.mockResolvedValue({ data: {
    year: 2026, entitlement_minutes: 0, carry_over_minutes: 0, available_minutes: 0, used_minutes: 0,
    planned_minutes: 0, remaining_minutes: 0, remaining_after_planned_minutes: 0,
    months: Array.from({ length: 12 }, (_, i) => ({ month: i + 1, minutes: 0, days: 0 })),
    total_days: 0, suggested_carry_over_minutes: 0, has_settings_row: true, vacation_projects_set: true,
  } })
  timeEntriesApi.overviewChecks.mockResolvedValue({ data: { year: 2026, findings: [], counts: {}, description_variants: [], total_days: 0 } })
})

const pdfButton = (w) => w.findAll('button').find((b) => b.text() === 'PDF')

describe('OverviewTab PDF export', () => {
  it('downloads the Norm overview as PDF for the year, user and language', async () => {
    const buf = new ArrayBuffer(4)
    timeEntriesApi.overviewNormPDF.mockResolvedValue(buf)
    const w = mount(OverviewTab, { props: { userId: 7 }, global: { plugins: [i18n] } })
    await flushPromises()

    const btn = pdfButton(w)
    expect(btn.attributes('aria-label')).toBe('Export PDF')
    await btn.trigger('click')
    await flushPromises()

    const year = new Date().getFullYear()
    expect(timeEntriesApi.overviewNormPDF).toHaveBeenCalledWith({ year, user_id: 7, lang: 'en' })
    expect(triggerDownload).toHaveBeenCalledWith(buf, `norm-overview-${year}.pdf`, 'application/pdf')
  })

  it('downloads the Travel overview as PDF from the Travel sub-tab', async () => {
    const buf = new ArrayBuffer(4)
    timeEntriesApi.overviewTravelPDF.mockResolvedValue(buf)
    const w = mount(OverviewTab, { global: { plugins: [i18n] } })
    await flushPromises()
    await w.get('#ov-tab-travel').trigger('click')
    await flushPromises()
    await pdfButton(w).trigger('click')
    await flushPromises()

    const year = new Date().getFullYear()
    expect(timeEntriesApi.overviewTravelPDF).toHaveBeenCalledWith({ year, lang: 'en' })
    expect(timeEntriesApi.overviewNormPDF).not.toHaveBeenCalled()
    expect(triggerDownload).toHaveBeenCalledWith(buf, `travel-overview-${year}.pdf`, 'application/pdf')
  })

  it('only offers the button on the Norm and Travel sub-tabs', async () => {
    const w = mount(OverviewTab, { global: { plugins: [i18n] } })
    await flushPromises()
    expect(pdfButton(w)).toBeTruthy()
    await w.get('#ov-tab-checks').trigger('click')
    await flushPromises()
    expect(pdfButton(w)).toBeUndefined()
    await w.get('#ov-tab-vacation').trigger('click')
    await flushPromises()
    expect(pdfButton(w)).toBeUndefined()
  })
})
