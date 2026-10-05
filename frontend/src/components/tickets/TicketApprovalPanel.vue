<template>
  <section v-if="visible" class="itil-panel approval-panel" aria-labelledby="ticket-approval-heading">
    <div class="panel-head">
      <h4 id="ticket-approval-heading">{{ $t('ticketApproval.title') }}</h4>
      <span v-if="status" class="approval-badge" :class="'approval-' + status">
        <span aria-hidden="true">{{ STATUS_ICON[status] }}</span>
        {{ $t('ticketApproval.status_' + status) }}
      </span>
    </div>

    <template v-if="status">
      <p v-if="info" class="approval-tally">
        {{ $t('ticketApproval.tally', { approved: info.approved, required: info.approvals_required || 1, rejected: info.rejected }) }}
      </p>
      <p v-if="status === 'pending'" class="itil-hint">{{ $t('ticketApproval.blocked_hint') }}</p>
      <p v-else-if="status === 'rejected'" class="itil-hint">{{ $t('ticketApproval.rejected_hint') }}</p>

      <div v-for="r in rounds" :key="r.round" class="approval-round">
        <h5>
          {{ $t('ticketApproval.round', { n: r.round }) }}
          <span v-if="r.round === currentRound" class="count">({{ $t('ticketApproval.current_round') }})</span>
        </h5>
        <ul v-if="r.decisions.length" class="decision-list">
          <li v-for="d in r.decisions" :key="d.id || (d.user_id + '-' + d.created_at)" class="decision-row">
            <span class="decision-mark" :class="'decision-' + d.decision" aria-hidden="true">{{ d.decision === 'approved' ? '✓' : '✕' }}</span>
            <span class="decision-who">{{ d.user?.display_name || d.user?.username || '—' }}</span>
            <span class="decision-what">{{ $t('ticketApproval.decision_' + d.decision) }}</span>
            <span class="decision-when">{{ formatDateTime(d.created_at) }}</span>
            <p v-if="d.comment" class="decision-comment">{{ d.comment }}</p>
          </li>
        </ul>
        <p v-else class="itil-hint">{{ $t('ticketApproval.no_decisions') }}</p>
      </div>

      <div v-if="info?.can_decide" class="decide-form">
        <label for="ticket-approval-comment">{{ $t('ticketApproval.comment') }}</label>
        <textarea
          id="ticket-approval-comment"
          v-model="comment"
          class="form-input"
          rows="2"
          aria-describedby="ticket-approval-comment-hint"
        ></textarea>
        <p id="ticket-approval-comment-hint" class="itil-hint">{{ $t('ticketApproval.comment_hint') }}</p>
        <p v-if="decideError" class="form-error" role="alert">{{ decideError }}</p>
        <div class="decide-actions">
          <button type="button" class="btn btn-primary btn-sm approve-btn" :disabled="busy" @click="decide('approved')">{{ $t('ticketApproval.approve') }}</button>
          <button type="button" class="btn btn-danger btn-sm reject-btn" :disabled="busy" @click="decide('rejected')">{{ $t('ticketApproval.reject') }}</button>
        </div>
      </div>
    </template>
    <p v-else class="itil-hint">{{ $t('ticketApproval.none_hint') }}</p>

    <div v-if="canRequest" class="request-approval">
      <button
        v-if="!requestOpen"
        type="button"
        class="btn btn-secondary btn-sm request-btn"
        :aria-expanded="false"
        aria-controls="ticket-approval-request-form"
        @click="openRequest"
      >{{ status ? $t('ticketApproval.request_new_round') : $t('ticketApproval.request') }}</button>
      <form v-else id="ticket-approval-request-form" class="request-form" @submit.prevent="submitRequest">
        <div class="itil-field">
          <label for="ticket-approval-group">{{ $t('ticketApproval.group') }}</label>
          <select id="ticket-approval-group" ref="groupSelect" v-model="requestGroupId" class="form-input form-input-sm" required>
            <option :value="null" disabled>{{ $t('common.select') }}</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>
        <div class="itil-field">
          <label for="ticket-approval-required">{{ $t('ticketApproval.required') }}</label>
          <input id="ticket-approval-required" v-model.number="requestRequired" type="number" min="1" max="99" class="form-input form-input-sm" />
        </div>
        <div class="decide-actions">
          <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !requestGroupId">{{ $t('ticketApproval.submit_request') }}</button>
          <button type="button" class="btn btn-ghost btn-sm" @click="requestOpen = false">{{ $t('common.cancel') }}</button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { approvalsApi } from '@/api/itsm'
import { ticketsApi } from '@/api/tickets'
import { groupsApi } from '@/api/groups'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useDateFormat } from '@/composables/useDateFormat'

const props = defineProps({
  ticket: { type: Object, required: true },
  customerId: { type: Number, default: null },
  isInbox: { type: Boolean, default: false },
})
const emit = defineEmits(['updated'])

const STATUS_ICON = { pending: '⏳', approved: '✓', rejected: '✕' }
const ERROR_CODES = ['comment_required', 'not_an_approver', 'no_approval_pending', 'use_change_submit']

const { t } = useI18n()
const ui = useUIStore()
const auth = useAuthStore()
const { formatDateTime } = useDateFormat()

const role = computed(() => auth.user?.global_role)
const isAgent = computed(() => role.value && role.value !== 'customer')
const status = computed(() => props.ticket?.approval_status || '')

const info = ref(null)
const groups = ref([])
const comment = ref('')
const decideError = ref('')
const busy = ref(false)
const requestOpen = ref(false)
const requestGroupId = ref(null)
const requestRequired = ref(1)
const groupSelect = ref(null)

const canRequest = computed(() =>
  !props.isInbox && isAgent.value && props.ticket?.type !== 'change_request' &&
  status.value !== 'pending' && groups.value.length > 0)
const visible = computed(() => !props.isInbox && !!props.customerId && (!!status.value || canRequest.value))
const currentRound = computed(() => info.value?.approval_round ?? props.ticket?.approval_round ?? 0)

// Decisions grouped per round, current round first; the current round is
// always listed (possibly empty) while an approval is pending.
const rounds = computed(() => {
  const by = new Map()
  for (const d of info.value?.decisions || []) {
    if (!by.has(d.round)) by.set(d.round, [])
    by.get(d.round).push(d)
  }
  if (status.value === 'pending' && currentRound.value && !by.has(currentRound.value)) by.set(currentRound.value, [])
  return [...by.entries()]
    .sort((a, b) => b[0] - a[0])
    .map(([round, decisions]) => ({ round, decisions }))
})

function errorText(e, fallback) {
  const code = e?.response?.data?.code
  if (code && ERROR_CODES.includes(code)) return t('ticketApproval.err_' + code)
  return e?.response?.data?.error || fallback
}

async function loadInfo() {
  if (props.isInbox || !props.customerId || !props.ticket?.id || !status.value) { info.value = null; return }
  try {
    const { data } = await approvalsApi.forTicket(props.customerId, props.ticket.id)
    info.value = data
  } catch (e) {
    info.value = null
    ui.error(errorText(e, t('ticketApproval.load_failed')))
  }
}

// Admins list every group; other agents fall back to the groups that have
// access to this customer (readable by customer managers only). When
// neither works the request control stays hidden.
async function loadGroups() {
  groups.value = []
  if (props.isInbox || !props.customerId || !isAgent.value || props.ticket?.type === 'change_request') return
  try {
    if (role.value === 'admin') {
      const { data } = await groupsApi.list()
      groups.value = (data || []).map(g => ({ id: g.id, name: g.name }))
    } else {
      const { data } = await groupsApi.listCustomerGroups(props.customerId)
      groups.value = (data || []).map(e => ({ id: e.group_id, name: e.group?.name || `#${e.group_id}` }))
    }
  } catch {
    groups.value = []
  }
}

watch(() => [props.ticket?.id, props.customerId, status.value, props.ticket?.approval_round], loadInfo, { immediate: true })
watch(() => [props.customerId, props.ticket?.type, role.value], loadGroups, { immediate: true })

async function decide(decision) {
  decideError.value = ''
  if (decision === 'rejected' && !comment.value.trim()) {
    decideError.value = t('ticketApproval.err_comment_required')
    return
  }
  busy.value = true
  try {
    await approvalsApi.decide(props.ticket.id, decision, comment.value.trim())
    comment.value = ''
    ui.success(t('ticketApproval.decided'))
    try {
      const { data } = await ticketsApi.get(props.customerId, props.ticket.id)
      emit('updated', data)
    } catch { /* the decision itself succeeded */ }
    await loadInfo()
  } catch (e) {
    decideError.value = errorText(e, t('ticketApproval.failed'))
  } finally {
    busy.value = false
  }
}

async function openRequest() {
  requestGroupId.value = props.ticket?.approval_group_id && groups.value.some(g => g.id === props.ticket.approval_group_id)
    ? props.ticket.approval_group_id
    : (groups.value.length === 1 ? groups.value[0].id : null)
  requestRequired.value = props.ticket?.approvals_required || 1
  requestOpen.value = true
  await nextTick()
  groupSelect.value?.focus()
}

async function submitRequest() {
  if (!requestGroupId.value) return
  busy.value = true
  try {
    const required = Math.max(1, Number(requestRequired.value) || 1)
    const { data } = await approvalsApi.request(props.customerId, props.ticket.id, requestGroupId.value, required)
    requestOpen.value = false
    ui.success(t('ticketApproval.requested'))
    emit('updated', data)
  } catch (e) {
    ui.error(errorText(e, t('ticketApproval.failed')))
  } finally {
    busy.value = false
  }
}
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
.itil-panel h5 { margin: 12px 0 6px; font-size: 13px; display: flex; align-items: center; gap: 6px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin-bottom: 8px; }
.count { font-weight: 400; color: var(--color-text-muted); }
.itil-hint { font-size: 12px; color: var(--color-text-muted); margin: 4px 0 0; }
.approval-badge { font-size: 12px; font-weight: 600; padding: 2px 8px; border-radius: 4px; border: 1px solid var(--color-border); color: var(--color-text); display: inline-flex; gap: 4px; align-items: center; }
.approval-pending { background: color-mix(in srgb, var(--color-warning) 18%, transparent); border-color: var(--color-warning); }
.approval-approved { background: color-mix(in srgb, var(--color-success) 18%, transparent); border-color: var(--color-success); }
.approval-rejected { background: color-mix(in srgb, var(--color-danger) 16%, transparent); border-color: var(--color-danger); }
.approval-tally { font-size: 13px; margin: 0; }
.decision-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.decision-row { display: flex; align-items: baseline; flex-wrap: wrap; gap: 8px; font-size: 13px; padding: 4px 8px; border-radius: 6px; background: var(--color-bg); border: 1px solid var(--color-border); }
.decision-mark { font-weight: 700; }
.decision-approved { color: var(--color-success); }
.decision-rejected { color: var(--color-danger); }
.decision-who { font-weight: 600; }
.decision-when { margin-left: auto; font-size: 11px; color: var(--color-text-muted); }
.decision-comment { flex-basis: 100%; margin: 0; white-space: pre-wrap; word-break: break-word; color: var(--color-text-muted); font-size: 12px; }
.decide-form { margin-top: 12px; display: flex; flex-direction: column; gap: 4px; }
.decide-form > label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.decide-actions { display: flex; gap: 6px; margin-top: 6px; align-items: flex-end; }
.form-error { font-size: 12px; color: var(--color-danger); margin: 4px 0 0; }
.request-approval { margin-top: 12px; }
.request-form { display: flex; flex-wrap: wrap; gap: 12px 16px; align-items: flex-end; }
.itil-field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; min-width: 150px; }
.itil-field > label { color: var(--color-text-muted); font-weight: 600; }
</style>
