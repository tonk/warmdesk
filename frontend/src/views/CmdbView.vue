<template>
  <main class="cm-main">
    <header class="cm-header">
      <div>
        <h1 class="cm-title">{{ $t('cmdb.title') }}</h1>
        <p class="cm-subtitle">{{ $t('cmdb.subtitle') }}</p>
      </div>
      <button v-if="!forbidden" type="button" class="btn btn-primary" @click="openCreate">+ {{ $t('cmdb.new_ci') }}</button>
    </header>

    <div v-if="!forbidden" class="cm-toolbar" role="search" :aria-label="$t('cmdb.filters')">
      <div class="cm-filter cm-filter-search">
        <label class="cm-filter-label" for="cm-search">{{ $t('common.search') }}</label>
        <input id="cm-search" v-model="filters.q" type="search" class="form-input" :placeholder="$t('cmdb.search_placeholder')" />
      </div>
      <div class="cm-filter">
        <label class="cm-filter-label" for="cm-type">{{ $t('cmdb.type') }}</label>
        <select id="cm-type" v-model="filters.type_id" class="form-input">
          <option value="">{{ $t('cmdb.all_types') }}</option>
          <option v-for="ty in types" :key="ty.id" :value="String(ty.id)">{{ ty.icon ? `${ty.icon} ` : '' }}{{ ty.name }}</option>
        </select>
      </div>
      <div class="cm-filter">
        <label class="cm-filter-label" for="cm-customer">{{ $t('cmdb.customer') }}</label>
        <select id="cm-customer" v-model="filters.customer_id" class="form-input">
          <option value="">{{ $t('cmdb.all_customers') }}</option>
          <option value="internal">{{ $t('cmdb.internal') }}</option>
          <option v-for="c in customers" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
        </select>
      </div>
      <div class="cm-filter">
        <label class="cm-filter-label" for="cm-status">{{ $t('common.status') }}</label>
        <select id="cm-status" v-model="filters.status" class="form-input">
          <option value="">{{ $t('cmdb.all_statuses') }}</option>
          <option v-for="s in STATUSES" :key="s" :value="s">{{ $t(`cmdb.status_${s}`) }}</option>
        </select>
      </div>
      <label class="cm-toggle">
        <input type="checkbox" v-model="filters.include_retired" :disabled="!!filters.status" />
        {{ $t('cmdb.include_retired') }}
      </label>
    </div>

    <div v-if="forbidden" class="cm-empty" role="alert">{{ $t('cmdb.forbidden') }}</div>

    <div v-else-if="loading && !cis.length" class="cm-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <template v-else>
      <p class="sr-only" aria-live="polite">{{ $t('cmdb.result_count', { n: cis.length }, cis.length) }}</p>
      <div v-if="!cis.length" class="cm-empty">{{ $t('cmdb.none') }}</div>
      <div v-else class="cm-table-wrap">
        <table class="cm-table">
          <caption class="sr-only">{{ $t('cmdb.title') }}</caption>
          <thead>
            <tr>
              <th scope="col">{{ $t('cmdb.name') }}</th>
              <th scope="col">{{ $t('cmdb.type') }}</th>
              <th scope="col">{{ $t('cmdb.customer') }}</th>
              <th scope="col">{{ $t('common.status') }}</th>
              <th scope="col">{{ $t('cmdb.environment') }}</th>
              <th scope="col">{{ $t('cmdb.criticality') }}</th>
              <th scope="col">{{ $t('cmdb.owner_group') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="ci in cis" :key="ci.id">
              <th scope="row" class="cm-name-cell">
                <RouterLink :to="`/cmdb/${ci.id}`" class="cm-link">{{ ci.name }}</RouterLink>
              </th>
              <td class="cm-nowrap"><span aria-hidden="true" class="cm-icon">{{ ci.type?.icon || '▫' }}</span> {{ ci.type?.name || '—' }}</td>
              <td>{{ ci.customer?.name || $t('cmdb.internal') }}</td>
              <td><span class="cm-badge" :class="`cm-st-${ci.status}`">{{ $t(`cmdb.status_${ci.status}`) }}</span></td>
              <td class="cm-nowrap">{{ $t(`cmdb.env_${ci.environment || 'none'}`) }}</td>
              <td>
                <span v-if="ci.criticality" class="cm-badge" :class="`cm-crit-${ci.criticality}`">{{ $t(`cmdb.crit_${ci.criticality}`) }}</span>
                <span v-else class="cm-muted">{{ $t('cmdb.crit_none') }}</span>
              </td>
              <td>
                <span v-if="ci.owner">{{ ci.owner.display_name || ci.owner.username }}</span>
                <span v-if="ci.owner && ci.group" class="cm-muted"> · </span>
                <span v-if="ci.group">{{ ci.group.name }}</span>
                <span v-if="!ci.owner && !ci.group" class="cm-muted">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <BaseModal v-if="showCreate" :title="$t('cmdb.new_ci')" width="640px" @close="showCreate = false">
      <CiForm form-id="cm-create-form" :show-actions="false" :saving="saving" @submit="create" />
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showCreate = false">{{ $t('common.cancel') }}</button>
        <button type="submit" form="cm-create-form" class="btn btn-primary" :disabled="saving">
          {{ saving ? $t('common.loading') : $t('common.create') }}
        </button>
      </template>
    </BaseModal>
  </main>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import BaseModal from '@/components/common/BaseModal.vue'
import CiForm from '@/components/cmdb/CiForm.vue'
import { cmdbApi } from '@/api/itsm'
import { useCustomersStore } from '@/stores/customers'
import { useUIStore } from '@/stores/ui'

const STATUSES = ['planned', 'active', 'maintenance', 'retired']

const { t } = useI18n()
const router = useRouter()
const ui = useUIStore()
const customersStore = useCustomersStore()

const cis = ref([])
const types = ref([])
const loading = ref(false)
const forbidden = ref(false)
const showCreate = ref(false)
const saving = ref(false)
const filters = ref({ q: '', type_id: '', customer_id: '', status: '', include_retired: false })

const customers = computed(() => [...customersStore.customers].sort((a, b) => a.name.localeCompare(b.name)))

let seq = 0
async function load() {
  const s = ++seq
  loading.value = true
  try {
    const params = {}
    const f = filters.value
    if (f.q.trim()) params.q = f.q.trim()
    if (f.type_id) params.type_id = f.type_id
    if (f.customer_id) params.customer_id = f.customer_id
    if (f.status) params.status = f.status
    else if (f.include_retired) params.include_retired = true
    const { data } = await cmdbApi.list(params)
    if (s !== seq) return
    cis.value = data || []
  } catch (e) {
    if (s !== seq) return
    if (e?.response?.status === 403) forbidden.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    if (s === seq) loading.value = false
  }
}

let debounce = null
watch(() => filters.value.q, () => {
  clearTimeout(debounce)
  debounce = setTimeout(load, 300)
})
watch(() => [filters.value.type_id, filters.value.customer_id, filters.value.status, filters.value.include_retired], load)
onUnmounted(() => clearTimeout(debounce))

function openCreate() {
  showCreate.value = true
}

async function create(payload) {
  saving.value = true
  try {
    const { data } = await cmdbApi.create(payload)
    ui.success(t('cmdb.created'))
    showCreate.value = false
    router.push(`/cmdb/${data.id}`)
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  if (!customersStore.customers.length) customersStore.fetchCustomers()
  load()
  try {
    const { data } = await cmdbApi.types()
    types.value = data || []
  } catch {}
})
</script>

<style scoped>
.cm-main { padding: 24px 32px; max-width: 1400px; margin: 0 auto; }
.cm-header { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 16px; }
.cm-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.cm-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.cm-toolbar {
  display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end;
  background: var(--color-surface); border: 1px solid var(--color-border);
  border-radius: var(--radius); padding: 12px 16px; margin-bottom: 16px;
}
.cm-filter { display: flex; flex-direction: column; gap: 4px; min-width: 150px; }
.cm-filter-search { flex: 1 1 240px; max-width: 380px; }
.cm-filter-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.cm-toggle { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; padding-bottom: 8px; }

.cm-loading { display: flex; justify-content: center; padding: 48px; }
.cm-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.cm-table-wrap { overflow-x: auto; }
.cm-table {
  width: 100%; border-collapse: collapse; font-size: 13px;
  background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius);
}
.cm-table th, .cm-table td { padding: 8px 12px; border-bottom: 1px solid var(--color-border); text-align: left; vertical-align: middle; }
.cm-table thead th { font-size: 12px; font-weight: 600; color: var(--color-text-muted); background: var(--color-bg); }
.cm-table tbody tr:last-child th, .cm-table tbody tr:last-child td { border-bottom: none; }
.cm-name-cell { font-weight: 600; }
.cm-link { color: var(--color-text); text-decoration: none; }
.cm-link:hover { color: var(--color-primary); text-decoration: underline; }
.cm-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.cm-nowrap { white-space: nowrap; }
.cm-muted { color: var(--color-text-muted); }

.cm-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px; white-space: nowrap;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
}
.cm-st-active { border-color: var(--color-success); }
.cm-st-maintenance { border-color: var(--color-warning); }
.cm-st-planned { border-color: var(--color-primary); }
.cm-st-retired { color: var(--color-text-muted); border-style: dashed; }
.cm-crit-high { border-color: var(--color-danger); }
.cm-crit-medium { border-color: var(--color-warning); }

@media (max-width: 640px) {
  .cm-main { padding: 16px; }
  .cm-filter { min-width: 0; flex: 1 1 140px; }
}
</style>
