import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TicketRequestPanel from '../TicketRequestPanel.vue'
import { makeI18n } from './helpers'

vi.mock('@/api/itsm', () => ({ catalogApi: { list: vi.fn() } }))
import { catalogApi } from '@/api/itsm'

function mountPanel(ticket) {
  const pinia = createPinia()
  setActivePinia(pinia)
  return mount(TicketRequestPanel, {
    props: { ticket, customerId: 1, isInbox: false },
    global: { plugins: [pinia, makeI18n()] },
  })
}

describe('TicketRequestPanel', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders nothing without catalogue data', async () => {
    const w = mountPanel({ id: 1 })
    await flushPromises()
    expect(w.find('section').exists()).toBe(false)
    expect(catalogApi.list).not.toHaveBeenCalled()
  })

  it('labels answers in field order and formats booleans', async () => {
    catalogApi.list.mockResolvedValue({ data: [{ id: 7, name: 'New laptop', fields: [
      { key: 'model', label: 'Model', type: 'select' },
      { key: 'rush', label: 'Urgent delivery', type: 'checkbox' },
    ] }] })
    const w = mountPanel({ id: 1, catalog_item_id: 7, request_data: { rush: true, model: 'X1', extra: 'kept' } })
    await flushPromises()
    expect(catalogApi.list).toHaveBeenCalledWith({ customer_id: 1 })
    expect(w.findAll('dt').map(d => d.text())).toEqual(['Model', 'Urgent delivery', 'extra'])
    expect(w.findAll('dd').map(d => d.text())).toEqual(['X1', 'Yes', 'kept'])
  })

  it('falls back to raw keys when the item is gone', async () => {
    catalogApi.list.mockResolvedValue({ data: [] })
    const w = mountPanel({ id: 1, catalog_item_id: 9, request_data: { model: 'X1' } })
    await flushPromises()
    expect(w.findAll('dt').map(d => d.text())).toEqual(['model'])
  })
})
