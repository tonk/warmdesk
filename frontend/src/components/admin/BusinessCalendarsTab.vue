<template>
  <div>
    <div class="tab-toolbar">
      <button class="btn btn-primary btn-sm" @click="startCreate">+ {{ $t('helpdeskAdmin.new_calendar') }}</button>
    </div>
    <p class="bc-hint bc-hint-block">{{ $t('helpdeskAdmin.calendars_hint') }}</p>

    <!-- Create / Edit form card -->
    <div v-if="editing" class="bc-form-card">
      <h3 class="bc-form-title">{{ editing.id ? $t('helpdeskAdmin.edit_calendar') : $t('helpdeskAdmin.new_calendar') }}</h3>

      <div class="bc-form-row">
        <div class="bc-form-field" style="flex:2;min-width:200px">
          <label class="bc-label" for="bc-name">{{ $t('helpdeskAdmin.name') }}</label>
          <input id="bc-name" ref="nameEl" class="form-input" v-model="form.name" />
        </div>
        <div class="bc-form-field" style="flex:2;min-width:200px">
          <label class="bc-label" for="bc-tz">{{ $t('helpdeskAdmin.timezone') }}</label>
          <input id="bc-tz" class="form-input" v-model="form.timezone" list="bc-tz-list" autocomplete="off" spellcheck="false" aria-describedby="bc-tz-hint" />
          <datalist id="bc-tz-list">
            <option v-for="tz in timezones" :key="tz" :value="tz" />
          </datalist>
          <small id="bc-tz-hint" class="bc-hint">{{ $t('helpdeskAdmin.timezone_hint') }}</small>
        </div>
      </div>

      <div class="bc-form-row">
        <fieldset class="bc-fieldset">
          <legend class="bc-label">{{ $t('helpdeskAdmin.work_days') }}</legend>
          <label v-for="d in WEEKDAY_ORDER" :key="d" class="bc-check">
            <input type="checkbox" :value="d" v-model="form.work_days" />
            {{ weekdayName(d) }}
          </label>
        </fieldset>
        <div class="bc-form-field">
          <label class="bc-label" for="bc-start">{{ $t('helpdeskAdmin.start_time') }}</label>
          <input id="bc-start" class="form-input bc-time" type="time" v-model="form.start_time" />
        </div>
        <div class="bc-form-field">
          <label class="bc-label" for="bc-end">{{ $t('helpdeskAdmin.end_time') }}</label>
          <input id="bc-end" class="form-input bc-time" type="time" v-model="form.end_time" />
        </div>
      </div>

      <!-- Holidays -->
      <div class="bc-holidays">
        <div class="bc-holidays-header">
          <h4 id="bc-holidays-title" class="bc-section-title">{{ $t('helpdeskAdmin.holidays') }}</h4>
          <button class="btn btn-secondary btn-sm" @click="addHoliday">+ {{ $t('helpdeskAdmin.add_holiday') }}</button>
        </div>

        <div class="bc-import" role="group" aria-labelledby="bc-import-title">
          <span id="bc-import-title" class="bc-import-title">{{ $t('helpdeskAdmin.public_holidays') }}</span>
          <div class="bc-form-field bc-import-year">
            <label class="bc-label" for="bc-import-year">{{ $t('helpdeskAdmin.year') }}</label>
            <input id="bc-import-year" class="form-input bc-year" type="number" min="2000" max="2100" v-model.number="importYear" />
          </div>
          <HolidayCountryMenu
            class="bc-import-menu"
            button-class="btn btn-secondary btn-sm"
            align="left"
            :label="$t('timeTracking.add_holidays')"
            :aria-label="$t('helpdeskAdmin.add_holidays_for_year', { year: importYear })"
            :disabled="importing"
            @select="addPublicHolidays"
          />
          <small class="bc-hint">{{ $t('helpdeskAdmin.add_holidays_hint') }}</small>
        </div>

        <div v-if="!form.holidays.length" class="bc-empty-inline">{{ $t('helpdeskAdmin.no_holidays') }}</div>
        <ul v-else class="bc-holiday-list" aria-labelledby="bc-holidays-title">
          <li v-for="(h, idx) in form.holidays" :key="h._key" class="bc-holiday-row">
            <DatePicker v-model="h.date" :label="$t('helpdeskAdmin.holiday_date_n', { n: idx + 1 })" />
            <label class="sr-only" :for="'bc-hol-name-' + h._key">{{ $t('helpdeskAdmin.holiday_name_n', { n: idx + 1 }) }}</label>
            <input :id="'bc-hol-name-' + h._key" class="form-input form-input-sm bc-hol-name" v-model="h.name" :placeholder="$t('helpdeskAdmin.holiday_name')" />
            <button class="btn-icon-xs" @click="removeHoliday(idx)" :aria-label="$t('helpdeskAdmin.remove_holiday_n', { n: idx + 1 })">✕</button>
          </li>
        </ul>
      </div>

      <div class="bc-form-footer">
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
          <th>{{ $t('helpdeskAdmin.timezone') }}</th>
          <th>{{ $t('helpdeskAdmin.work_days') }}</th>
          <th>{{ $t('helpdeskAdmin.hours') }}</th>
          <th>{{ $t('helpdeskAdmin.holidays') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in calendars" :key="c.id">
          <td><button type="button" class="name-link" @click="startEdit(c)">{{ c.name }}</button></td>
          <td>{{ c.timezone }}</td>
          <td>{{ fmtWorkDays(c.work_days) }}</td>
          <td>{{ c.start_time }}–{{ c.end_time }}</td>
          <td>{{ c.holidays?.length || 0 }}</td>
          <td>
            <div class="actions-cell">
              <button class="btn btn-ghost btn-sm" @click="startEdit(c)" :aria-label="`${$t('common.edit')}: ${c.name}`">{{ $t('common.edit') }}</button>
              <button class="btn btn-ghost btn-sm btn-danger" @click="deleteCalendar(c)" :aria-label="`${$t('common.delete')}: ${c.name}`">{{ $t('common.delete') }}</button>
            </div>
          </td>
        </tr>
        <tr v-if="!calendars.length && !editing">
          <td colspan="6" style="text-align:center;color:var(--color-text-muted)">{{ $t('helpdeskAdmin.no_calendars') }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { helpdeskApi } from '@/api/helpdesk'
import { useUIStore } from '@/stores/ui'
import DatePicker from '@/components/common/DatePicker.vue'
import HolidayCountryMenu from '@/components/common/HolidayCountryMenu.vue'

// Go weekday numbers (0 = Sunday), displayed Monday first.
const WEEKDAY_ORDER = [1, 2, 3, 4, 5, 6, 0]

// Locales the backend can generate public holidays for, with the country
// whose holidays each one produces (en = United Kingdom).

// Same short list the admin settings tab offers; the full IANA list from the
// browser is preferred where available.
const FALLBACK_TIMEZONES = [
  'UTC',
  'Europe/Amsterdam', 'Europe/Berlin', 'Europe/Brussels', 'Europe/London',
  'Europe/Madrid', 'Europe/Paris', 'Europe/Rome', 'Europe/Stockholm',
  'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles',
  'America/Toronto', 'America/Vancouver', 'America/Sao_Paulo',
  'Asia/Dubai', 'Asia/Istanbul', 'Asia/Jerusalem', 'Asia/Kolkata',
  'Asia/Singapore', 'Asia/Shanghai', 'Asia/Tokyo', 'Asia/Seoul',
  'Australia/Sydney', 'Pacific/Auckland',
]
const timezones = (() => {
  try {
    const all = Intl.supportedValuesOf?.('timeZone')
    if (all?.length) return all.includes('UTC') ? all : ['UTC', ...all]
  } catch { /* older engine */ }
  return FALLBACK_TIMEZONES
})()

const { t, locale } = useI18n()
const ui = useUIStore()
const loading = ref(true)
const saving = ref(false)
const importing = ref(false)
const calendars = ref([])
const editing = ref(null)
const form = ref(emptyForm())
const formSnapshot = ref('')
const nameEl = ref(null)
const importYear = ref(new Date().getFullYear())
let holidayKey = 0

onMounted(load)

function emptyForm() {
  return {
    name: '',
    timezone: defaultTimezone(),
    work_days: [1, 2, 3, 4, 5],
    start_time: '09:00',
    end_time: '17:00',
    holidays: [],
  }
}

function defaultTimezone() {
  try { return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC' } catch { return 'UTC' }
}

function withKeys(holidays) {
  return (holidays || []).map(h => ({ date: h.date, name: h.name || '', _key: ++holidayKey }))
}

function parseWorkDays(csv) {
  return (csv || '').split(',').map(x => parseInt(x.trim(), 10)).filter(n => n >= 0 && n <= 6)
}

function weekdayName(d, style = 'long') {
  // 2025-01-05 is a Sunday, so adding d days yields Go weekday d.
  const date = new Date(Date.UTC(2025, 0, 5 + d))
  try {
    return new Intl.DateTimeFormat(locale.value, { weekday: style, timeZone: 'UTC' }).format(date)
  } catch {
    return ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'][d]
  }
}

function fmtWorkDays(csv) {
  const days = parseWorkDays(csv)
  return WEEKDAY_ORDER.filter(d => days.includes(d)).map(d => weekdayName(d, 'short')).join(', ') || '—'
}


function snapshotForm(f) {
  return JSON.stringify({ ...f, holidays: f.holidays.map(h => ({ date: h.date, name: h.name })) })
}

function onEditEscape(e) {
  // Only close an untouched form, and not while e.g. the date picker is open.
  if (e.key !== 'Escape' || !editing.value || e.defaultPrevented) return
  if (snapshotForm(form.value) !== formSnapshot.value) return
  e.preventDefault()
  cancelEdit()
}

watch(editing, (val) => {
  if (val) window.addEventListener('keydown', onEditEscape)
  else window.removeEventListener('keydown', onEditEscape)
})

onUnmounted(() => window.removeEventListener('keydown', onEditEscape))

async function load() {
  loading.value = true
  try {
    const { data } = await helpdeskApi.listCalendars()
    calendars.value = data || []
  } catch { calendars.value = [] }
  loading.value = false
}

function openForm(f, target) {
  form.value = f
  editing.value = target
  formSnapshot.value = snapshotForm(f)
  nextTick(() => nameEl.value?.focus())
}

function startCreate() {
  openForm(emptyForm(), { id: 0 })
}

function startEdit(c) {
  openForm({
    name: c.name,
    timezone: c.timezone || 'UTC',
    work_days: parseWorkDays(c.work_days),
    start_time: c.start_time,
    end_time: c.end_time,
    holidays: withKeys(c.holidays),
  }, c)
}

function cancelEdit() {
  editing.value = null
}

function addHoliday() {
  form.value.holidays.push({ date: null, name: '', _key: ++holidayKey })
}

function removeHoliday(idx) {
  form.value.holidays.splice(idx, 1)
}

function validate() {
  const f = form.value
  if (!f.name.trim()) return t('helpdeskAdmin.name_required')
  if (!f.timezone.trim()) return t('helpdeskAdmin.timezone_required')
  if (!f.work_days.length) return t('helpdeskAdmin.work_days_required')
  if (!/^\d{2}:\d{2}$/.test(f.start_time || '') || !/^\d{2}:\d{2}$/.test(f.end_time || '') || f.end_time <= f.start_time) {
    return t('helpdeskAdmin.invalid_hours')
  }
  const dates = f.holidays.map(h => h.date).filter(Boolean)
  if (new Set(dates).size !== dates.length) return t('helpdeskAdmin.duplicate_holiday')
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
    timezone: f.timezone.trim(),
    work_days: WEEKDAY_ORDER.filter(d => f.work_days.includes(d)).sort((a, b) => a - b).join(','),
    start_time: f.start_time,
    end_time: f.end_time,
    // Rows without a date are dropped; the list replaces the stored one.
    holidays: f.holidays
      .filter(h => h.date)
      .map(h => ({ date: h.date, name: (h.name || '').trim() }))
      .sort((a, b) => a.date.localeCompare(b.date)),
  }
  saving.value = true
  try {
    if (editing.value.id) await helpdeskApi.updateCalendar(editing.value.id, payload)
    else await helpdeskApi.createCalendar(payload)
    editing.value = null
    await load()
    ui.success(t('helpdeskAdmin.calendar_saved'))
  } catch (e) {
    ui.error(saveError(e))
  } finally {
    saving.value = false
  }
}

function saveError(e) {
  const data = e.response?.data
  if (data?.code === 'invalid_calendar') return t('helpdeskAdmin.invalid_calendar')
  if (data?.error === 'unknown timezone') return t('helpdeskAdmin.unknown_timezone')
  return data?.error || t('helpdeskAdmin.save_failed')
}

// Add a country's public holidays for the chosen year to the form. They're
// saved together with the rest of the calendar, so this also works for a
// calendar that hasn't been saved yet.
async function addPublicHolidays(country) {
  const year = importYear.value
  if (!Number.isInteger(year) || year < 2000 || year > 2100) {
    ui.error(t('helpdeskAdmin.invalid_year'))
    return
  }
  importing.value = true
  try {
    const { data } = await helpdeskApi.publicHolidays(year, country)
    const existing = new Set(form.value.holidays.map(h => h.date))
    const fresh = withKeys((data || []).filter(h => !existing.has(h.date)))
    form.value.holidays = [...form.value.holidays, ...fresh]
      .sort((a, b) => (a.date || '9999').localeCompare(b.date || '9999'))
    const skipped = (data || []).length - fresh.length
    if (!fresh.length) ui.info(t('helpdeskAdmin.holidays_all_listed', { year }))
    else ui.success(t('helpdeskAdmin.holidays_added_unsaved', { added: fresh.length, skipped }))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.import_failed'))
  } finally {
    importing.value = false
  }
}

async function deleteCalendar(c) {
  if (!await ui.confirm(t('helpdeskAdmin.delete_calendar_confirm', { name: c.name }), { destructive: true })) return
  try {
    await helpdeskApi.deleteCalendar(c.id)
    if (editing.value?.id === c.id) editing.value = null
    await load()
    ui.success(t('helpdeskAdmin.calendar_deleted'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('helpdeskAdmin.delete_failed'))
  }
}
</script>

<style scoped>
.bc-form-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}
.bc-form-title { font-size: 15px; font-weight: 600; margin: 0 0 16px; }
.bc-section-title { font-size: 13px; font-weight: 600; margin: 0; }
.bc-form-row { display: flex; gap: 12px; align-items: flex-start; flex-wrap: wrap; margin-bottom: 12px; }
.bc-form-field { display: flex; flex-direction: column; gap: 4px; min-width: 120px; }
.bc-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.bc-hint { font-size: 11px; color: var(--color-text-muted); }
.bc-hint-block { display: block; margin: 0 0 16px; font-size: 12px; max-width: 72ch; }
.bc-fieldset { border: 1px solid var(--color-border); border-radius: 6px; padding: 8px 12px; margin: 0; display: flex; flex-wrap: wrap; gap: 4px 14px; }
.bc-fieldset legend { padding: 0 4px; }
.bc-check { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }
.bc-time { width: 120px; }
.bc-year { width: 100px; }
.bc-holidays { margin-top: 16px; }
.bc-holidays-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 8px; }
.bc-import { display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap; padding: 10px 12px; border: 1px dashed var(--color-border); border-radius: 6px; margin-bottom: 12px; }
.bc-import-title { flex: 1 0 100%; font-size: 13px; font-weight: 600; }
.bc-import-year { min-width: 90px; }
.bc-import-menu { margin-bottom: 2px; }
.bc-import .bc-hint { flex: 1 1 220px; align-self: center; }
.bc-empty-inline { font-size: 13px; color: var(--color-text-muted); padding: 8px 0; }
.bc-holiday-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.bc-holiday-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.bc-hol-name { flex: 1; min-width: 160px; }
.bc-form-footer { display: flex; gap: 8px; margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.loading-state { display: flex; justify-content: center; padding: 48px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); border: 0; }

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
