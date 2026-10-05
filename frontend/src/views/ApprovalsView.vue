<template>
  <main class="ap-main">
    <header class="ap-header">
      <div>
        <h1 class="ap-title">{{ $t('approvals.title') }}</h1>
        <p class="ap-subtitle">{{ $t('approvals.subtitle') }}</p>
      </div>
      <button v-if="!forbidden" type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load">
        {{ $t('approvals.refresh') }}
      </button>
    </header>

    <div v-if="forbidden" class="ap-empty" role="alert">{{ $t('approvals.forbidden') }}</div>

    <div v-else-if="loading && !rounds.length" class="ap-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <template v-else>
      <p class="sr-only" aria-live="polite">{{ $t('approvals.result_count', { n: rounds.length }, rounds.length) }}</p>
      <div v-if="!rounds.length" class="ap-empty">{{ $t('approvals.none') }}</div>

      <ul v-else class="ap-list">
        <li v-for="r in rounds" :key="`${r.ticket_id}-${r.approval_round}`" class="ap-item">
          <article :aria-labelledby="`ap-title-${r.ticket_id}`">
            <header class="ap-item-head">
              <span class="ap-id">#{{ r.ticket_id }}</span>
              <h2 :id="`ap-title-${r.ticket_id}`" class="ap-item-title">
                <RouterLink v-if="canOpenTicket" :to="ticketLink(r)" class="ap-link">{{ r.title }}</RouterLink>
                <template v-else>{{ r.title }}</template>
              </h2>
              <span class="ap-badge">{{ typeLabel(r.type) }}</span>
              <span v-if="r.change_type" class="ap-badge">{{ labelOr(`approvals.change_type_${r.change_type}`, r.change_type) }}</span>
              <span v-if="r.risk" class="ap-badge" :class="`ap-risk-${r.risk}`">{{ $t('approvals.risk_label', { risk: labelOr(`approvals.risk_${r.risk}`, r.risk) }) }}</span>
              <span v-if="r.priority" class="ap-badge">{{ labelOr(`ticket.priority_${r.priority}`, r.priority) }}</span>
            </header>

            <div class="ap-sub">
              <span>{{ r.customer_name || $t('approvals.inbox') }}</span>
              <span v-if="r.requested_by">{{ $t('approvals.requested_by', { name: r.requested_by }) }}</span>
              <span>{{ formatDateTime(r.created_at) }}</span>
              <span v-if="r.approval_round > 1">{{ $t('approvals.round', { n: r.approval_round }) }}</span>
            </div>

            <div class="ap-tally" :id="`ap-tally-${r.ticket_id}`">
              <span class="ap-tally-main">{{ $t('approvals.tally', { approved: r.approved || 0, required: r.approvals_required || 0 }) }}</span>
              <span v-if="r.rejected" class="ap-tally-rej">{{ $t('approvals.rejected_count', { n: r.rejected }, r.rejected) }}</span>
              <span
                class="ap-meter"
                role="img"
                :aria-label="$t('approvals.tally', { approved: r.approved || 0, required: r.approvals_required || 0 })"
              >
                <span class="ap-meter-fill" :style="{ width: meterPct(r) + '%' }"></span>
              </span>
            </div>

            <div class="ap-context">
              <section v-if="r.catalog_item_name || hasAnswers(r)" class="ap-block">
                <h3 class="ap-block-title">
                  {{ r.catalog_item_name ? $t('approvals.request_for', { item: r.catalog_item_name }) : $t('approvals.request_details') }}
                </h3>
                <dl v-if="hasAnswers(r)" class="ap-dl">
                  <template v-for="(v, k) in r.request_data" :key="k">
                    <dt>{{ humanKey(k) }}</dt>
                    <dd>{{ formatAnswer(v) }}</dd>
                  </template>
                </dl>
              </section>

              <section v-if="isChange(r)" class="ap-block">
                <h3 class="ap-block-title">{{ $t('approvals.change_plan') }}</h3>
                <dl class="ap-dl">
                  <dt>{{ $t('approvals.planned_window') }}</dt>
                  <dd>
                    <template v-if="r.planned_start || r.planned_end">
                      {{ r.planned_start ? formatDateTime(r.planned_start) : '…' }} – {{ r.planned_end ? formatDateTime(r.planned_end) : '…' }}
                    </template>
                    <span v-else class="ap-muted">{{ $t('approvals.not_planned') }}</span>
                  </dd>
                  <dt>{{ $t('approvals.implementation_plan') }}</dt>
                  <dd>
                    <div v-if="r.implementation_plan" class="markdown-body" v-html="md(r.implementation_plan)"></div>
                    <span v-else class="ap-muted">{{ $t('approvals.not_recorded') }}</span>
                  </dd>
                  <dt>{{ $t('approvals.backout_plan') }}</dt>
                  <dd>
                    <div v-if="r.backout_plan" class="markdown-body" v-html="md(r.backout_plan)"></div>
                    <span v-else class="ap-muted">{{ $t('approvals.not_recorded') }}</span>
                  </dd>
                </dl>
              </section>

              <details v-if="r.description" class="ap-block ap-desc">
                <summary>{{ $t('ticket.description') }}</summary>
                <div class="markdown-body" v-html="md(r.description)"></div>
              </details>

              <section v-if="r.decisions && r.decisions.length" class="ap-block">
                <h3 class="ap-block-title">{{ $t('approvals.decisions') }}</h3>
                <ul class="ap-decisions">
                  <li v-for="(d, i) in r.decisions" :key="i" class="ap-decision">
                    <span class="ap-badge" :class="`ap-dec-${d.decision}`">{{ labelOr(`approvals.decision_${d.decision}`, d.decision) }}</span>
                    <span class="ap-dec-user">{{ userName(d.user) }}</span>
                    <span class="ap-muted">{{ formatDateTime(d.created_at) }}</span>
                    <p v-if="d.comment" class="ap-dec-comment">{{ d.comment }}</p>
                  </li>
                </ul>
              </section>
            </div>

            <div v-if="r.my_decision" class="ap-mine" role="status">
              {{ $t('approvals.you_decided', { decision: labelOr(`approvals.decision_${r.my_decision}`, r.my_decision) }) }}
            </div>

            <form v-else class="ap-decide" novalidate @submit.prevent>
              <label class="form-label" :for="`ap-comment-${r.ticket_id}`">{{ $t('approvals.comment') }}</label>
              <textarea
                :id="`ap-comment-${r.ticket_id}`"
                v-model="comments[r.ticket_id]"
                class="form-input"
                rows="3"
                :aria-invalid="commentErrors[r.ticket_id] ? 'true' : 'false'"
                :aria-describedby="commentErrors[r.ticket_id] ? `ap-comment-help-${r.ticket_id} ap-comment-error-${r.ticket_id}` : `ap-comment-help-${r.ticket_id}`"
              ></textarea>
              <p :id="`ap-comment-help-${r.ticket_id}`" class="ap-help">{{ $t('approvals.comment_help') }}</p>
              <p v-if="commentErrors[r.ticket_id]" :id="`ap-comment-error-${r.ticket_id}`" class="ap-error" role="alert">
                {{ commentErrors[r.ticket_id] }}
              </p>
              <div class="ap-buttons">
                <button
                  type="button"
                  class="btn btn-primary btn-sm ap-approve"
                  :disabled="busy[r.ticket_id]"
                  :aria-describedby="`ap-title-${r.ticket_id}`"
                  @click="decide(r, 'approved')"
                ><span aria-hidden="true">✔</span> {{ $t('approvals.approve') }}</button>
                <button
                  type="button"
                  class="btn btn-danger btn-sm"
                  :disabled="busy[r.ticket_id]"
                  :aria-describedby="`ap-title-${r.ticket_id}`"
                  @click="decide(r, 'rejected')"
                ><span aria-hidden="true">✖</span> {{ $t('approvals.reject') }}</button>
              </div>
            </form>
          </article>
        </li>
      </ul>
    </template>
  </main>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { approvalsApi } from '@/api/itsm'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const { t, te } = useI18n()
const { formatDateTime } = useDateFormat()
const auth = useAuthStore()
const ui = useUIStore()

const isCustomer = computed(() => auth.user?.global_role === 'customer')
const canOpenTicket = computed(() => auth.user?.global_role === 'admin' || !!auth.user?.helpdesk_enabled)

const rounds = ref([])
const loading = ref(false)
const forbidden = ref(false)
const comments = reactive({})
const commentErrors = reactive({})
const busy = reactive({})

function md(text) { return DOMPurify.sanitize(marked.parse(text || '')) }
function labelOr(key, fallback) { return te(key) ? t(key) : fallback }
function typeLabel(type) { return labelOr(`ticket.type_${type}`, type || '') }
function userName(u) { return u?.display_name || u?.username || t('approvals.unknown_user') }
function isChange(r) { return r.type === 'change_request' || !!r.change_type }
function hasAnswers(r) { return r.request_data && Object.keys(r.request_data).length > 0 }
function humanKey(k) {
  const s = String(k).replace(/_/g, ' ')
  return s.charAt(0).toUpperCase() + s.slice(1)
}
function formatAnswer(v) {
  if (v === true) return t('common.yes')
  if (v === false) return t('common.no')
  if (v === null || v === undefined || v === '') return '—'
  return String(v)
}
function meterPct(r) {
  const req = r.approvals_required || 0
  if (!req) return 0
  return Math.min(100, Math.round(((r.approved || 0) / req) * 100))
}
function ticketLink(r) {
  return r.customer_id ? `/customers/${r.customer_id}/tickets/${r.ticket_id}` : `/tickets/inbox/${r.ticket_id}`
}

async function load() {
  if (isCustomer.value) {
    forbidden.value = true
    return
  }
  loading.value = true
  forbidden.value = false
  try {
    const { data } = await approvalsApi.mine()
    rounds.value = data || []
  } catch (e) {
    if (e?.response?.status === 403) forbidden.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    loading.value = false
  }
}

async function decide(r, decision) {
  const id = r.ticket_id
  const comment = (comments[id] || '').trim()
  commentErrors[id] = ''
  if (decision === 'rejected' && !comment) {
    commentErrors[id] = t('approvals.comment_required')
    await nextTick()
    document.getElementById(`ap-comment-${id}`)?.focus()
    return
  }
  busy[id] = true
  try {
    const { data } = await approvalsApi.decide(id, decision, comment)
    const st = data?.approval_status
    if (st === 'approved') ui.success(t('approvals.result_approved'))
    else if (st === 'rejected') ui.success(t('approvals.result_rejected'))
    else ui.success(t('approvals.decision_saved'))
    delete comments[id]
    await load()
  } catch (e) {
    const code = e?.response?.data?.code
    if (code === 'comment_required') {
      commentErrors[id] = t('approvals.comment_required')
      await nextTick()
      document.getElementById(`ap-comment-${id}`)?.focus()
    } else if (code === 'not_an_approver') {
      ui.error(t('approvals.not_an_approver'))
    } else if (code === 'no_approval_pending') {
      ui.error(t('approvals.no_longer_pending'))
      await load()
    } else {
      ui.error(e?.response?.data?.error || t('common.error'))
    }
  } finally {
    busy[id] = false
  }
}

onMounted(load)
</script>

<style scoped>
.ap-main { padding: 24px 32px; max-width: 1000px; margin: 0 auto; }
.ap-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 16px; }
.ap-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.ap-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.ap-loading { display: flex; justify-content: center; padding: 48px; }
.ap-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.ap-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 12px; }
.ap-item {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 14px 16px;
}
.ap-item:focus-within { border-color: var(--color-primary); }
.ap-item-head { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
.ap-id { font-size: 11px; font-weight: 700; color: var(--color-text-muted); }
.ap-item-title { font-size: 15px; font-weight: 700; margin: 0; flex: 1 1 240px; min-width: 0; overflow-wrap: anywhere; }
.ap-link { color: var(--color-text); text-decoration: none; }
.ap-link:hover { color: var(--color-primary); text-decoration: underline; }
.ap-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.ap-sub { display: flex; flex-wrap: wrap; gap: 4px 16px; margin-top: 6px; font-size: 12px; color: var(--color-text-muted); }

.ap-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
  white-space: nowrap;
}
.ap-risk-medium { border-color: var(--color-warning); }
.ap-risk-high { border-color: var(--color-danger); }
.ap-dec-approved { border-color: var(--color-success); }
.ap-dec-rejected { border-color: var(--color-danger); }

.ap-tally { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; margin-top: 10px; font-size: 13px; }
.ap-tally-main { font-weight: 700; }
.ap-tally-rej { color: var(--color-danger); font-weight: 600; }
.ap-meter { position: relative; width: 140px; height: 6px; border-radius: 3px; background: var(--color-bg); border: 1px solid var(--color-border); overflow: hidden; }
.ap-meter-fill { position: absolute; inset: 0 auto 0 0; background: var(--color-success); }

.ap-context { display: flex; flex-direction: column; gap: 10px; margin-top: 12px; }
.ap-block { border-top: 1px solid var(--color-border); padding-top: 10px; }
.ap-block-title { font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; color: var(--color-text-muted); margin: 0 0 6px; }
.ap-dl { display: grid; grid-template-columns: minmax(120px, max-content) 1fr; gap: 4px 16px; margin: 0; font-size: 13px; }
.ap-dl dt { font-weight: 600; color: var(--color-text-muted); }
.ap-dl dd { margin: 0; overflow-wrap: anywhere; white-space: pre-wrap; }
.ap-dl dd .markdown-body { white-space: normal; }
.ap-desc summary { cursor: pointer; font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; color: var(--color-text-muted); }
.ap-desc summary:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.ap-desc .markdown-body { margin-top: 6px; font-size: 13px; overflow-wrap: anywhere; }
.ap-muted { color: var(--color-text-muted); font-style: italic; }

.ap-decisions { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.ap-decision { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; font-size: 13px; }
.ap-dec-user { font-weight: 600; }
.ap-dec-comment { flex-basis: 100%; margin: 2px 0 0; padding-left: 10px; border-left: 2px solid var(--color-border); white-space: pre-wrap; overflow-wrap: anywhere; }

.ap-mine {
  margin-top: 12px; padding: 8px 12px; font-size: 13px; font-weight: 600;
  background: color-mix(in srgb, var(--color-primary) 8%, var(--color-surface));
  border-radius: var(--radius-sm);
}
.ap-decide { margin-top: 12px; border-top: 1px solid var(--color-border); padding-top: 12px; }
.ap-help { margin: 4px 0 0; font-size: 12px; color: var(--color-text-muted); }
.ap-error { margin: 4px 0 0; font-size: 12px; color: var(--color-danger); font-weight: 600; }
.form-input[aria-invalid='true'] { border-color: var(--color-danger); }
.ap-buttons { display: flex; gap: 8px; margin-top: 10px; }

@media (max-width: 640px) {
  .ap-main { padding: 16px; }
  .ap-dl { grid-template-columns: 1fr; }
}
</style>
