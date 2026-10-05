<template>
  <main class="kb-main">
    <header class="kb-header">
      <div>
        <h1 class="kb-title">{{ $t('kb.title') }}</h1>
        <p class="kb-subtitle">{{ isCustomer ? $t('kb.subtitle_customer') : $t('kb.subtitle') }}</p>
      </div>
      <RouterLink v-if="!isCustomer" :to="{ name: 'kb-new' }" class="btn btn-primary">
        <span aria-hidden="true">+</span> {{ $t('kb.new_article') }}
      </RouterLink>
    </header>

    <div class="kb-toolbar" role="search">
      <label class="sr-only" for="kb-search">{{ $t('kb.search_label') }}</label>
      <input
        id="kb-search"
        v-model="query"
        type="search"
        class="form-input kb-search"
        :placeholder="$t('kb.search_placeholder')"
      />

      <label class="sr-only" for="kb-filter-category">{{ $t('ticket.category') }}</label>
      <select id="kb-filter-category" v-model="categoryId" class="form-input kb-select">
        <option value="">{{ $t('kb.all_categories') }}</option>
        <option v-for="c in categories" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
      </select>

      <template v-if="!isCustomer">
        <label class="sr-only" for="kb-filter-status">{{ $t('common.status') }}</label>
        <select id="kb-filter-status" v-model="status" class="form-input kb-select">
          <option value="">{{ $t('kb.all_statuses') }}</option>
          <option value="draft">{{ $t('kb.status_draft') }}</option>
          <option value="published">{{ $t('kb.status_published') }}</option>
          <option value="archived">{{ $t('kb.status_archived') }}</option>
        </select>

        <label class="sr-only" for="kb-filter-visibility">{{ $t('kb.visibility') }}</label>
        <select id="kb-filter-visibility" v-model="visibility" class="form-input kb-select">
          <option value="">{{ $t('kb.all_visibilities') }}</option>
          <option value="internal">{{ $t('kb.visibility_internal') }}</option>
          <option value="public">{{ $t('kb.visibility_public') }}</option>
        </select>

        <label class="kb-toggle">
          <input type="checkbox" v-model="includeArchived" :disabled="status === 'archived'" />
          {{ $t('kb.include_archived') }}
        </label>
      </template>
    </div>

    <div v-if="loading && !articles.length" class="kb-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <template v-else>
      <p class="sr-only" aria-live="polite">{{ $t('kb.result_count', { n: articles.length }, articles.length) }}</p>
      <div v-if="!articles.length" class="kb-empty">{{ query.trim() ? $t('kb.none_found') : $t('kb.none') }}</div>

      <ul v-else class="kb-list">
        <li v-for="a in articles" :key="a.id" class="kb-item">
          <div class="kb-row">
            <RouterLink :to="{ name: 'kb-article', params: { id: a.id } }" class="kb-link">{{ a.title }}</RouterLink>
            <template v-if="!isCustomer">
              <span class="kb-badge" :class="`kb-st-${a.status}`">{{ statusLabel(a.status) }}</span>
              <span class="kb-badge" :class="`kb-vis-${a.visibility}`">{{ visibilityLabel(a.visibility) }}</span>
            </template>
            <span v-if="a.category" class="kb-badge kb-badge-cat">{{ a.category.name }}</span>
          </div>
          <p v-if="a.summary" class="kb-summary">{{ a.summary }}</p>
          <div class="kb-meta">
            <span v-if="!isCustomer && a.visibility === 'public'">
              {{ a.customer ? a.customer.name : $t('kb.all_customers') }}
            </span>
            <span>{{ $t('kb.updated_on', { date: formatDate(a.updated_at) }) }}</span>
            <span>{{ $t('kb.view_count', { n: a.views || 0 }, a.views || 0) }}</span>
            <span>
              <span aria-hidden="true">👍</span>
              <span class="sr-only">{{ $t('kb.helpful_yes_label') }}:</span> {{ a.helpful_yes || 0 }}
              <span aria-hidden="true" class="kb-sep">👎</span>
              <span class="sr-only">{{ $t('kb.helpful_no_label') }}:</span> {{ a.helpful_no || 0 }}
            </span>
          </div>
        </li>
      </ul>
    </template>
  </main>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { kbApi } from '@/api/itsm'
import { helpdeskApi } from '@/api/helpdesk'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const { t, te } = useI18n()
const { formatDate } = useDateFormat()
const auth = useAuthStore()
const ui = useUIStore()

const isCustomer = computed(() => auth.user?.global_role === 'customer')

const articles = ref([])
const categories = ref([])
const loading = ref(false)
const query = ref('')
const categoryId = ref('')
const status = ref('')
const visibility = ref('')
const includeArchived = ref(false)

let requestSeq = 0
async function load() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const params = {}
    if (query.value.trim()) params.q = query.value.trim()
    if (categoryId.value) params.category_id = categoryId.value
    if (!isCustomer.value) {
      if (status.value) params.status = status.value
      if (visibility.value) params.visibility = visibility.value
      if (includeArchived.value || status.value === 'archived') params.include_archived = true
    }
    const { data } = await kbApi.list(params)
    if (seq !== requestSeq) return
    articles.value = data || []
  } catch (e) {
    if (seq !== requestSeq) return
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

async function loadCategories() {
  try {
    const { data } = await helpdeskApi.listCategories()
    categories.value = (data || []).filter(c => c.is_active !== false)
  } catch {
    categories.value = []
  }
}

let debounceTimer = null
watch(query, () => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(load, 300)
})
watch([categoryId, status, visibility, includeArchived], load)
onUnmounted(() => clearTimeout(debounceTimer))

function labelOr(key, fallback) { return te(key) ? t(key) : fallback }
function statusLabel(s) { return labelOr(`kb.status_${s}`, s) }
function visibilityLabel(v) { return labelOr(`kb.visibility_${v}`, v) }

onMounted(() => {
  load()
  loadCategories()
})
</script>

<style scoped>
.kb-main { padding: 24px 32px; max-width: 1100px; margin: 0 auto; }
.kb-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 16px; }
.kb-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.kb-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.kb-toolbar { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 16px; }
.kb-search { flex: 1 1 260px; max-width: 420px; }
.kb-select { width: auto; min-width: 150px; }
.kb-toggle { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }

.kb-loading { display: flex; justify-content: center; padding: 48px; }
.kb-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.kb-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.kb-item {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 12px 14px;
}
.kb-item:focus-within { border-color: var(--color-primary); }
.kb-row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
.kb-link { font-weight: 600; font-size: 15px; color: var(--color-text); text-decoration: none; flex: 1 1 240px; min-width: 0; }
.kb-link:hover { color: var(--color-primary); text-decoration: underline; }
.kb-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.kb-summary { margin: 6px 0 0; font-size: 13px; color: var(--color-text); overflow-wrap: anywhere; }
.kb-meta { display: flex; flex-wrap: wrap; gap: 4px 16px; margin-top: 8px; font-size: 12px; color: var(--color-text-muted); }
.kb-sep { margin-left: 8px; }

.kb-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
  white-space: nowrap;
}
.kb-st-draft { border-color: var(--color-warning); }
.kb-st-published { border-color: var(--color-success); }
.kb-st-archived { color: var(--color-text-muted); }
.kb-vis-public { border-color: var(--color-primary); }
.kb-badge-cat { background: color-mix(in srgb, var(--color-primary) 10%, var(--color-surface)); }

@media (max-width: 640px) {
  .kb-main { padding: 16px; }
}
</style>
