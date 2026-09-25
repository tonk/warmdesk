import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'

vi.mock('@/api/timer', () => ({
  timerApi: { get: vi.fn(), targets: vi.fn(), start: vi.fn(), stop: vi.fn(), cancel: vi.fn() },
}))
const invoke = vi.fn()
let trayListener = null
vi.mock('@tauri-apps/api/core', () => ({ invoke: (...a) => invoke(...a) }))
vi.mock('@tauri-apps/api/event', () => ({
  listen: vi.fn(async (name, fn) => { if (name === 'tray-timer') trayListener = fn; return () => {} }),
}))

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })
const t = i18n.global.t

describe('trayTimerMenu', () => {
  let trayTimerMenu
  beforeEach(async () => { ({ trayTimerMenu } = await import('../useTrayTimer')) })

  it('is empty when the timer is unavailable or not allowed', () => {
    expect(trayTimerMenu(t, { canUse: false, available: true })).toBeNull()
    expect(trayTimerMenu(t, { canUse: true, available: false })).toBeNull()
    expect(trayTimerMenu(t, { canUse: true, available: null })).toBeNull()
  })

  it('offers a one-click start of the last pick when idle', () => {
    const menu = trayTimerMenu(t, { canUse: true, available: true, running: false, lastPick: { projectId: 1, label: 'Support (Acme Corp)' } })
    expect(menu.status).toBe('No timer running')
    expect(menu.actions).toEqual([
      { id: 'start-last', label: 'Start: Support (Acme Corp)' },
      { id: 'start', label: 'Start timer…' },
    ])
  })

  it('offers stop and switch while running', () => {
    const menu = trayTimerMenu(t, {
      canUse: true, available: true, running: true,
      timer: { project: { name: 'Support' }, customer: { name: 'Acme Corp' }, started_at: new Date().toISOString() },
    })
    expect(menu.status).toMatch(/^⏱ Support \(Acme Corp\) — since /)
    expect(menu.actions.map(a => a.id)).toEqual(['stop', 'switch'])
  })
})

describe('useTrayTimer in the desktop app', () => {
  beforeEach(() => {
    vi.resetModules()
    window.__TAURI_INTERNALS__ = {}
    invoke.mockReset()
    trayListener = null
    localStorage.clear()
  })
  afterEach(() => { delete window.__TAURI_INTERNALS__ })

  it('keeps the tray menu in step and stops the timer from the tray', async () => {
    const { timerApi } = await import('@/api/timer')
    timerApi.get.mockResolvedValue({ data: {
      running: true,
      timer: { project: { name: 'Travel' }, customer: { name: 'Globex' }, started_at: new Date().toISOString() },
    } })
    timerApi.stop.mockResolvedValue({ data: [{ minutes: 30, project: { name: 'Travel' }, customer: { name: 'Globex' } }] })

    setActivePinia(createPinia())
    const { useAuthStore } = await import('@/stores/auth')
    const { useTimerStore } = await import('@/stores/timer')
    const { useUIStore } = await import('@/stores/ui')
    const { useTrayTimer } = await import('../useTrayTimer')
    useAuthStore().user = { id: 1, global_role: 'user', time_tracking_enabled: true }
    const timerStore = useTimerStore()

    const Host = defineComponent({ setup() { useTrayTimer(); return () => h('div') } })
    const w = mount(Host, { global: { plugins: [i18n] } })
    await flushPromises()
    expect(invoke).toHaveBeenLastCalledWith('set_tray_timer', { timer: null }) // server not asked yet

    await timerStore.refresh()
    await flushPromises()
    const running = invoke.mock.calls.at(-1)[1].timer
    expect(running.actions.map(a => a.id)).toEqual(['stop', 'switch'])

    expect(trayListener).toBeTypeOf('function')
    await trayListener({ payload: 'stop' })
    await flushPromises()
    expect(timerApi.stop).toHaveBeenCalled()
    expect(useUIStore().toasts.map(x => x.message)).toContain('Booked 30m on Travel (Globex)')
    expect(invoke.mock.calls.at(-1)[1].timer.status).toBe('No timer running')

    await trayListener({ payload: 'switch' })
    expect(timerStore.panelRequest).toEqual({ n: 1, switching: true })
    w.unmount()
  })
})
