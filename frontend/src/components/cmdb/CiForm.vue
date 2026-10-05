<template>
  <form :id="formId" class="cif" novalidate @submit.prevent="submit">
    <div class="form-row">
      <div class="form-group">
        <label class="form-label" :for="`${formId}-name`">{{ $t('cmdb.name') }} <span aria-hidden="true">*</span></label>
        <input :id="`${formId}-name`" ref="nameEl" v-model="form.name" class="form-input" type="text" required maxlength="200"
          :aria-invalid="nameError ? 'true' : 'false'" :aria-describedby="nameError ? `${formId}-name-err` : undefined" />
        <p v-if="nameError" :id="`${formId}-name-err`" class="cif-error">{{ $t('cmdb.name_required') }}</p>
      </div>
      <div class="form-group">
        <label class="form-label" :for="`${formId}-type`">{{ $t('cmdb.type') }} <span aria-hidden="true">*</span></label>
        <select :id="`${formId}-type`" v-model="form.type_id" class="form-input" required
          :aria-invalid="typeError ? 'true' : 'false'" :aria-describedby="typeError ? `${formId}-type-err` : undefined">
          <option :value="null" disabled>{{ $t('common.select') }}</option>
          <option v-for="ty in types" :key="ty.id" :value="ty.id">{{ ty.icon ? `${ty.icon} ` : '' }}{{ ty.name }}</option>
        </select>
        <p v-if="typeError" :id="`${formId}-type-err`" class="cif-error">{{ $t('cmdb.type_required') }}</p>
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label class="form-label" :for="`${formId}-customer`">{{ $t('cmdb.customer') }}</label>
        <select :id="`${formId}-customer`" v-model="form.customer_id" class="form-input" :aria-describedby="`${formId}-customer-hint`">
          <option :value="null">{{ $t('cmdb.internal') }}</option>
          <option v-for="c in customerOptions" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <p :id="`${formId}-customer-hint`" class="form-label-hint cif-hint">{{ $t('cmdb.customer_hint') }}</p>
      </div>
      <div class="form-group">
        <label class="form-label" :for="`${formId}-status`">{{ $t('common.status') }}</label>
        <select :id="`${formId}-status`" v-model="form.status" class="form-input">
          <option v-for="s in STATUSES" :key="s" :value="s">{{ $t(`cmdb.status_${s}`) }}</option>
        </select>
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label class="form-label" :for="`${formId}-env`">{{ $t('cmdb.environment') }}</label>
        <select :id="`${formId}-env`" v-model="form.environment" class="form-input">
          <option v-for="e in ENVIRONMENTS" :key="e || 'none'" :value="e">{{ $t(`cmdb.env_${e || 'none'}`) }}</option>
        </select>
      </div>
      <div class="form-group">
        <label class="form-label" :for="`${formId}-crit`">{{ $t('cmdb.criticality') }}</label>
        <select :id="`${formId}-crit`" v-model="form.criticality" class="form-input">
          <option v-for="c in CRITICALITIES" :key="c || 'none'" :value="c">{{ $t(`cmdb.crit_${c || 'none'}`) }}</option>
        </select>
      </div>
    </div>

    <div class="form-row" v-if="userOptions.length || groupOptions.length">
      <div class="form-group" v-if="userOptions.length">
        <label class="form-label" :for="`${formId}-owner`">{{ $t('cmdb.owner') }}</label>
        <select :id="`${formId}-owner`" v-model="form.owner_id" class="form-input">
          <option :value="null">{{ $t('cmdb.nobody') }}</option>
          <option v-for="u in userOptions" :key="u.id" :value="u.id">{{ u.name }}</option>
        </select>
      </div>
      <div class="form-group" v-if="groupOptions.length">
        <label class="form-label" :for="`${formId}-group`">{{ $t('cmdb.group') }}</label>
        <select :id="`${formId}-group`" v-model="form.group_id" class="form-input">
          <option :value="null">{{ $t('cmdb.no_group') }}</option>
          <option v-for="g in groupOptions" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
      </div>
    </div>

    <div class="form-group">
      <label class="form-label" :for="`${formId}-desc`">{{ $t('cmdb.description') }}</label>
      <textarea :id="`${formId}-desc`" v-model="form.description" class="form-input" rows="3"></textarea>
    </div>

    <fieldset class="cif-attrs">
      <legend class="form-label">{{ $t('cmdb.attributes') }}</legend>
      <p class="form-label-hint cif-hint">{{ $t('cmdb.attributes_hint') }}</p>
      <div v-for="(row, i) in form.attributes" :key="row.uid" class="cif-attr-row">
        <label class="sr-only" :for="`${formId}-ak-${row.uid}`">{{ $t('cmdb.attribute_key_n', { n: i + 1 }) }}</label>
        <input :id="`${formId}-ak-${row.uid}`" v-model="row.key" class="form-input" type="text" :placeholder="$t('cmdb.attribute_key')" />
        <label class="sr-only" :for="`${formId}-av-${row.uid}`">{{ $t('cmdb.attribute_value_n', { n: i + 1 }) }}</label>
        <input :id="`${formId}-av-${row.uid}`" v-model="row.value" class="form-input" type="text" :placeholder="$t('cmdb.attribute_value')" />
        <button type="button" class="btn btn-ghost btn-sm" :aria-label="$t('cmdb.remove_attribute', { key: row.key || (i + 1) })" @click="removeAttr(i)">✕</button>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" @click="addAttr">+ {{ $t('cmdb.add_attribute') }}</button>
    </fieldset>

    <div v-if="showActions" class="cif-actions">
      <button type="button" class="btn btn-secondary" @click="$emit('cancel')">{{ $t('common.cancel') }}</button>
      <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('common.loading') : (submitLabel || $t('common.save')) }}</button>
    </div>
  </form>
</template>

<script setup>
// Create / edit form for a configuration item. Emits submit(payload) with an
// API-ready body (customer_id/owner_id/group_id 0 = clear). Pass showActions
// false and put buttons with :form="formId" in a modal footer instead.
import { ref, computed, watch, onMounted } from 'vue'
import { cmdbApi } from '@/api/itsm'
import { groupsApi } from '@/api/groups'
import { messagesApi } from '@/api/messages'
import client from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useCustomersStore } from '@/stores/customers'

const props = defineProps({
  ci: { type: Object, default: null },
  formId: { type: String, default: 'ci-form' },
  saving: { type: Boolean, default: false },
  showActions: { type: Boolean, default: true },
  submitLabel: { type: String, default: '' },
})
const emit = defineEmits(['submit', 'cancel'])

const STATUSES = ['planned', 'active', 'maintenance', 'retired']
const ENVIRONMENTS = ['production', 'acceptance', 'test', 'development', '']
const CRITICALITIES = ['high', 'medium', 'low', '']

const auth = useAuthStore()
const customersStore = useCustomersStore()

const types = ref([])
const users = ref([])
const groups = ref([])
const nameEl = ref(null)
const nameError = ref(false)
const typeError = ref(false)

let uidSeq = 0
function attrRows(obj) {
  return Object.entries(obj || {}).map(([key, value]) => ({ uid: ++uidSeq, key, value: String(value ?? '') }))
}
function initial(ci) {
  return {
    name: ci?.name || '',
    type_id: ci?.type_id ?? null,
    customer_id: ci?.customer_id ?? null,
    status: ci?.status || 'active',
    environment: ci?.environment || '',
    criticality: ci?.criticality || '',
    owner_id: ci?.owner_id ?? null,
    group_id: ci?.group_id ?? null,
    description: ci?.description || '',
    attributes: attrRows(ci?.attributes),
  }
}
const form = ref(initial(props.ci))
watch(() => props.ci, ci => { form.value = initial(ci) })

const customerOptions = computed(() => {
  const list = [...customersStore.customers].sort((a, b) => a.name.localeCompare(b.name))
  // Keep the current customer selectable even if it isn't in the list.
  if (props.ci?.customer && !list.some(c => c.id === props.ci.customer.id)) list.unshift(props.ci.customer)
  return list
})

const userOptions = computed(() => {
  const list = users.value.map(u => ({ id: u.id, name: u.display_name || u.username }))
  const own = props.ci?.owner
  if (own && !list.some(u => u.id === own.id)) list.unshift({ id: own.id, name: own.display_name || own.username })
  return list
})

const groupOptions = computed(() => {
  const list = [...groups.value]
  const g = props.ci?.group
  if (g && !list.some(x => x.id === g.id)) list.unshift({ id: g.id, name: g.name })
  return list
})

function addAttr() {
  form.value.attributes.push({ uid: ++uidSeq, key: '', value: '' })
}
function removeAttr(i) {
  form.value.attributes.splice(i, 1)
}

// Non-admins have no global group list; fall back to the chosen customer's
// groups (only readable by customer managers) and otherwise hide the field.
async function loadGroups() {
  try {
    if (auth.isAdmin) {
      const { data } = await groupsApi.list()
      groups.value = (data || []).map(g => ({ id: g.id, name: g.name }))
    } else if (form.value.customer_id) {
      const { data } = await client.get(`/customers/${form.value.customer_id}/groups`)
      groups.value = (data || []).map(e => ({ id: e.group_id, name: e.group?.name || '' }))
    } else {
      groups.value = []
    }
  } catch {
    groups.value = []
  }
}
watch(() => form.value.customer_id, () => { if (!auth.isAdmin) loadGroups() })

function submit() {
  nameError.value = !form.value.name.trim()
  typeError.value = !form.value.type_id
  if (nameError.value || typeError.value) {
    nameEl.value?.focus()
    return
  }
  const attributes = {}
  for (const row of form.value.attributes) {
    const k = row.key.trim()
    if (k) attributes[k] = row.value
  }
  emit('submit', {
    name: form.value.name.trim(),
    type_id: form.value.type_id,
    customer_id: form.value.customer_id || 0,
    status: form.value.status,
    environment: form.value.environment,
    criticality: form.value.criticality,
    owner_id: form.value.owner_id || 0,
    group_id: form.value.group_id || 0,
    description: form.value.description,
    attributes,
  })
}

onMounted(async () => {
  if (!customersStore.customers.length) customersStore.fetchCustomers()
  loadGroups()
  const [typeRes, userRes] = await Promise.all([
    cmdbApi.types().catch(() => ({ data: [] })),
    messagesApi.listUsers().catch(() => ({ data: [] })),
  ])
  types.value = typeRes.data || []
  users.value = (userRes.data || [])
    .filter(u => u.global_role !== 'customer')
    .sort((a, b) => (a.display_name || a.username).localeCompare(b.display_name || b.username))
})

defineExpose({ focus: () => nameEl.value?.focus() })
</script>

<style scoped>
.cif-hint { margin: 4px 0 0; }
.cif-error { margin: 4px 0 0; font-size: 12px; color: var(--color-danger); }
.cif-error::before { content: '⚠ '; }
.cif-attrs { border: 1px solid var(--color-border); border-radius: var(--radius); padding: 10px 12px 12px; margin: 0 0 16px; }
.cif-attrs legend { padding: 0 4px; margin-bottom: 0; }
.cif-attrs .cif-hint { margin: 0 0 8px; }
.cif-attr-row { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; }
.cif-attr-row .form-input { flex: 1 1 0; min-width: 0; }
.cif-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
