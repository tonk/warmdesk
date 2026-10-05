<template>
  <section v-if="!isInbox && customerId" class="itil-panel kb-panel" aria-labelledby="ticket-kb-heading">
    <div class="panel-head">
      <h4 id="ticket-kb-heading">{{ $t('ticketKb.title') }}</h4>
      <RouterLink v-if="isAgent" :to="createLink" class="btn btn-secondary btn-sm">{{ $t('ticketKb.create_from_ticket') }}</RouterLink>
    </div>

    <h5 id="ticket-kb-linked-heading">
      {{ $t('ticketKb.linked') }}
      <span v-if="linked.length" class="count">({{ linked.length }})</span>
    </h5>
    <ul v-if="linked.length" class="kb-list" aria-labelledby="ticket-kb-linked-heading">
      <li v-for="a in linked" :key="a.id" class="kb-row">
        <span class="kb-icon" aria-hidden="true">📄</span>
        <RouterLink :to="`/kb/${a.id}`" class="kb-title">{{ a.title }}</RouterLink>
        <span v-if="isAgent && a.status && a.status !== 'published'" class="kb-badge">{{ $t('ticketKb.status_' + a.status) }}</span>
        <span v-if="isAgent && a.visibility === 'internal'" class="kb-badge">{{ $t('ticketKb.internal') }}</span>
        <button
          v-if="isAgent"
          type="button"
          class="btn-icon-xs"
          :title="$t('ticketKb.unlink')"
          :aria-label="$t('ticketKb.unlink') + ': ' + a.title"
          :disabled="busy"
          @click="unlink(a)"
        >✕</button>
        <p v-if="a.summary" class="kb-summary">{{ a.summary }}</p>
      </li>
    </ul>
    <p v-else class="itil-hint kb-empty">{{ $t('ticketKb.no_linked') }}</p>

    <h5 id="ticket-kb-suggested-heading">{{ $t('ticketKb.suggested') }}</h5>
    <ul v-if="suggestions.length" class="kb-list" aria-labelledby="ticket-kb-suggested-heading">
      <li v-for="a in suggestions" :key="a.id" class="kb-row">
        <span class="kb-icon" aria-hidden="true">💡</span>
        <RouterLink :to="`/kb/${a.id}`" class="kb-title">{{ a.title }}</RouterLink>
        <span v-if="isAgent && a.status && a.status !== 'published'" class="kb-badge">{{ $t('ticketKb.status_' + a.status) }}</span>
        <button
          v-if="isAgent"
          type="button"
          class="btn btn-ghost btn-xs"
          :aria-label="$t('ticketKb.link') + ': ' + a.title"
          :disabled="busy"
          @click="link(a)"
        >{{ $t('ticketKb.link') }}</button>
        <p v-if="a.summary" class="kb-summary">{{ a.summary }}</p>
      </li>
    </ul>
    <p v-else class="itil-hint kb-empty">{{ loading ? $t('common.loading') : $t('ticketKb.no_suggestions') }}</p>

    <div v-if="isAgent" class="kb-search">
      <label for="ticket-kb-search" class="kb-search-label">{{ $t('ticketKb.search_label') }}</label>
      <input
        id="ticket-kb-search"
        v-model="searchQ"
        type="search"
        class="form-input form-input-sm"
        :placeholder="$t('ticketKb.search_placeholder')"
        autocomplete="off"
        @input="onSearchInput"
        @keydown.escape="clearSearch"
      />
      <p class="sr-only" role="status" aria-live="polite">{{ searchStatus }}</p>
      <ul v-if="searchResults.length" class="kb-list kb-results">
        <li v-for="a in searchResults" :key="a.id" class="kb-row">
          <RouterLink :to="`/kb/${a.id}`" class="kb-title">{{ a.title }}</RouterLink>
          <span v-if="a.status && a.status !== 'published'" class="kb-badge">{{ $t('ticketKb.status_' + a.status) }}</span>
          <span v-if="a.visibility === 'internal'" class="kb-badge">{{ $t('ticketKb.internal') }}</span>
          <button
            type="button"
            class="btn btn-secondary btn-xs"
            :aria-label="$t('ticketKb.link') + ': ' + a.title"
            :disabled="busy"
            @click="link(a)"
          >{{ $t('ticketKb.link') }}</button>
        </li>
      </ul>
      <p v-else-if="searchQ.trim() && searched && !searching" class="itil-hint">{{ $t('common.no_results') }}</p>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { kbApi } from '@/api/itsm'
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
const route = useRoute()

const isAgent = computed(() => auth.user?.global_role !== 'customer')
const linked = ref([])
const suggested = ref([])
const loading = ref(false)
const busy = ref(false)
const searchQ = ref('')
const searchRaw = ref([])
const searching = ref(false)
const searched = ref(false)
let searchTimer = null
let searchSeq = 0

const linkedIds = computed(() => new Set(linked.value.map(a => a.id)))
const suggestions = computed(() => suggested.value.filter(a => !linkedIds.value.has(a.id)))
const searchResults = computed(() => searchRaw.value.filter(a => !linkedIds.value.has(a.id)).slice(0, 10))
const searchStatus = computed(() => {
  if (!searchQ.value.trim() || !searched.value || searching.value) return ''
  return searchResults.value.length
    ? t('ticketKb.results_n', { n: searchResults.value.length })
    : t('common.no_results')
})

// Pre-fills the article editor with what this ticket taught us: the
// resolution notes, or for a problem its root cause and workaround.
function articleBody(tk) {
  if (tk.type === 'problem' && (tk.root_cause || tk.workaround)) {
    const parts = []
    if (tk.root_cause) parts.push(`## ${t('ticket.root_cause')}\n\n${tk.root_cause}`)
    if (tk.workaround) parts.push(`## ${t('ticket.workaround')}\n\n${tk.workaround}`)
    return parts.join('\n\n')
  }
  return tk.resolution_notes || ''
}

const createLink = computed(() => {
  const tk = props.ticket
  const query = { ticket_id: String(tk.id), title: tk.title || '' }
  const body = articleBody(tk)
  if (body) query.body = body
  if (tk.category_id) query.category_id = String(tk.category_id)
  if (props.customerId) query.customer_id = String(props.customerId)
  if (route?.path) query.return = route.path
  return { path: '/kb/new', query }
})

async function load() {
  if (props.isInbox || !props.customerId || !props.ticket?.id) return
  loading.value = true
  const [lk, sg] = await Promise.allSettled([
    kbApi.ticketArticles(props.customerId, props.ticket.id),
    props.ticket.title
      ? kbApi.suggest({ q: props.ticket.title, customer_id: props.customerId, limit: 5 })
      : Promise.resolve({ data: [] }),
  ])
  linked.value = lk.status === 'fulfilled' ? (lk.value.data || []) : []
  suggested.value = sg.status === 'fulfilled' ? (sg.value.data || []) : []
  loading.value = false
}

watch(() => [props.ticket?.id, props.customerId, props.isInbox], load, { immediate: true })
watch(() => props.ticket?.title, async (title, old) => {
  if (!title || title === old || props.isInbox || !props.customerId) return
  try {
    const { data } = await kbApi.suggest({ q: title, customer_id: props.customerId, limit: 5 })
    suggested.value = data || []
  } catch { /* keep the previous suggestions */ }
})

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
    const { data } = await kbApi.list({ q })
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

async function link(a) {
  busy.value = true
  try {
    await kbApi.linkArticle(props.customerId, props.ticket.id, a.id)
    if (!linkedIds.value.has(a.id)) linked.value = [...linked.value, a].sort((x, y) => x.title.localeCompare(y.title))
    ui.success(t('ticketKb.linked_ok'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticketKb.link_failed'))
  } finally {
    busy.value = false
  }
}

async function unlink(a) {
  busy.value = true
  try {
    await kbApi.unlinkArticle(props.customerId, props.ticket.id, a.id)
    linked.value = linked.value.filter(x => x.id !== a.id)
    ui.success(t('ticketKb.unlinked_ok'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticketKb.unlink_failed'))
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
.itil-panel h4 { margin: 0; font-size: 14px; }
.itil-panel h5 { margin: 14px 0 6px; font-size: 13px; display: flex; align-items: center; gap: 6px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.count { font-weight: 400; color: var(--color-text-muted); }
.itil-hint { font-size: 12px; color: var(--color-text-muted); margin: 4px 0 0; }
.kb-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
.kb-row { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 5px 8px; border-radius: 6px; font-size: 13px; }
.kb-row:hover { background: var(--color-bg-alt); }
.kb-icon { font-size: 12px; }
.kb-title { flex: 1; min-width: 0; color: var(--color-primary); text-decoration: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.kb-title:hover, .kb-title:focus-visible { text-decoration: underline; }
.kb-summary { flex-basis: 100%; margin: 0 0 0 20px; font-size: 12px; color: var(--color-text-muted); }
.kb-badge { font-size: 10px; font-weight: 600; padding: 1px 6px; border-radius: 4px; border: 1px solid var(--color-border); color: var(--color-text-muted); background: var(--color-bg); }
.kb-search { margin-top: 14px; display: flex; flex-direction: column; gap: 4px; }
.kb-search-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.kb-results { margin-top: 4px; border: 1px solid var(--color-border); border-radius: 6px; padding: 4px; background: var(--color-bg); }
.btn-icon-xs { background: none; border: none; cursor: pointer; color: var(--color-text-muted); padding: 2px; font-size: 12px; line-height: 1; }
.btn-icon-xs:hover, .btn-icon-xs:focus-visible { color: var(--color-danger); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); border: 0; }
</style>
