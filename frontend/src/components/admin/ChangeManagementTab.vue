<template>
  <div>
    <!-- CAB settings -->
    <section class="cm-card" aria-labelledby="cm-cab-title">
      <h3 id="cm-cab-title" class="cm-card-title">{{ $t('changeAdmin.cab_title') }}</h3>
      <p class="cm-hint cm-hint-block">{{ $t('changeAdmin.cab_hint') }}</p>

      <div v-if="settingsLoading" class="loading-state">
        <div class="spinner" style="width:24px;height:24px;border-width:3px"></div>
      </div>
      <template v-else>
        <div class="cm-form-row">
          <div class="cm-form-field" style="flex:1;min-width:220px;max-width:360px">
            <label class="cm-label" for="cm-cab-group">{{ $t('changeAdmin.cab_group') }}</label>
            <select id="cm-cab-group" class="form-input" v-model.number="cab.cab_group_id" aria-describedby="cm-cab-group-hint">
              <option :value="0">{{ $t('sla.none') }}</option>
              <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
            </select>
            <small id="cm-cab-group-hint" class="cm-hint">{{ $t('changeAdmin.cab_group_hint') }}</small>
          </div>
          <div class="cm-form-field" style="width:190px">
            <label class="cm-label" for="cm-approvals">{{ $t('changeAdmin.approvals_required') }}</label>
            <input id="cm-approvals" class="form-input" type="number" min="1" max="20" step="1" v-model.number="cab.approvals_required" aria-describedby="cm-approvals-hint" />
          </div>
          <div class="cm-form-field" style="width:220px">
            <label class="cm-label" for="cm-emergency">{{ $t('changeAdmin.emergency_approvals_required') }}</label>
            <input id="cm-emergency" class="form-input" type="number" min="1" max="20" step="1" v-model.number="cab.emergency_approvals_required" aria-describedby="cm-approvals-hint" />
          </div>
        </div>
        <small id="cm-approvals-hint" class="cm-hint cm-hint-row">{{ $t('changeAdmin.approvals_hint') }}</small>
        <div class="cm-form-footer">
          <button class="btn btn-primary btn-sm" @click="saveCab" :disabled="cabSaving">{{ $t('common.save') }}</button>
        </div>
      </template>
    </section>

    <!-- Freeze windows -->
    <section class="cm-card" aria-labelledby="cm-freeze-title">
      <div class="cm-card-header">
        <h3 id="cm-freeze-title" class="cm-card-title">{{ $t('changeAdmin.freezes_title') }}</h3>
        <button class="btn btn-primary btn-sm" @click="startCreate">+ {{ $t('changeAdmin.new_freeze') }}</button>
      </div>
      <p class="cm-hint cm-hint-block">{{ $t('changeAdmin.freezes_hint') }}</p>

      <div v-if="editing" class="cm-inline-form">
        <h4 class="cm-section-title">{{ editing.id ? $t('changeAdmin.edit_freeze') : $t('changeAdmin.new_freeze') }}</h4>
        <div class="cm-form-row">
          <div class="cm-form-field" style="flex:2;min-width:200px">
            <label class="cm-label" for="cm-fz-name">{{ $t('helpdeskAdmin.name') }}</label>
            <input id="cm-fz-name" ref="nameEl" class="form-input" v-model="form.name" maxlength="200" />
          </div>
          <div class="cm-form-field" style="flex:1;min-width:200px">
            <label class="cm-label" for="cm-fz-customer">{{ $t('sla.customer') }}</label>
            <select id="cm-fz-customer" class="form-input" v-model.number="form.customer_id">
              <option :value="0">{{ $t('changeAdmin.all_customers') }}</option>
              <option v-for="c in customers" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </div>
        </div>
        <div class="cm-form-row">
          <div class="cm-form-field">
            <label class="cm-label" for="cm-fz-start">{{ $t('changeAdmin.start') }}</label>
            <input id="cm-fz-start" class="form-input" type="datetime-local" v-model="form.start" />
          </div>
          <div class="cm-form-field">
            <label class="cm-label" for="cm-fz-end">{{ $t('changeAdmin.end') }}</label>
            <input id="cm-fz-end" class="form-input" type="datetime-local" v-model="form.end" :min="form.start || undefined" />
          </div>
        </div>
        <small class="cm-hint cm-hint-row">{{ $t('changeAdmin.local_time_hint', { tz: localTz }) }}</small>
        <div class="cm-form-footer">
          <button class="btn btn-primary btn-sm" @click="saveFreeze" :disabled="saving">{{ $t('common.save') }}</button>
          <button class="btn btn-secondary btn-sm" @click="editing = null">{{ $t('common.cancel') }}</button>
        </div>
      </div>

      <div v-if="loading" class="loading-state">
        <div class="spinner" style="width:24px;height:24px;border-width:3px"></div>
      </div>
      <table v-else class="data-table" aria-labelledby="cm-freeze-title">
        <thead>
          <tr>
            <th>{{ $t('helpdeskAdmin.name') }}</th>
            <th>{{ $t('changeAdmin.start') }}</th>
            <th>{{ $t('changeAdmin.end') }}</th>
            <th>{{ $t('sla.customer') }}</th>
            <th>{{ $t('common.status') }}</th>
            <th>{{ $t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in freezes" :key="f.id" :class="{ 'cm-past': freezeState(f) === 'past' }">
            <td><button type="button" class="name-link" @click="startEdit(f)">{{ f.name }}</button></td>
            <td>{{ formatDateTime(f.start_at) }}</td>
            <td>{{ formatDateTime(f.end_at) }}</td>
            <td>{{ f.customer_id ? customerName(f.customer_id) : $t('changeAdmin.all_customers') }}</td>
            <td><span :class="['cm-badge', 'cm-badge-' + freezeState(f)]">{{ $t('changeAdmin.state_' + freezeState(f)) }}</span></td>
            <td>
              <div class="actions-cell">
                <button class="btn btn-ghost btn-sm" @click="startEdit(f)" :aria-label="`${$t('common.edit')}: ${f.name}`">{{ $t('common.edit') }}</button>
                <button class="btn btn-ghost btn-sm btn-danger" @click="deleteFreeze(f)" :aria-label="`${$t('common.delete')}: ${f.name}`">{{ $t('common.delete') }}</button>
              </div>
            </td>
          </tr>
          <tr v-if="!freezes.length">
            <td colspan="6" style="text-align:center;color:var(--color-text-muted)">{{ $t('changeAdmin.no_freezes') }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { changesApi } from '@/api/itsm'
import { helpdeskApi } from '@/api/helpdesk'
import { groupsApi } from '@/api/groups'
import { customersApi } from '@/api/customers'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const { t } = useI18n()
const ui = useUIStore()
const { formatDateTime } = useDateFormat()

const groups = ref([])
const customers = ref([])
const settingsLoading = ref(true)
const cabSaving = ref(false)
const cab = ref({ cab_group_id: 0, approvals_required: 1, emergency_approvals_required: 1 })

const loading = ref(true)
const saving = ref(false)
const freezes = ref([])
const editing = ref(null)
const form = ref(emptyForm())
const nameEl = ref(null)

const localTz = (() => {
  try { return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC' } catch { return 'UTC' }
})()

onMounted(() => {
  loadSettings()
  loadFreezes()
  loadLookups()
})

async function loadLookups() {
  try { groups.value = (await groupsApi.list()).data || [] } catch { groups.value = [] }
  try { customers.value = (await customersApi.list()).data || [] } catch { customers.value = [] }
}

// ── CAB settings ───────────────────────────────────────────────────────────

async function loadSettings() {
  settingsLoading.value = true
  try {
    const { data } = await helpdeskApi.getSettings()
    const ch = data?.change || {}
    cab.value = {
      cab_group_id: ch.cab_group_id || 0,
      approvals_required: ch.approvals_required || 1,
      emergency_approvals_required: ch.emergency_approvals_required || 1,
    }
  } catch {
    ui.error(t('helpdeskAdmin.load_failed'))
  }
  settingsLoading.value = false
}

function validApprovals(n) {
  return Number.isInteger(n) && n >= 1 && n <= 20
}

async function saveCab() {
  const c = cab.value
  if (!validApprovals(c.approvals_required) || !validApprovals(c.emergency_approvals_required)) {
    ui.error(t('changeAdmin.approvals_invalid'))
    return
  }
  cabSaving.value = true
  try {
    await helpdeskApi.updateSettings({
      change: {
        cab_group_id: Number(c.cab_group_id) || 0,
        approvals_required: c.approvals_required,
        emergency_approvals_required: c.emergency_approvals_required,
      },
    })
    ui.success(t('changeAdmin.cab_saved'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.save_failed'))
  } finally {
    cabSaving.value = false
  }
}

// ── Freeze windows ─────────────────────────────────────────────────────────

function emptyForm() {
  return { name: '', customer_id: 0, start: '', end: '' }
}

function pad(n) { return String(n).padStart(2, '0') }

// ISO timestamp → value for <input type="datetime-local"> in local time.
function toLocalInput(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d)) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// datetime-local value (local wall clock) → RFC 3339 UTC string.
function fromLocalInput(val) {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(val || '')
  if (!m) return null
  const d = new Date(+m[1], +m[2] - 1, +m[3], +m[4], +m[5])
  return isNaN(d) ? null : d.toISOString()
}

function freezeState(f) {
  const now = Date.now()
  if (new Date(f.end_at).getTime() <= now) return 'past'
  if (new Date(f.start_at).getTime() <= now) return 'active'
  return 'upcoming'
}

function customerName(id) {
  return customers.value.find(c => c.id === id)?.name || `#${id}`
}

async function loadFreezes() {
  loading.value = true
  try {
    const { data } = await changesApi.freezes({ all: true })
    freezes.value = data || []
  } catch { freezes.value = [] }
  loading.value = false
}

function openForm(f, target) {
  form.value = f
  editing.value = target
  nextTick(() => nameEl.value?.focus())
}

function startCreate() {
  openForm(emptyForm(), { id: 0 })
}

function startEdit(f) {
  openForm({
    name: f.name,
    customer_id: f.customer_id || 0,
    start: toLocalInput(f.start_at),
    end: toLocalInput(f.end_at),
  }, f)
}

async function saveFreeze() {
  const f = form.value
  if (!f.name.trim()) { ui.error(t('helpdeskAdmin.name_required')); return }
  const start = fromLocalInput(f.start)
  const end = fromLocalInput(f.end)
  if (!start || !end) { ui.error(t('changeAdmin.dates_required')); return }
  if (new Date(end) <= new Date(start)) { ui.error(t('changeAdmin.end_after_start')); return }
  const payload = { name: f.name.trim(), start_at: start, end_at: end, customer_id: f.customer_id || 0 }
  saving.value = true
  try {
    if (editing.value.id) await changesApi.updateFreeze(editing.value.id, payload)
    else await changesApi.createFreeze(payload)
    editing.value = null
    await loadFreezes()
    ui.success(t('changeAdmin.freeze_saved'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.save_failed'))
  } finally {
    saving.value = false
  }
}

async function deleteFreeze(f) {
  if (!await ui.confirm(t('changeAdmin.delete_freeze_confirm', { name: f.name }), { destructive: true })) return
  try {
    await changesApi.deleteFreeze(f.id)
    if (editing.value?.id === f.id) editing.value = null
    await loadFreezes()
    ui.success(t('changeAdmin.freeze_deleted'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.delete_failed'))
  }
}
</script>

<style scoped>
.cm-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}
.cm-card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 8px; }
.cm-card-title { font-size: 15px; font-weight: 600; margin: 0 0 8px; }
.cm-card-header .cm-card-title { margin: 0; }
.cm-section-title { font-size: 13px; font-weight: 600; margin: 0 0 12px; }
.cm-inline-form { border: 1px solid var(--color-border); border-radius: 6px; padding: 14px 16px; margin-bottom: 16px; background: var(--color-bg); }
.cm-form-row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; margin-bottom: 12px; }
.cm-form-field { display: flex; flex-direction: column; gap: 4px; min-width: 120px; }
.cm-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.cm-hint { font-size: 11px; color: var(--color-text-muted); }
.cm-hint-row { display: block; margin: -4px 0 12px; }
.cm-hint-block { display: block; margin: 0 0 16px; font-size: 12px; max-width: 80ch; }
.cm-form-footer { display: flex; gap: 8px; }
.cm-past td { color: var(--color-text-muted); }
.cm-badge { display: inline-block; font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px; color: var(--color-text); }
.cm-badge-active { background: color-mix(in srgb, var(--color-danger) 18%, transparent); }
.cm-badge-upcoming { background: color-mix(in srgb, var(--color-warning) 22%, transparent); }
.cm-badge-past { background: color-mix(in srgb, var(--color-text-muted) 15%, transparent); color: var(--color-text-muted); }
.loading-state { display: flex; justify-content: center; padding: 24px; }

.data-table { width: 100%; border-collapse: collapse; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
.data-table th, .data-table td { padding: 12px 16px; text-align: left; border-bottom: 1px solid var(--color-border); font-size: 13px; vertical-align: middle; }
.data-table th { font-weight: 600; color: var(--color-text-muted); font-size: 12px; background: var(--color-bg); }
.actions-cell { display: flex; gap: 6px; flex-wrap: wrap; }
.name-link {
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  font-weight: 600;
  color: var(--color-primary);
  cursor: pointer;
  text-align: left;
}
.name-link:hover { text-decoration: underline; }
.name-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; border-radius: 2px; }
</style>
