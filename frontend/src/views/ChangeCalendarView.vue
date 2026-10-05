<template>
  <main class="cc-main">
    <header class="cc-header">
      <h1 class="cc-title">{{ $t('change.calendar_title') }}</h1>
      <p class="cc-subtitle">{{ $t('change.calendar_subtitle') }}</p>
    </header>

    <div v-if="forbidden" class="cc-empty" role="alert">{{ $t('change.forbidden') }}</div>

    <template v-else>
      <div class="cc-toolbar">
        <div class="cc-nav" role="group" :aria-label="viewMode === 'week' ? $t('change.week_navigation') : $t('change.month_navigation')">
          <button type="button" class="btn btn-secondary btn-sm" :aria-label="viewMode === 'week' ? $t('change.prev_week') : $t('change.prev_month')" @click="shift(-1)">‹</button>
          <button type="button" class="btn btn-secondary btn-sm" @click="goToday">{{ $t('change.today') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" :aria-label="viewMode === 'week' ? $t('change.next_week') : $t('change.next_month')" @click="shift(1)">›</button>
          <h2 class="cc-month" aria-live="polite">{{ viewMode === 'week' ? weekTitle : monthTitle }}</h2>
        </div>

        <div class="cc-filters">
          <div class="cc-filter">
            <label class="cc-filter-label" for="cc-customer">{{ $t('change.customer') }}</label>
            <select id="cc-customer" v-model="customerFilter" class="form-input">
              <option value="">{{ $t('change.all_customers') }}</option>
              <option value="internal">{{ $t('change.inbox') }}</option>
              <option v-for="c in customers" :key="c.id" :value="String(c.id)">{{ c.name }}</option>
            </select>
          </div>
          <div class="cc-filter">
            <label class="cc-filter-label" for="cc-status">{{ $t('change.status_filter') }}</label>
            <select id="cc-status" v-model="statusFilter" class="form-input">
              <option value="open">{{ $t('change.filter_open') }}</option>
              <option value="all">{{ $t('change.filter_all') }}</option>
            </select>
          </div>
        </div>

        <div class="cc-tabs" role="tablist" :aria-label="$t('change.view_mode')" @keydown="onTabKey">
          <button
            v-for="m in MODES" :key="m"
            :id="`cc-tab-${m}`"
            :ref="el => tabRefs[m] = el"
            type="button"
            role="tab"
            class="cc-tab"
            :class="{ active: viewMode === m }"
            :aria-selected="viewMode === m ? 'true' : 'false'"
            :aria-controls="`cc-panel-${m}`"
            :tabindex="viewMode === m ? 0 : -1"
            @click="viewMode = m"
          ><span aria-hidden="true">{{ MODE_ICONS[m] }}</span> {{ $t(`change.view_${m}`) }}</button>
        </div>
      </div>

      <div v-if="loading && !loaded" class="cc-loading" role="status">
        <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
        <span class="sr-only">{{ $t('common.loading') }}</span>
      </div>

      <template v-else>
        <p class="sr-only" aria-live="polite">{{ $t('change.result_count', { n: shownCount }, shownCount) }}</p>

        <!-- Month view -->
        <div v-show="viewMode === 'month'" id="cc-panel-month" role="tabpanel" aria-labelledby="cc-tab-month" class="cc-panel">
          <div class="cc-grid-wrap">
            <table class="cc-grid">
              <caption class="sr-only">{{ $t('change.calendar_caption', { month: monthTitle }) }}</caption>
              <thead>
                <tr>
                  <th v-for="wd in weekdays" :key="wd.long" scope="col">
                    <abbr :title="wd.long">{{ wd.short }}</abbr>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(week, wi) in weeks" :key="wi">
                  <td
                    v-for="day in week" :key="day.iso"
                    class="cc-day"
                    :class="{ 'cc-out': !day.inMonth, 'cc-today': day.isToday, 'cc-frozen': day.freezes.length }"
                    :aria-current="day.isToday ? 'date' : undefined"
                  >
                    <div class="cc-day-head">
                      <span class="cc-day-num" aria-hidden="true">{{ day.date.getDate() }}</span>
                      <span class="sr-only">{{ day.longLabel }}<template v-if="day.isToday">, {{ $t('change.today') }}</template></span>
                    </div>
                    <p v-for="f in day.freezes" :key="`f${f.id}`" class="cc-freeze-label">
                      <span aria-hidden="true">❄</span> {{ $t('change.freeze_label', { name: f.name }) }}
                    </p>
                    <ul v-if="day.changes.length" class="cc-day-list">
                      <li v-for="ch in visibleChanges(day)" :key="ch.id">
                        <RouterLink
                          :to="changeLink(ch)"
                          class="cc-chip"
                          :class="`cc-type-${ch.change_type || 'unset'}`"
                          :title="chipTitle(ch)"
                        >
                          <span class="cc-chip-type">{{ typeShort(ch.change_type) }}</span>
                          <span v-if="startsOn(ch, day)" class="cc-chip-time">{{ formatTime(ch.planned_start) }}</span>
                          <span class="cc-chip-title">{{ ch.title }}</span>
                          <span class="sr-only">— {{ typeLabel(ch.change_type) }}, {{ statusLabel(ch.change_status) }}<template v-if="ch.risk">, {{ riskLabel(ch.risk) }}</template></span>
                        </RouterLink>
                      </li>
                    </ul>
                    <button
                      v-if="day.changes.length > MAX_PER_DAY"
                      type="button"
                      class="cc-more"
                      :aria-expanded="expandedDays.has(day.iso) ? 'true' : 'false'"
                      @click="toggleDay(day.iso)"
                    >
                      {{ expandedDays.has(day.iso) ? $t('change.show_less') : $t('change.more_count', { n: day.changes.length - MAX_PER_DAY }) }}
                      <span class="sr-only">— {{ day.longLabel }}</span>
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="cc-legend">
            <span class="cc-legend-item"><span class="cc-legend-swatch cc-legend-freeze" aria-hidden="true"></span> ❄ {{ $t('change.legend_freeze') }}</span>
            <span class="cc-legend-item"><span class="cc-chip-type cc-type-emergency-inline">{{ typeShort('emergency') }}</span> {{ typeLabel('emergency') }}</span>
            <span class="cc-legend-item"><span class="cc-chip-type">{{ typeShort('normal') }}</span> {{ typeLabel('normal') }}</span>
            <span class="cc-legend-item"><span class="cc-chip-type">{{ typeShort('standard') }}</span> {{ typeLabel('standard') }}</span>
          </p>
        </div>

        <!-- Week view: a time grid with one column per day -->
        <div v-show="viewMode === 'week'" id="cc-panel-week" role="tabpanel" aria-labelledby="cc-tab-week" class="cc-panel">
          <div class="cc-week-wrap">
            <div class="cc-week">
              <div class="cc-week-head" aria-hidden="true">
                <div class="cc-week-corner"></div>
                <div v-for="day in weekDays" :key="day.iso" class="cc-week-dayhead" :class="{ 'cc-today-head': day.isToday }">
                  <span class="cc-week-wd">{{ day.weekday }}</span>
                  <span class="cc-week-dnum">{{ day.date.getDate() }}</span>
                </div>
              </div>
              <div ref="weekScrollEl" class="cc-week-scroll">
                <div class="cc-week-body" :style="{ height: `${HOUR_PX * 24}px` }">
                  <div class="cc-week-hours" aria-hidden="true">
                    <span v-for="h in 24" :key="h" class="cc-week-hour" :style="{ top: `${(h - 1) * HOUR_PX}px` }">{{ hourLabel(h - 1) }}</span>
                  </div>
                  <section
                    v-for="day in weekDays" :key="day.iso"
                    class="cc-week-col"
                    :class="{ 'cc-today-col': day.isToday }"
                    :aria-label="day.longLabel"
                  >
                    <h3 class="sr-only">{{ day.longLabel }}<template v-if="day.isToday">, {{ $t('change.today') }}</template></h3>
                    <div v-for="h in 23" :key="h" class="cc-week-line" :style="{ top: `${h * HOUR_PX}px` }" aria-hidden="true"></div>
                    <div
                      v-for="fz in day.freezes" :key="`f${fz.freeze.id}`"
                      class="cc-week-freeze"
                      :style="{ top: `${fz.top}px`, height: `${fz.height}px` }"
                    >
                      <span class="cc-week-freeze-label"><span aria-hidden="true">❄</span> {{ $t('change.freeze_label', { name: fz.freeze.name }) }}</span>
                    </div>
                    <div v-if="day.isToday" class="cc-week-now" :style="{ top: `${nowTop}px` }" aria-hidden="true"></div>
                    <p v-if="!day.blocks.length" class="sr-only">{{ $t('change.week_day_empty') }}</p>
                    <ul v-else class="cc-week-blocks">
                      <li
                        v-for="b in day.blocks" :key="b.change.id"
                        class="cc-week-li"
                        :style="{ top: `${b.top}px`, height: `${b.height}px`, left: `calc(${b.left}% + 2px)`, width: `calc(${b.width}% - 4px)` }"
                      >
                        <RouterLink
                          :to="changeLink(b.change)"
                          class="cc-week-block"
                          :class="[`cc-type-${b.change.change_type || 'unset'}`, { 'cc-week-block-short': b.height < 34 }]"
                          :title="chipTitle(b.change)"
                        >
                          <span class="cc-week-time">
                            <span v-if="b.continuesBefore" aria-hidden="true">↤ </span>{{ b.timeRange }}<span v-if="b.continuesAfter" aria-hidden="true"> ↦</span>
                          </span>
                          <span class="cc-week-btitle"><span class="cc-chip-type">{{ typeShort(b.change.change_type) }}</span> {{ b.change.title }}</span>
                          <span class="sr-only">— {{ typeLabel(b.change.change_type) }}, {{ statusLabel(b.change.change_status) }}<template v-if="b.change.risk">, {{ riskLabel(b.change.risk) }}</template><template v-if="b.continuesBefore">, {{ $t('change.continues_from_previous_day') }}</template><template v-if="b.continuesAfter">, {{ $t('change.continues_next_day') }}</template></span>
                        </RouterLink>
                      </li>
                    </ul>
                  </section>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- List view -->
        <div v-show="viewMode === 'list'" id="cc-panel-list" role="tabpanel" aria-labelledby="cc-tab-list" class="cc-panel">
          <div v-if="!monthChanges.length" class="cc-empty-small">{{ $t('change.none_scheduled') }}</div>
          <div v-else class="cc-table-wrap">
            <table class="cc-table">
              <caption class="sr-only">{{ $t('change.list_caption', { month: monthTitle }) }}</caption>
              <thead>
                <tr>
                  <th scope="col">{{ $t('change.window') }}</th>
                  <th scope="col">{{ $t('ticket.title') }}</th>
                  <th scope="col">{{ $t('change.customer') }}</th>
                  <th scope="col">{{ $t('change.type') }}</th>
                  <th scope="col">{{ $t('common.status') }}</th>
                  <th scope="col">{{ $t('change.risk') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="ch in monthChanges" :key="ch.id" :class="{ 'cc-row-frozen': overlapsFreeze(ch) }">
                  <td class="cc-nowrap">
                    {{ formatDateTime(ch.planned_start) }}<br />
                    <span class="cc-muted">– {{ ch.planned_end ? formatDateTime(ch.planned_end) : $t('change.no_end') }}</span>
                    <span v-if="overlapsFreeze(ch)" class="cc-badge cc-badge-freeze"><span aria-hidden="true">❄</span> {{ $t('change.in_freeze') }}</span>
                  </td>
                  <th scope="row" class="cc-title-cell"><RouterLink :to="changeLink(ch)" class="cc-link">#{{ ch.id }} {{ ch.title }}</RouterLink></th>
                  <td>{{ ch.customer_name || $t('change.inbox') }}</td>
                  <td><span class="cc-badge" :class="`cc-b-type-${ch.change_type || 'unset'}`">{{ typeLabel(ch.change_type) }}</span></td>
                  <td><span class="cc-badge" :class="`cc-b-st-${ch.change_status || 'unset'}`">{{ statusLabel(ch.change_status) }}</span></td>
                  <td><span v-if="ch.risk" class="cc-badge" :class="`cc-b-risk-${ch.risk}`">{{ riskLabel(ch.risk) }}</span><span v-else class="cc-muted">—</span></td>
                </tr>
              </tbody>
            </table>
          </div>

          <section v-if="monthFreezes.length" class="cc-section" aria-labelledby="cc-freezes-h">
            <h2 id="cc-freezes-h" class="cc-section-title">{{ $t('change.freezes_title') }}</h2>
            <ul class="cc-freeze-list">
              <li v-for="f in monthFreezes" :key="f.id">
                <span aria-hidden="true">❄</span>
                <strong>{{ f.name }}</strong>
                <span class="cc-muted">{{ formatDateTime(f.start_at) }} – {{ formatDateTime(f.end_at) }}</span>
                <span class="cc-muted">· {{ f.customer_id ? customerName(f.customer_id) : $t('change.freeze_global') }}</span>
              </li>
            </ul>
          </section>
        </div>

        <!-- Unscheduled -->
        <section class="cc-section" aria-labelledby="cc-unsched-h">
          <h2 id="cc-unsched-h" class="cc-section-title">{{ $t('change.unscheduled_title') }}</h2>
          <p class="cc-section-desc">{{ $t('change.unscheduled_desc') }}</p>
          <div v-if="!unscheduled.length" class="cc-empty-small">{{ $t('change.none_unscheduled') }}</div>
          <ul v-else class="cc-unsched-list">
            <li v-for="ch in unscheduled" :key="ch.id" class="cc-unsched-item">
              <RouterLink :to="changeLink(ch)" class="cc-link">#{{ ch.id }} {{ ch.title }}</RouterLink>
              <span class="cc-muted">{{ ch.customer_name || $t('change.inbox') }}</span>
              <span class="cc-badge" :class="`cc-b-type-${ch.change_type || 'unset'}`">{{ typeLabel(ch.change_type) }}</span>
              <span class="cc-badge" :class="`cc-b-st-${ch.change_status || 'unset'}`">{{ statusLabel(ch.change_status) }}</span>
              <span v-if="ch.risk" class="cc-badge" :class="`cc-b-risk-${ch.risk}`">{{ riskLabel(ch.risk) }}</span>
            </li>
          </ul>
        </section>
      </template>
    </template>
  </main>
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { changesApi } from '@/api/itsm'
import { useAuthStore } from '@/stores/auth'
import { useCustomersStore } from '@/stores/customers'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

const MODES = ['month', 'week', 'list']
const MODE_ICONS = { month: '▦', week: '▥', list: '☷' }
const HOUR_PX = 44
const MIN_BLOCK_MIN = 30 // shortest block drawn, in minutes, so tiny windows stay clickable
const MAX_PER_DAY = 3
const CLOSED_STATUSES = ['completed', 'failed', 'cancelled']
const VIEW_KEY = 'change_calendar_view'

const { t, te, locale } = useI18n()
const auth = useAuthStore()
const customersStore = useCustomersStore()
const ui = useUIStore()
const { formatDateTime, formatTime } = useDateFormat()

function readView() {
  try { const v = localStorage.getItem(VIEW_KEY); return MODES.includes(v) ? v : 'month' } catch { return 'month' }
}
const viewMode = ref(readView())
watch(viewMode, v => { try { localStorage.setItem(VIEW_KEY, v) } catch {} })

const now = new Date()
const cursor = ref({ y: now.getFullYear(), m: now.getMonth() })
const data = ref({ changes: [], freezes: [] })
const loading = ref(false)
const loaded = ref(false)
const forbidden = ref(false)
const customerFilter = ref('')
const statusFilter = ref('open')
const expandedDays = reactive(new Set())
const tabRefs = {}

const customers = computed(() => [...customersStore.customers].sort((a, b) => a.name.localeCompare(b.name)))
function customerName(id) {
  return customersStore.customers.find(c => c.id === id)?.name || `#${id}`
}

// ── Date helpers (local time) ───────────────────────────────────────────────
function isoDay(d) {
  const p = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
function addDays(d, n) {
  const r = new Date(d)
  r.setDate(r.getDate() + n)
  return r
}

const weekStart = computed(() => auth.user?.week_start || 'monday')

const weekdays = computed(() => {
  const shortFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
  const longFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'long' })
  // 2024-01-07 is a Sunday, 2024-01-01 a Monday.
  const first = weekStart.value === 'sunday' ? new Date(2024, 0, 7) : new Date(2024, 0, 1)
  return Array.from({ length: 7 }, (_, i) => {
    const d = addDays(first, i)
    return { short: shortFmt.format(d), long: longFmt.format(d) }
  })
})

// ── Week view ───────────────────────────────────────────────────────────────
function startOfWeek(d) {
  const day = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const dow = day.getDay()
  return addDays(day, -(weekStart.value === 'sunday' ? dow : (dow + 6) % 7))
}
const weekAnchor = ref(startOfWeek(now))
watch(weekStart, () => { weekAnchor.value = startOfWeek(weekAnchor.value) })

// ISO 8601 week number; for Sunday-based weeks the Monday after the anchor
// is used, so a week is numbered by the ISO week it mostly overlaps.
function isoWeekNumber(d) {
  const t0 = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))
  const dow = t0.getUTCDay() || 7
  t0.setUTCDate(t0.getUTCDate() + 4 - dow)
  const yearStart = new Date(Date.UTC(t0.getUTCFullYear(), 0, 1))
  return Math.ceil(((t0 - yearStart) / 86400000 + 1) / 7)
}

const weekTitle = computed(() => {
  const start = weekAnchor.value
  const end = addDays(start, 6)
  const fmt = new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' })
  const range = typeof fmt.formatRange === 'function' ? fmt.formatRange(start, end) : `${fmt.format(start)} – ${fmt.format(end)}`
  const monday = weekStart.value === 'sunday' ? addDays(start, 1) : start
  return `${t('change.week_number', { n: isoWeekNumber(monday) })} · ${range}`
})

function hourLabel(h) {
  return formatTime(new Date(2024, 0, 1, h, 0).toISOString())
}

// Lay out one day's blocks: clip each window to the day, then assign
// side-by-side lanes to overlapping blocks (greedy, per overlap cluster).
function layoutDay(dayStart, dayEnd) {
  const dayMs = dayEnd - dayStart
  const items = scheduled.value
    .filter(ch => overlaps(ch, 'planned_start', 'planned_end', dayStart, dayEnd))
    .map(ch => {
      const [s, e0] = windowOf(ch, 'planned_start', 'planned_end')
      const e = e0 > s ? e0 : new Date(s.getTime() + 60 * 60000)
      const cs = Math.max(s, dayStart), ce = Math.min(e, dayEnd)
      const startMin = (cs - dayStart) / 60000
      const endMin = Math.max((ce - dayStart) / 60000, startMin + MIN_BLOCK_MIN)
      return {
        change: ch, startMin, endMin: Math.min(endMin, dayMs / 60000),
        continuesBefore: s < dayStart, continuesAfter: e > dayEnd,
        timeRange: `${formatTime(new Date(cs).toISOString())}–${formatTime(new Date(ce).toISOString())}`,
      }
    })
    .sort((a, b) => a.startMin - b.startMin || b.endMin - a.endMin)

  const out = []
  let cluster = [], clusterEnd = -1
  const flush = () => {
    const laneEnds = []
    for (const it of cluster) {
      let lane = laneEnds.findIndex(end => end <= it.startMin)
      if (lane === -1) { lane = laneEnds.length; laneEnds.push(0) }
      laneEnds[lane] = it.endMin
      it.lane = lane
    }
    for (const it of cluster) {
      const w = 100 / laneEnds.length
      out.push({ ...it, left: it.lane * w, width: w })
    }
    cluster = []
  }
  for (const it of items) {
    if (cluster.length && it.startMin >= clusterEnd) flush()
    cluster.push(it)
    clusterEnd = Math.max(clusterEnd, it.endMin)
  }
  if (cluster.length) flush()
  return out.map(b => ({
    ...b,
    top: b.startMin / 60 * HOUR_PX,
    height: Math.max((b.endMin - b.startMin) / 60 * HOUR_PX, MIN_BLOCK_MIN / 60 * HOUR_PX),
  }))
}

const weekDays = computed(() => {
  const wdFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
  const longFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
  const todayIso = isoDay(new Date())
  return Array.from({ length: 7 }, (_, i) => {
    const d = addDays(weekAnchor.value, i)
    const next = addDays(d, 1)
    const freezes = filteredFreezes.value
      .filter(f => overlaps(f, 'start_at', 'end_at', d, next))
      .map(f => {
        const s = Math.max(new Date(f.start_at), d), e = Math.min(new Date(f.end_at), next)
        return { freeze: f, top: (s - d) / 3600000 * HOUR_PX, height: Math.max((e - s) / 3600000 * HOUR_PX, 2) }
      })
    return {
      iso: isoDay(d), date: d, isToday: isoDay(d) === todayIso,
      weekday: wdFmt.format(d), longLabel: longFmt.format(d),
      blocks: layoutDay(d, next), freezes,
    }
  })
})

const weekChangeCount = computed(() => new Set(weekDays.value.flatMap(d => d.blocks.map(b => b.change.id))).size)
const shownCount = computed(() => viewMode.value === 'week' ? weekChangeCount.value : monthChanges.value.length)

const nowTick = ref(Date.now())
let nowTimer = null
const nowTop = computed(() => {
  const n = new Date(nowTick.value)
  return (n.getHours() * 60 + n.getMinutes()) / 60 * HOUR_PX
})

// Scroll the week grid to 07:00, or earlier when a change starts earlier
// (blocks carried over from the previous night don't count).
const weekScrollEl = ref(null)
function scrollWeekIntoView() {
  nextTick(() => {
    const el = weekScrollEl.value
    if (!el) return
    const starts = weekDays.value.flatMap(d => d.blocks.filter(b => !b.continuesBefore).map(b => b.top))
    const first = Math.min(...starts, 7 * HOUR_PX)
    el.scrollTop = Math.max(first - HOUR_PX / 2, 0)
  })
}

// Keep the month cursor on the week's month, so the month grid's data range
// (whole weeks around the month) always covers the shown week.
function syncCursorToWeek() {
  const mid = addDays(weekAnchor.value, 3)
  if (mid.getFullYear() !== cursor.value.y || mid.getMonth() !== cursor.value.m) {
    cursor.value = { y: mid.getFullYear(), m: mid.getMonth() }
  }
}

const monthTitle = computed(() =>
  new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }).format(new Date(cursor.value.y, cursor.value.m, 1)))

const monthStart = computed(() => new Date(cursor.value.y, cursor.value.m, 1))
const monthEnd = computed(() => new Date(cursor.value.y, cursor.value.m + 1, 1))

const gridStart = computed(() => {
  const first = monthStart.value
  const dow = first.getDay()
  const offset = weekStart.value === 'sunday' ? dow : (dow + 6) % 7
  return addDays(first, -offset)
})
const gridEnd = computed(() => {
  // Exclusive end: fill out the last week.
  const last = addDays(monthEnd.value, -1)
  const dow = last.getDay()
  const lastCol = weekStart.value === 'sunday' ? dow : (dow + 6) % 7
  return addDays(last, 7 - lastCol)
})

// ── Data ────────────────────────────────────────────────────────────────────
function windowOf(item, startKey, endKey) {
  const s = new Date(item[startKey])
  const e = item[endKey] ? new Date(item[endKey]) : s
  return [s, e]
}
function overlaps(item, startKey, endKey, from, to) {
  const [s, e] = windowOf(item, startKey, endKey)
  if (e <= s) return s >= from && s < to
  return s < to && e > from
}

function matchesCustomer(customerId) {
  if (!customerFilter.value) return true
  if (customerFilter.value === 'internal') return customerId == null
  return String(customerId) === customerFilter.value
}

const filteredChanges = computed(() => (data.value.changes || []).filter(ch =>
  matchesCustomer(ch.customer_id) &&
  (statusFilter.value === 'all' || !CLOSED_STATUSES.includes(ch.change_status))))

// Global freezes (no customer) always apply; customer freezes only when that
// customer (or every customer) is shown.
const filteredFreezes = computed(() => (data.value.freezes || []).filter(f =>
  f.customer_id == null || !customerFilter.value || String(f.customer_id) === customerFilter.value))

const scheduled = computed(() => filteredChanges.value.filter(ch => ch.planned_start))
const unscheduled = computed(() => filteredChanges.value.filter(ch => !ch.planned_start))

const monthChanges = computed(() => scheduled.value
  .filter(ch => overlaps(ch, 'planned_start', 'planned_end', monthStart.value, monthEnd.value))
  .sort((a, b) => new Date(a.planned_start) - new Date(b.planned_start)))
const monthFreezes = computed(() => filteredFreezes.value
  .filter(f => overlaps(f, 'start_at', 'end_at', monthStart.value, monthEnd.value)))

function overlapsFreeze(ch) {
  const [s, e] = windowOf(ch, 'planned_start', 'planned_end')
  return filteredFreezes.value.some(f => overlaps(f, 'start_at', 'end_at', s, e > s ? e : new Date(s.getTime() + 1)))
}

const weeks = computed(() => {
  const longFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
  const todayIso = isoDay(new Date())
  const out = []
  let week = []
  for (let d = new Date(gridStart.value); d < gridEnd.value; d = addDays(d, 1)) {
    const next = addDays(d, 1)
    const iso = isoDay(d)
    week.push({
      iso,
      date: new Date(d),
      dayStart: new Date(d),
      inMonth: d.getMonth() === cursor.value.m,
      isToday: iso === todayIso,
      longLabel: longFmt.format(d),
      changes: scheduled.value.filter(ch => overlaps(ch, 'planned_start', 'planned_end', d, next)),
      freezes: filteredFreezes.value.filter(f => overlaps(f, 'start_at', 'end_at', d, next)),
    })
    if (week.length === 7) { out.push(week); week = [] }
  }
  if (week.length) out.push(week)
  return out
})

function visibleChanges(day) {
  return expandedDays.has(day.iso) ? day.changes : day.changes.slice(0, MAX_PER_DAY)
}
function toggleDay(iso) {
  if (expandedDays.has(iso)) expandedDays.delete(iso)
  else expandedDays.add(iso)
}
function startsOn(ch, day) {
  return isoDay(new Date(ch.planned_start)) === day.iso
}

let seq = 0
async function load() {
  const s = ++seq
  loading.value = true
  try {
    // One day of slack on each side absorbs the UTC/local date difference.
    const { data: res } = await changesApi.calendar({
      from: isoDay(addDays(gridStart.value, -1)),
      to: isoDay(gridEnd.value),
      unscheduled: true,
    })
    if (s !== seq) return
    data.value = { changes: res?.changes || [], freezes: res?.freezes || [] }
    loaded.value = true
  } catch (e) {
    if (s !== seq) return
    if (e?.response?.status === 403) forbidden.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    if (s === seq) loading.value = false
  }
}

function shiftMonth(n) {
  const d = new Date(cursor.value.y, cursor.value.m + n, 1)
  cursor.value = { y: d.getFullYear(), m: d.getMonth() }
}
function shift(n) {
  if (viewMode.value === 'week') {
    weekAnchor.value = addDays(weekAnchor.value, 7 * n)
    syncCursorToWeek()
    scrollWeekIntoView()
  } else {
    shiftMonth(n)
  }
}
function goToday() {
  const d = new Date()
  cursor.value = { y: d.getFullYear(), m: d.getMonth() }
  weekAnchor.value = startOfWeek(d)
  if (viewMode.value === 'week') scrollWeekIntoView()
}
// Switching to the week view shows a week of the month being looked at:
// the current week when that month is shown, else the month's first week.
watch(viewMode, (m, old) => {
  if (m !== 'week' || old === 'week') return
  const today = new Date()
  const inShownMonth = today.getFullYear() === cursor.value.y && today.getMonth() === cursor.value.m
  const anchorInMonth = addDays(weekAnchor.value, 3).getMonth() === cursor.value.m
  if (!anchorInMonth) weekAnchor.value = startOfWeek(inShownMonth ? today : new Date(cursor.value.y, cursor.value.m, 1))
  scrollWeekIntoView()
})
watch(cursor, () => { expandedDays.clear(); load() })
watch(weekStart, load)

function onTabKey(e) {
  const i = MODES.indexOf(viewMode.value)
  let next = null
  if (e.key === 'ArrowRight') next = MODES[(i + 1) % MODES.length]
  else if (e.key === 'ArrowLeft') next = MODES[(i - 1 + MODES.length) % MODES.length]
  else if (e.key === 'Home') next = MODES[0]
  else if (e.key === 'End') next = MODES[MODES.length - 1]
  if (!next) return
  e.preventDefault()
  viewMode.value = next
  nextTick(() => tabRefs[next]?.focus())
}

// ── Labels ──────────────────────────────────────────────────────────────────
function labelOr(key, fallback) { return te(key) ? t(key) : fallback }
function typeLabel(v) { return labelOr(`change.type_${v || 'unset'}`, v) }
function typeShort(v) { return labelOr(`change.type_short_${v || 'unset'}`, v) }
function statusLabel(v) { return labelOr(`change.status_${v || 'unset'}`, v) }
function riskLabel(v) { return t('change.risk_value', { risk: labelOr(`change.risk_${v}`, v) }) }
function changeLink(ch) {
  return ch.customer_id ? `/customers/${ch.customer_id}/tickets/${ch.id}` : `/tickets/inbox/${ch.id}`
}
function chipTitle(ch) {
  const parts = [`#${ch.id} ${ch.title}`, typeLabel(ch.change_type), statusLabel(ch.change_status)]
  if (ch.risk) parts.push(riskLabel(ch.risk))
  parts.push(`${formatDateTime(ch.planned_start)} – ${ch.planned_end ? formatDateTime(ch.planned_end) : t('change.no_end')}`)
  if (ch.customer_name) parts.push(ch.customer_name)
  return parts.join(' · ')
}

onMounted(async () => {
  if (!customersStore.customers.length) customersStore.fetchCustomers()
  nowTimer = setInterval(() => { nowTick.value = Date.now() }, 60000)
  if (viewMode.value === 'week') syncCursorToWeek()
  await load()
  if (viewMode.value === 'week') scrollWeekIntoView()
})
onUnmounted(() => clearInterval(nowTimer))
</script>

<style scoped>
.cc-main { padding: 24px 32px; max-width: 1400px; margin: 0 auto; }
.cc-header { margin-bottom: 16px; }
.cc-title { font-size: 22px; font-weight: 700; margin: 0 0 4px; }
.cc-subtitle { color: var(--color-text-muted); margin: 0; font-size: 13px; }

.cc-toolbar { display: flex; flex-wrap: wrap; gap: 12px 20px; align-items: flex-end; justify-content: space-between; margin-bottom: 14px; }
.cc-nav { display: flex; align-items: center; gap: 6px; }
.cc-month { font-size: 17px; font-weight: 700; margin: 0 0 0 8px; text-transform: capitalize; min-width: 170px; }
.cc-filters { display: flex; flex-wrap: wrap; gap: 12px; }
.cc-filter { display: flex; flex-direction: column; gap: 4px; min-width: 150px; }
.cc-filter-label { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }

.cc-tabs { display: inline-flex; border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
.cc-tab {
  background: var(--color-surface); color: var(--color-text); border: none; padding: 6px 12px;
  font-size: 13px; cursor: pointer;
}
.cc-tab + .cc-tab { border-left: 1px solid var(--color-border); }
.cc-tab.active { background: var(--color-primary); color: var(--color-surface); font-weight: 600; }
.cc-tab:focus-visible { outline: 2px solid var(--color-primary); outline-offset: -2px; box-shadow: inset 0 0 0 4px var(--color-surface); }

.cc-loading { display: flex; justify-content: center; padding: 48px; }
.cc-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }
.cc-empty-small { color: var(--color-text-muted); font-size: 13px; padding: 8px 0; }
.cc-muted { color: var(--color-text-muted); font-size: 12px; }
.cc-nowrap { white-space: nowrap; }

/* Month grid */
.cc-grid-wrap { overflow-x: auto; }
.cc-grid { width: 100%; min-width: 720px; border-collapse: collapse; table-layout: fixed; background: var(--color-surface); }
.cc-grid th {
  font-size: 12px; font-weight: 600; color: var(--color-text-muted); padding: 6px;
  border: 1px solid var(--color-border); background: var(--color-bg); text-transform: capitalize;
}
.cc-grid abbr { text-decoration: none; }
.cc-day { vertical-align: top; height: 110px; padding: 4px; border: 1px solid var(--color-border); }
.cc-out { background: var(--color-bg); }
.cc-out .cc-day-num { color: var(--color-text-muted); }
.cc-frozen {
  background-image: repeating-linear-gradient(45deg,
    color-mix(in srgb, var(--color-primary) 10%, transparent) 0 6px,
    transparent 6px 12px);
}
.cc-day-head { display: flex; justify-content: flex-end; }
.cc-day-num { font-size: 12px; font-weight: 600; min-width: 22px; height: 22px; line-height: 22px; text-align: center; border-radius: 11px; }
.cc-today .cc-day-num { background: var(--color-primary); color: var(--color-surface); }
.cc-today { box-shadow: inset 0 0 0 2px var(--color-primary); }
.cc-freeze-label {
  margin: 2px 0; font-size: 11px; font-weight: 600; color: var(--color-text);
  background: color-mix(in srgb, var(--color-primary) 16%, var(--color-surface));
  border-radius: var(--radius-sm); padding: 1px 4px;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.cc-day-list { list-style: none; margin: 2px 0 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
.cc-chip {
  display: flex; align-items: baseline; gap: 4px; font-size: 11px; line-height: 1.35;
  padding: 1px 4px; border-radius: var(--radius-sm); text-decoration: none; color: var(--color-text);
  border: 1px solid var(--color-border); border-left-width: 3px; background: var(--color-surface);
  overflow: hidden;
}
.cc-chip:hover { border-color: var(--color-primary); }
.cc-chip:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 1px; }
.cc-type-emergency { border-left-color: var(--color-danger); }
.cc-type-normal { border-left-color: var(--color-primary); }
.cc-type-standard { border-left-color: var(--color-success); }
.cc-chip-type {
  flex: 0 0 auto; font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: .02em;
  padding: 0 3px; border-radius: 3px; border: 1px solid var(--color-border); color: var(--color-text);
}
.cc-type-emergency .cc-chip-type, .cc-type-emergency-inline { border-color: var(--color-danger); }
.cc-chip-time { flex: 0 0 auto; color: var(--color-text-muted); font-variant-numeric: tabular-nums; }
.cc-chip-title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cc-more {
  margin-top: 2px; background: none; border: none; padding: 1px 2px; font-size: 11px;
  color: var(--color-primary); cursor: pointer; text-decoration: underline;
}
.cc-more:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 1px; }

.cc-legend { display: flex; flex-wrap: wrap; gap: 6px 16px; font-size: 12px; color: var(--color-text-muted); margin: 8px 0 0; }
.cc-legend-item { display: inline-flex; align-items: center; gap: 6px; }
.cc-legend-swatch { width: 16px; height: 12px; border: 1px solid var(--color-border); display: inline-block; }
.cc-legend-freeze { background-image: repeating-linear-gradient(45deg, color-mix(in srgb, var(--color-primary) 30%, transparent) 0 3px, transparent 3px 6px); }

/* Week view */
.cc-week-wrap { overflow-x: auto; }
.cc-week { min-width: 760px; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); }
.cc-week-head { display: grid; grid-template-columns: 56px repeat(7, 1fr); border-bottom: 1px solid var(--color-border); background: var(--color-bg); }
.cc-week-dayhead { display: flex; align-items: baseline; justify-content: center; gap: 6px; padding: 6px 4px; border-left: 1px solid var(--color-border); }
.cc-week-wd { font-size: 12px; font-weight: 600; color: var(--color-text-muted); text-transform: capitalize; }
.cc-week-dnum { font-size: 15px; font-weight: 700; min-width: 26px; height: 26px; line-height: 26px; text-align: center; border-radius: 13px; }
.cc-today-head .cc-week-dnum { background: var(--color-primary); color: var(--color-surface); }
.cc-week-scroll { max-height: 70vh; overflow-y: auto; }
.cc-week-body { position: relative; display: grid; grid-template-columns: 56px repeat(7, 1fr); }
.cc-week-hours { position: relative; }
.cc-week-hour {
  position: absolute; right: 6px; transform: translateY(-50%); font-size: 11px;
  color: var(--color-text-muted); font-variant-numeric: tabular-nums;
}
.cc-week-hour:first-child { transform: none; }
.cc-week-col { position: relative; border-left: 1px solid var(--color-border); }
.cc-today-col { background: color-mix(in srgb, var(--color-primary) 5%, transparent); }
.cc-week-line { position: absolute; left: 0; right: 0; border-top: 1px solid color-mix(in srgb, var(--color-border) 70%, transparent); }
.cc-week-freeze {
  position: absolute; left: 0; right: 0; overflow: hidden;
  background-image: repeating-linear-gradient(45deg,
    color-mix(in srgb, var(--color-primary) 12%, transparent) 0 6px,
    transparent 6px 12px);
}
.cc-week-freeze-label {
  display: inline-block; margin: 2px; font-size: 10px; font-weight: 600; color: var(--color-text);
  background: color-mix(in srgb, var(--color-primary) 16%, var(--color-surface));
  border-radius: var(--radius-sm); padding: 0 4px; max-width: calc(100% - 4px);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.cc-week-now { position: absolute; left: 0; right: 0; border-top: 2px solid var(--color-danger); z-index: 2; }
.cc-week-now::before {
  content: ''; position: absolute; left: -4px; top: -5px; width: 8px; height: 8px;
  border-radius: 50%; background: var(--color-danger);
}
.cc-week-blocks { list-style: none; margin: 0; padding: 0; }
.cc-week-li { position: absolute; z-index: 1; }
.cc-week-block {
  display: flex; flex-direction: column; gap: 1px; height: 100%; box-sizing: border-box;
  padding: 2px 5px; font-size: 11px; line-height: 1.3; overflow: hidden;
  text-decoration: none; color: var(--color-text);
  background: color-mix(in srgb, var(--color-primary) 10%, var(--color-surface));
  border: 1px solid var(--color-border); border-left-width: 3px; border-radius: var(--radius-sm);
}
.cc-week-block.cc-type-emergency { background: color-mix(in srgb, var(--color-danger) 10%, var(--color-surface)); }
.cc-week-block.cc-type-standard { background: color-mix(in srgb, var(--color-success) 10%, var(--color-surface)); }
.cc-week-block:hover { border-color: var(--color-primary); z-index: 3; }
.cc-week-block:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 1px; z-index: 3; }
.cc-week-block-short { flex-direction: row; align-items: center; gap: 4px; }
.cc-week-time { color: var(--color-text-muted); font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.cc-week-block-short .cc-week-time { white-space: nowrap; }
.cc-week-btitle { font-weight: 600; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.cc-week-block-short .cc-week-btitle { white-space: nowrap; }

/* List view */
.cc-table-wrap { overflow-x: auto; }
.cc-table {
  width: 100%; border-collapse: collapse; font-size: 13px;
  background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius);
}
.cc-table th, .cc-table td { padding: 8px 12px; border-bottom: 1px solid var(--color-border); text-align: left; vertical-align: top; }
.cc-table thead th { font-size: 12px; font-weight: 600; color: var(--color-text-muted); background: var(--color-bg); }
.cc-table tbody tr:last-child th, .cc-table tbody tr:last-child td { border-bottom: none; }
.cc-title-cell { font-weight: 400; }
.cc-link { color: var(--color-text); text-decoration: none; font-weight: 600; }
.cc-link:hover { color: var(--color-primary); text-decoration: underline; }
.cc-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.cc-row-frozen td:first-child { box-shadow: inset 3px 0 0 var(--color-primary); }

.cc-badge {
  display: inline-block; font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px; white-space: nowrap;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
}
.cc-badge-freeze { margin-top: 4px; border-color: var(--color-primary); }
.cc-b-type-emergency { border-color: var(--color-danger); }
.cc-b-type-emergency::before { content: '⚡ '; }
.cc-b-type-normal { border-color: var(--color-primary); }
.cc-b-type-standard { border-color: var(--color-success); }
.cc-b-st-approved, .cc-b-st-scheduled, .cc-b-st-completed { border-color: var(--color-success); }
.cc-b-st-awaiting_approval, .cc-b-st-review { border-color: var(--color-warning); }
.cc-b-st-implementing { border-color: var(--color-primary); }
.cc-b-st-failed { border-color: var(--color-danger); }
.cc-b-st-cancelled { color: var(--color-text-muted); border-style: dashed; }
.cc-b-risk-high { border-color: var(--color-danger); }
.cc-b-risk-medium { border-color: var(--color-warning); }

.cc-section { margin-top: 28px; }
.cc-section-title { font-size: 16px; font-weight: 700; margin: 0 0 10px; }
.cc-section-desc { font-size: 12px; color: var(--color-text-muted); margin: -4px 0 10px; }
.cc-freeze-list, .cc-unsched-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.cc-freeze-list li { font-size: 13px; display: flex; flex-wrap: wrap; gap: 4px 8px; align-items: baseline; }
.cc-unsched-item {
  display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; font-size: 13px;
  background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); padding: 8px 12px;
}
.cc-unsched-item .cc-link { flex: 1 1 240px; min-width: 0; }

@media (max-width: 640px) {
  .cc-main { padding: 16px; }
  .cc-filter { min-width: 0; flex: 1 1 140px; }
}
</style>
