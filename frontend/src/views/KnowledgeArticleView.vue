<template>
  <main class="ka-main">
    <RouterLink :to="{ name: 'kb' }" class="ka-back">
      <span aria-hidden="true">←</span> {{ $t('kb.back_to_list') }}
    </RouterLink>

    <div v-if="loading" class="ka-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <div v-else-if="notFound" class="ka-empty" role="alert">{{ $t('kb.not_found') }}</div>

    <article v-else-if="article" class="ka-article" aria-labelledby="ka-title">
      <header class="ka-header">
        <div class="ka-heading">
          <h1 id="ka-title" class="ka-title">{{ article.title }}</h1>
          <div v-if="!isCustomer" class="ka-actions">
            <RouterLink :to="{ name: 'kb-edit', params: { id: article.id } }" class="btn btn-secondary btn-sm">
              {{ $t('common.edit') }}
            </RouterLink>
            <button type="button" class="btn btn-danger btn-sm" :disabled="deleting" @click="remove">
              {{ $t('common.delete') }}
            </button>
          </div>
        </div>
        <p v-if="article.summary" class="ka-summary">{{ article.summary }}</p>
        <div class="ka-badges">
          <template v-if="!isCustomer">
            <span class="ka-badge" :class="`ka-st-${article.status}`">{{ labelOr(`kb.status_${article.status}`, article.status) }}</span>
            <span class="ka-badge" :class="`ka-vis-${article.visibility}`">{{ labelOr(`kb.visibility_${article.visibility}`, article.visibility) }}</span>
          </template>
          <span v-if="article.category" class="ka-badge ka-badge-cat">{{ article.category.name }}</span>
        </div>
        <dl class="ka-meta">
          <template v-if="!isCustomer && article.visibility === 'public'">
            <dt>{{ $t('kb.customer') }}</dt>
            <dd>{{ article.customer ? article.customer.name : $t('kb.all_customers') }}</dd>
          </template>
          <template v-if="article.author">
            <dt>{{ $t('kb.author') }}</dt>
            <dd>{{ userName(article.author) }}</dd>
          </template>
          <dt>{{ $t('kb.last_updated') }}</dt>
          <dd>
            {{ formatDateTime(article.updated_at) }}
            <template v-if="article.updated_by && article.updated_by.id !== article.author?.id">
              · {{ userName(article.updated_by) }}
            </template>
          </dd>
          <template v-if="article.published_at">
            <dt>{{ $t('kb.published_at') }}</dt>
            <dd>{{ formatDate(article.published_at) }}</dd>
          </template>
          <dt>{{ $t('kb.views') }}</dt>
          <dd>{{ article.views || 0 }}</dd>
          <template v-if="!isCustomer && article.keywords">
            <dt>{{ $t('kb.keywords') }}</dt>
            <dd>{{ article.keywords }}</dd>
          </template>
        </dl>
      </header>

      <div class="ka-body markdown-body" v-html="renderedBody"></div>
      <p v-if="!article.body" class="ka-empty-body">{{ $t('kb.empty_body') }}</p>

      <section class="ka-feedback" aria-labelledby="ka-feedback-title">
        <h2 id="ka-feedback-title" class="ka-feedback-title">{{ $t('kb.was_helpful') }}</h2>
        <div v-if="!voted" class="ka-feedback-buttons">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="voting" @click="vote(true)">
            <span aria-hidden="true">👍</span> {{ $t('common.yes') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="voting" @click="vote(false)">
            <span aria-hidden="true">👎</span> {{ $t('common.no') }}
          </button>
        </div>
        <p v-else class="ka-feedback-thanks" role="status">{{ $t('kb.thanks_feedback') }}</p>
        <p class="ka-feedback-counts">
          {{ $t('kb.helpful_counts', { yes: article.helpful_yes || 0, no: article.helpful_no || 0 }) }}
        </p>
      </section>
    </article>
  </main>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { kbApi } from '@/api/itsm'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const { t, te } = useI18n()
const { formatDate, formatDateTime } = useDateFormat()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ui = useUIStore()

const isCustomer = computed(() => auth.user?.global_role === 'customer')

const article = ref(null)
const loading = ref(true)
const notFound = ref(false)
const deleting = ref(false)
const voting = ref(false)
const voted = ref(false)

const VOTE_KEY = 'kb_voted_articles'

function readVotes() {
  try {
    const raw = localStorage.getItem(VOTE_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}
function rememberVote(id) {
  try {
    const list = readVotes().filter(x => x !== id)
    list.push(id)
    localStorage.setItem(VOTE_KEY, JSON.stringify(list.slice(-500)))
  } catch { /* storage unavailable — the vote still counted server-side */ }
}

const renderedBody = computed(() => DOMPurify.sanitize(marked.parse(article.value?.body || '')))

function labelOr(key, fallback) { return te(key) ? t(key) : fallback }
function userName(u) { return u?.display_name || u?.username || '' }

async function load() {
  const id = Number(route.params.id)
  loading.value = true
  notFound.value = false
  article.value = null
  try {
    const { data } = await kbApi.get(id)
    article.value = data
    voted.value = readVotes().includes(data.id)
  } catch (e) {
    if (e?.response?.status === 404 || e?.response?.status === 403) notFound.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    loading.value = false
  }
}

async function vote(helpful) {
  if (!article.value || voting.value) return
  voting.value = true
  try {
    const { data } = await kbApi.feedback(article.value.id, helpful)
    article.value.helpful_yes = data.helpful_yes
    article.value.helpful_no = data.helpful_no
    voted.value = true
    rememberVote(article.value.id)
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    voting.value = false
  }
}

async function remove() {
  if (!article.value) return
  if (!await ui.confirm(t('kb.confirm_delete', { title: article.value.title }), { destructive: true })) return
  deleting.value = true
  try {
    await kbApi.remove(article.value.id)
    ui.success(t('kb.deleted'))
    router.push({ name: 'kb' })
  } catch (e) {
    if (e?.response?.status === 403) ui.error(t('kb.delete_forbidden'))
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    deleting.value = false
  }
}

watch(() => route.params.id, (v, old) => { if (v && v !== old) load() })
onMounted(load)
</script>

<style scoped>
.ka-main { padding: 24px 32px; max-width: 900px; margin: 0 auto; }
.ka-back { display: inline-flex; align-items: center; gap: 4px; font-size: 13px; color: var(--color-text-muted); text-decoration: none; margin-bottom: 16px; }
.ka-back:hover { color: var(--color-primary); text-decoration: underline; }
.ka-back:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }

.ka-loading { display: flex; justify-content: center; padding: 48px; }
.ka-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.ka-article {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 20px 24px;
}
.ka-header { border-bottom: 1px solid var(--color-border); padding-bottom: 14px; margin-bottom: 16px; }
.ka-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.ka-title { font-size: 22px; font-weight: 700; margin: 0; overflow-wrap: anywhere; }
.ka-actions { display: flex; gap: 8px; }
.ka-summary { margin: 8px 0 0; color: var(--color-text-muted); font-size: 14px; }
.ka-badges { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.ka-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
  white-space: nowrap;
}
.ka-st-draft { border-color: var(--color-warning); }
.ka-st-published { border-color: var(--color-success); }
.ka-st-archived { color: var(--color-text-muted); }
.ka-vis-public { border-color: var(--color-primary); }
.ka-badge-cat { background: color-mix(in srgb, var(--color-primary) 10%, var(--color-surface)); }

.ka-meta { display: grid; grid-template-columns: max-content 1fr; gap: 4px 16px; margin: 12px 0 0; font-size: 12px; }
.ka-meta dt { color: var(--color-text-muted); font-weight: 600; }
.ka-meta dd { margin: 0; overflow-wrap: anywhere; }

.ka-body { font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.ka-empty-body { color: var(--color-text-muted); font-style: italic; }

.ka-feedback { border-top: 1px solid var(--color-border); margin-top: 24px; padding-top: 14px; }
.ka-feedback-title { font-size: 14px; font-weight: 700; margin: 0 0 8px; }
.ka-feedback-buttons { display: flex; gap: 8px; }
.ka-feedback-thanks { margin: 0; font-size: 13px; color: var(--color-text); }
.ka-feedback-counts { margin: 8px 0 0; font-size: 12px; color: var(--color-text-muted); }

@media (max-width: 640px) {
  .ka-main { padding: 16px; }
  .ka-article { padding: 16px; }
}
</style>
