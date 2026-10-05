<template>
  <div>
    <div class="tab-toolbar">
      <button class="btn btn-primary btn-sm" @click="startCreate">+ {{ $t('cmdbAdmin.new_type') }}</button>
    </div>
    <p class="ct-hint ct-hint-block">{{ $t('cmdbAdmin.hint') }}</p>

    <div v-if="editing" class="ct-form-card">
      <h3 class="ct-form-title">{{ editing.id ? $t('cmdbAdmin.edit_type') : $t('cmdbAdmin.new_type') }}</h3>
      <div class="ct-form-row">
        <div class="ct-form-field" style="width:90px;min-width:90px">
          <label class="ct-label" for="ct-icon">{{ $t('cmdbAdmin.icon') }}</label>
          <input id="ct-icon" class="form-input ct-icon-input" v-model="form.icon" maxlength="16" aria-describedby="ct-icon-hint" />
        </div>
        <div class="ct-form-field" style="flex:1;min-width:220px">
          <label class="ct-label" for="ct-name">{{ $t('helpdeskAdmin.name') }}</label>
          <input id="ct-name" ref="nameEl" class="form-input" v-model="form.name" maxlength="100" @keydown.enter="save" />
        </div>
        <div class="ct-form-field" style="width:110px">
          <label class="ct-label" for="ct-order">{{ $t('macro.sort_order') }}</label>
          <input id="ct-order" class="form-input" type="number" step="1" v-model.number="form.sort_order" />
        </div>
      </div>
      <small id="ct-icon-hint" class="ct-hint ct-hint-row">{{ $t('cmdbAdmin.icon_hint') }}</small>
      <div class="ct-form-footer">
        <button class="btn btn-primary btn-sm" @click="save" :disabled="saving">{{ $t('common.save') }}</button>
        <button class="btn btn-secondary btn-sm" @click="editing = null">{{ $t('common.cancel') }}</button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner" style="width:32px;height:32px;border-width:3px"></div>
    </div>
    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('helpdeskAdmin.name') }}</th>
          <th>{{ $t('macro.sort_order') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ty in types" :key="ty.id">
          <td>
            <span v-if="ty.icon" class="ct-row-icon" aria-hidden="true">{{ ty.icon }}</span>
            <button type="button" class="name-link" @click="startEdit(ty)">{{ ty.name }}</button>
          </td>
          <td>{{ ty.sort_order }}</td>
          <td>
            <div class="actions-cell">
              <button class="btn btn-ghost btn-sm" @click="startEdit(ty)" :aria-label="`${$t('common.edit')}: ${ty.name}`">{{ $t('common.edit') }}</button>
              <button class="btn btn-ghost btn-sm btn-danger" @click="remove(ty)" :aria-label="`${$t('common.delete')}: ${ty.name}`">{{ $t('common.delete') }}</button>
            </div>
          </td>
        </tr>
        <tr v-if="!types.length">
          <td colspan="3" style="text-align:center;color:var(--color-text-muted)">{{ $t('cmdbAdmin.no_types') }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { cmdbApi } from '@/api/itsm'
import { useUIStore } from '@/stores/ui'

const { t } = useI18n()
const ui = useUIStore()
const loading = ref(true)
const saving = ref(false)
const types = ref([])
const editing = ref(null)
const form = ref({ name: '', icon: '', sort_order: 0 })
const nameEl = ref(null)

onMounted(load)

async function load() {
  loading.value = true
  try { types.value = (await cmdbApi.types()).data || [] } catch { types.value = [] }
  loading.value = false
}

function openForm(f, target) {
  form.value = f
  editing.value = target
  nextTick(() => nameEl.value?.focus())
}

function startCreate() {
  const next = types.value.reduce((m, ty) => Math.max(m, ty.sort_order || 0), 0) + 10
  openForm({ name: '', icon: '', sort_order: types.value.length ? next : 0 }, { id: 0 })
}

function startEdit(ty) {
  openForm({ name: ty.name, icon: ty.icon || '', sort_order: ty.sort_order || 0 }, ty)
}

async function save() {
  const f = form.value
  if (!f.name.trim()) { ui.error(t('helpdeskAdmin.name_required')); return }
  const payload = { name: f.name.trim(), icon: f.icon.trim(), sort_order: Number(f.sort_order) || 0 }
  saving.value = true
  try {
    if (editing.value.id) await cmdbApi.updateType(editing.value.id, payload)
    else await cmdbApi.createType(payload)
    editing.value = null
    await load()
    ui.success(t('cmdbAdmin.saved'))
  } catch (e) {
    ui.error(e.response?.status === 409 ? t('cmdbAdmin.name_taken') : (e.response?.data?.error || t('helpdeskAdmin.save_failed')))
  } finally {
    saving.value = false
  }
}

async function remove(ty) {
  if (!await ui.confirm(t('cmdbAdmin.delete_confirm', { name: ty.name }), { destructive: true })) return
  try {
    await cmdbApi.deleteType(ty.id)
    if (editing.value?.id === ty.id) editing.value = null
    await load()
    ui.success(t('cmdbAdmin.deleted'))
  } catch (e) {
    if (e.response?.data?.code === 'ci_type_in_use') ui.error(t('cmdbAdmin.type_in_use', { name: ty.name }))
    else ui.error(e.response?.data?.error || t('helpdeskAdmin.delete_failed'))
  }
}
</script>

<style scoped>
.ct-form-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}
.ct-form-title { font-size: 15px; font-weight: 600; margin: 0 0 16px; }
.ct-form-row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; margin-bottom: 12px; }
.ct-form-field { display: flex; flex-direction: column; gap: 4px; min-width: 120px; }
.ct-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.ct-hint { font-size: 11px; color: var(--color-text-muted); }
.ct-hint-row { display: block; margin: -8px 0 12px; }
.ct-hint-block { display: block; margin: 0 0 16px; font-size: 12px; max-width: 72ch; }
.ct-icon-input { text-align: center; }
.ct-row-icon { margin-right: 6px; }
.ct-form-footer { display: flex; gap: 8px; margin-top: 8px; padding-top: 16px; border-top: 1px solid var(--color-border); }
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
