import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TicketChangePanel from '../TicketChangePanel.vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { makeI18n, makeRouter } from './helpers'

vi.mock('@/api/itsm', () => ({ changesApi: { check: vi.fn() } }))
vi.mock('@/api/tickets', () => ({ ticketsApi: { update: vi.fn() } }))
import { changesApi } from '@/api/itsm'
import { ticketsApi } from '@/api/tickets'

const base = { id: 5, type: 'change_request', change_type: 'normal', change_status: 'draft', risk: 'low' }

async function mountPanel(ticket, role = 'member') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { id: 1, global_role: role }
  const router = await makeRouter()
  return mount(TicketChangePanel, {
    props: { ticket: { ...base, ...ticket }, customerId: 1, isInbox: false },
    global: { plugins: [pinia, makeI18n(), router] },
  })
}

const actionKeys = (w) => w.findAll('[data-action]').map(b => b.attributes('data-action'))

describe('TicketChangePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    changesApi.check.mockResolvedValue({ data: { freezes: [], conflicts: [] } })
  })

  it('renders only for change requests seen by agents', async () => {
    let w = await mountPanel({ type: 'incident' })
    expect(w.find('section').exists()).toBe(false)
    w = await mountPanel({}, 'customer')
    expect(w.find('section').exists()).toBe(false)
  })

  it('offers only the valid next actions per status', async () => {
    const cases = {
      draft: ['submit', 'cancel'],
      awaiting_approval: ['withdraw', 'cancel'],
      approved: ['schedule', 'back_to_draft', 'cancel'],
      scheduled: ['start', 'unschedule', 'cancel'],
      implementing: ['review', 'fail'],
      review: ['complete', 'fail'],
      completed: [],
    }
    for (const [status, keys] of Object.entries(cases)) {
      const w = await mountPanel({ change_status: status })
      expect(actionKeys(w)).toEqual(keys)
    }
  })

  it('submits a transition and emits the updated ticket', async () => {
    ticketsApi.update.mockResolvedValue({ data: { ...base, change_status: 'awaiting_approval' } })
    const w = await mountPanel({})
    await w.find('[data-action="submit"]').trigger('click')
    await flushPromises()
    expect(ticketsApi.update).toHaveBeenCalledWith(1, 5, { change_status: 'awaiting_approval' })
    expect(w.emitted('updated')[0][0].change_status).toBe('awaiting_approval')
  })

  it('shows a translated error for a rejected transition', async () => {
    ticketsApi.update.mockRejectedValue({ response: { status: 400, data: { error: 'x', code: 'no_cab_group' } } })
    const w = await mountPanel({})
    await w.find('[data-action="submit"]').trigger('click')
    await flushPromises()
    const alert = w.find('[role="alert"]')
    expect(alert.exists()).toBe(true)
    expect(alert.text()).not.toBe('x')
  })

  it('completes a reviewed change with outcome and notes', async () => {
    ticketsApi.update.mockResolvedValue({ data: { ...base, change_status: 'completed' } })
    const w = await mountPanel({ change_status: 'review' })
    expect(w.find('[data-action="complete"]').attributes('disabled')).toBeDefined()
    await w.find('#change-outcome').setValue('successful')
    await w.find('#change-review-notes').setValue('went fine')
    await w.find('[data-action="complete"]').trigger('click')
    await flushPromises()
    expect(ticketsApi.update).toHaveBeenLastCalledWith(1, 5, { change_status: 'completed', change_outcome: 'successful', review_notes: 'went fine' })
  })

  it('confirms before cancelling', async () => {
    const w = await mountPanel({})
    const ui = useUIStore()
    ui.confirm = vi.fn().mockResolvedValue(false)
    await w.find('[data-action="cancel"]').trigger('click')
    await flushPromises()
    expect(ui.confirm).toHaveBeenCalled()
    expect(ticketsApi.update).not.toHaveBeenCalled()
  })

  it('sends planned times as RFC 3339 and null to clear', async () => {
    ticketsApi.update.mockResolvedValue({ data: base })
    const w = await mountPanel({})
    await w.find('#change-planned-start').setValue('2026-11-01T22:00')
    await flushPromises()
    expect(ticketsApi.update).toHaveBeenLastCalledWith(1, 5, { planned_start: new Date('2026-11-01T22:00').toISOString() })
    await w.find('#change-planned-start').setValue('')
    await flushPromises()
    expect(ticketsApi.update).toHaveBeenLastCalledWith(1, 5, { planned_start: null })
  })

  it('warns about freezes and conflicting changes', async () => {
    changesApi.check.mockResolvedValue({ data: {
      freezes: [{ id: 1, name: 'Year end', start_at: '2026-12-20T00:00:00Z', end_at: '2027-01-02T00:00:00Z' }],
      conflicts: [{ ticket_id: 9, customer_id: 1, title: 'DB upgrade', planned_start: '2026-12-21T00:00:00Z', planned_end: '2026-12-21T02:00:00Z', cis: ['db01'] }],
    } })
    const w = await mountPanel({ planned_start: '2026-12-21T00:00:00Z', planned_end: '2026-12-21T03:00:00Z' })
    await flushPromises()
    expect(changesApi.check).toHaveBeenCalledWith(1, 5)
    expect(w.text()).toContain('Year end')
    const link = w.findAll('a').find(a => a.text().includes('DB upgrade'))
    expect(link.attributes('href')).toBe('/customers/1/tickets/9')
    expect(w.text()).toContain('db01')
  })
})
