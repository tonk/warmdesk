import { createI18n } from 'vue-i18n'
import { createRouter, createMemoryHistory } from 'vue-router'
import en from '@/i18n/en.json'

// Interpolated strings the tests assert on, used until (and unless) en.json
// defines them; en.json wins once the panel keys are merged in.
const FALLBACK = {
  ticketChange: { conflict_cis: 'shared: {cis}' },
  ticketCi: { hidden: 'not visible to you: {n}' },
}

function merge(base, extra) {
  const out = { ...extra }
  for (const [k, v] of Object.entries(base)) {
    out[k] = v && typeof v === 'object' && extra[k] ? merge(v, extra[k]) : v
  }
  return out
}

export function makeI18n() {
  return createI18n({ legacy: false, locale: 'en', messages: { en: merge(en, FALLBACK) }, missingWarn: false, fallbackWarn: false })
}

export async function makeRouter(path = '/customers/1/tickets/5') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
  })
  router.push(path)
  await router.isReady()
  return router
}
