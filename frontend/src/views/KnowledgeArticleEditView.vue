<template>
  <main class="ke-main">
    <div class="ke-nav">
      <RouterLink v-if="returnPath" :to="returnPath" class="ke-back">
        <span aria-hidden="true">←</span> {{ $t('kb.back_to_ticket') }}
      </RouterLink>
      <RouterLink v-else-if="isEdit" :to="{ name: 'kb-article', params: { id: route.params.id } }" class="ke-back">
        <span aria-hidden="true">←</span> {{ $t('kb.back_to_article') }}
      </RouterLink>
      <RouterLink v-else :to="{ name: 'kb' }" class="ke-back">
        <span aria-hidden="true">←</span> {{ $t('kb.back_to_list') }}
      </RouterLink>
    </div>

    <h1 class="ke-title">{{ isEdit ? $t('kb.edit_article') : $t('kb.new_article') }}</h1>

    <div v-if="isCustomer" class="ke-empty" role="alert">{{ $t('kb.no_permission_edit') }}</div>

    <div v-else-if="loading" class="ke-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <div v-else-if="notFound" class="ke-empty" role="alert">{{ $t('kb.not_found') }}</div>

    <form v-else class="ke-form" novalidate @submit.prevent="save">
      <p v-if="ticketId" class="ke-ticket-note">{{ $t('kb.will_link_ticket', { id: ticketId }) }}</p>

      <div class="form-group">
        <label class="form-label" for="ke-title">{{ $t('ticket.title') }} <span aria-hidden="true">*</span></label>
        <input
          id="ke-title"
          ref="titleInput"
          v-model="form.title"
          type="text"
          class="form-input"
          maxlength="300"
          required
          :aria-invalid="titleError ? 'true' : 'false'"
          :aria-describedby="titleError ? 'ke-title-error' : undefined"
        />
        <p v-if="titleError" id="ke-title-error" class="ke-error" role="alert">{{ titleError }}</p>
      </div>

      <div class="form-group">
        <label class="form-label" for="ke-summary">{{ $t('kb.summary') }}</label>
        <input id="ke-summary" v-model="form.summary" type="text" class="form-input" maxlength="500" aria-describedby="ke-summary-help" />
        <p id="ke-summary-help" class="ke-help">{{ $t('kb.summary_help') }}</p>
      </div>

      <div class="form-group">
        <label class="form-label" for="ke-keywords">{{ $t('kb.keywords') }}</label>
        <input id="ke-keywords" v-model="form.keywords" type="text" class="form-input" maxlength="500" aria-describedby="ke-keywords-help" />
        <p id="ke-keywords-help" class="ke-help">{{ $t('kb.keywords_help') }}</p>
      </div>

      <div class="ke-grid">
        <div class="form-group">
          <label class="form-label" for="ke-category">{{ $t('ticket.category') }}</label>
          <select id="ke-category" v-model="form.category_id" class="form-input">
            <option value="">{{ $t('kb.no_category') }}</option>
            <option v-for="c in categories" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
          </select>
        </div>

        <div class="form-group">
          <label class="form-label" for="ke-status">{{ $t('common.status') }}</label>
          <select id="ke-status" v-model="form.status" class="form-input">
            <option value="draft">{{ $t('kb.status_draft') }}</option>
            <option value="published">{{ $t('kb.status_published') }}</option>
            <option value="archived">{{ $t('kb.status_archived') }}</option>
          </select>
        </div>

        <div class="form-group">
          <label class="form-label" for="ke-visibility">{{ $t('kb.visibility') }}</label>
          <select id="ke-visibility" v-model="form.visibility" class="form-input" aria-describedby="ke-visibility-help">
            <option value="internal">{{ $t('kb.visibility_internal') }}</option>
            <option value="public">{{ $t('kb.visibility_public') }}</option>
          </select>
          <p id="ke-visibility-help" class="ke-help">
            {{ form.visibility === 'public' ? $t('kb.visibility_public_help') : $t('kb.visibility_internal_help') }}
          </p>
        </div>

        <div class="form-group">
          <label class="form-label" for="ke-customer">{{ $t('kb.customer') }}</label>
          <select
            id="ke-customer"
            v-model="form.customer_id"
            class="form-input"
            :disabled="form.visibility !== 'public'"
            aria-describedby="ke-customer-help"
          >
            <option value="">{{ $t('kb.all_customers') }}</option>
            <option v-for="c in customers" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
          </select>
          <p id="ke-customer-help" class="ke-help">{{ $t('kb.customer_help') }}</p>
        </div>
      </div>

      <div class="form-group">
        <span id="ke-body-label" class="form-label">{{ $t('kb.body') }}</span>
        <div class="md-editor">
          <div class="md-editor-tabs" role="tablist" aria-labelledby="ke-body-label">
            <button
              id="ke-body-tab-edit"
              role="tab"
              type="button"
              :aria-selected="bodyTab === 'edit'"
              aria-controls="ke-body-panel-edit"
              :class="['md-tab', { active: bodyTab === 'edit' }]"
              @click="bodyTab = 'edit'"
            >{{ $t('common.edit') }}</button>
            <button
              id="ke-body-tab-preview"
              role="tab"
              type="button"
              :aria-selected="bodyTab === 'preview'"
              aria-controls="ke-body-panel-preview"
              :class="['md-tab', { active: bodyTab === 'preview' }]"
              @click="bodyTab = 'preview'"
            >{{ $t('common.preview') }}</button>
          </div>
          <textarea
            v-if="bodyTab === 'edit'"
            id="ke-body-panel-edit"
            v-model="form.body"
            role="tabpanel"
            aria-labelledby="ke-body-tab-edit"
            class="form-input md-textarea"
            rows="18"
            :placeholder="$t('kb.body_placeholder')"
          ></textarea>
          <div
            v-else
            id="ke-body-panel-preview"
            role="tabpanel"
            aria-labelledby="ke-body-tab-preview"
            tabindex="0"
            class="md-preview markdown-body"
            v-html="renderedBody"
          ></div>
        </div>
      </div>

      <div class="ke-footer">
        <RouterLink :to="cancelTarget" class="btn btn-secondary">{{ $t('common.cancel') }}</RouterLink>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          {{ saving ? $t('common.loading') : $t('common.save') }}
        </button>
      </div>
    </form>
  </main>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { kbApi } from '@/api/itsm'
import { helpdeskApi } from '@/api/helpdesk'
import { customersApi } from '@/api/customers'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ui = useUIStore()

const isCustomer = computed(() => auth.user?.global_role === 'customer')
const isEdit = computed(() => route.name === 'kb-edit' || (!!route.params.id && route.name !== 'kb-new'))

const form = reactive({
  title: '',
  summary: '',
  keywords: '',
  body: '',
  category_id: '',
  customer_id: '',
  visibility: 'internal',
  status: 'draft',
})
const categories = ref([])
const customers = ref([])
const loading = ref(false)
const notFound = ref(false)
const saving = ref(false)
const titleError = ref('')
const titleInput = ref(null)
const bodyTab = ref('edit')

const ticketId = computed(() => {
  if (isEdit.value) return null
  const n = Number(route.query.ticket_id)
  return Number.isInteger(n) && n > 0 ? n : null
})
// Only accept in-app paths so the link cannot point elsewhere.
const returnPath = computed(() => {
  const r = route.query.return
  return typeof r === 'string' && r.startsWith('/') && !r.startsWith('//') ? r : ''
})
const cancelTarget = computed(() => {
  if (returnPath.value) return returnPath.value
  if (isEdit.value) return { name: 'kb-article', params: { id: route.params.id } }
  return { name: 'kb' }
})

const renderedBody = computed(() => DOMPurify.sanitize(marked.parse(form.body || '')))

function q(name) {
  const v = route.query[name]
  return typeof v === 'string' ? v : ''
}

async function loadLookups() {
  const [cats, custs] = await Promise.allSettled([helpdeskApi.listCategories(), customersApi.list()])
  if (cats.status === 'fulfilled') categories.value = (cats.value.data || []).filter(c => c.is_active !== false)
  if (custs.status === 'fulfilled') {
    const d = custs.value.data
    customers.value = Array.isArray(d) ? d : (d?.customers || d?.data || [])
  }
}

async function loadArticle() {
  loading.value = true
  try {
    const { data } = await kbApi.get(Number(route.params.id), false)
    form.title = data.title || ''
    form.summary = data.summary || ''
    form.keywords = data.keywords || ''
    form.body = data.body || ''
    form.category_id = data.category_id ? String(data.category_id) : ''
    form.customer_id = data.customer_id ? String(data.customer_id) : ''
    form.visibility = data.visibility || 'internal'
    form.status = data.status || 'draft'
  } catch (e) {
    if (e?.response?.status === 404) notFound.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    loading.value = false
  }
}

function prefillFromQuery() {
  form.title = q('title')
  form.body = q('body')
  form.summary = q('summary')
  if (/^\d+$/.test(q('category_id'))) form.category_id = q('category_id')
  if (/^\d+$/.test(q('customer_id'))) {
    form.customer_id = q('customer_id')
    form.visibility = 'public'
  }
}

async function save() {
  titleError.value = ''
  if (!form.title.trim()) {
    titleError.value = t('kb.title_required')
    await nextTick()
    titleInput.value?.focus()
    return
  }
  const payload = {
    title: form.title.trim(),
    summary: form.summary,
    keywords: form.keywords,
    body: form.body,
    status: form.status,
    visibility: form.visibility,
    category_id: form.category_id ? Number(form.category_id) : 0,
    customer_id: form.visibility === 'public' && form.customer_id ? Number(form.customer_id) : 0,
  }
  saving.value = true
  try {
    let id
    if (isEdit.value) {
      const { data } = await kbApi.update(Number(route.params.id), payload)
      id = data?.id || Number(route.params.id)
    } else {
      if (ticketId.value) payload.ticket_id = ticketId.value
      const { data } = await kbApi.create(payload)
      id = data.id
    }
    ui.success(t('kb.saved'))
    router.push({ name: 'kb-article', params: { id } })
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  if (isCustomer.value) return
  loadLookups()
  if (isEdit.value) loadArticle()
  else prefillFromQuery()
})
</script>

<style scoped>
.ke-main { padding: 24px 32px; max-width: 900px; margin: 0 auto; }
.ke-nav { margin-bottom: 12px; }
.ke-back { display: inline-flex; align-items: center; gap: 4px; font-size: 13px; color: var(--color-text-muted); text-decoration: none; }
.ke-back:hover { color: var(--color-primary); text-decoration: underline; }
.ke-back:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.ke-title { font-size: 22px; font-weight: 700; margin: 0 0 16px; }

.ke-loading { display: flex; justify-content: center; padding: 48px; }
.ke-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.ke-form {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 20px 24px;
}
.ke-ticket-note {
  margin: 0 0 16px; padding: 8px 12px; font-size: 13px;
  border-left: 3px solid var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 8%, var(--color-surface));
  border-radius: var(--radius-sm);
}
.ke-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 0 16px; }
.ke-help { margin: 4px 0 0; font-size: 12px; color: var(--color-text-muted); }
.ke-error { margin: 4px 0 0; font-size: 12px; color: var(--color-danger); font-weight: 600; }
.form-input[aria-invalid='true'] { border-color: var(--color-danger); }

.md-editor { border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
.md-editor-tabs { display: flex; background: var(--color-bg); border-bottom: 1px solid var(--color-border); }
.md-tab { padding: 6px 16px; font-size: 12px; font-weight: 600; cursor: pointer; background: none; border: none; border-bottom: 2px solid transparent; color: var(--color-text-muted); transition: color .15s, border-color .15s; }
.md-tab:hover { color: var(--color-text); }
.md-tab.active { color: var(--color-primary); border-bottom-color: var(--color-primary); }
.md-tab:focus-visible { outline: 2px solid var(--color-primary); outline-offset: -2px; }
.md-textarea { border: none !important; border-radius: 0 !important; resize: vertical; min-height: 320px; font-family: var(--font-mono, monospace); }
.md-preview { padding: 10px 12px; min-height: 320px; overflow-wrap: anywhere; }
.md-preview:focus-visible { outline: 2px solid var(--color-primary); outline-offset: -2px; }

.ke-footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }

@media (max-width: 640px) {
  .ke-main { padding: 16px; }
  .ke-form { padding: 16px; }
}
</style>
