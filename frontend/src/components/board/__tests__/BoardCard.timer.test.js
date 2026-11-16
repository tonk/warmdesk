import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'
import BoardCard from '../BoardCard.vue'
import { useAuthStore } from '@/stores/auth'
import { useTimerStore } from '@/stores/timer'
import { useProjectStore } from '@/stores/project'

vi.mock('@/api/timer', () => ({
  timerApi: { get: vi.fn(), targets: vi.fn(), start: vi.fn(), stop: vi.fn(), cancel: vi.fn() },
}))
import { timerApi } from '@/api/timer'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })
const card = { id: 7, card_number: 3, title: 'Build login', priority: 'none' }

function setup({ timeTracking = true, available = true } = {}) {
  setActivePinia(createPinia())
  useAuthStore().user = { id: 1, global_role: 'user', time_tracking_enabled: timeTracking }
  useProjectStore().currentProject = { key_prefix: 'WEB' }
  useTimerStore().available = available
  return mount(BoardCard, { props: { card }, global: { plugins: [i18n] } })
}

describe('BoardCard timer button', () => {
  beforeEach(() => vi.clearAllMocks())

  it('is hidden without time tracking', () => {
    expect(setup({ timeTracking: false }).find('.card-timer-btn').exists()).toBe(false)
    expect(setup({ available: false }).find('.card-timer-btn').exists()).toBe(false)
  })

  it('starts a timer on the card without opening it', async () => {
    timerApi.start.mockResolvedValue({ data: { running: true, timer: { card_id: 7, started_at: new Date().toISOString() } } })
    const w = setup()
    const btn = w.find('.card-timer-btn')
    expect(btn.attributes('aria-label')).toBe('Start timer on WEB-3')
    expect(btn.attributes('aria-pressed')).toBe('false')

    await btn.trigger('click')
    await flushPromises()
    expect(timerApi.start).toHaveBeenCalledWith(expect.objectContaining({ card_id: 7, description: '' }))
    expect(w.emitted('open')).toBeUndefined()
    expect(btn.attributes('aria-pressed')).toBe('true')
    expect(btn.attributes('aria-label')).toBe('Stop timer on WEB-3')
    expect(w.find('.board-card').classes()).toContain('board-card--timing')
  })

  it('stops a timer running on this card', async () => {
    timerApi.stop.mockResolvedValue({ data: [{ minutes: 15, description: 'x' }] })
    const w = setup()
    const timer = useTimerStore()
    timer.running = true
    timer.timer = { card_id: 7 }
    await flushPromises()
    await w.find('.card-timer-btn').trigger('click')
    await flushPromises()
    expect(timerApi.stop).toHaveBeenCalled()
    expect(timer.running).toBe(false)
  })
})
