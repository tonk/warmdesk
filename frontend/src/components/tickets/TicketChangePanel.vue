<template>
  <section v-if="visible" class="itil-panel change-panel" aria-labelledby="ticket-change-heading">
    <h4 id="ticket-change-heading">{{ $t('ticketChange.title') }}</h4>

    <!-- Change status workflow -->
    <ol class="change-stepper" :aria-label="$t('ticketChange.progress')">
      <li
        v-for="(s, i) in steps"
        :key="s"
        class="change-step"
        :class="{ done: isDone(i), current: i === currentIndex, failed: (s === 'failed' || s === 'cancelled') && i === currentIndex }"
        :aria-current="i === currentIndex ? 'step' : undefined"
      >
        <span class="step-dot" aria-hidden="true">{{ isDone(i) ? '✓' : ((s === 'failed' || s === 'cancelled') && i === currentIndex ? '✕' : i + 1) }}</span>
        <span class="step-label">{{ $t('ticketChange.status_' + s) }}</span>
        <span v-if="isDone(i)" class="sr-only">({{ $t('ticketChange.step_done') }})</span>
      </li>
    </ol>
    <p class="change-current" role="status">
      {{ $t('ticketChange.status') }}: <strong>{{ $t('ticketChange.status_' + changeStatus) }}</strong>
    </p>
    <p v-if="changeStatus === 'cancelled'" class="itil-hint">{{ $t('ticketChange.cancelled_hint') }}</p>
    <p v-if="changeStatus === 'awaiting_approval'" class="itil-hint">{{ $t('ticketChange.awaiting_cab') }}</p>
    <p v-if="changeStatus === 'draft' && ticket.change_type === 'standard'" class="itil-hint">{{ $t('ticketChange.standard_hint') }}</p>

    <div v-if="changeStatus === 'review'" class="itil-grid review-grid">
      <div class="itil-field">
        <label for="change-outcome">{{ $t('ticketChange.outcome') }}</label>
        <select id="change-outcome" v-model="outcomeDraft" class="form-input form-input-sm" aria-describedby="change-outcome-hint">
          <option value="">—</option>
          <option v-for="o in OUTCOMES" :key="o" :value="o">{{ $t('ticketChange.outcome_' + o) }}</option>
        </select>
      </div>
      <div class="itil-field itil-field-wide">
        <label for="change-review-notes">{{ $t('ticketChange.review_notes') }}</label>
        <textarea id="change-review-notes" v-model="drafts.review_notes" class="form-input" rows="2" @blur="saveText('review_notes')"></textarea>
      </div>
      <p id="change-outcome-hint" class="itil-hint itil-field-wide">{{ $t('ticketChange.outcome_hint') }}</p>
    </div>
    <p v-else-if="ticket.change_outcome" class="itil-hint">
      {{ $t('ticketChange.outcome') }}: {{ $t('ticketChange.outcome_' + ticket.change_outcome) }}
    </p>

    <p v-if="actionError" class="form-error" role="alert">{{ actionError }}</p>
    <div v-if="actions.length" class="change-actions">
      <button
        v-for="a in actions"
        :key="a.key"
        type="button"
        :data-action="a.key"
        class="btn btn-sm"
        :class="a.variant === 'primary' ? 'btn-primary' : a.variant === 'danger' ? 'btn-danger' : 'btn-secondary'"
        :disabled="busy || a.disabled"
        @click="runAction(a)"
      >{{ $t('ticketChange.action_' + a.key) }}</button>
    </div>

    <!-- Change details -->
    <h5>{{ $t('ticketChange.details') }}</h5>
    <div class="itil-grid">
      <div class="itil-field">
        <label for="change-type">{{ $t('ticketChange.change_type') }}</label>
        <select
          id="change-type"
          class="form-input form-input-sm"
          :value="ticket.change_type || 'normal'"
          :disabled="!isDraft || busy"
          :aria-describedby="!isDraft ? 'change-type-locked' : undefined"
          @change="saveFields({ change_type: $event.target.value })"
        >
          <option v-for="ct in CHANGE_TYPES" :key="ct" :value="ct">{{ $t('ticketChange.type_' + ct) }}</option>
        </select>
      </div>
      <div class="itil-field">
        <label for="change-risk">{{ $t('ticketChange.risk') }}</label>
        <select id="change-risk" class="form-input form-input-sm" :value="ticket.risk || ''" :disabled="busy" @change="saveFields({ risk: $event.target.value })">
          <option value="">—</option>
          <option v-for="r in RISKS" :key="r" :value="r">{{ $t('ticketChange.risk_' + r) }}</option>
        </select>
      </div>
      <div class="itil-field">
        <label for="change-planned-start">{{ $t('ticketChange.planned_start') }}</label>
        <input
          id="change-planned-start"
          type="datetime-local"
          class="form-input form-input-sm"
          :value="toLocalInput(ticket.planned_start)"
          :disabled="busy"
          @change="saveWindow('planned_start', $event.target.value)"
        />
      </div>
      <div class="itil-field">
        <label for="change-planned-end">{{ $t('ticketChange.planned_end') }}</label>
        <input
          id="change-planned-end"
          type="datetime-local"
          class="form-input form-input-sm"
          :value="toLocalInput(ticket.planned_end)"
          :min="toLocalInput(ticket.planned_start) || undefined"
          :disabled="busy"
          @change="saveWindow('planned_end', $event.target.value)"
        />
      </div>
      <p v-if="!isDraft" id="change-type-locked" class="itil-hint itil-field-wide">{{ $t('ticketChange.type_locked_hint') }}</p>
      <p v-if="changeStatus === 'scheduled'" class="itil-hint itil-field-wide">{{ $t('ticketChange.reschedule_hint') }}</p>
      <div v-for="f in PLAN_FIELDS" :key="f" class="itil-field itil-field-wide">
        <label :for="'change-' + f">{{ $t('ticketChange.' + f) }}</label>
        <textarea :id="'change-' + f" v-model="drafts[f]" class="form-input" rows="3" @blur="saveText(f)"></textarea>
      </div>
    </div>

    <!-- Schedule check -->
    <div v-if="hasWindow" class="schedule-check" aria-labelledby="change-check-heading" role="region">
      <h5 id="change-check-heading">{{ $t('ticketChange.schedule_check') }}</h5>
      <p v-if="checkLoading" class="itil-hint">{{ $t('common.loading') }}</p>
      <p v-else-if="checkFailed" class="itil-hint">{{ $t('ticketChange.check_failed') }}</p>
      <template v-else-if="check">
        <div v-if="check.freezes?.length" class="check-warning">
          <p><span aria-hidden="true">⚠ </span>{{ $t('ticketChange.freeze_warning') }}</p>
          <ul>
            <li v-for="f in check.freezes" :key="f.id || f.name">
              <strong>{{ f.name }}</strong> — {{ formatDateTime(f.start_at) }} – {{ formatDateTime(f.end_at) }}
            </li>
          </ul>
          <p class="itil-hint">{{ ticket.change_type === 'emergency' ? $t('ticketChange.freeze_emergency') : $t('ticketChange.freeze_blocks') }}</p>
        </div>
        <div v-if="check.conflicts?.length" class="check-warning">
          <p><span aria-hidden="true">⚠ </span>{{ $t('ticketChange.conflicts_warning') }}</p>
          <ul>
            <li v-for="c in check.conflicts" :key="c.ticket_id">
              <RouterLink :to="c.customer_id ? `/customers/${c.customer_id}/tickets/${c.ticket_id}` : `/tickets/inbox/${c.ticket_id}`">#{{ c.ticket_id }} {{ c.title }}</RouterLink>
              — {{ formatDateTime(c.planned_start) }} – {{ formatDateTime(c.planned_end) }}
              <span v-if="c.cis?.length" class="itil-hint">· {{ $t('ticketChange.conflict_cis', { cis: c.cis.join(', ') }) }}</span>
            </li>
          </ul>
        </div>
        <p v-if="!check.freezes?.length && !check.conflicts?.length" class="check-ok">
          <span aria-hidden="true">✓ </span>{{ $t('ticketChange.no_conflicts') }}
        </p>
      </template>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ticketsApi } from '@/api/tickets'
import { changesApi } from '@/api/itsm'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useDateFormat } from '@/composables/useDateFormat'

const props = defineProps({
  ticket: { type: Object, required: true },
  customerId: { type: Number, default: null },
  isInbox: { type: Boolean, default: false },
})
const emit = defineEmits(['updated'])

const CHANGE_TYPES = ['standard', 'normal', 'emergency']
const RISKS = ['low', 'medium', 'high']
const OUTCOMES = ['successful', 'partially_successful', 'failed']
const PLAN_FIELDS = ['implementation_plan', 'backout_plan', 'test_plan']
const ERROR_CODES = [
  'change_incomplete', 'no_cab_group', 'approval_required', 'planned_window_required',
  'change_freeze', 'outcome_required', 'invalid_change_transition', 'change_not_draft',
]
const BASE_STEPS = ['draft', 'awaiting_approval', 'approved', 'scheduled', 'implementing', 'review']

const { t } = useI18n()
const ui = useUIStore()
const auth = useAuthStore()
const { formatDateTime } = useDateFormat()

const isAgent = computed(() => auth.user?.global_role !== 'customer')
const visible = computed(() =>
  !props.isInbox && !!props.customerId && isAgent.value && props.ticket?.type === 'change_request')
const changeStatus = computed(() => props.ticket?.change_status || 'draft')
const isDraft = computed(() => changeStatus.value === 'draft')
const hasWindow = computed(() => !!(props.ticket?.planned_start && props.ticket?.planned_end))

const steps = computed(() => {
  const s = changeStatus.value
  if (s === 'cancelled') return [...BASE_STEPS, 'cancelled']
  return [...BASE_STEPS, s === 'failed' ? 'failed' : 'completed']
})
const currentIndex = computed(() => {
  const idx = steps.value.indexOf(changeStatus.value)
  return idx === -1 ? 0 : idx
})
// A cancelled change never went through the later steps.
function isDone(i) {
  return changeStatus.value !== 'cancelled' && i < currentIndex.value
}

const busy = ref(false)
const actionError = ref('')
const outcomeDraft = ref('')
const drafts = ref({ implementation_plan: '', backout_plan: '', test_plan: '', review_notes: '' })
const check = ref(null)
const checkLoading = ref(false)
const checkFailed = ref(false)

watch(() => props.ticket, (tk) => {
  drafts.value = {
    implementation_plan: tk?.implementation_plan || '',
    backout_plan: tk?.backout_plan || '',
    test_plan: tk?.test_plan || '',
    review_notes: tk?.review_notes || '',
  }
  outcomeDraft.value = tk?.change_outcome || ''
}, { immediate: true })

const actions = computed(() => {
  switch (changeStatus.value) {
    case 'draft': return [
      { key: 'submit', to: 'awaiting_approval', variant: 'primary' },
      { key: 'cancel', to: 'cancelled', confirm: 'confirm_cancel' },
    ]
    case 'awaiting_approval': return [
      { key: 'withdraw', to: 'draft' },
      { key: 'cancel', to: 'cancelled', confirm: 'confirm_cancel' },
    ]
    case 'approved': return [
      { key: 'schedule', to: 'scheduled', variant: 'primary' },
      { key: 'back_to_draft', to: 'draft' },
      { key: 'cancel', to: 'cancelled', confirm: 'confirm_cancel' },
    ]
    case 'scheduled': return [
      { key: 'start', to: 'implementing', variant: 'primary' },
      { key: 'unschedule', to: 'approved' },
      { key: 'cancel', to: 'cancelled', confirm: 'confirm_cancel' },
    ]
    case 'implementing': return [
      { key: 'review', to: 'review', variant: 'primary' },
      { key: 'fail', to: 'failed', variant: 'danger', confirm: 'confirm_fail', outcome: 'failed' },
    ]
    case 'review': return [
      { key: 'complete', to: 'completed', variant: 'primary', disabled: !outcomeDraft.value, withReview: true },
      { key: 'fail', to: 'failed', variant: 'danger', confirm: 'confirm_fail', withReview: true },
    ]
    default: return []
  }
})

function errorText(e, fallback) {
  const code = e?.response?.data?.code
  if (code && ERROR_CODES.includes(code)) return t('ticketChange.err_' + code)
  return e?.response?.data?.error || fallback
}

async function saveFields(fields, successMsg = t('ticket.updated')) {
  busy.value = true
  try {
    const { data } = await ticketsApi.update(props.customerId, props.ticket.id, fields)
    emit('updated', data)
    ui.success(successMsg)
    return true
  } catch (e) {
    ui.error(errorText(e, t('ticket.update_failed')))
    return false
  } finally {
    busy.value = false
  }
}

function saveText(field) {
  if (drafts.value[field] === (props.ticket?.[field] || '')) return
  saveFields({ [field]: drafts.value[field] })
}

function toLocalInput(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d)) return ''
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function saveWindow(field, local) {
  if (local && isNaN(new Date(local))) return
  const value = local ? new Date(local).toISOString() : null
  saveFields({ [field]: value })
}

async function runAction(a) {
  actionError.value = ''
  if (a.confirm && !await ui.confirm(t('ticketChange.' + a.confirm), { destructive: true })) return
  const fields = { change_status: a.to }
  if (a.withReview) {
    fields.review_notes = drafts.value.review_notes
    fields.change_outcome = a.to === 'failed' ? (outcomeDraft.value || 'failed') : outcomeDraft.value
  } else if (a.outcome) {
    fields.change_outcome = a.outcome
  }
  busy.value = true
  try {
    const { data } = await ticketsApi.update(props.customerId, props.ticket.id, fields)
    emit('updated', data)
    ui.success(t('ticketChange.status_changed'))
  } catch (e) {
    actionError.value = errorText(e, t('ticket.update_failed'))
  } finally {
    busy.value = false
  }
}

let checkSeq = 0
async function loadCheck() {
  check.value = null
  checkFailed.value = false
  if (!visible.value || !hasWindow.value) return
  const seq = ++checkSeq
  checkLoading.value = true
  try {
    const { data } = await changesApi.check(props.customerId, props.ticket.id)
    if (seq === checkSeq) check.value = data
  } catch {
    if (seq === checkSeq) checkFailed.value = true
  } finally {
    if (seq === checkSeq) checkLoading.value = false
  }
}

watch(
  () => [visible.value, props.ticket?.id, props.ticket?.planned_start, props.ticket?.planned_end, props.ticket?.change_type, props.ticket?.updated_at],
  loadCheck,
  { immediate: true },
)
</script>

<style scoped>
.itil-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.itil-panel h4 { margin: 0 0 10px; font-size: 14px; }
.itil-panel h5 { margin: 14px 0 8px; font-size: 13px; }
.change-panel { border-left: 3px solid var(--color-primary); }
.itil-grid { display: flex; flex-wrap: wrap; gap: 12px 16px; align-items: flex-end; }
.itil-field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; min-width: 150px; }
.itil-field > label { color: var(--color-text-muted); font-weight: 600; }
.itil-field-wide { flex: 1 1 100%; }
.itil-hint { font-size: 12px; color: var(--color-text-muted); margin: 6px 0 0; }
.form-error { font-size: 12px; color: var(--color-danger); margin: 8px 0 0; }
.review-grid { margin-top: 8px; }

.change-stepper { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px; }
.change-step { display: flex; align-items: center; gap: 6px; font-size: 12px; padding: 3px 8px 3px 4px; border-radius: 999px; border: 1px solid var(--color-border); color: var(--color-text-muted); background: var(--color-bg); }
.step-dot { display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; border-radius: 50%; font-size: 10px; font-weight: 700; border: 1px solid var(--color-border); background: var(--color-surface); }
.change-step.done { color: var(--color-text); }
.change-step.done .step-dot { background: color-mix(in srgb, var(--color-success) 22%, transparent); border-color: var(--color-success); }
.change-step.current { color: var(--color-text); font-weight: 700; border-color: var(--color-primary); background: color-mix(in srgb, var(--color-primary) 12%, transparent); }
.change-step.current .step-dot { background: var(--color-primary); border-color: var(--color-primary); color: var(--color-surface); }
.change-step.failed { border-color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 12%, transparent); }
.change-step.failed .step-dot { background: var(--color-danger); border-color: var(--color-danger); }
.change-current { font-size: 13px; margin: 8px 0 0; }
.change-actions { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }

.schedule-check { margin-top: 4px; }
.check-warning { font-size: 13px; padding: 8px 10px; border-radius: 6px; border: 1px solid var(--color-warning); background: color-mix(in srgb, var(--color-warning) 12%, transparent); margin-bottom: 8px; }
.check-warning p { margin: 0; }
.check-warning ul { margin: 6px 0 0; padding-left: 18px; }
.check-warning a { color: var(--color-primary); }
.check-ok { font-size: 13px; margin: 0; color: var(--color-text); }
.check-ok span { color: var(--color-success); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); border: 0; }
</style>
