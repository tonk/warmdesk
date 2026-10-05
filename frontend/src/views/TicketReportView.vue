<template>
  <main class="tr-main">
    <header class="tr-header">
      <h1 class="tr-title">{{ $t('ticketReport.title') }}</h1>
      <p class="tr-subtitle">{{ $t('ticketReport.subtitle') }}</p>
    </header>

    <!-- Filters -->
    <form class="tr-filters" @submit.prevent="loadReport" :aria-label="$t('ticketReport.filters')">
      <div class="tr-filter-row">
        <div class="tr-filter">
          <span class="tr-filter-label">{{ $t('ticketReport.from') }}</span>
          <DatePicker v-model="filters.from" :label="$t('ticketReport.from')" />
        </div>
        <div class="tr-filter">
          <span class="tr-filter-label">{{ $t('ticketReport.to') }}</span>
          <DatePicker v-model="filters.to" :label="$t('ticketReport.to')" />
        </div>
        <div class="tr-filter">
          <label class="tr-filter-label" for="tr-customer">{{ $t('ticketReport.customer') }}</label>
          <select id="tr-customer" class="form-input" v-model="filters.customer_id">
            <option value="">{{ $t('ticketReport.all_customers') }}</option>
            <option v-for="c in customers" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
          </select>
        </div>
        <div class="tr-filter" v-if="groups.length">
          <label class="tr-filter-label" for="tr-group">{{ $t('ticket.group') }}</label>
          <select id="tr-group" class="form-input" v-model="filters.group_id">
            <option value="">{{ $t('ticketReport.all_groups') }}</option>
            <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}</option>
          </select>
        </div>
        <div class="tr-filter">
          <label class="tr-filter-label" for="tr-assignee">{{ $t('ticket.assigned_to') }}</label>
          <select id="tr-assignee" class="form-input" v-model="filters.assigned_to_id">
            <option value="">{{ $t('ticketReport.all_assignees') }}</option>
            <option v-for="u in users" :key="u.id" :value="String(u.id)">{{ u.display_name || u.username }}</option>
          </select>
        </div>
        <div class="tr-filter">
          <label class="tr-filter-label" for="tr-type">{{ $t('ticketReport.type') }}</label>
          <select id="tr-type" class="form-input" v-model="filters.type">
            <option value="">{{ $t('ticketReport.all_types') }}</option>
            <option v-for="ty in TYPES" :key="ty" :value="ty">{{ $t(`ticket.type_${ty}`) }}</option>
          </select>
        </div>
        <div class="tr-filter">
          <label class="tr-filter-label" for="tr-priority">{{ $t('ticketReport.priority') }}</label>
          <select id="tr-priority" class="form-input" v-model="filters.priority">
            <option value="">{{ $t('ticketReport.all_priorities') }}</option>
            <option v-for="p in PRIORITIES" :key="p" :value="p">{{ $t(`ticket.priority_${p}`) }}</option>
          </select>
        </div>
        <div class="tr-filter" v-if="categoryOptions.length">
          <label class="tr-filter-label" for="tr-category">{{ $t('ticketReport.category') }}</label>
          <select id="tr-category" class="form-input" v-model="filters.category_id">
            <option value="">{{ $t('ticketReport.all_categories') }}</option>
            <option v-for="c in categoryOptions" :key="c.id" :value="String(c.id)">{{ c.label }}</option>
          </select>
        </div>
        <div class="tr-filter tr-filter-actions">
          <button type="submit" class="btn btn-primary" :disabled="loading">
            {{ loading ? $t('common.loading') : $t('ticketReport.run') }}
          </button>
          <button type="button" class="btn btn-secondary" @click="resetFilters">{{ $t('ticketReport.reset') }}</button>
        </div>
      </div>

      <div class="tr-export-row" v-if="report">
        <div class="tr-filter">
          <label class="tr-filter-label" for="tr-pdf-font">{{ $t('report.pdf_font') }}</label>
          <select id="tr-pdf-font" class="form-input" v-model="pdfFont">
            <option value="inter">Inter</option>
            <option value="roboto">Roboto</option>
            <option value="opensans">Open Sans</option>
            <option value="sourcecode">Source Code Pro</option>
            <option value="freesans">{{ $t('report.pdf_font_freesans') }}</option>
            <option value="freeserif">{{ $t('report.pdf_font_freeserif') }}</option>
            <option value="freemono">{{ $t('report.pdf_font_freemono') }}</option>
          </select>
        </div>
        <div class="tr-filter">
          <label class="tr-filter-label" for="tr-pdf-lang">{{ $t('report.pdf_lang') }}</label>
          <select id="tr-pdf-lang" class="form-input" v-model="pdfLang">
            <option value="auto">{{ $t('report.pdf_lang_auto') }}</option>
            <option value="en">English</option>
            <option value="nl">Nederlands</option>
            <option value="de">Deutsch</option>
            <option value="fr">Français</option>
            <option value="es">Español</option>
            <option value="da">Dansk</option>
            <option value="sv">Svenska</option>
            <option value="nb">Norsk</option>
            <option value="fi">Suomi</option>
            <option value="is">Íslenska</option>
            <option value="pt">Português</option>
            <option value="it">Italiano</option>
          </select>
        </div>
        <button type="button" class="btn btn-secondary" @click="exportPDF" :disabled="exporting">{{ $t('report.export_pdf') }}</button>
        <button type="button" class="btn btn-secondary" @click="exportXLSX" :disabled="exporting">{{ $t('report.export_xlsx') }}</button>
      </div>
    </form>

    <div v-if="forbidden" class="tr-empty" role="alert">{{ $t('ticketReport.forbidden') }}</div>

    <div v-else-if="loading && !report" class="tr-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <template v-else-if="report">
      <p class="tr-period" aria-live="polite">
        {{ $t('ticketReport.period_label', { from: formatDay(report.from), to: formatDay(report.to) }) }}
        <span class="tr-generated">· {{ $t('ticketReport.generated_at', { at: formatDateTime(report.generated_at) }) }}</span>
      </p>

      <!-- KPI cards -->
      <section aria-labelledby="tr-kpi-heading">
        <h2 id="tr-kpi-heading" class="tr-section-title">{{ $t('ticketReport.kpis') }}</h2>
        <ul class="tr-kpi-grid">
          <li v-for="k in kpis" :key="k.key" class="tr-kpi" :class="k.tone ? `tr-kpi-${k.tone}` : ''">
            <span class="tr-kpi-label">{{ k.label }}</span>
            <span class="tr-kpi-value">{{ k.value }}</span>
            <span class="tr-kpi-desc">{{ k.desc }}</span>
          </li>
        </ul>
      </section>

      <!-- Ageing -->
      <section aria-labelledby="tr-ageing-heading" class="tr-section">
        <h2 id="tr-ageing-heading" class="tr-section-title">{{ $t('ticketReport.ageing') }}</h2>
        <p class="tr-section-desc">{{ $t('ticketReport.ageing_desc') }}</p>
        <table class="tr-table tr-ageing-table">
          <caption class="sr-only">{{ $t('ticketReport.ageing') }}</caption>
          <thead>
            <tr>
              <th scope="col">{{ $t('ticketReport.age') }}</th>
              <th scope="col" class="num">{{ $t('ticketReport.count') }}</th>
              <th scope="col" class="tr-bar-col"><span class="sr-only">{{ $t('ticketReport.share') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="b in report.ageing || []" :key="b.key">
              <th scope="row">{{ ageLabel(b.key) }}</th>
              <td class="num">{{ b.count }}</td>
              <td class="tr-bar-col" aria-hidden="true">
                <span class="tr-bar"><span class="tr-bar-fill" :style="{ width: barWidth(b.count) }"></span></span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <!-- Breakdowns -->
      <section v-for="bd in breakdowns" :key="bd.key" :aria-labelledby="`tr-bd-${bd.key}`" class="tr-section">
        <h2 :id="`tr-bd-${bd.key}`" class="tr-section-title">{{ bd.title }}</h2>
        <div v-if="!bd.rows.length" class="tr-empty-small">{{ $t('ticketReport.no_data') }}</div>
        <div v-else class="tr-table-wrap">
          <table class="tr-table">
            <caption class="sr-only">{{ bd.title }}</caption>
            <thead>
              <tr>
                <th scope="col">{{ bd.column }}</th>
                <th scope="col" class="num">{{ $t('ticketReport.created') }}</th>
                <th scope="col" class="num">{{ $t('ticketReport.resolved') }}</th>
                <th scope="col" class="num">{{ $t('ticketReport.breached') }}</th>
                <th scope="col" class="num">{{ $t('ticketReport.mttr_short') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in bd.rows" :key="row.key">
                <th scope="row">{{ bd.label(row) }}</th>
                <td class="num">{{ row.count }}</td>
                <td class="num">{{ row.resolved }}</td>
                <td class="num" :class="{ 'tr-breached': row.breached > 0 }">{{ row.breached }}</td>
                <td class="num">{{ formatDuration(row.mttr_minutes) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </main>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import DatePicker from '@/components/common/DatePicker.vue'
import { helpdeskApi } from '@/api/helpdesk'
import { groupsApi } from '@/api/groups'
import { messagesApi } from '@/api/messages'
import client, { triggerDownload } from '@/api/client'
import { useCustomersStore } from '@/stores/customers'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const TYPES = ['incident', 'problem', 'service_request', 'change_request']
const PRIORITIES = ['low', 'medium', 'high', 'critical']

const { t, te, locale } = useI18n()
const { formatDate, formatDateTime } = useDateFormat()
const customersStore = useCustomersStore()
const auth = useAuthStore()
const ui = useUIStore()

const report = ref(null)
const loading = ref(false)
const exporting = ref(false)
const forbidden = ref(false)
const groups = ref([])
const users = ref([])
const categories = ref([])

const pdfFont = ref(localStorage.getItem('report.pdfFont') || 'inter')
watch(pdfFont, v => { try { localStorage.setItem('report.pdfFont', v) } catch {} })
const pdfLang = ref(localStorage.getItem('report.pdfLang') || 'auto')
watch(pdfLang, v => { try { localStorage.setItem('report.pdfLang', v) } catch {} })

function isoDay(d) {
  const p = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
function defaultFilters() {
  const to = new Date()
  const from = new Date(to)
  from.setDate(from.getDate() - 29)
  return {
    from: isoDay(from), to: isoDay(to),
    customer_id: '', group_id: '', assigned_to_id: '',
    type: '', priority: '', category_id: '',
  }
}
const filters = ref(defaultFilters())

const customers = computed(() => [...customersStore.customers].sort((a, b) => a.name.localeCompare(b.name)))

// Categories are a two-level tree; show children as "Parent › Child".
const categoryOptions = computed(() => {
  const byId = new Map(categories.value.map(c => [c.id, c]))
  return categories.value
    .map(c => ({ id: c.id, label: c.parent_id && byId.get(c.parent_id) ? `${byId.get(c.parent_id).name} › ${c.name}` : c.name }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

function buildParams() {
  const params = {}
  for (const [k, v] of Object.entries(filters.value)) {
    if (v !== '' && v != null) params[k] = v
  }
  return params
}

async function loadReport() {
  if (filters.value.from && filters.value.to && filters.value.to < filters.value.from) {
    ui.error(t('ticketReport.invalid_range'))
    return
  }
  loading.value = true
  forbidden.value = false
  try {
    const { data } = await helpdeskApi.ticketReport(buildParams())
    report.value = data
  } catch (e) {
    if (e?.response?.status === 403) {
      forbidden.value = true
      report.value = null
    } else {
      ui.error(e?.response?.data?.error || t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.value = defaultFilters()
  loadReport()
}

// Non-admins have no global group list; fall back to the selected
// customer's groups (only readable by customer managers).
async function loadGroups() {
  try {
    if (auth.isAdmin) {
      const { data } = await groupsApi.list()
      groups.value = (data || []).map(g => ({ id: g.id, name: g.name }))
    } else if (filters.value.customer_id) {
      const { data } = await client.get(`/customers/${filters.value.customer_id}/groups`)
      groups.value = (data || []).map(e => ({ id: e.group_id, name: e.group?.name || '' }))
    } else {
      groups.value = []
    }
  } catch {
    groups.value = []
  }
  if (filters.value.group_id && !groups.value.some(g => String(g.id) === filters.value.group_id)) {
    filters.value.group_id = ''
  }
}
watch(() => filters.value.customer_id, () => { if (!auth.isAdmin) loadGroups() })

// ── Formatting ──────────────────────────────────────────────────────────────
function formatDay(ymd) {
  return ymd ? formatDate(`${ymd}T12:00:00`) : ''
}
function formatDuration(minutes) {
  if (minutes == null) return '—'
  const m = Math.round(minutes)
  const d = Math.floor(m / 1440)
  const h = Math.floor((m % 1440) / 60)
  const mm = m % 60
  if (d > 0) return t('ticketReport.duration_dh', { d, h })
  if (h > 0) return t('ticketReport.duration_hm', { h, m: mm })
  return t('ticketReport.duration_m', { m: mm })
}
function formatPct(v) {
  if (v == null) return '—'
  return `${(Math.round(v * 10) / 10).toLocaleString(locale.value)}%`
}
function ageLabel(key) {
  return {
    '0-1d': t('ticketReport.age_0_1'),
    '1-3d': t('ticketReport.age_1_3'),
    '3-7d': t('ticketReport.age_3_7'),
    '7d+': t('ticketReport.age_7_plus'),
  }[key] || key
}
const maxAgeing = computed(() => Math.max(1, ...(report.value?.ageing || []).map(b => b.count)))
function barWidth(count) {
  return `${Math.round((count / maxAgeing.value) * 100)}%`
}

// Compliance below 90% is flagged with a warning accent; the value itself is
// always shown, so colour is never the only carrier of information.
function complianceTone(v) {
  if (v == null) return ''
  return v >= 90 ? 'good' : 'warn'
}

const kpis = computed(() => {
  const r = report.value
  if (!r) return []
  return [
    { key: 'created', label: t('ticketReport.created'), value: r.created, desc: t('ticketReport.created_desc') },
    { key: 'resolved', label: t('ticketReport.resolved'), value: r.resolved, desc: t('ticketReport.resolved_desc') },
    { key: 'backlog', label: t('ticketReport.backlog'), value: r.backlog, desc: t('ticketReport.backlog_desc') },
    { key: 'major', label: t('ticketReport.major_incidents'), value: r.major_incidents, desc: t('ticketReport.major_incidents_desc'), tone: r.major_incidents > 0 ? 'warn' : '' },
    { key: 'mttr', label: t('ticketReport.mttr'), value: formatDuration(r.mttr_minutes), desc: t('ticketReport.mttr_desc') },
    { key: 'mfr', label: t('ticketReport.mean_first_response'), value: formatDuration(r.mean_first_response_minutes), desc: t('ticketReport.mean_first_response_desc') },
    { key: 'rsp', label: t('ticketReport.response_compliance'), value: formatPct(r.response_compliance), desc: t('ticketReport.response_compliance_desc'), tone: complianceTone(r.response_compliance) },
    { key: 'rsl', label: t('ticketReport.resolution_compliance'), value: formatPct(r.resolution_compliance), desc: t('ticketReport.resolution_compliance_desc'), tone: complianceTone(r.resolution_compliance) },
    { key: 'fcr', label: t('ticketReport.fcr_rate'), value: formatPct(r.fcr_rate), desc: t('ticketReport.fcr_rate_desc') },
    { key: 'reopen', label: t('ticketReport.reopen_rate'), value: formatPct(r.reopen_rate), desc: t('ticketReport.reopen_rate_desc') },
  ]
})

function translated(prefix, row) {
  const key = `${prefix}${row.key}`
  return te(key) ? t(key) : (row.label || row.key)
}

const breakdowns = computed(() => {
  const r = report.value
  if (!r) return []
  return [
    { key: 'priority', title: t('ticketReport.by_priority'), column: t('ticketReport.priority'), rows: r.by_priority || [], label: row => translated('ticket.priority_', row) },
    { key: 'type', title: t('ticketReport.by_type'), column: t('ticketReport.type'), rows: r.by_type || [], label: row => translated('ticket.type_', row) },
    { key: 'category', title: t('ticketReport.by_category'), column: t('ticketReport.category'), rows: r.by_category || [], label: row => row.key === '' ? t('ticketReport.uncategorised') : (row.label || row.key) },
    { key: 'customer', title: t('ticketReport.by_customer'), column: t('ticketReport.customer'), rows: r.by_customer || [], label: row => row.key === '' ? t('ticketReport.inbox') : (row.label || row.key) },
  ]
})

// ── Exports (generated server-side) ───────────────────────────────────────────
function exportFilename(ext) {
  const r = report.value
  return `ticket-report-${r?.from || filters.value.from}_${r?.to || filters.value.to}.${ext}`
}

async function exportPDF() {
  exporting.value = true
  try {
    const params = { ...buildParams(), font: pdfFont.value, lang: pdfLang.value === 'auto' ? locale.value : pdfLang.value }
    const data = await helpdeskApi.ticketReportPdf(params)
    await triggerDownload(data, exportFilename('pdf'), 'application/pdf')
  } catch (e) {
    console.error('[export] ticket report PDF failed:', e)
    ui.error(String(e?.message || e))
  } finally {
    exporting.value = false
  }
}

async function exportXLSX() {
  exporting.value = true
  try {
    const params = { ...buildParams(), lang: pdfLang.value === 'auto' ? locale.value : pdfLang.value }
    const data = await helpdeskApi.ticketReportXlsx(params)
    await triggerDownload(data, exportFilename('xlsx'), 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
  } catch (e) {
    console.error('[export] ticket report XLSX failed:', e)
    ui.error(String(e?.message || e))
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  if (!customersStore.customers.length) customersStore.fetchCustomers()
  loadReport()
  loadGroups()
  try {
    const [catRes, userRes] = await Promise.all([
      helpdeskApi.listCategories().catch(() => ({ data: [] })),
      messagesApi.listUsers().catch(() => ({ data: [] })),
    ])
    categories.value = catRes.data || []
    users.value = (userRes.data || [])
      .filter(u => u.global_role !== 'customer')
      .sort((a, b) => (a.display_name || a.username).localeCompare(b.display_name || b.username))
  } catch {}
})
</script>

<style scoped>
.tr-main { padding: 24px 32px; max-width: 1400px; margin: 0 auto; }
.tr-header { margin-bottom: 16px; }
.tr-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.tr-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.tr-filters {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 14px 16px;
  margin-bottom: 20px;
}
.tr-filter-row, .tr-export-row { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; }
.tr-export-row { margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--color-border); }
.tr-filter { display: flex; flex-direction: column; gap: 4px; min-width: 150px; }
.tr-filter-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.tr-filter-actions { flex-direction: row; gap: 8px; min-width: 0; }

.tr-loading { display: flex; justify-content: center; padding: 48px; }
.tr-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }
.tr-empty-small { color: var(--color-text-muted); font-size: 13px; padding: 8px 0; }
.tr-period { font-size: 14px; font-weight: 600; margin: 0 0 16px; }
.tr-generated { font-weight: 400; color: var(--color-text-muted); font-size: 12px; }

.tr-section { margin-top: 28px; }
.tr-section-title { font-size: 16px; font-weight: 700; margin: 0 0 10px; }
.tr-section-desc { font-size: 12px; color: var(--color-text-muted); margin: -4px 0 10px; }

.tr-kpi-grid {
  list-style: none; margin: 0; padding: 0;
  display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px;
}
.tr-kpi {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-left: 4px solid var(--color-primary);
  border-radius: var(--radius);
  padding: 12px 14px;
  display: flex; flex-direction: column; gap: 4px;
}
.tr-kpi-good { border-left-color: var(--color-success); }
.tr-kpi-warn { border-left-color: var(--color-warning); }
.tr-kpi-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); text-transform: uppercase; letter-spacing: .03em; }
.tr-kpi-value { font-size: 26px; font-weight: 700; color: var(--color-text); font-variant-numeric: tabular-nums; }
.tr-kpi-desc { font-size: 12px; color: var(--color-text-muted); line-height: 1.4; }

.tr-table-wrap { overflow-x: auto; }
.tr-table {
  width: 100%; border-collapse: collapse;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  font-size: 13px;
}
.tr-table th, .tr-table td { padding: 8px 12px; border-bottom: 1px solid var(--color-border); text-align: left; }
.tr-table thead th { font-size: 12px; font-weight: 600; color: var(--color-text-muted); background: var(--color-bg); }
.tr-table tbody th { font-weight: 500; }
.tr-table tbody tr:last-child th, .tr-table tbody tr:last-child td { border-bottom: none; }
.tr-table .num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
.tr-breached { color: var(--color-danger); font-weight: 600; }
.tr-ageing-table { max-width: 640px; }
.tr-bar-col { width: 50%; }
.tr-bar { display: block; height: 10px; background: var(--color-bg); border-radius: 5px; overflow: hidden; }
.tr-bar-fill { display: block; height: 100%; background: var(--color-primary); border-radius: 5px; }

@media (max-width: 640px) {
  .tr-main { padding: 16px; }
  .tr-filter { min-width: 0; flex: 1 1 140px; }
}
</style>
