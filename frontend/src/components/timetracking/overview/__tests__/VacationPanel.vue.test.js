import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'
import VacationPanel from '../VacationPanel.vue'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

const base = {
  year: 2026, entitlement_minutes: 12000, carry_over_minutes: 600, available_minutes: 12600,
  used_minutes: 4800, planned_minutes: 960, remaining_minutes: 7800, remaining_after_planned_minutes: 6840,
  months: Array.from({ length: 12 }, (_, i) => ({ month: i + 1, minutes: 0, days: 0 })),
  total_days: 12, suggested_carry_over_minutes: 900, has_settings_row: true, vacation_projects_set: true,
}

beforeEach(() => setActivePinia(createPinia()))

const mountIt = (data = base) => mount(VacationPanel, { props: { data }, global: { plugins: [i18n] } })

describe('VacationPanel', () => {
  it('prefills the carry-over input in hours', () => {
    expect(mountIt().get('#ov-carry-over').element.value).toBe('10')
  })

  it('suggestion button fills the input and emits save-carry-over', async () => {
    const w = mountIt()
    const btn = w.get('[data-test="use-suggestion"]')
    expect(btn.text()).toContain('15')
    await btn.trigger('click')
    expect(w.get('#ov-carry-over').element.value).toBe('15')
    expect(w.emitted('save-carry-over')[0]).toEqual([15])
  })

  it('hides the suggestion button when there is no suggestion', () => {
    expect(mountIt({ ...base, suggested_carry_over_minutes: null }).find('[data-test="use-suggestion"]').exists()).toBe(false)
  })

  it('emits null when the input is cleared', async () => {
    const w = mountIt()
    await w.get('#ov-carry-over').setValue('')
    await w.get('form').trigger('submit')
    expect(w.emitted('save-carry-over')[0]).toEqual([null])
  })
})
