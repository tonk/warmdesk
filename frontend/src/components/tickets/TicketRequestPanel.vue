<template>
  <section v-if="visible" class="itil-panel request-panel" aria-labelledby="ticket-request-heading">
    <h4 id="ticket-request-heading">
      <span v-if="item?.icon" aria-hidden="true">{{ item.icon }}</span>
      {{ item ? $t('ticketRequest.item', { name: item.name }) : $t('ticketRequest.title') }}
    </h4>
    <p v-if="item?.description" class="itil-hint">{{ item.description }}</p>
    <p v-if="loaded && ticket.catalog_item_id && !item" class="itil-hint">{{ $t('ticketRequest.item_removed') }}</p>

    <dl v-if="rows.length" class="request-answers">
      <div v-for="r in rows" :key="r.key" class="request-row">
        <dt>{{ r.label }}</dt>
        <dd>{{ r.value }}</dd>
      </div>
    </dl>
    <p v-else class="itil-hint">{{ $t('ticketRequest.no_answers') }}</p>
  </section>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { catalogApi } from '@/api/itsm'
import { useDateFormat } from '@/composables/useDateFormat'

const props = defineProps({
  ticket: { type: Object, required: true },
  customerId: { type: Number, default: null },
  isInbox: { type: Boolean, default: false },
})
defineEmits(['updated'])

const { t } = useI18n()
const { formatDate } = useDateFormat()

const item = ref(null)
const loaded = ref(false)

const answers = computed(() => {
  const d = props.ticket?.request_data
  return d && typeof d === 'object' && !Array.isArray(d) ? d : {}
})
const visible = computed(() => !!props.ticket?.catalog_item_id || Object.keys(answers.value).length > 0)

function formatValue(value, field) {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'boolean') return value ? t('common.yes') : t('common.no')
  if (field?.type === 'checkbox' && (value === 'true' || value === 'false')) {
    return value === 'true' ? t('common.yes') : t('common.no')
  }
  if (Array.isArray(value)) return value.length ? value.join(', ') : '—'
  if (field?.type === 'date' && /^\d{4}-\d{2}-\d{2}$/.test(String(value))) {
    return formatDate(`${value}T12:00:00`)
  }
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

// Answers in the catalogue item's field order, labelled; keys the item no
// longer defines (or every key, when the item is gone) follow under their
// raw key.
const rows = computed(() => {
  const data = answers.value
  const out = []
  const seen = new Set()
  for (const f of item.value?.fields || []) {
    if (!(f.key in data)) continue
    seen.add(f.key)
    out.push({ key: f.key, label: f.label || f.key, value: formatValue(data[f.key], f) })
  }
  for (const key of Object.keys(data)) {
    if (seen.has(key)) continue
    out.push({ key, label: key, value: formatValue(data[key]) })
  }
  return out
})

async function loadItem() {
  item.value = null
  loaded.value = false
  const id = props.ticket?.catalog_item_id
  if (!id) return
  try {
    const params = props.customerId && !props.isInbox ? { customer_id: props.customerId } : {}
    const { data } = await catalogApi.list(params)
    item.value = (data || []).find(it => it.id === id) || null
  } catch {
    item.value = null
  } finally {
    loaded.value = true
  }
}

watch(() => [props.ticket?.catalog_item_id, props.customerId], loadItem, { immediate: true })
</script>

<style scoped>
.itil-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.itil-panel h4 { margin: 0 0 10px; font-size: 14px; display: flex; align-items: center; gap: 6px; }
.request-panel { border-left: 3px solid var(--color-primary); }
.itil-hint { font-size: 12px; color: var(--color-text-muted); margin: 0 0 8px; }
.request-answers { margin: 0; display: grid; grid-template-columns: minmax(120px, max-content) 1fr; gap: 6px 16px; font-size: 13px; }
.request-row { display: contents; }
.request-answers dt { color: var(--color-text-muted); font-weight: 600; }
.request-answers dd { margin: 0; white-space: pre-wrap; word-break: break-word; }
@media (max-width: 600px) {
  .request-answers { grid-template-columns: 1fr; gap: 2px; }
  .request-answers dd { margin-bottom: 8px; }
}
</style>
