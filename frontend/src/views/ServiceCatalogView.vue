<template>
  <main class="sc-main">
    <header class="sc-header">
      <h1 class="sc-title">{{ $t('catalog.title') }}</h1>
      <p class="sc-subtitle">{{ $t('catalog.subtitle') }}</p>
    </header>

    <div v-if="loadingCustomers" class="sc-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <div v-else-if="!customers.length" class="sc-empty" role="alert">{{ $t('catalog.no_customers') }}</div>

    <template v-else>
      <div v-if="customers.length > 1" class="sc-customer">
        <label class="form-label" for="sc-customer-select">{{ $t('catalog.customer_label') }}</label>
        <select id="sc-customer-select" v-model="customerId" class="form-input sc-customer-select">
          <option value="">{{ $t('catalog.choose_customer') }}</option>
          <option v-for="c in customers" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
        </select>
      </div>

      <p v-if="!customerId" class="sc-empty">{{ $t('catalog.pick_customer_first') }}</p>

      <div v-else-if="loadingItems" class="sc-loading" role="status">
        <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
        <span class="sr-only">{{ $t('common.loading') }}</span>
      </div>

      <!-- Item list -->
      <template v-else-if="!selected">
        <div v-if="!items.length" class="sc-empty">{{ $t('catalog.none') }}</div>
        <ul v-else class="sc-grid">
          <li v-for="item in items" :key="item.id" class="sc-card">
            <div class="sc-card-head">
              <span class="sc-icon" aria-hidden="true">{{ item.icon || '📦' }}</span>
              <h2 :id="`sc-item-${item.id}`" class="sc-card-title">{{ item.name }}</h2>
            </div>
            <div v-if="item.description" class="sc-card-desc markdown-body" v-html="md(item.description)"></div>
            <p v-if="item.approvals_required > 0" class="sc-approval-note">
              <span aria-hidden="true">✔</span>
              {{ $t('catalog.needs_approval', { n: item.approvals_required }, item.approvals_required) }}
            </p>
            <button
              type="button"
              class="btn btn-primary btn-sm sc-card-btn"
              :aria-describedby="`sc-item-${item.id}`"
              @click="choose(item)"
            >{{ $t('catalog.request') }}</button>
          </li>
        </ul>
      </template>

      <!-- Order form -->
      <section v-else class="sc-form-wrap" aria-labelledby="sc-form-title">
        <button type="button" class="btn btn-ghost btn-sm sc-back" @click="unselect">
          <span aria-hidden="true">←</span> {{ $t('catalog.back_to_catalog') }}
        </button>
        <div class="sc-card-head">
          <span class="sc-icon" aria-hidden="true">{{ selected.icon || '📦' }}</span>
          <h2 id="sc-form-title" ref="formTitle" tabindex="-1" class="sc-form-title">{{ selected.name }}</h2>
        </div>
        <div v-if="selected.description" class="sc-card-desc markdown-body" v-html="md(selected.description)"></div>
        <p v-if="selected.approvals_required > 0" class="sc-approval-note sc-approval-block">
          {{ $t('catalog.needs_approval_long', { n: selected.approvals_required }, selected.approvals_required) }}
        </p>

        <form class="sc-form" novalidate @submit.prevent="submit">
          <p v-if="errorCount" class="sc-error-summary" role="alert">
            {{ $t('catalog.fix_errors', { n: errorCount }, errorCount) }}
          </p>

          <div v-for="f in fields" :key="f.key" class="form-group">
            <template v-if="f.type === 'checkbox'">
              <label class="sc-check">
                <input
                  :id="fieldId(f)"
                  v-model="answers[f.key]"
                  type="checkbox"
                  :aria-invalid="errors[f.key] ? 'true' : 'false'"
                  :aria-describedby="describedBy(f)"
                  :aria-required="f.required ? 'true' : undefined"
                />
                {{ f.label }} <span v-if="f.required" aria-hidden="true">*</span>
              </label>
            </template>
            <template v-else>
              <label v-if="f.type !== 'date'" class="form-label" :for="fieldId(f)">
                {{ f.label }} <span v-if="f.required" aria-hidden="true">*</span>
              </label>
              <span v-else :id="`${fieldId(f)}-label`" class="form-label">
                {{ f.label }} <span v-if="f.required" aria-hidden="true">*</span>
              </span>

              <textarea
                v-if="f.type === 'textarea'"
                :id="fieldId(f)"
                v-model="answers[f.key]"
                class="form-input"
                rows="4"
                :required="f.required"
                :aria-invalid="errors[f.key] ? 'true' : 'false'"
                :aria-describedby="describedBy(f)"
              ></textarea>
              <select
                v-else-if="f.type === 'select'"
                :id="fieldId(f)"
                v-model="answers[f.key]"
                class="form-input"
                :required="f.required"
                :aria-invalid="errors[f.key] ? 'true' : 'false'"
                :aria-describedby="describedBy(f)"
              >
                <option value="">{{ $t('common.select') }}</option>
                <option v-for="o in f.options || []" :key="o" :value="o">{{ o }}</option>
              </select>
              <div
                v-else-if="f.type === 'date'"
                :id="fieldId(f)"
                class="sc-date"
                :class="{ 'sc-date-invalid': errors[f.key] }"
                role="group"
                :aria-labelledby="`${fieldId(f)}-label`"
                :aria-describedby="describedBy(f)"
              >
                <DatePicker v-model="answers[f.key]" :label="f.label" />
              </div>
              <input
                v-else-if="f.type === 'number'"
                :id="fieldId(f)"
                v-model="answers[f.key]"
                type="number"
                step="any"
                inputmode="decimal"
                class="form-input"
                :required="f.required"
                :aria-invalid="errors[f.key] ? 'true' : 'false'"
                :aria-describedby="describedBy(f)"
              />
              <input
                v-else
                :id="fieldId(f)"
                v-model="answers[f.key]"
                type="text"
                class="form-input"
                :required="f.required"
                :aria-invalid="errors[f.key] ? 'true' : 'false'"
                :aria-describedby="describedBy(f)"
              />
            </template>
            <p v-if="f.help" :id="`${fieldId(f)}-help`" class="sc-help">{{ f.help }}</p>
            <p v-if="errors[f.key]" :id="`${fieldId(f)}-error`" class="sc-error">{{ errors[f.key] }}</p>
          </div>

          <div class="form-group">
            <label class="form-label" for="sc-title">{{ $t('ticket.title') }}</label>
            <input id="sc-title" v-model="title" type="text" class="form-input" maxlength="300" :placeholder="selected.name" aria-describedby="sc-title-help" />
            <p id="sc-title-help" class="sc-help">{{ $t('catalog.title_help') }}</p>
          </div>

          <div class="form-group">
            <label class="form-label" for="sc-description">{{ $t('ticket.description') }}</label>
            <textarea id="sc-description" v-model="description" class="form-input" rows="4" aria-describedby="sc-description-help"></textarea>
            <p id="sc-description-help" class="sc-help">{{ $t('catalog.description_help') }}</p>
          </div>

          <div class="sc-footer">
            <button type="button" class="btn btn-secondary" @click="unselect">{{ $t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="submitting">
              {{ submitting ? $t('common.loading') : $t('catalog.submit') }}
            </button>
          </div>
        </form>
      </section>
    </template>
  </main>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { catalogApi } from '@/api/itsm'
import { customersApi } from '@/api/customers'
import { useUIStore } from '@/stores/ui'
import DatePicker from '@/components/common/DatePicker.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const ui = useUIStore()

const customers = ref([])
const customerId = ref('')
const loadingCustomers = ref(true)
const items = ref([])
const loadingItems = ref(false)
const selected = ref(null)
const formTitle = ref(null)

const answers = reactive({})
const errors = reactive({})
const title = ref('')
const description = ref('')
const submitting = ref(false)

const fields = computed(() => (selected.value?.fields || []).filter(f => f && f.key))
const errorCount = computed(() => Object.values(errors).filter(Boolean).length)

function md(text) { return DOMPurify.sanitize(marked.parse(text || '')) }
function fieldId(f) { return `sc-field-${f.key}` }
function describedBy(f) {
  const ids = []
  if (f.help) ids.push(`${fieldId(f)}-help`)
  if (errors[f.key]) ids.push(`${fieldId(f)}-error`)
  return ids.length ? ids.join(' ') : undefined
}

async function loadCustomers() {
  loadingCustomers.value = true
  try {
    const { data } = await customersApi.list()
    customers.value = Array.isArray(data) ? data : []
    const pre = typeof route.query.customer_id === 'string' ? route.query.customer_id : ''
    if (pre && customers.value.some(c => String(c.id) === pre)) customerId.value = pre
    else if (customers.value.length === 1) customerId.value = String(customers.value[0].id)
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    loadingCustomers.value = false
  }
}

let seq = 0
async function loadItems() {
  const my = ++seq
  items.value = []
  selected.value = null
  if (!customerId.value) return
  loadingItems.value = true
  try {
    const { data } = await catalogApi.list({ customer_id: customerId.value })
    if (my !== seq) return
    items.value = data || []
  } catch (e) {
    if (my !== seq) return
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    if (my === seq) loadingItems.value = false
  }
}

function resetForm() {
  for (const k of Object.keys(answers)) delete answers[k]
  for (const k of Object.keys(errors)) delete errors[k]
  title.value = ''
  description.value = ''
}

async function choose(item) {
  resetForm()
  for (const f of item.fields || []) {
    if (!f?.key) continue
    answers[f.key] = f.type === 'checkbox' ? false : (f.type === 'date' ? null : '')
  }
  selected.value = item
  await nextTick()
  formTitle.value?.focus()
}

function unselect() {
  selected.value = null
  resetForm()
}

function isEmpty(f, v) {
  if (f.type === 'checkbox') return v !== true
  return v === null || v === undefined || String(v).trim() === ''
}

function validate() {
  for (const k of Object.keys(errors)) delete errors[k]
  for (const f of fields.value) {
    const v = answers[f.key]
    if (f.required && isEmpty(f, v)) {
      errors[f.key] = f.type === 'checkbox'
        ? t('catalog.must_check', { field: f.label })
        : t('catalog.field_required', { field: f.label })
    } else if (f.type === 'number' && !isEmpty(f, v) && Number.isNaN(Number(v))) {
      errors[f.key] = t('catalog.must_be_number', { field: f.label })
    }
  }
  return errorCount.value === 0
}

function buildAnswers() {
  const out = {}
  for (const f of fields.value) {
    const v = answers[f.key]
    if (f.type === 'checkbox') out[f.key] = v === true
    else if (f.type === 'number') { if (!isEmpty(f, v)) out[f.key] = Number(v) }
    else if (!isEmpty(f, v)) out[f.key] = String(v).trim()
  }
  return out
}

async function submit() {
  if (!validate()) {
    await nextTick()
    const first = fields.value.find(f => errors[f.key])
    const el = first && document.getElementById(fieldId(first))
    const focusable = el?.matches('input, select, textarea') ? el : el?.querySelector('[tabindex], input, select, textarea, button')
    focusable?.focus()
    return
  }
  submitting.value = true
  try {
    const cid = Number(customerId.value)
    const { data } = await catalogApi.order(cid, {
      catalog_item_id: selected.value.id,
      title: title.value.trim() || undefined,
      description: description.value.trim() || undefined,
      answers: buildAnswers(),
    })
    ui.success(t('catalog.ordered', { id: data.id }))
    router.push(`/customers/${data.customer_id || cid}/tickets/${data.id}`)
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    submitting.value = false
  }
}

watch(customerId, (v) => {
  if (String(route.query.customer_id || '') !== v) {
    router.replace({ query: { ...route.query, customer_id: v || undefined } })
  }
  loadItems()
})

onMounted(loadCustomers)
</script>

<style scoped>
.sc-main { padding: 24px 32px; max-width: 1200px; margin: 0 auto; }
.sc-header { margin-bottom: 16px; }
.sc-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.sc-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.sc-customer { margin-bottom: 16px; max-width: 360px; }
.sc-loading { display: flex; justify-content: center; padding: 48px; }
.sc-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.sc-grid {
  list-style: none; margin: 0; padding: 0;
  display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 12px;
}
.sc-card {
  display: flex; flex-direction: column; gap: 8px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 14px 16px;
}
.sc-card:focus-within { border-color: var(--color-primary); }
.sc-card-head { display: flex; align-items: center; gap: 10px; }
.sc-icon {
  font-size: 22px; line-height: 1; width: 40px; height: 40px;
  display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0;
  border-radius: var(--radius);
  background: color-mix(in srgb, var(--color-primary) 10%, var(--color-surface));
}
.sc-card-title { font-size: 15px; font-weight: 700; margin: 0; overflow-wrap: anywhere; }
.sc-card-desc { font-size: 13px; color: var(--color-text); overflow-wrap: anywhere; flex: 1; }
.sc-approval-note { margin: 0; font-size: 12px; color: var(--color-text-muted); }
.sc-approval-block {
  padding: 8px 12px; margin: 8px 0 0;
  border-left: 3px solid var(--color-warning);
  background: color-mix(in srgb, var(--color-warning) 10%, var(--color-surface));
  border-radius: var(--radius-sm); color: var(--color-text);
}
.sc-card-btn { align-self: flex-start; }

.sc-form-wrap {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 16px 24px 20px;
  max-width: 760px;
}
.sc-back { margin-bottom: 8px; }
.sc-form-title { font-size: 18px; font-weight: 700; margin: 0; }
.sc-form-title:focus { outline: none; }
.sc-form-title:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.sc-form { margin-top: 16px; }
.sc-check { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 500; cursor: pointer; }
.sc-help { margin: 4px 0 0; font-size: 12px; color: var(--color-text-muted); }
.sc-error { margin: 4px 0 0; font-size: 12px; color: var(--color-danger); font-weight: 600; }
.sc-error-summary {
  margin: 0 0 16px; padding: 8px 12px; font-size: 13px; font-weight: 600;
  border-left: 3px solid var(--color-danger);
  background: color-mix(in srgb, var(--color-danger) 8%, var(--color-surface));
  border-radius: var(--radius-sm);
}
.form-input[aria-invalid='true'] { border-color: var(--color-danger); }
.sc-date { max-width: 240px; }
.sc-date-invalid { outline: 1px solid var(--color-danger); border-radius: var(--radius-sm); }
.sc-footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }

@media (max-width: 640px) {
  .sc-main { padding: 16px; }
  .sc-form-wrap { padding: 12px 16px 16px; }
}
</style>
