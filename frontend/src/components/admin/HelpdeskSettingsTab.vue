<template>
  <div class="hd-settings">
    <div v-if="loading" class="loading-state">
      <div class="spinner" style="width:32px;height:32px;border-width:3px"></div>
    </div>

    <template v-else>
      <!-- General settings + priority matrix -->
      <section class="hd-card" aria-labelledby="hd-general-title">
        <h3 id="hd-general-title" class="hd-card-title">{{ $t('helpdeskAdmin.general') }}</h3>

        <div class="hd-form-row">
          <div class="hd-form-field">
            <label class="hd-label" for="hd-autoclose">{{ $t('helpdeskAdmin.resolved_autoclose_days') }}</label>
            <div class="hd-input-unit">
              <input id="hd-autoclose" class="form-input" type="number" min="0" max="365" v-model.number="settings.resolved_autoclose_days" aria-describedby="hd-autoclose-hint" />
              <span class="hd-unit">{{ $t('helpdeskAdmin.days') }}</span>
            </div>
            <small id="hd-autoclose-hint" class="hd-hint">{{ $t('helpdeskAdmin.resolved_autoclose_hint') }}</small>
          </div>
          <div class="hd-form-field" style="flex:1;min-width:200px">
            <label class="hd-label" for="hd-major-group">{{ $t('helpdeskAdmin.major_incident_group') }}</label>
            <select id="hd-major-group" class="form-input" v-model="settings.major_incident_group_id" aria-describedby="hd-major-group-hint">
              <option :value="0">{{ $t('sla.none') }}</option>
              <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
            </select>
            <small id="hd-major-group-hint" class="hd-hint">{{ $t('helpdeskAdmin.major_incident_group_hint') }}</small>
          </div>
        </div>

        <div class="hd-form-row">
          <label class="toggle-label">
            <input type="checkbox" v-model="settings.require_resolution_code" aria-describedby="hd-require-code-hint" />
            {{ $t('helpdeskAdmin.require_resolution_code') }}
          </label>
          <small id="hd-require-code-hint" class="hd-hint" style="align-self:center">{{ $t('helpdeskAdmin.require_resolution_code_hint') }}</small>
        </div>

        <h4 id="hd-matrix-title" class="hd-section-title">{{ $t('helpdeskAdmin.priority_matrix') }}</h4>
        <p id="hd-matrix-hint" class="hd-hint hd-hint-block">{{ $t('helpdeskAdmin.priority_matrix_hint') }}</p>
        <table class="data-table hd-matrix" aria-labelledby="hd-matrix-title" aria-describedby="hd-matrix-hint">
          <thead>
            <tr>
              <td class="hd-matrix-corner">
                <span class="hd-matrix-axis">{{ $t('ticket.impact') }} ↓ / {{ $t('ticket.urgency') }} →</span>
              </td>
              <th v-for="u in LEVELS" :key="u" scope="col">{{ $t('ticket.urgency') }}: {{ $t('ticket.urgency_' + u) }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="i in LEVELS" :key="i">
              <th scope="row">{{ $t('ticket.impact') }}: {{ $t('ticket.impact_' + i) }}</th>
              <td v-for="u in LEVELS" :key="u">
                <label class="sr-only" :for="`hd-matrix-${i}-${u}`">{{ $t('helpdeskAdmin.matrix_cell_label', { impact: $t('ticket.impact_' + i), urgency: $t('ticket.urgency_' + u) }) }}</label>
                <select :id="`hd-matrix-${i}-${u}`" class="form-input form-input-sm" v-model="settings.priority_matrix[`${i}:${u}`]">
                  <option v-for="p in PRIORITIES" :key="p" :value="p">{{ $t('ticket.priority_' + p) }}</option>
                </select>
              </td>
            </tr>
          </tbody>
        </table>

        <div class="hd-form-footer">
          <button class="btn btn-primary btn-sm" @click="saveSettings" :disabled="savingSettings">{{ $t('common.save') }}</button>
        </div>
      </section>

      <!-- Categories -->
      <section class="hd-card" aria-labelledby="hd-cat-title">
        <div class="hd-card-header">
          <h3 id="hd-cat-title" class="hd-card-title">{{ $t('helpdeskAdmin.categories') }}</h3>
          <button class="btn btn-primary btn-sm" @click="startCreateCategory">+ {{ $t('helpdeskAdmin.new_category') }}</button>
        </div>
        <p class="hd-hint hd-hint-block">{{ $t('helpdeskAdmin.categories_hint') }}</p>

        <div v-if="catEditing" class="hd-form-card">
          <h4 class="hd-section-title" style="margin-top:0">{{ catEditing.id ? $t('helpdeskAdmin.edit_category') : $t('helpdeskAdmin.new_category') }}</h4>
          <div class="hd-form-row">
            <div class="hd-form-field" style="flex:2">
              <label class="hd-label" for="hd-cat-name">{{ $t('helpdeskAdmin.name') }}</label>
              <input id="hd-cat-name" ref="catNameEl" class="form-input" v-model="catForm.name" @keydown.enter="saveCategory" @keydown.esc="catEditing = null" />
            </div>
            <div class="hd-form-field" style="flex:2">
              <label class="hd-label" for="hd-cat-parent">{{ $t('helpdeskAdmin.parent_category') }}</label>
              <select id="hd-cat-parent" class="form-input" v-model="catForm.parent_id" :disabled="catHasChildren" :aria-describedby="catHasChildren ? 'hd-cat-parent-hint' : undefined">
                <option :value="0">{{ $t('helpdeskAdmin.no_parent') }}</option>
                <option v-for="c in parentOptions" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>
              <small v-if="catHasChildren" id="hd-cat-parent-hint" class="hd-hint">{{ $t('helpdeskAdmin.parent_disabled_hint') }}</small>
            </div>
            <div class="hd-form-field" style="flex:0 0 90px;min-width:90px">
              <label class="hd-label" for="hd-cat-order">{{ $t('macro.sort_order') }}</label>
              <input id="hd-cat-order" class="form-input" type="number" min="0" v-model.number="catForm.sort_order" />
            </div>
            <div class="hd-form-field" style="align-self:flex-end;flex:0 0 auto;min-width:0">
              <label class="toggle-label">
                <input type="checkbox" v-model="catForm.is_active" />
                {{ $t('sla.active') }}
              </label>
            </div>
          </div>
          <div class="hd-form-footer">
            <button class="btn btn-primary btn-sm" @click="saveCategory" :disabled="savingCat">{{ $t('common.save') }}</button>
            <button class="btn btn-secondary btn-sm" @click="catEditing = null">{{ $t('common.cancel') }}</button>
          </div>
        </div>

        <table class="data-table">
          <thead>
            <tr>
              <th>{{ $t('helpdeskAdmin.name') }}</th>
              <th>{{ $t('macro.sort_order') }}</th>
              <th>{{ $t('sla.active') }}</th>
              <th>{{ $t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in categoryTree" :key="c.id">
              <td :class="{ 'hd-sub': c.parent_id }">
                <span v-if="c.parent_id" class="hd-sub-marker" aria-hidden="true">↳</span>
                <button type="button" class="name-link" @click="startEditCategory(c)">{{ c.name }}</button>
                <span v-if="c.parent_id" class="sr-only">({{ $t('helpdeskAdmin.subcategory_of', { parent: c.parentName }) }})</span>
              </td>
              <td>{{ c.sort_order }}</td>
              <td>
                <span :class="['badge', c.is_active ? 'badge-active' : 'badge-inactive']">
                  {{ c.is_active ? $t('sla.yes') : $t('sla.no') }}
                </span>
              </td>
              <td>
                <div class="actions-cell">
                  <button class="btn btn-ghost btn-sm" @click="startEditCategory(c)" :aria-label="`${$t('common.edit')}: ${c.name}`">{{ $t('common.edit') }}</button>
                  <button class="btn btn-ghost btn-sm btn-danger" @click="deleteCategory(c)" :aria-label="`${$t('common.delete')}: ${c.name}`">{{ $t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="!categoryTree.length">
              <td colspan="4" class="hd-empty">{{ $t('helpdeskAdmin.no_categories') }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <!-- Resolution codes -->
      <section class="hd-card" aria-labelledby="hd-rc-title">
        <div class="hd-card-header">
          <h3 id="hd-rc-title" class="hd-card-title">{{ $t('helpdeskAdmin.resolution_codes') }}</h3>
          <button class="btn btn-primary btn-sm" @click="startCreateCode">+ {{ $t('helpdeskAdmin.new_resolution_code') }}</button>
        </div>

        <div v-if="rcEditing" class="hd-form-card">
          <h4 class="hd-section-title" style="margin-top:0">{{ rcEditing.id ? $t('helpdeskAdmin.edit_resolution_code') : $t('helpdeskAdmin.new_resolution_code') }}</h4>
          <div class="hd-form-row">
            <div class="hd-form-field" style="flex:2">
              <label class="hd-label" for="hd-rc-name">{{ $t('helpdeskAdmin.name') }}</label>
              <input id="hd-rc-name" ref="rcNameEl" class="form-input" v-model="rcForm.name" @keydown.enter="saveCode" @keydown.esc="rcEditing = null" />
            </div>
            <div class="hd-form-field" style="flex:0 0 90px;min-width:90px">
              <label class="hd-label" for="hd-rc-order">{{ $t('macro.sort_order') }}</label>
              <input id="hd-rc-order" class="form-input" type="number" min="0" v-model.number="rcForm.sort_order" />
            </div>
            <div class="hd-form-field" style="align-self:flex-end;flex:0 0 auto;min-width:0">
              <label class="toggle-label">
                <input type="checkbox" v-model="rcForm.is_active" />
                {{ $t('sla.active') }}
              </label>
            </div>
          </div>
          <div class="hd-form-footer">
            <button class="btn btn-primary btn-sm" @click="saveCode" :disabled="savingRc">{{ $t('common.save') }}</button>
            <button class="btn btn-secondary btn-sm" @click="rcEditing = null">{{ $t('common.cancel') }}</button>
          </div>
        </div>

        <table class="data-table">
          <thead>
            <tr>
              <th>{{ $t('helpdeskAdmin.name') }}</th>
              <th>{{ $t('macro.sort_order') }}</th>
              <th>{{ $t('sla.active') }}</th>
              <th>{{ $t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in resolutionCodes" :key="r.id">
              <td><button type="button" class="name-link" @click="startEditCode(r)">{{ r.name }}</button></td>
              <td>{{ r.sort_order }}</td>
              <td>
                <span :class="['badge', r.is_active ? 'badge-active' : 'badge-inactive']">
                  {{ r.is_active ? $t('sla.yes') : $t('sla.no') }}
                </span>
              </td>
              <td>
                <div class="actions-cell">
                  <button class="btn btn-ghost btn-sm" @click="startEditCode(r)" :aria-label="`${$t('common.edit')}: ${r.name}`">{{ $t('common.edit') }}</button>
                  <button class="btn btn-ghost btn-sm btn-danger" @click="deleteCode(r)" :aria-label="`${$t('common.delete')}: ${r.name}`">{{ $t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="!resolutionCodes.length">
              <td colspan="4" class="hd-empty">{{ $t('helpdeskAdmin.no_resolution_codes') }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { helpdeskApi } from '@/api/helpdesk'
import { groupsApi } from '@/api/groups'
import { useUIStore } from '@/stores/ui'

// Rows (impact) and columns (urgency) of the matrix, most severe first.
const LEVELS = ['high', 'medium', 'low']
const PRIORITIES = ['low', 'medium', 'high', 'critical']
const DEFAULT_MATRIX = {
  'high:high': 'critical', 'high:medium': 'high', 'high:low': 'medium',
  'medium:high': 'high', 'medium:medium': 'medium', 'medium:low': 'low',
  'low:high': 'medium', 'low:medium': 'low', 'low:low': 'low',
}

const { t } = useI18n()
const ui = useUIStore()

const loading = ref(true)
const groups = ref([])
const settings = ref(normalizeSettings({}))
const savingSettings = ref(false)

const categories = ref([])
const catEditing = ref(null)
const catForm = ref(emptyCategory())
const savingCat = ref(false)
const catNameEl = ref(null)

const resolutionCodes = ref([])
const rcEditing = ref(null)
const rcForm = ref(emptyCode())
const savingRc = ref(false)
const rcNameEl = ref(null)

onMounted(async () => {
  await Promise.all([loadSettings(), loadGroups(), loadCategories(), loadCodes()])
  loading.value = false
})

function normalizeSettings(s) {
  return {
    priority_matrix: { ...DEFAULT_MATRIX, ...(s.priority_matrix || {}) },
    resolved_autoclose_days: s.resolved_autoclose_days ?? 5,
    require_resolution_code: !!s.require_resolution_code,
    major_incident_group_id: s.major_incident_group_id || 0,
  }
}

async function loadSettings() {
  try {
    const { data } = await helpdeskApi.getSettings()
    settings.value = normalizeSettings(data || {})
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.load_failed'))
  }
}

async function loadGroups() {
  try { groups.value = (await groupsApi.list()).data || [] } catch { groups.value = [] }
}

async function saveSettings() {
  const days = settings.value.resolved_autoclose_days
  if (!Number.isInteger(days) || days < 0 || days > 365) {
    ui.error(t('helpdeskAdmin.autoclose_invalid'))
    return
  }
  savingSettings.value = true
  try {
    const { data } = await helpdeskApi.updateSettings({
      priority_matrix: settings.value.priority_matrix,
      resolved_autoclose_days: days,
      require_resolution_code: settings.value.require_resolution_code,
      major_incident_group_id: Number(settings.value.major_incident_group_id) || 0,
    })
    if (data && data.priority_matrix) settings.value = normalizeSettings(data)
    ui.success(t('helpdeskAdmin.settings_saved'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.save_failed'))
  } finally {
    savingSettings.value = false
  }
}

// ── Categories ──────────────────────────────────────────────────────────────

function emptyCategory() {
  return { name: '', parent_id: 0, is_active: true, sort_order: 0 }
}

async function loadCategories() {
  try { categories.value = (await helpdeskApi.adminListCategories()).data || [] } catch { categories.value = [] }
}

// Top-level categories each followed by their subcategories. The backend
// returns them sorted by sort_order, name; that order is kept within a level.
const categoryTree = computed(() => {
  const byId = Object.fromEntries(categories.value.map(c => [c.id, c]))
  const tops = categories.value.filter(c => !c.parent_id || !byId[c.parent_id])
  const out = []
  for (const top of tops) {
    out.push({ ...top, parent_id: null })
    for (const child of categories.value.filter(c => c.parent_id === top.id)) {
      out.push({ ...child, parentName: top.name })
    }
  }
  return out
})

const catHasChildren = computed(() =>
  !!catEditing.value?.id && categories.value.some(c => c.parent_id === catEditing.value.id))

const parentOptions = computed(() =>
  categories.value.filter(c => !c.parent_id && c.id !== catEditing.value?.id))

function focusEl(elRef) {
  nextTick(() => elRef.value?.focus())
}

function startCreateCategory() {
  catForm.value = emptyCategory()
  catEditing.value = { id: 0 }
  focusEl(catNameEl)
}

function startEditCategory(c) {
  catForm.value = { name: c.name, parent_id: c.parent_id || 0, is_active: c.is_active, sort_order: c.sort_order }
  catEditing.value = c
  focusEl(catNameEl)
}

async function saveCategory() {
  const name = catForm.value.name.trim()
  if (!name) {
    ui.error(t('helpdeskAdmin.name_required'))
    return
  }
  const payload = {
    name,
    parent_id: Number(catForm.value.parent_id) || 0,
    is_active: !!catForm.value.is_active,
    sort_order: Number(catForm.value.sort_order) || 0,
  }
  savingCat.value = true
  try {
    if (catEditing.value.id) await helpdeskApi.updateCategory(catEditing.value.id, payload)
    else await helpdeskApi.createCategory(payload)
    catEditing.value = null
    await loadCategories()
    ui.success(t('helpdeskAdmin.category_saved'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.save_failed'))
  } finally {
    savingCat.value = false
  }
}

async function deleteCategory(c) {
  const childCount = categories.value.filter(x => x.parent_id === c.id).length
  const msg = childCount
    ? t('helpdeskAdmin.delete_category_with_children', { name: c.name, count: childCount })
    : t('helpdeskAdmin.delete_category_confirm', { name: c.name })
  if (!await ui.confirm(msg, { destructive: true })) return
  try {
    await helpdeskApi.deleteCategory(c.id)
    if (catEditing.value?.id === c.id) catEditing.value = null
    await loadCategories()
    ui.success(t('helpdeskAdmin.category_deleted'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.delete_failed'))
  }
}

// ── Resolution codes ────────────────────────────────────────────────────────

function emptyCode() {
  return { name: '', is_active: true, sort_order: 0 }
}

async function loadCodes() {
  try { resolutionCodes.value = (await helpdeskApi.adminListResolutionCodes()).data || [] } catch { resolutionCodes.value = [] }
}

function startCreateCode() {
  rcForm.value = emptyCode()
  rcEditing.value = { id: 0 }
  focusEl(rcNameEl)
}

function startEditCode(r) {
  rcForm.value = { name: r.name, is_active: r.is_active, sort_order: r.sort_order }
  rcEditing.value = r
  focusEl(rcNameEl)
}

async function saveCode() {
  const name = rcForm.value.name.trim()
  if (!name) {
    ui.error(t('helpdeskAdmin.name_required'))
    return
  }
  const payload = { name, is_active: !!rcForm.value.is_active, sort_order: Number(rcForm.value.sort_order) || 0 }
  savingRc.value = true
  try {
    if (rcEditing.value.id) await helpdeskApi.updateResolutionCode(rcEditing.value.id, payload)
    else await helpdeskApi.createResolutionCode(payload)
    rcEditing.value = null
    await loadCodes()
    ui.success(t('helpdeskAdmin.resolution_code_saved'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.save_failed'))
  } finally {
    savingRc.value = false
  }
}

async function deleteCode(r) {
  if (!await ui.confirm(t('helpdeskAdmin.delete_resolution_code_confirm', { name: r.name }), { destructive: true })) return
  try {
    await helpdeskApi.deleteResolutionCode(r.id)
    if (rcEditing.value?.id === r.id) rcEditing.value = null
    await loadCodes()
    ui.success(t('helpdeskAdmin.resolution_code_deleted'))
  } catch (e) {
    if (e.response?.status === 409 && e.response?.data?.code === 'resolution_code_in_use') {
      ui.error(t('helpdeskAdmin.resolution_code_in_use'))
    } else {
      ui.error(e.response?.data?.error || t('helpdeskAdmin.delete_failed'))
    }
  }
}
</script>

<style scoped>
.hd-settings { display: flex; flex-direction: column; gap: 20px; }
.hd-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 20px;
}
.hd-card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 8px; flex-wrap: wrap; }
.hd-card-title { font-size: 15px; font-weight: 600; margin: 0 0 12px; }
.hd-card-header .hd-card-title { margin: 0; }
.hd-section-title { font-size: 13px; font-weight: 600; margin: 20px 0 4px; }
.hd-form-card {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}
.hd-form-row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; margin-bottom: 12px; }
.hd-form-field { display: flex; flex-direction: column; gap: 4px; min-width: 120px; }
.hd-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.hd-input-unit { display: flex; align-items: center; gap: 6px; }
.hd-input-unit .form-input { width: 110px; }
.hd-unit { font-size: 12px; color: var(--color-text-muted); white-space: nowrap; }
.hd-hint { font-size: 11px; color: var(--color-text-muted); }
.hd-hint-block { display: block; margin: 0 0 12px; font-size: 12px; max-width: 72ch; }
.hd-form-footer { display: flex; gap: 8px; margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.hd-matrix { width: auto; max-width: 100%; }
.hd-matrix .form-input-sm { min-width: 120px; }
.hd-matrix-corner { background: var(--color-bg); }
.hd-matrix-axis { font-size: 11px; color: var(--color-text-muted); white-space: nowrap; }
.hd-sub { padding-left: 32px !important; }
.hd-sub-marker { color: var(--color-text-muted); margin-right: 6px; }
.hd-empty { text-align: center; color: var(--color-text-muted); }
.loading-state { display: flex; justify-content: center; padding: 48px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); border: 0; }

.data-table { width: 100%; border-collapse: collapse; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; border-bottom: 1px solid var(--color-border); font-size: 13px; vertical-align: middle; }
.data-table th { font-weight: 600; color: var(--color-text-muted); font-size: 12px; background: var(--color-bg); }
.actions-cell { display: flex; gap: 6px; flex-wrap: wrap; }
.badge-active { background: color-mix(in srgb, var(--color-success) 20%, transparent); color: var(--color-text); }
.badge-inactive { background: color-mix(in srgb, var(--color-danger) 20%, transparent); color: var(--color-text); }
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
