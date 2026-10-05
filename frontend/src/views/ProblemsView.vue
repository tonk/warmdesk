<template>
  <main class="pb-main">
    <header class="pb-header">
      <h1 class="pb-title">{{ $t('problems.title') }}</h1>
      <p class="pb-subtitle">{{ $t('problems.subtitle') }}</p>
    </header>

    <div class="pb-toolbar" role="search">
      <label class="sr-only" for="pb-search">{{ $t('problems.search_label') }}</label>
      <input
        id="pb-search"
        v-model="query"
        type="search"
        class="form-input pb-search"
        :placeholder="$t('problems.search_placeholder')"
      />
      <label class="pb-toggle">
        <input type="checkbox" v-model="knownErrorOnly" />
        {{ $t('problems.known_errors_only') }}
      </label>
      <label class="pb-toggle">
        <input type="checkbox" v-model="includeResolved" />
        {{ $t('problems.include_resolved') }}
      </label>
    </div>

    <div v-if="forbidden" class="pb-empty" role="alert">{{ $t('problems.forbidden') }}</div>

    <div v-else-if="loading && !problems.length" class="pb-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <template v-else>
      <p class="sr-only" aria-live="polite">{{ $t('problems.result_count', { n: problems.length }, problems.length) }}</p>
      <div v-if="!problems.length" class="pb-empty">{{ $t('problems.none') }}</div>

      <ul v-else class="pb-list">
        <li v-for="p in problems" :key="p.id" class="pb-item" :class="{ 'pb-item-open': expanded.has(p.id) }">
          <div class="pb-row">
            <span class="pb-id">#{{ p.id }}</span>
            <RouterLink :to="ticketLink(p)" class="pb-link">{{ p.title }}</RouterLink>
            <span v-if="p.is_known_error" class="pb-badge pb-badge-ke">{{ $t('ticket.known_error') }}</span>
            <span class="pb-badge" :class="`pb-ps-${p.problem_status || 'none'}`">{{ problemStatusLabel(p.problem_status) }}</span>
            <span class="pb-meta">{{ statusLabel(p.status) }}</span>
            <span class="pb-meta" v-if="p.priority">{{ priorityLabel(p.priority) }}</span>
            <span class="pb-meta">{{ p.customer_name || $t('problems.inbox') }}</span>
            <span class="pb-meta pb-incidents" :title="$t('problems.linked_incidents')">
              {{ $t('problems.linked_count', { n: p.linked_incidents || 0 }, p.linked_incidents || 0) }}
            </span>
            <span class="pb-meta pb-updated">{{ formatDate(p.updated_at) }}</span>
            <button
              type="button"
              class="btn btn-secondary btn-sm pb-expand"
              :aria-expanded="expanded.has(p.id)"
              :aria-controls="`pb-detail-${p.id}`"
              @click="toggle(p.id)"
            >
              <span aria-hidden="true" class="pb-chevron" :class="{ open: expanded.has(p.id) }">›</span>
              {{ expanded.has(p.id) ? $t('problems.hide_details') : $t('problems.show_details') }}
              <span class="sr-only">— {{ p.title }}</span>
            </button>
          </div>
          <div v-show="expanded.has(p.id)" :id="`pb-detail-${p.id}`" class="pb-detail">
            <div class="pb-detail-block">
              <h2 class="pb-detail-title">{{ $t('ticket.root_cause') }}</h2>
              <p v-if="p.root_cause" class="pb-detail-text">{{ p.root_cause }}</p>
              <p v-else class="pb-detail-empty">{{ $t('problems.not_recorded') }}</p>
            </div>
            <div class="pb-detail-block">
              <h2 class="pb-detail-title">{{ $t('ticket.workaround') }}</h2>
              <p v-if="p.workaround" class="pb-detail-text">{{ p.workaround }}</p>
              <p v-else class="pb-detail-empty">{{ $t('problems.not_recorded') }}</p>
            </div>
          </div>
        </li>
      </ul>
    </template>
  </main>
</template>

<script setup>
import { ref, reactive, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { helpdeskApi } from '@/api/helpdesk'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const { t, te } = useI18n()
const { formatDate } = useDateFormat()
const ui = useUIStore()

const problems = ref([])
const loading = ref(false)
const forbidden = ref(false)
const query = ref('')
const knownErrorOnly = ref(false)
const includeResolved = ref(false)
const expanded = reactive(new Set())

let requestSeq = 0
async function load() {
  const seq = ++requestSeq
  loading.value = true
  forbidden.value = false
  try {
    const params = {}
    if (query.value.trim()) params.q = query.value.trim()
    if (knownErrorOnly.value) params.known_error = true
    if (includeResolved.value) params.include_resolved = true
    const { data } = await helpdeskApi.listProblems(params)
    if (seq !== requestSeq) return
    problems.value = data || []
  } catch (e) {
    if (seq !== requestSeq) return
    if (e?.response?.status === 403) forbidden.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

let debounceTimer = null
watch(query, () => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(load, 300)
})
watch([knownErrorOnly, includeResolved], load)
onUnmounted(() => clearTimeout(debounceTimer))

function toggle(id) {
  if (expanded.has(id)) expanded.delete(id)
  else expanded.add(id)
}

function ticketLink(p) {
  return p.customer_id ? `/customers/${p.customer_id}/tickets/${p.id}` : `/tickets/inbox/${p.id}`
}

function labelOr(key, fallback) {
  return te(key) ? t(key) : fallback
}
function problemStatusLabel(s) {
  return s ? labelOr(`ticket.problem_status_${s}`, s) : t('problems.status_unset')
}
function statusLabel(s) { return labelOr(`ticket.status_${s}`, s) }
function priorityLabel(p) { return labelOr(`ticket.priority_${p}`, p) }

onMounted(load)
</script>

<style scoped>
.pb-main { padding: 24px 32px; max-width: 1200px; margin: 0 auto; }
.pb-header { margin-bottom: 16px; }
.pb-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.pb-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.pb-toolbar { display: flex; flex-wrap: wrap; gap: 16px; align-items: center; margin-bottom: 16px; }
.pb-search { flex: 1 1 260px; max-width: 420px; }
.pb-toggle { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }

.pb-loading { display: flex; justify-content: center; padding: 48px; }
.pb-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.pb-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.pb-item {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
}
.pb-item-open { border-color: var(--color-primary); }
.pb-row { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; padding: 10px 14px; }
.pb-id { font-size: 11px; font-weight: 700; color: var(--color-text-muted); }
.pb-link { font-weight: 600; color: var(--color-text); text-decoration: none; flex: 1 1 240px; min-width: 0; }
.pb-link:hover { color: var(--color-primary); text-decoration: underline; }
.pb-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.pb-meta { font-size: 12px; color: var(--color-text-muted); white-space: nowrap; }
.pb-incidents { font-weight: 600; }

.pb-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
  white-space: nowrap;
}
.pb-badge-ke { border-color: var(--color-warning); color: var(--color-text); }
.pb-badge-ke::before { content: '⚠ '; }
.pb-ps-investigating { border-color: var(--color-primary); }
.pb-ps-root_cause_identified { border-color: var(--color-primary); }
.pb-ps-known_error { border-color: var(--color-warning); }
.pb-ps-resolved { border-color: var(--color-success); }

.pb-expand { display: inline-flex; align-items: center; gap: 4px; }
.pb-chevron { display: inline-block; transition: transform .15s; }
.pb-chevron.open { transform: rotate(90deg); }

.pb-detail {
  border-top: 1px solid var(--color-border);
  padding: 12px 14px;
  display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 16px;
}
.pb-detail-title { font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; color: var(--color-text-muted); margin: 0 0 4px; }
.pb-detail-text { margin: 0; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
.pb-detail-empty { margin: 0; font-size: 13px; color: var(--color-text-muted); font-style: italic; }

@media (max-width: 640px) {
  .pb-main { padding: 16px; }
}
</style>
