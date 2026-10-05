<template>
  <div>
    <div class="tab-toolbar">
      <button class="btn btn-primary btn-sm" @click="startCreate">+ {{ $t('catalogAdmin.new_item') }}</button>
    </div>
    <p class="sc-hint sc-hint-block">{{ $t('catalogAdmin.hint') }}</p>

    <!-- Create / Edit form card -->
    <div v-if="editing" class="sc-form-card">
      <h3 class="sc-form-title">{{ editing.id ? $t('catalogAdmin.edit_item') : $t('catalogAdmin.new_item') }}</h3>

      <div class="sc-form-row">
        <div class="sc-form-field" style="width:90px;min-width:90px">
          <label class="sc-label" for="sc-icon">{{ $t('catalogAdmin.icon') }}</label>
          <input id="sc-icon" class="form-input sc-icon-input" v-model="form.icon" maxlength="16" aria-describedby="sc-icon-hint" />
        </div>
        <div class="sc-form-field" style="flex:3;min-width:220px">
          <label class="sc-label" for="sc-name">{{ $t('helpdeskAdmin.name') }}</label>
          <input id="sc-name" ref="nameEl" class="form-input" v-model="form.name" maxlength="200" />
        </div>
        <div class="sc-form-field" style="width:110px">
          <label class="sc-label" for="sc-order">{{ $t('macro.sort_order') }}</label>
          <input id="sc-order" class="form-input" type="number" step="1" v-model.number="form.sort_order" />
        </div>
        <div class="sc-form-field sc-check-field">
          <label class="toggle-label sc-check">
            <input type="checkbox" v-model="form.is_active" />
            {{ $t('macro.active') }}
          </label>
        </div>
      </div>
      <small id="sc-icon-hint" class="sc-hint sc-hint-row">{{ $t('catalogAdmin.icon_hint') }}</small>

      <div class="sc-form-field sc-full">
        <label class="sc-label" for="sc-desc">{{ $t('catalogAdmin.description') }}</label>
        <textarea id="sc-desc" class="form-input" rows="4" v-model="form.description" aria-describedby="sc-desc-hint"></textarea>
        <small id="sc-desc-hint" class="sc-hint">{{ $t('catalogAdmin.description_hint') }}</small>
      </div>

      <div class="sc-form-row">
        <div class="sc-form-field" style="flex:1;min-width:200px">
          <label class="sc-label" for="sc-category">{{ $t('sla.category') }}</label>
          <select id="sc-category" class="form-input" v-model.number="form.category_id">
            <option :value="0">{{ $t('sla.none') }}</option>
            <option v-for="c in categoryOptions" :key="c.id" :value="c.id">{{ c.label }}</option>
          </select>
        </div>
        <div class="sc-form-field" style="flex:1;min-width:200px">
          <label class="sc-label" for="sc-customer">{{ $t('sla.customer') }}</label>
          <select id="sc-customer" class="form-input" v-model.number="form.customer_id" aria-describedby="sc-customer-hint">
            <option :value="0">{{ $t('catalogAdmin.all_customers') }}</option>
            <option v-for="c in customers" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
          <small id="sc-customer-hint" class="sc-hint">{{ $t('catalogAdmin.customer_hint') }}</small>
        </div>
        <div class="sc-form-field" style="width:160px">
          <label class="sc-label" for="sc-priority">{{ $t('catalogAdmin.default_priority') }}</label>
          <select id="sc-priority" class="form-input" v-model="form.default_priority">
            <option v-for="p in PRIORITIES" :key="p" :value="p">{{ $t('ticket.priority_' + p) }}</option>
          </select>
        </div>
      </div>

      <div class="sc-form-row">
        <div class="sc-form-field" style="flex:1;min-width:200px">
          <label class="sc-label" for="sc-group">{{ $t('catalogAdmin.fulfilment_group') }}</label>
          <select id="sc-group" class="form-input" v-model.number="form.group_id" aria-describedby="sc-group-hint">
            <option :value="0">{{ $t('sla.none') }}</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
          <small id="sc-group-hint" class="sc-hint">{{ $t('catalogAdmin.fulfilment_group_hint') }}</small>
        </div>
        <div class="sc-form-field" style="flex:1;min-width:200px">
          <label class="sc-label" for="sc-checklist">{{ $t('catalogAdmin.checklist_template') }}</label>
          <select id="sc-checklist" class="form-input" v-model.number="form.checklist_template_id" aria-describedby="sc-checklist-hint">
            <option :value="0">{{ $t('sla.none') }}</option>
            <option v-for="tpl in checklistTemplates" :key="tpl.id" :value="tpl.id">{{ tpl.name }}</option>
          </select>
          <small id="sc-checklist-hint" class="sc-hint">{{ $t('catalogAdmin.checklist_template_hint') }}</small>
        </div>
      </div>

      <fieldset class="sc-fieldset">
        <legend class="sc-label">{{ $t('catalogAdmin.approval') }}</legend>
        <div class="sc-form-row" style="margin-bottom:0">
          <div class="sc-form-field" style="flex:1;min-width:200px">
            <label class="sc-label" for="sc-approval-group">{{ $t('catalogAdmin.approver_group') }}</label>
            <select id="sc-approval-group" class="form-input" v-model.number="form.approval_group_id" :aria-invalid="approvalInvalid ? 'true' : undefined" aria-describedby="sc-approval-hint">
              <option :value="0">{{ $t('sla.none') }}</option>
              <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
            </select>
          </div>
          <div class="sc-form-field" style="width:170px">
            <label class="sc-label" for="sc-approvals">{{ $t('catalogAdmin.approvals_required') }}</label>
            <input id="sc-approvals" class="form-input" type="number" min="0" max="20" step="1" v-model.number="form.approvals_required" aria-describedby="sc-approval-hint" />
          </div>
        </div>
        <small id="sc-approval-hint" :class="['sc-hint', { 'sc-error': approvalInvalid }]" :role="approvalInvalid ? 'alert' : undefined">
          {{ approvalInvalid ? $t('catalogAdmin.approver_group_required') : $t('catalogAdmin.approval_hint') }}
        </small>
      </fieldset>

      <!-- Form field builder -->
      <section class="sc-fields" aria-labelledby="sc-fields-title">
        <div class="sc-fields-header">
          <h4 id="sc-fields-title" class="sc-section-title">{{ $t('catalogAdmin.fields') }}</h4>
          <button class="btn btn-secondary btn-sm" @click="addField">+ {{ $t('catalogAdmin.add_field') }}</button>
        </div>
        <p class="sc-hint sc-hint-block" style="margin-bottom:8px">{{ $t('catalogAdmin.fields_hint') }}</p>

        <div v-if="!form.fields.length" class="sc-empty-inline">{{ $t('catalogAdmin.no_fields') }}</div>
        <ol v-else class="sc-field-list" aria-labelledby="sc-fields-title">
          <li v-for="(f, idx) in form.fields" :key="f._key" class="sc-field-row" :aria-label="$t('catalogAdmin.field_n', { n: idx + 1 })">
            <div class="sc-field-head">
              <span class="sc-field-num" aria-hidden="true">{{ idx + 1 }}</span>
              <div class="sc-field-move">
                <button class="btn-icon-xs" :disabled="idx === 0" @click="moveField(idx, -1)" :aria-label="$t('catalogAdmin.move_up_n', { n: idx + 1 })">↑</button>
                <button class="btn-icon-xs" :disabled="idx === form.fields.length - 1" @click="moveField(idx, 1)" :aria-label="$t('catalogAdmin.move_down_n', { n: idx + 1 })">↓</button>
                <button class="btn-icon-xs" @click="removeField(idx)" :aria-label="$t('catalogAdmin.remove_field_n', { n: idx + 1 })">✕</button>
              </div>
            </div>
            <div class="sc-form-row">
              <div class="sc-form-field" style="flex:2;min-width:180px">
                <label class="sc-label" :for="'sc-f-label-' + f._key">{{ $t('catalogAdmin.field_label') }}</label>
                <input :id="'sc-f-label-' + f._key" class="form-input form-input-sm" v-model="f.label" @input="onLabelInput(f)" />
              </div>
              <div class="sc-form-field" style="flex:1;min-width:150px">
                <label class="sc-label" :for="'sc-f-key-' + f._key">{{ $t('catalogAdmin.field_key') }}</label>
                <input :id="'sc-f-key-' + f._key" class="form-input form-input-sm sc-mono" v-model="f.key" maxlength="50" spellcheck="false" autocomplete="off"
                  @input="f.keyTouched = true" :aria-invalid="keyError(f, idx) ? 'true' : undefined" :aria-describedby="'sc-f-key-hint-' + f._key" />
                <small :id="'sc-f-key-hint-' + f._key" :class="['sc-hint', { 'sc-error': keyError(f, idx) }]">{{ keyError(f, idx) || $t('catalogAdmin.field_key_hint') }}</small>
              </div>
              <div class="sc-form-field" style="width:150px">
                <label class="sc-label" :for="'sc-f-type-' + f._key">{{ $t('catalogAdmin.field_type') }}</label>
                <select :id="'sc-f-type-' + f._key" class="form-input form-input-sm" v-model="f.type">
                  <option v-for="ft in FIELD_TYPES" :key="ft" :value="ft">{{ $t('catalogAdmin.type_' + ft) }}</option>
                </select>
              </div>
              <div class="sc-form-field sc-check-field">
                <label class="toggle-label sc-check">
                  <input type="checkbox" v-model="f.required" />
                  {{ $t('catalogAdmin.field_required') }}
                </label>
              </div>
            </div>
            <div class="sc-form-row" style="margin-bottom:0">
              <div v-if="f.type === 'select'" class="sc-form-field" style="flex:1;min-width:200px">
                <label class="sc-label" :for="'sc-f-opts-' + f._key">{{ $t('catalogAdmin.field_options') }}</label>
                <textarea :id="'sc-f-opts-' + f._key" class="form-input form-input-sm" rows="3" v-model="f.optionsText"
                  :aria-invalid="!parseOptions(f.optionsText).length ? 'true' : undefined" :aria-describedby="'sc-f-opts-hint-' + f._key"></textarea>
                <small :id="'sc-f-opts-hint-' + f._key" :class="['sc-hint', { 'sc-error': !parseOptions(f.optionsText).length }]">{{ $t('catalogAdmin.field_options_hint') }}</small>
              </div>
              <div class="sc-form-field" style="flex:1;min-width:200px">
                <label class="sc-label" :for="'sc-f-help-' + f._key">{{ $t('catalogAdmin.field_help') }}</label>
                <input :id="'sc-f-help-' + f._key" class="form-input form-input-sm" v-model="f.help" />
              </div>
            </div>
          </li>
        </ol>

        <!-- Live preview -->
        <details v-if="form.fields.length" class="sc-preview">
          <summary>{{ $t('catalogAdmin.preview') }}</summary>
          <div class="sc-preview-body">
            <p class="sc-hint sc-hint-block">{{ $t('catalogAdmin.preview_hint') }}</p>
            <div v-for="f in form.fields" :key="'pv-' + f._key" class="sc-form-field sc-preview-field">
              <template v-if="f.type === 'checkbox'">
                <label class="toggle-label sc-check">
                  <input type="checkbox" :aria-describedby="f.help ? 'sc-pv-help-' + f._key : undefined" />
                  {{ f.label || '—' }}<span v-if="f.required" class="sc-req" aria-hidden="true">*</span>
                </label>
              </template>
              <template v-else>
                <label class="sc-label" :for="'sc-pv-' + f._key">{{ f.label || '—' }}<span v-if="f.required" class="sc-req" aria-hidden="true">*</span></label>
                <textarea v-if="f.type === 'textarea'" :id="'sc-pv-' + f._key" class="form-input form-input-sm" rows="3" :aria-required="f.required ? 'true' : undefined" :aria-describedby="f.help ? 'sc-pv-help-' + f._key : undefined"></textarea>
                <select v-else-if="f.type === 'select'" :id="'sc-pv-' + f._key" class="form-input form-input-sm" :aria-required="f.required ? 'true' : undefined" :aria-describedby="f.help ? 'sc-pv-help-' + f._key : undefined">
                  <option value="">{{ $t('common.select') }}</option>
                  <option v-for="o in parseOptions(f.optionsText)" :key="o" :value="o">{{ o }}</option>
                </select>
                <input v-else :id="'sc-pv-' + f._key" class="form-input form-input-sm" :type="f.type === 'number' ? 'number' : f.type === 'date' ? 'date' : 'text'" :aria-required="f.required ? 'true' : undefined" :aria-describedby="f.help ? 'sc-pv-help-' + f._key : undefined" />
              </template>
              <small v-if="f.help" :id="'sc-pv-help-' + f._key" class="sc-hint">{{ f.help }}</small>
            </div>
          </div>
        </details>
      </section>

      <div class="sc-form-footer">
        <button class="btn btn-primary btn-sm" @click="confirmSave" :disabled="saving">{{ $t('common.save') }}</button>
        <button class="btn btn-secondary btn-sm" @click="cancelEdit">{{ $t('common.cancel') }}</button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner" style="width:32px;height:32px;border-width:3px"></div>
    </div>

    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('helpdeskAdmin.name') }}</th>
          <th>{{ $t('sla.category') }}</th>
          <th>{{ $t('sla.customer') }}</th>
          <th>{{ $t('catalogAdmin.approval') }}</th>
          <th>{{ $t('catalogAdmin.fields') }}</th>
          <th>{{ $t('macro.active') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="it in items" :key="it.id" :class="{ 'sc-inactive': !it.is_active }">
          <td>
            <span v-if="it.icon" class="sc-row-icon" aria-hidden="true">{{ it.icon }}</span>
            <button type="button" class="name-link" @click="startEdit(it)">{{ it.name }}</button>
          </td>
          <td>{{ categoryLabel(it.category_id) }}</td>
          <td>{{ it.customer_id ? customerName(it.customer_id) : $t('catalogAdmin.all_customers') }}</td>
          <td>{{ it.approvals_required > 0 ? $t('catalogAdmin.approvals_summary', { n: it.approvals_required, group: groupName(it.approval_group_id) }) : $t('catalogAdmin.no_approval') }}</td>
          <td>{{ it.fields?.length || 0 }}</td>
          <td>{{ it.is_active ? $t('common.yes') : $t('common.no') }}</td>
          <td>
            <div class="actions-cell">
              <button class="btn btn-ghost btn-sm" @click="startEdit(it)" :aria-label="`${$t('common.edit')}: ${it.name}`">{{ $t('common.edit') }}</button>
              <button class="btn btn-ghost btn-sm btn-danger" @click="deleteItem(it)" :aria-label="`${$t('common.delete')}: ${it.name}`">{{ $t('common.delete') }}</button>
            </div>
          </td>
        </tr>
        <tr v-if="!items.length && !editing">
          <td colspan="7" style="text-align:center;color:var(--color-text-muted)">{{ $t('catalogAdmin.no_items') }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { catalogApi } from '@/api/itsm'
import { helpdeskApi } from '@/api/helpdesk'
import { groupsApi } from '@/api/groups'
import { customersApi } from '@/api/customers'
import { ticketChecklistsApi } from '@/api/ticketChecklists'
import { useUIStore } from '@/stores/ui'

const PRIORITIES = ['low', 'medium', 'high', 'critical']
const FIELD_TYPES = ['text', 'textarea', 'select', 'checkbox', 'date', 'number']
// Same rule as the backend (catalogFieldKeyRe).
const KEY_RE = /^[a-z][a-z0-9_]{0,49}$/

const { t } = useI18n()
const ui = useUIStore()
const loading = ref(true)
const saving = ref(false)
const items = ref([])
const groups = ref([])
const customers = ref([])
const categories = ref([])
const checklistTemplates = ref([])
const editing = ref(null)
const form = ref(emptyForm())
const nameEl = ref(null)
let fieldKey = 0

onMounted(async () => {
  await Promise.all([load(), loadLookups()])
})

function emptyForm() {
  return {
    name: '',
    description: '',
    icon: '',
    category_id: 0,
    customer_id: 0,
    default_priority: 'medium',
    group_id: 0,
    approval_group_id: 0,
    approvals_required: 0,
    checklist_template_id: 0,
    is_active: true,
    sort_order: 0,
    fields: [],
  }
}

async function load() {
  loading.value = true
  try { items.value = (await catalogApi.adminList()).data || [] } catch { items.value = [] }
  loading.value = false
}

async function loadLookups() {
  const settle = async (fn, target) => {
    try { target.value = (await fn()).data || [] } catch { target.value = [] }
  }
  await Promise.all([
    settle(() => groupsApi.list(), groups),
    settle(() => customersApi.list(), customers),
    settle(() => helpdeskApi.adminListCategories(), categories),
    settle(() => ticketChecklistsApi.adminList(), checklistTemplates),
  ])
}

// "Parent › Child" labels, each top-level category followed by its children.
const categoryOptions = computed(() => {
  const byId = Object.fromEntries(categories.value.map(c => [c.id, c]))
  const out = []
  const tops = categories.value.filter(c => !c.parent_id || !byId[c.parent_id])
  for (const top of tops) {
    out.push({ id: top.id, label: top.name })
    for (const child of categories.value.filter(c => c.parent_id === top.id)) {
      out.push({ id: child.id, label: `${top.name} › ${child.name}` })
    }
  }
  return out
})

function categoryLabel(id) {
  if (!id) return '—'
  return categoryOptions.value.find(c => c.id === id)?.label || `#${id}`
}
function customerName(id) {
  return customers.value.find(c => c.id === id)?.name || `#${id}`
}
function groupName(id) {
  if (!id) return '—'
  return groups.value.find(g => g.id === id)?.name || `#${id}`
}

const approvalInvalid = computed(() =>
  (Number(form.value.approvals_required) || 0) > 0 && !form.value.approval_group_id)

// ── Field builder ──────────────────────────────────────────────────────────

function parseOptions(text) {
  const raw = (text || '').trim()
  if (!raw) return []
  const parts = raw.includes('\n') ? raw.split('\n') : raw.split(',')
  const seen = new Set()
  const out = []
  for (const p of parts.map(s => s.trim()).filter(Boolean)) {
    if (!seen.has(p)) { seen.add(p); out.push(p) }
  }
  return out
}

function slugKey(label) {
  let k = (label || '')
    .normalize('NFD').replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
  if (!k) return ''
  if (!/^[a-z]/.test(k)) k = 'f_' + k
  return k.slice(0, 50).replace(/_+$/, '')
}

function uniqueKey(base, self) {
  const taken = new Set(form.value.fields.filter(f => f !== self).map(f => f.key))
  if (!taken.has(base)) return base
  for (let n = 2; ; n++) {
    const suffix = '_' + n
    const k = base.slice(0, 50 - suffix.length) + suffix
    if (!taken.has(k)) return k
  }
}

function onLabelInput(f) {
  if (f.keyTouched && f.key) return
  f.keyTouched = false
  const base = slugKey(f.label)
  f.key = base ? uniqueKey(base, f) : ''
}

function keyError(f, idx) {
  if (!f.key) return f.label ? t('catalogAdmin.key_required') : ''
  if (!KEY_RE.test(f.key)) return t('catalogAdmin.key_invalid')
  if (form.value.fields.some((o, i) => i !== idx && o.key === f.key)) return t('catalogAdmin.key_duplicate')
  return ''
}

function addField() {
  form.value.fields.push({ _key: ++fieldKey, key: '', label: '', type: 'text', required: false, optionsText: '', help: '', keyTouched: false })
  const k = fieldKey
  nextTick(() => document.getElementById('sc-f-label-' + k)?.focus())
}

function removeField(idx) {
  form.value.fields.splice(idx, 1)
}

function moveField(idx, dir) {
  const to = idx + dir
  const list = form.value.fields
  if (to < 0 || to >= list.length) return
  const [f] = list.splice(idx, 1)
  list.splice(to, 0, f)
  // Keep keyboard focus on the same button of the moved field.
  nextTick(() => {
    const row = document.getElementById('sc-f-label-' + f._key)?.closest('.sc-field-row')
    const btns = row?.querySelectorAll('.sc-field-move button')
    const btn = btns?.[dir < 0 ? 0 : 1]
    ;(btn && !btn.disabled ? btn : btns?.[dir < 0 ? 1 : 0])?.focus()
  })
}

// ── Form open / save ───────────────────────────────────────────────────────

function openForm(f, target) {
  form.value = f
  editing.value = target
  nextTick(() => nameEl.value?.focus())
}

function startCreate() {
  openForm(emptyForm(), { id: 0 })
}

function startEdit(it) {
  openForm({
    name: it.name || '',
    description: it.description || '',
    icon: it.icon || '',
    category_id: it.category_id || 0,
    customer_id: it.customer_id || 0,
    default_priority: it.default_priority || 'medium',
    group_id: it.group_id || 0,
    approval_group_id: it.approval_group_id || 0,
    approvals_required: it.approvals_required || 0,
    checklist_template_id: it.checklist_template_id || 0,
    is_active: !!it.is_active,
    sort_order: it.sort_order || 0,
    fields: (it.fields || []).map(f => ({
      _key: ++fieldKey,
      key: f.key,
      label: f.label || '',
      type: f.type || 'text',
      required: !!f.required,
      optionsText: (f.options || []).join('\n'),
      help: f.help || '',
      keyTouched: true,
    })),
  }, it)
}

function cancelEdit() {
  editing.value = null
}

function validate() {
  const f = form.value
  if (!f.name.trim()) return t('helpdeskAdmin.name_required')
  const n = Number(f.approvals_required)
  if (!Number.isInteger(n) || n < 0 || n > 20) return t('catalogAdmin.approvals_invalid')
  if (n > 0 && !f.approval_group_id) return t('catalogAdmin.approver_group_required')
  for (const [i, fld] of f.fields.entries()) {
    if (!fld.label.trim()) return t('catalogAdmin.field_label_required', { n: i + 1 })
    const err = keyError(fld, i) || (!fld.key ? t('catalogAdmin.key_required') : '')
    if (err) return `${t('catalogAdmin.field_n', { n: i + 1 })}: ${err}`
    if (fld.type === 'select' && !parseOptions(fld.optionsText).length) return t('catalogAdmin.options_required', { n: i + 1 })
  }
  return null
}

async function confirmSave() {
  const err = validate()
  if (err) {
    ui.error(err)
    return
  }
  const f = form.value
  const payload = {
    name: f.name.trim(),
    description: f.description,
    icon: f.icon.trim(),
    category_id: f.category_id || 0,
    customer_id: f.customer_id || 0,
    default_priority: f.default_priority,
    group_id: f.group_id || 0,
    approval_group_id: f.approval_group_id || 0,
    approvals_required: Number(f.approvals_required) || 0,
    checklist_template_id: f.checklist_template_id || 0,
    is_active: f.is_active,
    sort_order: Number(f.sort_order) || 0,
    fields: f.fields.map(fld => {
      const out = { key: fld.key, label: fld.label.trim(), type: fld.type, required: fld.required }
      if (fld.type === 'select') out.options = parseOptions(fld.optionsText)
      if (fld.help.trim()) out.help = fld.help.trim()
      return out
    }),
  }
  saving.value = true
  try {
    if (editing.value.id) await catalogApi.update(editing.value.id, payload)
    else await catalogApi.create(payload)
    editing.value = null
    await load()
    ui.success(t('catalogAdmin.saved'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.save_failed'))
  } finally {
    saving.value = false
  }
}

async function deleteItem(it) {
  if (!await ui.confirm(t('catalogAdmin.delete_confirm', { name: it.name }), { destructive: true })) return
  try {
    await catalogApi.remove(it.id)
    if (editing.value?.id === it.id) editing.value = null
    await load()
    ui.success(t('catalogAdmin.deleted'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.delete_failed'))
  }
}
</script>

<style scoped>
.sc-form-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}
.sc-form-title { font-size: 15px; font-weight: 600; margin: 0 0 16px; }
.sc-section-title { font-size: 13px; font-weight: 600; margin: 0; }
.sc-form-row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; margin-bottom: 12px; }
.sc-form-field { display: flex; flex-direction: column; gap: 4px; min-width: 120px; }
.sc-full { margin-bottom: 12px; }
.sc-check-field { justify-content: flex-end; align-self: flex-end; min-width: auto; padding-bottom: 6px; }
.sc-check { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }
.sc-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.sc-hint { font-size: 11px; color: var(--color-text-muted); }
.sc-hint-row { display: block; margin: -8px 0 12px; }
.sc-hint-block { display: block; margin: 0 0 16px; font-size: 12px; max-width: 72ch; }
.sc-error { color: var(--color-danger); }
.sc-icon-input { text-align: center; }
.sc-mono { font-family: var(--font-mono, monospace); }
.sc-fieldset { border: 1px solid var(--color-border); border-radius: 6px; padding: 8px 12px 10px; margin: 0 0 12px; }
.sc-fieldset legend { padding: 0 4px; }
.sc-fields { margin-top: 16px; }
.sc-fields-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 6px; }
.sc-empty-inline { font-size: 13px; color: var(--color-text-muted); padding: 8px 0; }
.sc-field-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 10px; }
.sc-field-row { border: 1px solid var(--color-border); border-radius: 6px; padding: 10px 12px; background: var(--color-bg); }
.sc-field-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.sc-field-num {
  font-size: 11px; font-weight: 700; color: var(--color-text-muted);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-radius: 10px; padding: 1px 8px;
}
.sc-field-move { display: flex; gap: 4px; }
.sc-preview { margin-top: 12px; border: 1px dashed var(--color-border); border-radius: 6px; padding: 8px 12px; }
.sc-preview summary { cursor: pointer; font-size: 13px; font-weight: 600; }
.sc-preview summary:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; border-radius: 2px; }
.sc-preview-body { padding-top: 10px; max-width: 480px; }
.sc-preview-field { margin-bottom: 10px; }
.sc-req { color: var(--color-danger); margin-left: 2px; }
.sc-row-icon { margin-right: 6px; }
.sc-inactive td { color: var(--color-text-muted); }
.sc-form-footer { display: flex; gap: 8px; margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.loading-state { display: flex; justify-content: center; padding: 48px; }

.tab-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
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
