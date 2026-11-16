import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '@/api/client'
import { i18n } from '@/i18n'
import { useUIStore } from '@/stores/ui'

// Weekly contract hours: how much of each contract's hours_per_week has been
// booked in an ISO week, by all users together. The backend does the
// counting — it returns a contract_usage object with every time-entry save
// and serves GET /time-entries/contract-usage for the week grid's counters.
// This store keeps those numbers and raises the "limit exceeded" warning.

export function formatContractHours(minutes) {
  return (Math.round(minutes / 60 * 100) / 100).toLocaleString(i18n.global.locale.value, { maximumFractionDigits: 2 })
}

// Bulk actions (macros, fill-from-slots, paste) save many entries in a row;
// collect their warnings briefly so each contract is mentioned once.
const WARN_DEBOUNCE_MS = 400

export const useContractUsageStore = defineStore('contractUsage', () => {
  // `${contract_id}|${week_start}` → { contract_id, week_start, booked_minutes, limit_minutes, exceeded }
  const usage = ref({})
  // What the week grid currently shows, so a delete can refresh it.
  let tracked = { week: null, ids: [] }
  let pendingWarnings = new Map()
  let warnTimer = null

  function key(contractId, weekStart) { return `${contractId}|${weekStart}` }

  function get(contractId, weekStart) {
    return usage.value[key(contractId, weekStart)] || null
  }

  function apply(u) {
    if (!u) return
    usage.value = { ...usage.value, [key(u.contract_id, u.week_start)]: u }
  }

  async function load(weekStart, contractIds) {
    const ids = [...new Set(contractIds.filter(Boolean))].sort((a, b) => a - b)
    tracked = { week: weekStart, ids }
    if (!weekStart || !ids.length) return
    try {
      const { data } = await client.get('/time-entries/contract-usage', {
        params: { date: weekStart, contract_ids: ids.join(',') },
      })
      ;(data || []).forEach(apply)
    } catch {}
  }

  function refresh() {
    return load(tracked.week, tracked.ids)
  }

  function flushWarnings() {
    warnTimer = null
    const ui = useUIStore()
    for (const { u, name } of pendingWarnings.values()) {
      ui.warning(i18n.global.t('timeTracking.contract_hours_exceeded', {
        contract: name || '#' + u.contract_id,
        booked: formatContractHours(u.booked_minutes),
        limit: formatContractHours(u.limit_minutes),
      }))
    }
    pendingWarnings = new Map()
  }

  // Called with every saved time entry; warns on each booking while the
  // contract's week is over its limit.
  function onEntrySaved(entry) {
    const u = entry?.contract_usage
    if (!u) return
    apply(u)
    if (!u.exceeded) return
    pendingWarnings.set(key(u.contract_id, u.week_start), { u, name: entry.contract?.name })
    if (!warnTimer) warnTimer = setTimeout(flushWarnings, WARN_DEBOUNCE_MS)
  }

  return { usage, get, apply, load, refresh, onEntrySaved }
})
