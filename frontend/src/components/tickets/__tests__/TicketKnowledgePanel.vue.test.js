import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TicketKnowledgePanel from '../TicketKnowledgePanel.vue'
import { useAuthStore } from '@/stores/auth'
import { makeI18n, makeRouter } from './helpers'

vi.mock('@/api/itsm', () => ({
  kbApi: {
    ticketArticles: vi.fn(), suggest: vi.fn(), list: vi.fn(),
    linkArticle: vi.fn(), unlinkArticle: vi.fn(),
  },
}))
import { kbApi } from '@/api/itsm'

const ticket = { id: 5, title: 'VPN down', type: 'problem', root_cause: 'Expired cert', workaround: 'Use backup VPN', category_id: 3 }

async function mountPanel(props = {}, role = 'member') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { id: 1, global_role: role }
  const router = await makeRouter()
  return mount(TicketKnowledgePanel, {
    props: { ticket, customerId: 1, isInbox: false, ...props },
    global: { plugins: [pinia, makeI18n(), router] },
  })
}

describe('TicketKnowledgePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    kbApi.ticketArticles.mockResolvedValue({ data: [{ id: 1, title: 'Linked one', status: 'published' }] })
    kbApi.suggest.mockResolvedValue({ data: [{ id: 1, title: 'Linked one' }, { id: 2, title: 'Fix VPN', status: 'published' }] })
    kbApi.linkArticle.mockResolvedValue({ data: {} })
    kbApi.unlinkArticle.mockResolvedValue({ data: {} })
  })

  it('renders nothing for inbox tickets', async () => {
    const w = await mountPanel({ isInbox: true, customerId: null })
    await flushPromises()
    expect(w.find('section').exists()).toBe(false)
    expect(kbApi.ticketArticles).not.toHaveBeenCalled()
  })

  it('lists linked and suggested articles, excluding linked suggestions', async () => {
    const w = await mountPanel()
    await flushPromises()
    expect(kbApi.suggest).toHaveBeenCalledWith({ q: 'VPN down', customer_id: 1, limit: 5 })
    const links = w.findAll('a.kb-title').map(a => a.text())
    expect(links).toEqual(['Linked one', 'Fix VPN'])
  })

  it('prefills the create-article link from the problem record', async () => {
    const w = await mountPanel()
    await flushPromises()
    const href = decodeURIComponent(w.find('a.btn').attributes('href').replace(/\+/g, ' '))
    expect(href).toContain('/kb/new?')
    expect(href).toContain('ticket_id=5')
    expect(href).toContain('Expired cert')
    expect(href).toContain('Use backup VPN')
    expect(href).toContain('category_id=3')
    expect(href).toContain('return=/customers/1/tickets/5')
  })

  it('links a suggested article', async () => {
    const w = await mountPanel()
    await flushPromises()
    const linkBtn = w.findAll('button').find(b => b.attributes('aria-label')?.endsWith('Fix VPN'))
    await linkBtn.trigger('click')
    await flushPromises()
    expect(kbApi.linkArticle).toHaveBeenCalledWith(1, 5, 2)
    expect(w.findAll('ul')[0].text()).toContain('Fix VPN')
  })

  it('is read-only for customer-portal users', async () => {
    const w = await mountPanel({}, 'customer')
    await flushPromises()
    expect(w.findAll('button').length).toBe(0)
    expect(w.find('#ticket-kb-search').exists()).toBe(false)
    expect(w.find('a.btn').exists()).toBe(false)
  })
})
