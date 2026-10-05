import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TicketApprovalPanel from '../TicketApprovalPanel.vue'
import { useAuthStore } from '@/stores/auth'
import { makeI18n } from './helpers'

vi.mock('@/api/itsm', () => ({ approvalsApi: { forTicket: vi.fn(), decide: vi.fn(), request: vi.fn() } }))
vi.mock('@/api/tickets', () => ({ ticketsApi: { get: vi.fn() } }))
vi.mock('@/api/groups', () => ({ groupsApi: { list: vi.fn(), listCustomerGroups: vi.fn() } }))
import { approvalsApi } from '@/api/itsm'
import { ticketsApi } from '@/api/tickets'
import { groupsApi } from '@/api/groups'

function mountPanel(ticket, role = 'member') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { id: 1, global_role: role }
  return mount(TicketApprovalPanel, {
    props: { ticket, customerId: 1, isInbox: false },
    global: { plugins: [pinia, makeI18n()] },
  })
}

const pendingInfo = {
  approval_status: 'pending', approvals_required: 2, approval_round: 2, approved: 1, rejected: 0, can_decide: true,
  decisions: [
    { id: 3, round: 2, decision: 'approved', comment: 'ok', user: { username: 'ann' }, created_at: '2026-01-02T10:00:00Z' },
    { id: 1, round: 1, decision: 'rejected', comment: 'no', user: { username: 'bob' }, created_at: '2026-01-01T10:00:00Z' },
  ],
}

describe('TicketApprovalPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    groupsApi.listCustomerGroups.mockResolvedValue({ data: [{ group_id: 4, group: { name: 'Managers' } }] })
    groupsApi.list.mockResolvedValue({ data: [{ id: 4, name: 'Managers' }] })
  })

  it('shows rounds newest first and lets an approver decide', async () => {
    approvalsApi.forTicket.mockResolvedValue({ data: pendingInfo })
    approvalsApi.decide.mockResolvedValue({ data: {} })
    ticketsApi.get.mockResolvedValue({ data: { id: 5, approval_status: 'approved' } })
    const w = mountPanel({ id: 5, type: 'service_request', approval_status: 'pending', approval_round: 2 })
    await flushPromises()
    const rounds = w.findAll('.approval-round')
    expect(rounds).toHaveLength(2)
    expect(rounds[0].text()).toContain('ann')
    expect(rounds[1].text()).toContain('bob')
    await w.find('.approve-btn').trigger('click')
    await flushPromises()
    expect(approvalsApi.decide).toHaveBeenCalledWith(5, 'approved', '')
    expect(ticketsApi.get).toHaveBeenCalledWith(1, 5)
    expect(w.emitted('updated')[0][0]).toEqual({ id: 5, approval_status: 'approved' })
  })

  it('requires a comment to reject', async () => {
    approvalsApi.forTicket.mockResolvedValue({ data: pendingInfo })
    const w = mountPanel({ id: 5, type: 'service_request', approval_status: 'pending', approval_round: 2 })
    await flushPromises()
    await w.find('.reject-btn').trigger('click')
    await flushPromises()
    expect(approvalsApi.decide).not.toHaveBeenCalled()
    expect(w.find('[role="alert"]').exists()).toBe(true)
  })

  it('lets an agent request approval with a customer group', async () => {
    approvalsApi.request.mockResolvedValue({ data: { id: 5, approval_status: 'pending' } })
    const w = mountPanel({ id: 5, type: 'service_request', approval_status: '' })
    await flushPromises()
    expect(approvalsApi.forTicket).not.toHaveBeenCalled()
    await w.find('.request-btn').trigger('click')
    await flushPromises()
    await w.find('#ticket-approval-required').setValue(2)
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(approvalsApi.request).toHaveBeenCalledWith(1, 5, 4, 2)
    expect(w.emitted('updated')[0][0].approval_status).toBe('pending')
  })

  it('offers no request control for change requests or customers', async () => {
    let w = mountPanel({ id: 5, type: 'change_request', approval_status: '' })
    await flushPromises()
    expect(w.find('section').exists()).toBe(false)
    w = mountPanel({ id: 5, type: 'service_request', approval_status: '' }, 'customer')
    await flushPromises()
    expect(w.find('section').exists()).toBe(false)
    expect(groupsApi.listCustomerGroups).not.toHaveBeenCalled()
  })
})
