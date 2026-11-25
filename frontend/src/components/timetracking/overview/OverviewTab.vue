<template>
  <div class="ov-root">
    <div class="ov-toolbar">
      <div class="ov-year" role="group" :aria-label="$t('timeOverview.year')">
        <button type="button" class="ov-btn" :aria-label="$t('timeOverview.year_prev')" @click="year--">&#9664;</button>
        <span class="ov-year-label" aria-live="polite">{{ year }}</span>
        <button type="button" class="ov-btn" :aria-label="$t('timeOverview.year_next')" @click="year++">&#9654;</button>
      </div>
      <div class="ov-subtabs" role="tablist" :aria-label="$t('timeOverview.nav_label')" @keydown="onKey">
        <button
          v-for="s in subs" :key="s"
          :id="'ov-tab-' + s" type="button" role="tab" class="ov-sub-btn"
          :class="{ active: sub === s }" :aria-selected="sub === s"
          :aria-controls="'ov-panel-' + s" :tabindex="sub === s ? 0 : -1"
          @click="sub = s"
        >{{ $t('timeOverview.sub_' + s) }}</button>
      </div>
      <button
        v-if="pdfExporters[sub]" type="button" class="ov-btn"
        :disabled="exporting || loading || !data" :aria-busy="String(exporting)"
        :aria-label="$t('timeTracking.export_pdf')"
        @click="exportPdf"
      >PDF</button>
      <button type="button" class="ov-btn" @click="settingsOpen = true">{{ $t('timeOverview.settings') }}</button>
    </div>

    <div :id="'ov-panel-' + sub" role="tabpanel" :aria-labelledby="'ov-tab-' + sub" class="ov-body" :aria-busy="String(loading)">
      <p v-if="loading" role="status" aria-live="polite">{{ $t('common.loading') }}</p>
      <div v-else-if="error" role="alert" class="ov-error">
        {{ error }} <button type="button" class="ov-link" @click="load">{{ $t('timeOverview.retry') }}</button>
      </div>
      <template v-else-if="data">
        <NormPanel v-if="sub === 'norm'" :data="data" />
        <ChecksPanel v-else-if="sub === 'checks'" :key="year" :data="data" />
        <TravelPanel v-else-if="sub === 'travel'" :data="data" />
        <VacationPanel
          v-else-if="sub === 'vacation'" :data="data" :saving="saving"
          @save-carry-over="saveCarryOver" @open-settings="settingsOpen = true"
        />
      </template>
    </div>

    <OverviewSettingsModal v-if="settingsOpen" :year="year" :user-id="userId" @close="settingsOpen = false" @saved="load" />
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { timeEntriesApi } from '@/api/timeEntries'
import { triggerDownload } from '@/api/client'
import { useUIStore } from '@/stores/ui'
import NormPanel from './NormPanel.vue'
import ChecksPanel from './ChecksPanel.vue'
import TravelPanel from './TravelPanel.vue'
import VacationPanel from './VacationPanel.vue'
import OverviewSettingsModal from './OverviewSettingsModal.vue'
import './overview.css'

// userId: 0/null = the signed-in user; otherwise the employee an admin/viewer is looking at.
const props = defineProps({ userId: { type: Number, default: 0 } })
const { t, locale } = useI18n()
const ui = useUIStore()

const subs = ['norm', 'checks', 'travel', 'vacation']
const fetchers = {
  norm: timeEntriesApi.overviewNorm,
  checks: timeEntriesApi.overviewChecks,
  travel: timeEntriesApi.overviewTravel,
  vacation: timeEntriesApi.overviewVacation,
}

const year = ref(new Date().getFullYear())
const sub = ref('norm')
const data = ref(null)
const loading = ref(false)
const error = ref('')
const saving = ref(false)
const settingsOpen = ref(false)
const exporting = ref(false)
let token = 0

const baseParams = () => ({ year: year.value, ...(props.userId ? { user_id: props.userId } : {}) })

async function load() {
  const my = ++token
  loading.value = true
  error.value = ''
  data.value = null
  try {
    const res = await fetchers[sub.value](baseParams())
    if (my !== token) return
    data.value = res.data
  } catch {
    if (my === token) error.value = t('timeOverview.load_error')
  } finally {
    if (my === token) loading.value = false
  }
}

// Sub-tabs with a PDF export, rendered server-side in the UI language.
const pdfExporters = {
  norm: { fetch: timeEntriesApi.overviewNormPDF, name: 'norm-overview' },
  travel: { fetch: timeEntriesApi.overviewTravelPDF, name: 'travel-overview' },
}

async function exportPdf() {
  const exporter = pdfExporters[sub.value]
  if (!exporter) return
  exporting.value = true
  try {
    const pdf = await exporter.fetch({ ...baseParams(), lang: locale.value })
    await triggerDownload(pdf, `${exporter.name}-${year.value}.pdf`, 'application/pdf')
  } catch (e) {
    console.error(`[export] ${sub.value} overview PDF failed:`, e)
    ui.error(String(e?.message || e))
  } finally {
    exporting.value = false
  }
}

async function saveCarryOver(hours) {
  saving.value = true
  try {
    // Only the carry-over is sent: keys that are left out are not touched, so the
    // year's hours per week and vacation stay inherited.
    await timeEntriesApi.setOverviewSettings(
      { year: year.value, carry_over_hours: hours },
      props.userId ? { user_id: props.userId } : undefined,
    )
    ui.success(t('timeOverview.settings_saved'))
    await load()
  } catch {
    ui.error(t('timeOverview.save_error'))
  } finally {
    saving.value = false
  }
}

function onKey(e) {
  const i = subs.indexOf(sub.value)
  let n = -1
  if (e.key === 'ArrowRight') n = (i + 1) % subs.length
  else if (e.key === 'ArrowLeft') n = (i + subs.length - 1) % subs.length
  else if (e.key === 'Home') n = 0
  else if (e.key === 'End') n = subs.length - 1
  if (n < 0) return
  e.preventDefault()
  sub.value = subs[n]
  document.getElementById('ov-tab-' + subs[n])?.focus()
}

watch([year, sub, () => props.userId], load)
onMounted(load)
</script>

<style scoped>
.ov-root { padding: 16px; display: flex; flex-direction: column; gap: 14px; }
.ov-toolbar { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; }
.ov-year { display: flex; align-items: center; gap: 8px; }
.ov-year-label { font-weight: 700; min-width: 3.5em; text-align: center; color: var(--color-text); }
.ov-subtabs { display: flex; gap: 4px; flex: 1; flex-wrap: wrap; }
.ov-sub-btn { padding: 5px 14px; border: 1px solid var(--color-border); border-radius: 6px; background: var(--color-surface); color: var(--color-text); cursor: pointer; font-size: 13px; }
.ov-sub-btn.active { background: var(--color-primary); border-color: var(--color-primary); color: #fff; font-weight: 600; }
.ov-body { min-height: 120px; color: var(--color-text); }
</style>
