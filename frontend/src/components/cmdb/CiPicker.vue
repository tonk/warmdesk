<template>
  <div class="cip" ref="rootEl">
    <label :for="inputId" :class="hideLabel ? 'sr-only' : 'form-label'">{{ label || $t('cmdb.picker_label') }}</label>
    <input
      :id="inputId"
      ref="inputEl"
      v-model="query"
      type="text"
      class="form-input"
      role="combobox"
      autocomplete="off"
      aria-autocomplete="list"
      :aria-expanded="open ? 'true' : 'false'"
      :aria-controls="listId"
      :aria-activedescendant="open && activeIndex >= 0 ? `${listId}-${activeIndex}` : undefined"
      :aria-describedby="hintId"
      :placeholder="$t('cmdb.picker_placeholder')"
      :disabled="disabled"
      @input="onInput"
      @focus="onFocus"
      @keydown="onKeydown"
      @blur="onBlur"
    />
    <span :id="hintId" class="sr-only">{{ $t('cmdb.picker_hint') }}</span>
    <ul
      v-show="open"
      :id="listId"
      class="cip-list"
      role="listbox"
      :aria-label="label || $t('cmdb.picker_label')"
    >
      <li v-if="loading" class="cip-empty" role="presentation">{{ $t('common.loading') }}</li>
      <li v-else-if="!results.length" class="cip-empty" role="presentation">{{ $t('common.no_results') }}</li>
      <li
        v-for="(ci, i) in results"
        v-else
        :id="`${listId}-${i}`"
        :key="ci.id"
        role="option"
        :aria-selected="i === activeIndex ? 'true' : 'false'"
        class="cip-option"
        :class="{ 'cip-active': i === activeIndex }"
        @mousedown.prevent="choose(ci)"
        @mouseenter="activeIndex = i"
      >
        <span class="cip-icon" aria-hidden="true">{{ ci.type?.icon || '▫' }}</span>
        <span class="cip-name">{{ ci.name }}</span>
        <span class="cip-meta">{{ ci.type?.name }} · {{ ci.customer?.name || $t('cmdb.internal') }}</span>
      </li>
    </ul>
    <span class="sr-only" aria-live="polite">{{ liveText }}</span>
  </div>
</template>

<script setup>
// Reusable search picker for configuration items (WAI-ARIA combobox).
//
// Props:
//   modelValue  – id of the selected CI (optional; v-model)
//   excludeIds  – CI ids that must not be offered (e.g. the current CI)
//   label       – accessible label (defaults to "Configuration item")
//   hideLabel   – render the label visually hidden
//   customerId  – optional customer filter passed to the API ('internal' or id)
//   disabled
// Emits:
//   select(ci)              – the full CI object that was picked
//   update:modelValue(id)
import { ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { cmdbApi } from '@/api/itsm'

const props = defineProps({
  modelValue: { type: [Number, null], default: null },
  excludeIds: { type: Array, default: () => [] },
  label: { type: String, default: '' },
  hideLabel: { type: Boolean, default: false },
  customerId: { type: [Number, String, null], default: null },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['select', 'update:modelValue'])
const { t } = useI18n()

const uid = Math.random().toString(36).slice(2, 8)
const inputId = `cip-input-${uid}`
const listId = `cip-list-${uid}`
const hintId = `cip-hint-${uid}`

const rootEl = ref(null)
const inputEl = ref(null)
const query = ref('')
const raw = ref([])
const loading = ref(false)
const open = ref(false)
const activeIndex = ref(-1)
const selectedName = ref('')

const results = computed(() => {
  const ex = new Set(props.excludeIds.map(Number))
  return raw.value.filter(ci => !ex.has(ci.id)).slice(0, 50)
})
const liveText = computed(() => {
  if (!open.value || loading.value) return ''
  return t('cmdb.picker_results', { n: results.value.length }, results.value.length)
})

let seq = 0
async function search() {
  const s = ++seq
  loading.value = true
  try {
    const params = {}
    if (query.value.trim() && query.value !== selectedName.value) params.q = query.value.trim()
    if (props.customerId != null && props.customerId !== '') params.customer_id = props.customerId
    const { data } = await cmdbApi.list(params)
    if (s !== seq) return
    raw.value = data || []
    activeIndex.value = results.value.length ? 0 : -1
  } catch {
    if (s === seq) raw.value = []
  } finally {
    if (s === seq) loading.value = false
  }
}

let timer = null
function onInput() {
  open.value = true
  clearTimeout(timer)
  timer = setTimeout(search, 250)
}
function onFocus() {
  if (!open.value) {
    open.value = true
    search()
  }
}
function onBlur() {
  // Delay so a click on an option (mousedown.prevent) still registers.
  setTimeout(() => { open.value = false }, 120)
}
onUnmounted(() => clearTimeout(timer))

function choose(ci) {
  selectedName.value = ci.name
  query.value = ci.name
  open.value = false
  emit('update:modelValue', ci.id)
  emit('select', ci)
}

function onKeydown(e) {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (!open.value) { open.value = true; search(); return }
    if (results.value.length) activeIndex.value = (activeIndex.value + 1) % results.value.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (results.value.length) activeIndex.value = (activeIndex.value - 1 + results.value.length) % results.value.length
  } else if (e.key === 'Enter') {
    if (open.value && activeIndex.value >= 0 && results.value[activeIndex.value]) {
      e.preventDefault()
      choose(results.value[activeIndex.value])
    }
  } else if (e.key === 'Escape') {
    if (open.value) {
      e.preventDefault()
      e.stopPropagation()
      open.value = false
    }
  }
}

// Clearing modelValue from outside (e.g. after the parent saved) resets the input.
watch(() => props.modelValue, v => {
  if (v == null) { query.value = ''; selectedName.value = '' }
})

defineExpose({ focus: () => inputEl.value?.focus() })
</script>

<style scoped>
.cip { position: relative; }
.cip-list {
  position: absolute; z-index: 50; left: 0; right: 0; top: 100%;
  margin: 4px 0 0; padding: 4px 0; list-style: none;
  max-height: 280px; overflow-y: auto;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  box-shadow: 0 6px 20px color-mix(in srgb, var(--color-text) 15%, transparent);
}
.cip-option {
  display: flex; align-items: baseline; gap: 8px;
  padding: 6px 10px; cursor: pointer; font-size: 13px;
}
.cip-active { background: color-mix(in srgb, var(--color-primary) 14%, transparent); }
.cip-icon { flex: 0 0 auto; }
.cip-name { font-weight: 600; color: var(--color-text); overflow-wrap: anywhere; }
.cip-meta { margin-left: auto; font-size: 11px; color: var(--color-text-muted); white-space: nowrap; }
.cip-empty { padding: 8px 10px; font-size: 13px; color: var(--color-text-muted); }
</style>
