import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'
import TimerButton from '../TimerButton.vue'
import { useUIStore } from '@/stores/ui'
import { useTimerStore } from '@/stores/timer'

vi.mock('@/api/timer', () => ({
  timerApi: {
    get: vi.fn(),
    targets: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
    cancel: vi.fn(),
  },
}))
import { timerApi } from '@/api/timer'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

const targets = {
  projects: [
    { id: 1, name: 'Website', customer_id: 10, customer_name: 'Acme' },
    { id: 2, name: 'Travel', time_tracking_only: true },
  ],
  customers: [{ id: 10, name: 'Acme' }, { id: 11, name: 'Globex' }],
}

function mountButton() {
  return mount(TimerButton, { global: { plugins: [i18n] }, attachTo: document.body })
}

describe('TimerButton', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorage.clear()
    timerApi.get.mockResolvedValue({ data: { running: false } })
    timerApi.targets.mockResolvedValue({ data: targets })
  })

  it('starts a timer on a board project, which fixes the customer', async () => {
    const started = new Date().toISOString()
    timerApi.start.mockResolvedValue({ data: {
      running: true,
      timer: { project: { name: 'Website' }, customer: { name: 'Acme' }, description: 'homepage', started_at: started },
    } })
    const w = mountButton()
    await flushPromises()
    expect(w.find('.timer-btn').attributes('aria-label')).toBe('Start timer')

    await w.find('.timer-btn').trigger('click')
    await flushPromises()
    const panel = w.find('[role="dialog"]')
    expect(panel.exists()).toBe(true)
    expect(panel.attributes('aria-labelledby')).toBe('timer-panel-title')
    expect(document.activeElement?.id).toBe('timer-project')
    expect(w.find('button[type="submit"]').attributes('disabled')).toBeDefined()

    await w.find('#timer-project').setValue(1)
    const customer = w.find('#timer-customer')
    expect(customer.attributes('disabled')).toBeDefined()
    expect(customer.element.value).toBe('10')

    await w.find('#timer-description').setValue('homepage')
    await w.find('form').trigger('submit')
    await flushPromises()

    expect(timerApi.start).toHaveBeenCalledWith(expect.objectContaining({
      project_id: 1, customer_id: 10, description: 'homepage',
    }))
    expect(w.find('[role="dialog"]').exists()).toBe(false)
    expect(w.find('.timer-btn').classes()).toContain('running')
    expect(w.find('.timer-elapsed').text()).toBe('0:00')
    expect(w.find('.timer-btn').attributes('aria-label')).toContain('Website (Acme) — homepage')
    w.unmount()
  })

  it('lets a time-tracking project take any customer', async () => {
    const w = mountButton()
    await flushPromises()
    await w.find('.timer-btn').trigger('click')
    await flushPromises()
    await w.find('#timer-project').setValue(2)
    const customer = w.find('#timer-customer')
    expect(customer.attributes('disabled')).toBeUndefined()
    expect(customer.findAll('option').map(o => o.text())).toEqual(['— No customer —', 'Acme', 'Globex'])
    w.unmount()
  })

  it('stops a running timer and reports what was booked', async () => {
    timerApi.get.mockResolvedValue({ data: {
      running: true,
      timer: { project: { name: 'Travel' }, customer: { name: 'Globex' }, started_at: new Date(Date.now() - 75 * 60000).toISOString() },
    } })
    timerApi.stop.mockResolvedValue({ data: [
      { minutes: 90, project: { name: 'Travel' }, customer: { name: 'Globex' } },
    ] })
    const w = mountButton()
    await flushPromises()
    expect(w.find('.timer-elapsed').text()).toBe('1:15')

    await w.find('.timer-btn').trigger('click')
    await flushPromises()
    expect(w.find('.timer-label').text()).toBe('Travel (Globex)')

    const store = useTimerStore()
    const before = store.bookedVersion
    const stopBtn = w.findAll('.timer-actions button').find(b => b.text() === 'Stop and book')
    await stopBtn.trigger('click')
    await flushPromises()

    expect(timerApi.stop).toHaveBeenCalled()
    expect(store.running).toBe(false)
    expect(store.bookedVersion).toBe(before + 1)
    expect(useUIStore().toasts.map(t => t.message)).toContain('Booked 1h 30m on Travel (Globex)')
    w.unmount()
  })

  it('closes on Escape and returns focus to the button', async () => {
    const w = mountButton()
    await flushPromises()
    await w.find('.timer-btn').trigger('click')
    await flushPromises()
    await w.find('[role="dialog"]').trigger('keydown', { key: 'Escape' })
    expect(w.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(w.find('.timer-btn').element)
    w.unmount()
  })

  it('hides itself on a server without the timer (404, pre-v0.33.0)', async () => {
    timerApi.get.mockRejectedValue({ response: { status: 404 } })
    const w = mountButton()
    await flushPromises()
    expect(w.find('.timer-btn').exists()).toBe(false)
    expect(useTimerStore().available).toBe(false)
    w.unmount()
  })

  it('stays visible after a transient error, so clicking can report it', async () => {
    timerApi.get.mockRejectedValue(new Error('Network Error'))
    const w = mountButton()
    await flushPromises()
    expect(w.find('.timer-btn').exists()).toBe(true)
    expect(useTimerStore().available).toBe(true)
    w.unmount()
  })

  it('renders nothing until the server has answered', async () => {
    let answer
    timerApi.get.mockReturnValue(new Promise(r => { answer = r }))
    const w = mountButton()
    await flushPromises()
    expect(w.find('.timer-btn').exists()).toBe(false)
    answer({ data: { running: false } })
    await flushPromises()
    expect(w.find('.timer-btn').exists()).toBe(true)
    w.unmount()
  })

  it('picks up a change made elsewhere (CLI, other tab)', async () => {
    const w = mountButton()
    await flushPromises()
    const store = useTimerStore()
    timerApi.get.mockResolvedValue({ data: {
      running: true, timer: { project: { name: 'Website' }, started_at: new Date().toISOString() },
    } })
    store.onChanged({ running: true, booked: 2 })
    await flushPromises()
    expect(store.running).toBe(true)
    expect(store.bookedVersion).toBe(1)
    expect(w.find('.timer-btn').classes()).toContain('running')
    w.unmount()
  })
})
