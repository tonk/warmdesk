<template>
  <div>
    <div class="tab-toolbar">
      <button class="btn btn-primary btn-sm" @click="startCreate">+ {{ $t('sla.new_policy') }}</button>
    </div>

    <!-- Create / Edit form card -->
    <div v-if="editing" class="sla-form-card">
      <h3 class="sla-form-title">{{ editing.id ? $t('sla.edit_policy') : $t('sla.new_policy') }}</h3>

      <div class="sla-form-row">
        <div class="sla-form-field" style="flex:2">
          <label class="sla-label" for="sla-name">{{ $t('sla.name') }}</label>
          <input id="sla-name" class="form-input" v-model="form.name" :placeholder="$t('sla.name')" />
        </div>
        <div class="sla-form-field" style="align-self:flex-end">
          <label class="toggle-label">
            <input type="checkbox" v-model="form.is_active" />
            {{ $t('sla.active') }}
          </label>
        </div>
      </div>

      <div class="sla-form-row">
        <div class="sla-form-field">
          <label class="sla-label" for="sla-resp">{{ $t('sla.response_time') }}</label>
          <div class="sla-input-unit">
            <input id="sla-resp" class="form-input" type="number" min="0" v-model.number="form.response_time_minutes" placeholder="0" />
            <span class="sla-unit">min</span>
          </div>
        </div>
        <div class="sla-form-field">
          <label class="sla-label" for="sla-resol">{{ $t('sla.resolution_time') }}</label>
          <div class="sla-input-unit">
            <input id="sla-resol" class="form-input" type="number" min="0" v-model.number="form.resolution_time_minutes" placeholder="0" />
            <span class="sla-unit">min</span>
          </div>
        </div>
        <div class="sla-form-field">
          <label class="sla-label" for="sla-warn">{{ $t('sla.warning_percent') }}</label>
          <div class="sla-input-unit">
            <input id="sla-warn" class="form-input" type="number" min="0" max="100" v-model.number="form.warning_percent" aria-describedby="sla-warn-hint" />
            <span class="sla-unit">%</span>
          </div>
          <small id="sla-warn-hint" class="sla-hint">{{ $t('sla.warning_percent_hint') }}</small>
        </div>
        <div class="sla-form-field" style="flex:1;min-width:180px">
          <label class="sla-label" for="sla-calendar">{{ $t('sla.calendar') }}</label>
          <select id="sla-calendar" class="form-input" v-model="form.calendar_id">
            <option :value="0">{{ $t('sla.calendar_24_7') }}</option>
            <option v-for="c in calendars" :key="c.id" :value="c.id">{{ c.name }} ({{ c.timezone }})</option>
          </select>
        </div>
      </div>

      <h4 class="sla-section-title">{{ $t('sla.matching') }}</h4>
      <p class="sla-hint sla-hint-block">{{ $t('sla.matching_hint') }}</p>

      <div class="sla-form-row">
        <fieldset class="sla-fieldset">
          <legend class="sla-label">{{ $t('sla.priority_filter') }}</legend>
          <label v-for="p in PRIORITIES" :key="p" class="sla-check">
            <input type="checkbox" :value="p" v-model="form.priorities" />
            {{ $t('ticket.priority_' + p) }}
          </label>
        </fieldset>
        <fieldset class="sla-fieldset">
          <legend class="sla-label">{{ $t('sla.type_filter') }}</legend>
          <label v-for="ty in TYPES" :key="ty" class="sla-check">
            <input type="checkbox" :value="ty" v-model="form.types" />
            {{ $t('ticket.type_' + ty) }}
          </label>
        </fieldset>
      </div>

      <div class="sla-form-row">
        <div class="sla-form-field" style="flex:1;min-width:180px">
          <label class="sla-label" for="sla-customer">{{ $t('sla.customer') }}</label>
          <select id="sla-customer" class="form-input" v-model="form.customer_id">
            <option :value="0">{{ $t('sla.any_customer') }}</option>
            <option v-for="c in customers" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
        <div class="sla-form-field" style="flex:1;min-width:180px">
          <label class="sla-label" for="sla-category">{{ $t('sla.category') }}</label>
          <select id="sla-category" class="form-input" v-model="form.category_id">
            <option :value="0">{{ $t('sla.any_category') }}</option>
            <option v-for="c in categoryOptions" :key="c.id" :value="c.id">{{ c.label }}</option>
          </select>
        </div>
      </div>

      <h4 class="sla-section-title">{{ $t('sla.escalation') }}</h4>
      <p class="sla-hint sla-hint-block">{{ $t('sla.escalation_hint') }}</p>

      <div class="sla-form-row">
        <div class="sla-form-field" style="flex:1;min-width:180px">
          <label class="sla-label" for="sla-esc-group">{{ $t('sla.escalation_group') }}</label>
          <select id="sla-esc-group" class="form-input" v-model="form.escalation_group_id">
            <option :value="0">{{ $t('sla.none') }}</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>
        <div class="sla-form-field" style="flex:1;min-width:180px">
          <label class="sla-label" for="sla-esc-user">{{ $t('sla.escalation_user') }}</label>
          <select id="sla-esc-user" class="form-input" v-model="form.escalation_user_id">
            <option :value="0">{{ $t('sla.none') }}</option>
            <option v-for="u in users" :key="u.id" :value="u.id">{{ userLabel(u) }}</option>
          </select>
        </div>
        <div class="sla-form-field" style="align-self:flex-end">
          <label class="toggle-label">
            <input type="checkbox" v-model="form.escalation_reassign" />
            {{ $t('sla.escalation_reassign') }}
          </label>
        </div>
      </div>

      <div class="sla-form-footer">
        <button class="btn btn-primary btn-sm" @click="confirmSave">{{ $t('common.save') }}</button>
        <button class="btn btn-secondary btn-sm" @click="cancelEdit">{{ $t('common.cancel') }}</button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner" style="width:32px;height:32px;border-width:3px"></div>
    </div>

    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('sla.name') }}</th>
          <th>{{ $t('sla.response_time') }}</th>
          <th>{{ $t('sla.resolution_time') }}</th>
          <th>{{ $t('sla.matching') }}</th>
          <th>{{ $t('sla.calendar') }}</th>
          <th>{{ $t('sla.escalation') }}</th>
          <th>{{ $t('sla.active') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in policies" :key="p.id">
          <td><button type="button" class="name-link" @click="startEdit(p)">{{ p.name }}</button></td>
          <td>{{ fmtMinutes(p.response_time_minutes) }}</td>
          <td>{{ fmtMinutes(p.resolution_time_minutes) }}</td>
          <td>
            <div class="sla-match-list">
              <span>{{ fmtFilter(p.priority_filter, 'priority') || $t('sla.all') }}</span>
              <small v-if="p.type_filter">{{ fmtFilter(p.type_filter, 'type') }}</small>
              <small v-if="p.customer_id">{{ $t('sla.customer') }}: {{ nameOf(customers, p.customer_id) }}</small>
              <small v-if="p.category_id">{{ $t('sla.category') }}: {{ categoryLabel(p.category_id) }}</small>
            </div>
          </td>
          <td>{{ p.calendar_id ? nameOf(calendars, p.calendar_id) : $t('sla.calendar_24_7') }}</td>
          <td>
            <div class="sla-match-list">
              <span v-if="!p.escalation_group_id && !p.escalation_user_id">—</span>
              <span v-if="p.escalation_group_id">{{ nameOf(groups, p.escalation_group_id) }}</span>
              <span v-if="p.escalation_user_id">{{ userLabel(users.find(u => u.id === p.escalation_user_id)) }}</span>
              <small v-if="(p.escalation_group_id || p.escalation_user_id) && p.escalation_reassign">{{ $t('sla.reassigns') }}</small>
              <small v-if="p.warning_percent > 0">{{ $t('sla.warns_at', { percent: p.warning_percent }) }}</small>
            </div>
          </td>
          <td>
            <span :class="['badge', p.is_active ? 'badge-active' : 'badge-inactive']">
              {{ p.is_active ? $t('sla.yes') : $t('sla.no') }}
            </span>
          </td>
          <td>
            <div class="actions-cell">
              <button class="btn btn-ghost btn-sm" @click="startEdit(p)">{{ $t('common.edit') }}</button>
              <button class="btn btn-ghost btn-sm btn-danger" @click="deletePolicy(p)">{{ $t('common.delete') }}</button>
            </div>
          </td>
        </tr>
        <tr v-if="!policies.length && !editing">
          <td colspan="8" style="text-align:center;color:var(--color-text-muted)">{{ $t('sla.no_policies') }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { slaApi } from '@/api/sla'
import { helpdeskApi } from '@/api/helpdesk'
import { customersApi } from '@/api/customers'
import { groupsApi } from '@/api/groups'
import { adminApi } from '@/api/admin'
import { useUIStore } from '@/stores/ui'

const PRIORITIES = ['low', 'medium', 'high', 'critical']
const TYPES = ['incident', 'problem', 'service_request', 'change_request']

const { t } = useI18n()
const ui = useUIStore()
const loading = ref(true)
const policies = ref([])
const editing = ref(null)
const form = ref(emptyForm())

// Lookup data for the selects and the table.
const customers = ref([])
const groups = ref([])
const users = ref([])
const categories = ref([])
const calendars = ref([])

onMounted(() => { load(); loadLookups() })

function emptyForm() {
  return {
    name: '', response_time_minutes: 0, resolution_time_minutes: 0,
    priorities: [], types: [],
    customer_id: 0, category_id: 0, calendar_id: 0,
    warning_percent: 75,
    escalation_group_id: 0, escalation_user_id: 0, escalation_reassign: true,
    is_active: true,
  }
}

function splitCsv(v) {
  return (v || '').split(',').map(x => x.trim()).filter(Boolean)
}

async function load() {
  loading.value = true
  try {
    const { data } = await slaApi.list()
    policies.value = data || []
  } catch { policies.value = [] }
  loading.value = false
}

// Each lookup is loaded independently so one failing endpoint doesn't
// leave the other selects empty. Reloaded when the form opens, so
// calendars/categories created in other tabs show up without a page reload.
async function loadLookups() {
  const settle = async (fn, target) => {
    try { const { data } = await fn(); target.value = data || [] } catch { /* keep previous */ }
  }
  await Promise.all([
    settle(() => customersApi.list(), customers),
    settle(() => groupsApi.list(), groups),
    settle(() => adminApi.listUsers(), users),
    settle(() => helpdeskApi.adminListCategories(), categories),
    settle(() => helpdeskApi.listCalendars(), calendars),
  ])
}

const categoryOptions = computed(() => {
  const byId = Object.fromEntries(categories.value.map(c => [c.id, c]))
  return categories.value
    .map(c => ({ id: c.id, label: categoryLabel(c.id, byId), sortKey: categorySortKey(c, byId) }))
    .sort((a, b) => a.sortKey.localeCompare(b.sortKey))
})

function categorySortKey(c, byId) {
  const parent = c.parent_id ? byId[c.parent_id] : null
  const pad = (n) => String(n ?? 0).padStart(6, '0')
  return parent
    ? `${pad(parent.sort_order)}${parent.name}\u0001${pad(c.sort_order)}${c.name}`
    : `${pad(c.sort_order)}${c.name}`
}

function categoryLabel(id, byId) {
  const map = byId || Object.fromEntries(categories.value.map(c => [c.id, c]))
  const c = map[id]
  if (!c) return '#' + id
  const parent = c.parent_id ? map[c.parent_id] : null
  return parent ? `${parent.name} › ${c.name}` : c.name
}

function nameOf(list, id) {
  return list.find(x => x.id === id)?.name || '#' + id
}

function userLabel(u) {
  if (!u) return '—'
  const full = u.display_name || [u.first_name, u.last_name].filter(Boolean).join(' ')
  return full ? `${full} (${u.username})` : u.username
}

function fmtFilter(csv, kind) {
  return splitCsv(csv).map(v => t(`ticket.${kind}_${v}`)).join(', ')
}

function startCreate() {
  form.value = emptyForm()
  editing.value = { id: 0 }
  loadLookups()
}

function startEdit(p) {
  form.value = {
    name: p.name,
    response_time_minutes: p.response_time_minutes,
    resolution_time_minutes: p.resolution_time_minutes,
    priorities: splitCsv(p.priority_filter),
    types: splitCsv(p.type_filter),
    customer_id: p.customer_id || 0,
    category_id: p.category_id || 0,
    calendar_id: p.calendar_id || 0,
    warning_percent: p.warning_percent ?? 75,
    escalation_group_id: p.escalation_group_id || 0,
    escalation_user_id: p.escalation_user_id || 0,
    escalation_reassign: p.escalation_reassign ?? true,
    is_active: p.is_active,
  }
  editing.value = p
  loadLookups()
}

function cancelEdit() {
  editing.value = null
}

function buildPayload() {
  const f = form.value
  const warn = Number.isFinite(f.warning_percent) ? Math.min(100, Math.max(0, Math.round(f.warning_percent))) : 75
  return {
    name: f.name.trim(),
    response_time_minutes: f.response_time_minutes || 0,
    resolution_time_minutes: f.resolution_time_minutes || 0,
    // Keep the canonical order so the stored CSV is stable.
    priority_filter: PRIORITIES.filter(p => f.priorities.includes(p)).join(','),
    type_filter: TYPES.filter(ty => f.types.includes(ty)).join(','),
    customer_id: Number(f.customer_id) || 0,
    category_id: Number(f.category_id) || 0,
    calendar_id: Number(f.calendar_id) || 0,
    warning_percent: warn,
    escalation_group_id: Number(f.escalation_group_id) || 0,
    escalation_user_id: Number(f.escalation_user_id) || 0,
    escalation_reassign: !!f.escalation_reassign,
    is_active: !!f.is_active,
  }
}

async function confirmSave() {
  if (!form.value.name.trim()) {
    ui.error('Name is required')
    return
  }
  try {
    const payload = buildPayload()
    if (editing.value.id === 0) {
      await slaApi.create(payload)
      ui.success('SLA policy created')
    } else {
      await slaApi.update(editing.value.id, payload)
      ui.success('SLA policy updated')
    }
    editing.value = null
    await load()
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to save SLA policy')
  }
}

async function deletePolicy(p) {
  if (!await ui.confirm('Delete this SLA policy?', { destructive: true })) return
  try {
    await slaApi.delete(p.id)
    ui.success('SLA policy deleted')
    await load()
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to delete SLA policy')
  }
}

function fmtMinutes(m) {
  if (!m || m <= 0) return '—'
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  const min = m % 60
  return min > 0 ? `${h}h ${min}m` : `${h}h`
}
</script>

<style scoped>
.sla-form-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}
.sla-form-title { font-size: 15px; font-weight: 600; margin: 0 0 16px; }
.sla-form-row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; margin-bottom: 12px; }
.sla-form-field { display: flex; flex-direction: column; gap: 4px; min-width: 120px; }
.sla-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.sla-input-unit { display: flex; align-items: center; gap: 6px; }
.sla-input-unit .form-input { width: 110px; }
.sla-unit { font-size: 12px; color: var(--color-text-muted); white-space: nowrap; }
.sla-hint { font-size: 11px; color: var(--color-text-muted); }
.sla-hint-block { display: block; margin: 0 0 12px; font-size: 12px; max-width: 72ch; }
.sla-section-title { font-size: 13px; font-weight: 600; margin: 16px 0 4px; }
.sla-fieldset { border: 1px solid var(--color-border); border-radius: 6px; padding: 8px 12px; margin: 0; display: flex; flex-wrap: wrap; gap: 4px 14px; min-width: 220px; }
.sla-fieldset legend { padding: 0 4px; }
.sla-check { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }
.sla-match-list { display: flex; flex-direction: column; gap: 2px; }
.sla-form-footer { display: flex; gap: 8px; margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.loading-state { display: flex; justify-content: center; padding: 48px; }

.tab-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.data-table { width: 100%; border-collapse: collapse; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
.data-table th, .data-table td { padding: 12px 16px; text-align: left; border-bottom: 1px solid var(--color-border); font-size: 13px; vertical-align: middle; }
.data-table th { font-weight: 600; color: var(--color-text-muted); font-size: 12px; background: var(--color-bg); }
.data-table small { color: var(--color-text-muted); font-size: 11px; }
.actions-cell { display: flex; gap: 6px; flex-wrap: wrap; }
.badge-active { background: #dcfce7; color: #166534; }
.badge-inactive { background: #fee2e2; color: #991b1b; }
[data-theme="dark"] .badge-active { background: #14532d; color: #86efac; }
[data-theme="dark"] .badge-inactive { background: #450a0a; color: #fca5a5; }

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
