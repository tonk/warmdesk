<template>
  <section v-if="visible" class="itil-panel ci-panel" aria-labelledby="ticket-ci-heading">
    <h4 id="ticket-ci-heading">
      {{ $t('ticketCi.title') }}
      <span v-if="linked.length" class="count">({{ linked.length }})</span>
    </h4>

    <ul v-if="linked.length" class="ci-list" aria-labelledby="ticket-ci-heading">
      <li v-for="ci in linked" :key="ci.id" class="ci-row">
        <span class="ci-icon" aria-hidden="true">{{ ci.type?.icon || '▣' }}</span>
        <RouterLink :to="`/cmdb/${ci.id}`" class="ci-name">{{ ci.name }}</RouterLink>
        <span v-if="ci.type?.name" class="ci-type">{{ ci.type.name }}</span>
        <span v-if="ci.status" class="ci-chip" :class="'ci-status-' + ci.status">{{ $t('ticketCi.status_' + ci.status) }}</span>
        <span v-if="ci.environment" class="ci-chip">{{ $t('ticketCi.env_' + ci.environment) }}</span>
        <button
          type="button"
          class="btn-icon-xs"
          :title="$t('ticketCi.unlink')"
          :aria-label="$t('ticketCi.unlink') + ': ' + ci.name"
          :disabled="busy"
          @click="unlink(ci)"
        >✕</button>
      </li>
    </ul>
    <p v-else class="itil-hint">{{ loading ? $t('common.loading') : $t('ticketCi.none') }}</p>

    <div class="ci-search">
      <label for="ticket-ci-search" class="ci-search-label">{{ $t('ticketCi.search_label') }}</label>
      <input
        id="ticket-ci-search"
        v-model="searchQ"
        type="search"
        class="form-input form-input-sm"
        :placeholder="$t('ticketCi.search_placeholder')"
        autocomplete="off"
        @input="onSearchInput"
        @keydown.escape="clearSearch"
      />
      <p class="sr-only" role="status" aria-live="polite">{{ searchStatus }}</p>
      <ul v-if="searchResults.length" class="ci-list ci-results">
        <li v-for="ci in searchResults" :key="ci.id" class="ci-row">
          <span class="ci-icon" aria-hidden="true">{{ ci.type?.icon || '▣' }}</span>
          <span class="ci-name-plain">{{ ci.name }}</span>
          <span v-if="ci.type?.name" class="ci-type">{{ ci.type.name }}</span>
          <span v-if="ci.customer?.name" class="ci-type">{{ ci.customer.name }}</span>
          <button
            type="button"
            class="btn btn-secondary btn-xs"
            :aria-label="$t('ticketCi.link') + ': ' + ci.name"
            :disabled="busy"
            @click="link(ci)"
          >{{ $t('ticketCi.link') }}</button>
        </li>
      </ul>
      <p v-else-if="searchQ.trim() && searched && !searching" class="itil-hint">{{ $t('common.no_results') }}</p>
    </div>

    <div v-if="isChange && linked.length" class="impact" role="region" aria-labelledby="ticket-ci-impact-heading">
      <h5 id="ticket-ci-impact-heading">{{ $t('ticketCi.impact_title') }}</h5>
      <p v-if="impactLoading" class="itil-hint">{{ $t('common.loading') }}</p>
      <template v-else>
        <p class="itil-hint">
          {{ $t('ticketCi.impacted_n', { n: impacted.length }) }}
          <template v-if="impactHidden"> · {{ $t('ticketCi.hidden', { n: impactHidden }) }}</template>
        </p>
        <ul v-if="impacted.length" class="ci-list">
          <li v-for="e in impacted" :key="e.ci.id" class="ci-row">
            <span class="ci-icon" aria-hidden="true">{{ e.ci.type?.icon || '▣' }}</span>
            <RouterLink :to="`/cmdb/${e.ci.id}`" class="ci-name">{{ e.ci.name }}</RouterLink>
            <span v-if="e.ci.environment" class="ci-chip">{{ $t('ticketCi.env_' + e.ci.environment) }}</span>
            <span class="ci-tickets" :class="{ 'has-open': e.openTickets > 0 }">{{ $t('ticketCi.open_tickets', { n: e.openTickets }) }}</span>
          </li>
        </ul>
        <p v-else-if="!impactHidden" class="itil-hint">{{ $t('ticketCi.impact_none') }}</p>
      </template>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { cmdbApi } from '@/api/itsm'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'

const props = defineProps({
  ticket: { type: Object, required: true },
  customerId: { type: Number, default: null },
  isInbox: { type: Boolean, default: false },
})
defineEmits(['updated'])

const { t } = useI18n()
const ui = useUIStore()
const auth = useAuthStore()

const isAgent = computed(() => auth.user?.global_role !== 'customer')
const visible = computed(() => !props.isInbox && !!props.customerId && isAgent.value)
const isChange = computed(() => props.ticket?.type === 'change_request')

const linked = ref([])
const loading = ref(false)
const busy = ref(false)
const searchQ = ref('')
const searchRaw = ref([])
const searching = ref(false)
const searched = ref(false)
const impacted = ref([])
const impactHidden = ref(0)
const impactLoading = ref(false)
let searchTimer = null
let searchSeq = 0
let impactSeq = 0

const linkedIds = computed(() => new Set(linked.value.map(c => c.id)))
const searchResults = computed(() => searchRaw.value.filter(c => !linkedIds.value.has(c.id)).slice(0, 10))
const searchStatus = computed(() => {
  if (!searchQ.value.trim() || !searched.value || searching.value) return ''
  return searchResults.value.length
    ? t('ticketCi.results_n', { n: searchResults.value.length })
    : t('common.no_results')
})

async function load() {
  linked.value = []
  if (!visible.value || !props.ticket?.id) return
  loading.value = true
  try {
    const { data } = await cmdbApi.ticketCIs(props.customerId, props.ticket.id)
    linked.value = data || []
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticketCi.load_failed'))
  } finally {
    loading.value = false
  }
}

// Impact of a change: everything that depends (transitively) on one of the
// linked CIs, deduplicated across them and without the linked CIs themselves.
async function loadImpact() {
  impacted.value = []
  impactHidden.value = 0
  if (!visible.value || !isChange.value || !linked.value.length) { impactSeq++; impactLoading.value = false; return }
  const seq = ++impactSeq
  impactLoading.value = true
  const results = await Promise.allSettled(linked.value.map(ci => cmdbApi.impact(ci.id)))
  if (seq !== impactSeq) return
  const own = linkedIds.value
  const byId = new Map()
  let hidden = 0
  for (const r of results) {
    if (r.status !== 'fulfilled') continue
    hidden += r.value.data?.hidden || 0
    for (const e of r.value.data?.impacted || []) {
      if (!e?.ci || own.has(e.ci.id)) continue
      const prev = byId.get(e.ci.id)
      if (!prev || e.depth < prev.depth) {
        byId.set(e.ci.id, { ci: e.ci, depth: e.depth, openTickets: (e.open_tickets || []).length })
      }
    }
  }
  impacted.value = [...byId.values()].sort((a, b) => a.depth - b.depth || a.ci.name.localeCompare(b.ci.name))
  impactHidden.value = hidden
  impactLoading.value = false
}

watch(() => [props.ticket?.id, props.customerId, visible.value], load, { immediate: true })
watch(() => [linked.value.map(c => c.id).join(','), isChange.value], loadImpact)

function onSearchInput() {
  clearTimeout(searchTimer)
  const q = searchQ.value.trim()
  if (!q) { searchRaw.value = []; searched.value = false; return }
  searchTimer = setTimeout(() => runSearch(q), 300)
}

async function runSearch(q) {
  const seq = ++searchSeq
  searching.value = true
  try {
    const { data } = await cmdbApi.list({ q })
    if (seq === searchSeq) searchRaw.value = data || []
  } catch {
    if (seq === searchSeq) searchRaw.value = []
  } finally {
    if (seq === searchSeq) { searching.value = false; searched.value = true }
  }
}

function clearSearch() {
  clearTimeout(searchTimer)
  searchQ.value = ''
  searchRaw.value = []
  searched.value = false
}

async function link(ci) {
  busy.value = true
  try {
    await cmdbApi.linkCI(props.customerId, props.ticket.id, ci.id)
    if (!linkedIds.value.has(ci.id)) linked.value = [...linked.value, ci].sort((a, b) => a.name.localeCompare(b.name))
    ui.success(t('ticketCi.linked_ok'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticketCi.link_failed'))
  } finally {
    busy.value = false
  }
}

async function unlink(ci) {
  busy.value = true
  try {
    await cmdbApi.unlinkCI(props.customerId, props.ticket.id, ci.id)
    linked.value = linked.value.filter(c => c.id !== ci.id)
    ui.success(t('ticketCi.unlinked_ok'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticketCi.unlink_failed'))
  } finally {
    busy.value = false
  }
}

onBeforeUnmount(() => clearTimeout(searchTimer))
</script>

<style scoped>
.itil-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.itil-panel h4 { margin: 0 0 10px; font-size: 14px; display: flex; align-items: center; gap: 6px; }
.itil-panel h5 { margin: 14px 0 6px; font-size: 13px; }
.count { font-weight: 400; color: var(--color-text-muted); }
.itil-hint { font-size: 12px; color: var(--color-text-muted); margin: 4px 0 0; }
.ci-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
.ci-row { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 5px 8px; border-radius: 6px; font-size: 13px; }
.ci-row:hover { background: var(--color-bg-alt); }
.ci-icon { width: 18px; text-align: center; }
.ci-name { color: var(--color-primary); text-decoration: none; font-weight: 600; }
.ci-name:hover, .ci-name:focus-visible { text-decoration: underline; }
.ci-name-plain { font-weight: 600; }
.ci-type { font-size: 11px; color: var(--color-text-muted); }
.ci-chip { font-size: 10px; font-weight: 600; padding: 1px 6px; border-radius: 4px; border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg); }
.ci-status-active { border-color: var(--color-success); }
.ci-status-maintenance { border-color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 12%, transparent); }
.ci-status-retired { color: var(--color-text-muted); }
.ci-row .btn-icon-xs { margin-left: auto; }
.ci-tickets { margin-left: auto; font-size: 11px; color: var(--color-text-muted); }
.ci-tickets.has-open { color: var(--color-text); font-weight: 600; }
.ci-search { margin-top: 12px; display: flex; flex-direction: column; gap: 4px; }
.ci-search-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.ci-results { margin-top: 4px; border: 1px solid var(--color-border); border-radius: 6px; padding: 4px; background: var(--color-bg); }
.ci-results .btn { margin-left: auto; }
.impact { margin-top: 8px; }
.btn-icon-xs { background: none; border: none; cursor: pointer; color: var(--color-text-muted); padding: 2px; font-size: 12px; line-height: 1; }
.btn-icon-xs:hover, .btn-icon-xs:focus-visible { color: var(--color-danger); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); border: 0; }
</style>
