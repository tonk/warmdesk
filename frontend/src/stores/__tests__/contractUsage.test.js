import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({ default: { get: vi.fn() } }))

import client from '@/api/client'
import { useContractUsageStore } from '@/stores/contractUsage'
import { useUIStore } from '@/stores/ui'

const usage = (over = {}) => ({
  contract_id: 7, week_start: '2026-11-16', booked_minutes: 660, limit_minutes: 600, exceeded: true, ...over,
})

describe('contractUsage store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    client.get.mockReset()
  })
  afterEach(() => vi.useRealTimers())

  it('warns once per contract for a burst of saves over the limit', () => {
    const store = useContractUsageStore()
    const ui = useUIStore()
    const warn = vi.spyOn(ui, 'warning')
    store.onEntrySaved({ contract: { name: 'Retainer' }, contract_usage: usage({ booked_minutes: 630 }) })
    store.onEntrySaved({ contract: { name: 'Retainer' }, contract_usage: usage() })
    vi.runAllTimers()
    expect(warn).toHaveBeenCalledTimes(1)
    expect(warn.mock.calls[0][0]).toContain('Retainer')
    expect(warn.mock.calls[0][0]).toContain('11')
    expect(store.get(7, '2026-11-16').booked_minutes).toBe(660)
  })

  it('warns again on each later booking while over the limit', () => {
    const store = useContractUsageStore()
    const warn = vi.spyOn(useUIStore(), 'warning')
    store.onEntrySaved({ contract_usage: usage() })
    vi.runAllTimers()
    store.onEntrySaved({ contract_usage: usage({ booked_minutes: 720 }) })
    vi.runAllTimers()
    expect(warn).toHaveBeenCalledTimes(2)
  })

  it('stays silent under the limit and for entries without usage', () => {
    const store = useContractUsageStore()
    const warn = vi.spyOn(useUIStore(), 'warning')
    store.onEntrySaved({ contract_usage: usage({ booked_minutes: 540, exceeded: false }) })
    store.onEntrySaved({ id: 1 })
    vi.runAllTimers()
    expect(warn).not.toHaveBeenCalled()
    expect(store.get(7, '2026-11-16').exceeded).toBe(false)
  })

  it('loads the grid counters and refreshes them for the same week', async () => {
    client.get.mockResolvedValue({ data: [usage()] })
    const store = useContractUsageStore()
    await store.load('2026-11-16', [9, 7, 7, null])
    expect(client.get).toHaveBeenCalledWith('/time-entries/contract-usage', {
      params: { date: '2026-11-16', contract_ids: '7,9' },
    })
    expect(store.get(7, '2026-11-16').limit_minutes).toBe(600)
    await store.refresh()
    expect(client.get).toHaveBeenCalledTimes(2)
  })
})
