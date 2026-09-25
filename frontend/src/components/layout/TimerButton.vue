<template>
  <div v-if="timerStore.available" class="timer-widget" ref="rootRef">
    <button
      ref="buttonRef"
      type="button"
      class="btn-icon timer-btn"
      :class="{ running: timerStore.running }"
      :aria-label="buttonLabel"
      :title="buttonLabel"
      :aria-expanded="open"
      aria-haspopup="dialog"
      aria-controls="timer-panel"
      @click.stop="toggle"
    >
      <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="13" r="8"/>
        <polyline points="12 9 12 13 14.5 15"/>
        <line x1="10" y1="2" x2="14" y2="2"/>
      </svg>
      <span v-if="timerStore.running" class="timer-elapsed" aria-hidden="true">{{ clock(elapsedMinutes) }}</span>
    </button>

    <div
      v-if="open"
      id="timer-panel"
      ref="panelRef"
      class="timer-panel"
      role="dialog"
      aria-labelledby="timer-panel-title"
      @keydown.esc.stop="close(true)"
      @click.stop
    >
      <!-- Running: what's being timed, and stop / switch / discard -->
      <template v-if="timerStore.running && !switching">
        <h2 id="timer-panel-title" class="timer-title">{{ $t('timer.running_title') }}</h2>
        <p class="timer-label">{{ runningLabel }}</p>
        <p class="timer-meta">
          {{ $t('timer.since', { time: startedAt, duration: duration(elapsedMinutes) }) }}
        </p>
        <div class="timer-actions">
          <button ref="firstRef" type="button" class="btn btn-primary btn-sm" :disabled="busy" @click="stop">{{ $t('timer.stop') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="beginSwitch">{{ $t('timer.switch') }}</button>
          <button type="button" class="btn btn-ghost btn-sm timer-discard" :disabled="busy" @click="discard">{{ $t('timer.discard') }}</button>
        </div>
      </template>

      <!-- Idle, or switching: pick what to time -->
      <form v-else @submit.prevent="start">
        <h2 id="timer-panel-title" class="timer-title">{{ switching ? $t('timer.switch_title') : $t('timer.start_title') }}</h2>
        <p v-if="switching" class="timer-meta">{{ $t('timer.switch_hint', { label: runningLabel }) }}</p>

        <div class="form-group">
          <label class="form-label" for="timer-project">{{ $t('timer.project') }}</label>
          <select id="timer-project" ref="firstRef" v-model="projectId" class="form-input" @change="onProjectChange">
            <option :value="null">{{ $t('timer.no_project') }}</option>
            <optgroup v-if="boardProjects.length" :label="$t('timer.board_projects')">
              <option v-for="p in boardProjects" :key="p.id" :value="p.id">
                {{ p.customer_name ? `${p.name} (${p.customer_name})` : p.name }}
              </option>
            </optgroup>
            <optgroup v-if="ttProjects.length" :label="$t('timer.tt_projects')">
              <option v-for="p in ttProjects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </optgroup>
          </select>
        </div>

        <div class="form-group">
          <label class="form-label" for="timer-customer">{{ $t('timer.customer') }}</label>
          <select id="timer-customer" v-model="customerId" class="form-input" :disabled="!!projectCustomer" :aria-describedby="projectCustomer ? 'timer-customer-hint' : undefined">
            <option v-if="projectCustomer" :value="projectCustomer.id">{{ projectCustomer.name }}</option>
            <template v-else>
              <option :value="null">{{ $t('timer.no_customer') }}</option>
              <option v-for="c in targets.customers" :key="c.id" :value="c.id">{{ c.name }}</option>
            </template>
          </select>
          <span v-if="projectCustomer" id="timer-customer-hint" class="form-hint">{{ $t('timer.customer_from_project') }}</span>
        </div>

        <div class="form-group">
          <label class="form-label" for="timer-description">{{ $t('timer.description') }}</label>
          <input id="timer-description" v-model="description" class="form-input" type="text" maxlength="500" />
        </div>

        <p class="form-hint timer-rounding">{{ $t('timer.rounding_hint') }}</p>
        <div class="timer-actions">
          <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || (!projectId && !customerId)">
            {{ switching ? $t('timer.switch_start') : $t('timer.start') }}
          </button>
          <button v-if="switching" type="button" class="btn btn-secondary btn-sm" @click="switching = false">{{ $t('common.cancel') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTimerStore } from '@/stores/timer'
import { useUIStore } from '@/stores/ui'
import { timerApi } from '@/api/timer'

const { t } = useI18n()
const timerStore = useTimerStore()
const ui = useUIStore()

const LAST_KEY = 'warmdesk_timer_last' // this browser's last pick, a convenience only

const open = ref(false)
const switching = ref(false)
const busy = ref(false)
const targets = ref({ projects: [], customers: [] })
const projectId = ref(null)
const customerId = ref(null)
const description = ref('')
const now = ref(Date.now())

const rootRef = ref(null)
const buttonRef = ref(null)
const panelRef = ref(null)
const firstRef = ref(null)

const boardProjects = computed(() => targets.value.projects.filter(p => !p.time_tracking_only))
const ttProjects = computed(() => targets.value.projects.filter(p => p.time_tracking_only))
const selectedProject = computed(() => targets.value.projects.find(p => p.id === projectId.value) || null)
// A board project with its own customer fixes the customer.
const projectCustomer = computed(() => {
  const p = selectedProject.value
  return p?.customer_id ? { id: p.customer_id, name: p.customer_name } : null
})

const elapsedMinutes = computed(() => {
  const started = timerStore.timer?.started_at
  return started ? Math.max(0, Math.floor((now.value - new Date(started).getTime()) / 60000)) : 0
})
const startedAt = computed(() => {
  const started = timerStore.timer?.started_at
  return started ? new Date(started).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''
})

function labelFor(project, customer, desc) {
  let s = project && customer ? `${project} (${customer})` : (project || customer || '')
  if (desc) s += ` — ${desc}`
  return s
}
const runningLabel = computed(() => {
  const tm = timerStore.timer
  return tm ? labelFor(tm.project?.name, tm.customer?.name, tm.description) : ''
})
const buttonLabel = computed(() => timerStore.running
  ? t('timer.button_running', { label: runningLabel.value, duration: duration(elapsedMinutes.value) })
  : t('timer.button_idle'))

function clock(min) {
  return `${Math.floor(min / 60)}:${String(min % 60).padStart(2, '0')}`
}
function duration(min) {
  return min < 60 ? t('timer.minutes', { m: min }) : t('timer.hours_minutes', { h: Math.floor(min / 60), m: String(min % 60).padStart(2, '0') })
}

function onProjectChange() {
  customerId.value = projectCustomer.value ? projectCustomer.value.id : (targets.value.customers.some(c => c.id === customerId.value) ? customerId.value : null)
}

async function loadTargets() {
  try {
    const { data } = await timerApi.targets()
    targets.value = { projects: data?.projects || [], customers: data?.customers || [] }
  } catch {
    targets.value = { projects: [], customers: [] }
  }
  let last = null
  try { last = JSON.parse(localStorage.getItem(LAST_KEY) || 'null') } catch {}
  if (last && projectId.value == null && customerId.value == null) {
    if (targets.value.projects.some(p => p.id === last.projectId)) projectId.value = last.projectId
    if (targets.value.customers.some(c => c.id === last.customerId)) customerId.value = last.customerId
  }
  onProjectChange()
}

async function openPanel() {
  open.value = true
  switching.value = false
  if (!timerStore.running) await loadTargets()
  await nextTick()
  firstRef.value?.focus()
}

function close(restoreFocus = false) {
  open.value = false
  switching.value = false
  if (restoreFocus) buttonRef.value?.focus()
}

function toggle() {
  if (open.value) close()
  else openPanel()
}

async function beginSwitch() {
  switching.value = true
  await loadTargets()
  await nextTick()
  firstRef.value?.focus()
}

function bookedMessage(entries) {
  const total = entries.reduce((sum, e) => sum + (e.minutes || 0), 0)
  const e = entries[0]
  return t('timer.booked', { duration: duration(total), label: labelFor(e?.project?.name, e?.customer?.name, e?.description) })
}

function apiError(e) {
  return e?.response?.data?.error || t('timer.failed')
}

async function start() {
  if (!projectId.value && !customerId.value) return
  busy.value = true
  try {
    const data = await timerStore.start({ projectId: projectId.value, customerId: customerId.value, description: description.value.trim() })
    try { localStorage.setItem(LAST_KEY, JSON.stringify({ projectId: projectId.value, customerId: customerId.value })) } catch {}
    if (data?.stopped_entries?.length) ui.success(bookedMessage(data.stopped_entries))
    ui.success(t('timer.started', { label: runningLabel.value }))
    description.value = ''
    close(true)
  } catch (e) {
    ui.error(apiError(e))
  } finally {
    busy.value = false
  }
}

async function stop() {
  busy.value = true
  try {
    const entries = await timerStore.stop()
    if (entries.length) ui.success(bookedMessage(entries))
    close(true)
  } catch (e) {
    ui.error(apiError(e))
    timerStore.refresh()
  } finally {
    busy.value = false
  }
}

async function discard() {
  const ok = await ui.confirm(t('timer.discard_confirm'), { confirmLabel: t('timer.discard'), destructive: true })
  if (!ok) return
  busy.value = true
  try {
    await timerStore.cancel()
    ui.info(t('timer.discarded'))
    close(true)
  } catch (e) {
    ui.error(apiError(e))
    timerStore.refresh()
  } finally {
    busy.value = false
  }
}

// Keep the elapsed time current while a timer runs.
let tick = null
watch(() => timerStore.running, (running) => {
  clearInterval(tick)
  tick = null
  if (running) {
    now.value = Date.now()
    tick = setInterval(() => { now.value = Date.now() }, 15000)
  }
}, { immediate: true })

function onDocumentClick(e) {
  if (open.value && rootRef.value && !rootRef.value.contains(e.target)) close()
}

onMounted(() => {
  timerStore.refresh()
  document.addEventListener('click', onDocumentClick)
})
onBeforeUnmount(() => {
  clearInterval(tick)
  document.removeEventListener('click', onDocumentClick)
})
</script>

<style scoped>
.timer-widget { position: relative; }
.timer-btn { display: inline-flex; align-items: center; gap: 4px; width: auto; }
.timer-btn.running { color: var(--color-primary); }
.timer-elapsed { font-size: 12px; font-variant-numeric: tabular-nums; font-weight: 600; }
.timer-panel {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 200;
  width: 300px;
  padding: 14px;
  background: var(--color-surface);
  color: var(--color-text);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-md);
}
.timer-title { font-size: 14px; margin: 0 0 8px; }
.timer-label { margin: 0 0 4px; font-weight: 600; word-break: break-word; }
.timer-meta { margin: 0 0 12px; font-size: 12px; color: var(--color-text-muted); }
.timer-panel .form-group { margin-bottom: 10px; }
.timer-rounding { margin: 0 0 10px; }
.timer-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.timer-discard { color: var(--color-danger); }
</style>
