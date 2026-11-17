<template>
  <BaseModal :title="$t('timeOverview.settings_title', { year })" @close="$emit('close')">
    <p v-if="loading" role="status" aria-live="polite">{{ $t('common.loading') }}</p>
    <p v-else-if="error" class="ov-error" role="alert">{{ error }}</p>
    <form v-else id="ov-settings-form" @submit.prevent="save">
      <div class="ov-field">
        <label for="ovs-hpw">{{ $t('timeOverview.hours_per_week') }}</label>
        <input id="ovs-hpw" v-model="form.hours_per_week" class="ov-input" type="number" min="1" max="168" step="0.25" required />
      </div>
      <div class="ov-field">
        <label for="ovs-vac">{{ $t('timeOverview.vacation_hours') }}</label>
        <input id="ovs-vac" v-model="form.vacation_hours" class="ov-input" type="number" min="0" max="2000" step="0.25" />
      </div>
      <div class="ov-field">
        <label for="ovs-carry">{{ $t('timeOverview.carry_over_label', { year }) }}</label>
        <input id="ovs-carry" v-model="form.carry_over_hours" class="ov-input" type="number" min="-2000" max="2000" step="0.25" />
      </div>

      <fieldset class="ovs-cats">
        <legend>{{ $t('timeOverview.categories') }}</legend>
        <p v-if="autoCategories" class="ov-muted">{{ $t('timeOverview.auto_categories_note') }}</p>
        <p class="ov-muted">{{ $t('timeOverview.categories_hint') }}</p>
        <div v-for="cat in cats" :key="cat" class="ov-field">
          <label :for="'ovs-cat-' + cat">{{ $t('timeOverview.cat_' + cat) }}</label>
          <select :id="'ovs-cat-' + cat" v-model="form.categories[cat]" class="ov-select" multiple size="5">
            <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}<template v-if="p.customer_name"> ({{ p.customer_name }})</template></option>
          </select>
        </div>
        <button type="button" class="ov-link" @click="resetCategories">{{ $t('timeOverview.reset_categories') }}</button>
      </fieldset>
    </form>
    <template #footer>
      <button type="button" class="ov-btn" @click="$emit('close')">{{ $t('common.cancel') }}</button>
      <button type="submit" form="ov-settings-form" class="ov-btn ov-btn-primary" :disabled="loading || !!error || saving">{{ $t('common.save') }}</button>
    </template>
  </BaseModal>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseModal from '@/components/common/BaseModal.vue'
import { useUIStore } from '@/stores/ui'
import { timeEntriesApi } from '@/api/timeEntries'
import { timerApi } from '@/api/timer'
import './overview.css'

const props = defineProps({ year: { type: Number, required: true }, userId: { type: Number, default: 0 } })
const emit = defineEmits(['close', 'saved'])
const { t } = useI18n()
const ui = useUIStore()

const cats = ['sick', 'vacation', 'travel']
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const autoCategories = ref(false)
const projects = ref([])
const form = reactive({ hours_per_week: 40, vacation_hours: 0, carry_over_hours: '', categories: { sick: [], vacation: [], travel: [] } })

const params = () => ({ year: props.year, ...(props.userId ? { user_id: props.userId } : {}) })

onMounted(async () => {
  try {
    const [{ data: s }, { data: targets }] = await Promise.all([
      timeEntriesApi.getOverviewSettings(params()),
      timerApi.targets().catch(() => ({ data: { projects: [] } })),
    ])
    form.hours_per_week = s.hours_per_week
    form.vacation_hours = s.vacation_hours
    form.carry_over_hours = s.carry_over_hours
    for (const c of cats) form.categories[c] = [...(s.categories?.[c] || [])]
    autoCategories.value = !!s.auto_categories
    const list = [...(targets.projects || [])]
    // keep projects that are mapped but no longer a timer target selectable
    const known = new Set(list.map(p => p.id))
    for (const c of cats) for (const id of form.categories[c]) if (!known.has(id)) { known.add(id); list.push({ id, name: '#' + id }) }
    projects.value = list.sort((a, b) => a.name.localeCompare(b.name))
  } catch {
    error.value = t('timeOverview.load_error')
  } finally {
    loading.value = false
  }
})

function resetCategories() {
  for (const c of cats) form.categories[c] = []
}

function num(v) {
  if (v === '' || v == null) return null
  const n = parseFloat(String(v).replace(',', '.'))
  return Number.isFinite(n) ? n : null
}

async function save() {
  saving.value = true
  try {
    const body = {
      year: props.year,
      hours_per_week: num(form.hours_per_week),
      vacation_hours: num(form.vacation_hours),
      carry_over_hours: num(form.carry_over_hours),
      categories: Object.fromEntries(cats.map(c => [c, form.categories[c]]).filter(([, ids]) => ids.length)),
    }
    await timeEntriesApi.setOverviewSettings(body, props.userId ? { user_id: props.userId } : undefined)
    ui.success(t('timeOverview.settings_saved'))
    emit('saved')
    emit('close')
  } catch {
    ui.error(t('timeOverview.save_error'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.ovs-cats { border: 1px solid var(--color-border); border-radius: 8px; padding: 8px 12px; color: var(--color-text); }
.ovs-cats legend { font-weight: 600; font-size: 13px; padding: 0 6px; }
</style>
