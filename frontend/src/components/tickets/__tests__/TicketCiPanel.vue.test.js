import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TicketCiPanel from '../TicketCiPanel.vue'
import { useAuthStore } from '@/stores/auth'
import { makeI18n, makeRouter } from './helpers'

vi.mock('@/api/itsm', () => ({
  cmdbApi: { ticketCIs: vi.fn(), list: vi.fn(), impact: vi.fn(), linkCI: vi.fn(), unlinkCI: vi.fn() },
}))
import { cmdbApi } from '@/api/itsm'

const web = { id: 1, name: 'web01', status: 'active', environment: 'production', type: { name: 'Server', icon: '🖥' } }
const db = { id: 2, name: 'db01', status: 'active', type: { name: 'Database', icon: '🗄' } }

async function mountPanel(ticket = {}, role = 'member') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { id: 1, global_role: role }
  const router = await makeRouter()
  return mount(TicketCiPanel, {
    props: { ticket: { id: 5, type: 'incident', ...ticket }, customerId: 1, isInbox: false },
    global: { plugins: [pinia, makeI18n(), router] },
  })
}

describe('TicketCiPanel', () => {
  beforeEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    cmdbApi.ticketCIs.mockResolvedValue({ data: [web] })
    cmdbApi.linkCI.mockResolvedValue({ data: {} })
    cmdbApi.unlinkCI.mockResolvedValue({ data: {} })
  })

  it('is hidden for customers and inbox tickets', async () => {
    const w = await mountPanel({}, 'customer')
    await flushPromises()
    expect(w.find('section').exists()).toBe(false)
    expect(cmdbApi.ticketCIs).not.toHaveBeenCalled()
  })

  it('lists linked CIs with links and unlinks them', async () => {
    const w = await mountPanel()
    await flushPromises()
    const a = w.find('a.ci-name')
    expect(a.text()).toBe('web01')
    expect(a.attributes('href')).toBe('/cmdb/1')
    await w.find('button.btn-icon-xs').trigger('click')
    await flushPromises()
    expect(cmdbApi.unlinkCI).toHaveBeenCalledWith(1, 5, 1)
    expect(w.find('a.ci-name').exists()).toBe(false)
  })

  it('searches (debounced), excludes linked CIs and links a result', async () => {
    vi.useFakeTimers()
    cmdbApi.list.mockResolvedValue({ data: [web, db] })
    const w = await mountPanel()
    await flushPromises()
    await w.find('#ticket-ci-search').setValue('01')
    expect(cmdbApi.list).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()
    expect(cmdbApi.list).toHaveBeenCalledWith({ q: '01' })
    const results = w.findAll('.ci-results li')
    expect(results).toHaveLength(1)
    await results[0].find('button').trigger('click')
    await flushPromises()
    expect(cmdbApi.linkCI).toHaveBeenCalledWith(1, 5, 2)
    vi.useRealTimers()
  })

  it('summarises the impact of a change, deduplicated', async () => {
    cmdbApi.ticketCIs.mockResolvedValue({ data: [web, db] })
    const app = { id: 3, name: 'shop', type: { icon: '🛒' } }
    cmdbApi.impact.mockImplementation((id) => Promise.resolve({ data: id === 1
      ? { impacted: [{ ci: app, depth: 1, open_tickets: [{ id: 11 }] }], hidden: 1 }
      : { impacted: [{ ci: app, depth: 2, open_tickets: [{ id: 11 }] }, { ci: web, depth: 1, open_tickets: [] }], hidden: 2 } }))
    const w = await mountPanel({ type: 'change_request' })
    await flushPromises()
    expect(cmdbApi.impact).toHaveBeenCalledTimes(2)
    const rows = w.findAll('.impact li')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('shop')
    expect(w.find('.impact').text()).toContain('3')
  })
})
