<template>
  <div class="tt-calendar" @keydown="onCalendarKeyDown">
    <p class="cal-hint sr-only">{{ $t('timeTracking.calendar_keyboard_hint') }}</p>

    <div class="cal-toolbar">
      <div v-if="selectedEntries.length" class="cal-selection" role="group" :aria-label="$t('timeTracking.calendar_selected', { n: selectedEntries.length })">
        <span class="cal-selection-count" aria-live="polite">{{ $t('timeTracking.calendar_selected', { n: selectedEntries.length }) }}</span>
        <button type="button" class="btn btn-ghost btn-sm" @click="copyEntries(selectedEntries)">{{ $t('common.copy') }}</button>
        <button type="button" class="btn btn-ghost btn-sm cal-selection-delete" @click="deleteSelection">{{ $t('common.delete') }}</button>
        <button type="button" class="btn btn-ghost btn-sm" @click="clearSelection">{{ $t('timeTracking.calendar_clear_selection') }}</button>
      </div>
      <div class="cal-zoom" role="group" :aria-label="$t('timeTracking.calendar_zoom')">
        <button
          type="button" class="btn btn-ghost btn-sm cal-zoom-btn"
          :disabled="zoomIndex <= 0"
          :title="$t('timeTracking.calendar_zoom_out')" :aria-label="$t('timeTracking.calendar_zoom_out')"
          @click="zoomOut"
        >−</button>
        <button
          type="button" class="btn btn-ghost btn-sm cal-zoom-btn"
          :disabled="zoomIndex >= ZOOM_LEVELS.length - 1"
          :title="$t('timeTracking.calendar_zoom_in')" :aria-label="$t('timeTracking.calendar_zoom_in')"
          @click="zoomIn"
        >+</button>
      </div>
    </div>

    <TimeTrackingCalendarWeekGrid
      v-if="viewGranularity === 'week'"
      :week-days="weekDays"
      :entries="entries"
      :px-per-hour="pxPerHour"
      :customer-name="customerNameFor"
      :project-name="projectNameFor"
      :entry-color="entryColorFor"
      :read-only="readOnly"
      :selected-ids="selectedIds"
      @block-toggle-select="toggleSelect"
      @day-select="onDaySelect"
      @day-contextmenu="onDayContextMenu"
      @slot-click="onSlotClick"
      @slot-contextmenu="onSlotContextMenu"
      @block-contextmenu="onBlockContextMenu"
      @block-open="openEditModal"
      @block-move="(payload) => $emit('move-entry', payload)"
      @block-resize="(payload) => $emit('resize-entry', payload)"
    />
    <!-- Month view is a future granularity; the emit payload shapes above are already
         month-ready (newDate/newStartTime/newEndTime/newMinutes), so adding a
         TimeTrackingCalendarMonthGrid sibling here won't require changes elsewhere. -->

    <ContextMenu
      v-if="ctxMenu"
      :x="ctxMenu.x"
      :y="ctxMenu.y"
      :items="ctxMenu.items"
      @select="onCtxSelect"
      @close="ctxMenu = null"
    />

    <TimeEntryModal
      v-if="modalState"
      :entry="modalState.entry"
      :prefill="modalState.prefill"
      :all-customers="allCustomers"
      :all-projects="allProjects"
      :tt-customers="ttCustomers"
      :tt-projects="ttProjects"
      :projects="projects"
      @save="onModalSave"
      @delete="onModalDelete"
      @close="modalState = null"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import TimeTrackingCalendarWeekGrid from './TimeTrackingCalendarWeekGrid.vue'
import ContextMenu from '@/components/common/ContextMenu.vue'
import TimeEntryModal from './TimeEntryModal.vue'
import { parseWallClock, fmtWallClock } from '@/utils/shiftTimeEntries'
import { buildPastePayloads } from '@/utils/calendarClipboard'
import { useUIStore } from '@/stores/ui'
import { assignCustomerColors, assignProjectColors, NO_CUSTOMER_COLOR } from '@/utils/calendarColors'
import { useAuthStore } from '@/stores/auth'

const ZOOM_STORAGE_KEY = 'tt_calendar_zoom'
const ZOOM_LEVELS = [20, 30, 45, 60, 90, 120, 160] // px per hour

function loadZoomIndex() {
  const stored = Number(localStorage.getItem(ZOOM_STORAGE_KEY))
  const idx = ZOOM_LEVELS.indexOf(stored)
  return idx >= 0 ? idx : ZOOM_LEVELS.indexOf(60)
}

const props = defineProps({
  entries: { type: Array, default: () => [] },
  weekDays: { type: Array, required: true },
  allCustomers: { type: Array, default: () => [] },
  allProjects: { type: Array, default: () => [] },
  ttCustomers: { type: Array, default: () => [] },
  ttProjects: { type: Array, default: () => [] },
  projects: { type: Array, default: () => [] },
  viewGranularity: { type: String, default: 'week' }, // 'week' | 'month' (month not yet implemented)
  readOnly: { type: Boolean, default: false },
})
// create-entries / delete-entries carry a whole multi-selection (paste, bulk delete)
// so the parent can confirm once and report one error instead of one per entry.
const emit = defineEmits(['save-entry', 'delete-entry', 'create-entries', 'delete-entries', 'move-entry', 'resize-entry'])

const { t } = useI18n()
const auth = useAuthStore()
const ui = useUIStore()

const ctxMenu = ref(null)
const modalState = ref(null) // { entry } | { prefill }
// Entries last copied (one block, a multi-selection or a whole day), ready to paste.
// Kept across week navigation so a day can be copied into another week.
const copiedEntries = ref([])

// Multi-selection: Ctrl/Cmd/Shift+click (or +Enter/Space) toggles a block, clicking a
// day header selects that whole day. Cleared when the displayed week changes.
const selectedIds = ref(new Set())
const selectedEntries = computed(() => props.entries.filter((e) => selectedIds.value.has(e.id)))

watch(() => props.weekDays.map((d) => d.iso).join(), () => clearSelection())

function clearSelection() {
  if (selectedIds.value.size) selectedIds.value = new Set()
}

function toggleSelect(entry) {
  if (props.readOnly) return
  const next = new Set(selectedIds.value)
  if (next.has(entry.id)) next.delete(entry.id)
  else next.add(entry.id)
  selectedIds.value = next
}

function entriesOnDay(date) {
  return props.entries.filter((e) => e.date && e.date.slice(0, 10) === date)
}

// A plain click selects just that day (or clears it when it already is the whole
// selection); with a modifier it adds the day to, or removes it from, the selection.
function onDaySelect({ date, additive }) {
  if (props.readOnly) return
  const ids = entriesOnDay(date).map((e) => e.id)
  if (!ids.length) return
  const allSelected = ids.every((id) => selectedIds.value.has(id))
  if (!additive) {
    const onlyThisDay = allSelected && selectedIds.value.size === ids.length
    selectedIds.value = onlyThisDay ? new Set() : new Set(ids)
    return
  }
  const next = new Set(selectedIds.value)
  for (const id of ids) allSelected ? next.delete(id) : next.add(id)
  selectedIds.value = next
}

function copyEntries(list) {
  if (!list.length) return
  copiedEntries.value = [...list]
  ui.success(t('timeTracking.calendar_copied', { n: list.length }, list.length))
}

function deleteSelection() {
  const list = selectedEntries.value
  if (!list.length) return
  emit('delete-entries', list)
  clearSelection()
}

function pasteLabel() {
  const n = copiedEntries.value.length
  return n > 1 ? `${t('common.paste')} (${n})` : t('common.paste')
}

function pasteItems() {
  const empty = !copiedEntries.value.length
  return [
    { key: 'paste', label: pasteLabel(), disabled: empty },
    { key: 'paste-keep', label: t('timeTracking.paste_keep_times'), disabled: empty },
  ]
}

function onCalendarKeyDown(e) {
  if (props.readOnly || ctxMenu.value || modalState.value) return
  if (e.key === 'Escape' && selectedIds.value.size) { e.preventDefault(); clearSelection(); return }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'c' && selectedIds.value.size
      && !window.getSelection()?.toString()) {
    e.preventDefault()
    copyEntries(selectedEntries.value)
  }
}

// Esc should drop the selection even when focus is outside the calendar (e.g. on the
// toolbar or the page body after clicking a day header with the mouse).
function onDocKeyDown(e) {
  if (e.key !== 'Escape' || !selectedIds.value.size || ctxMenu.value || modalState.value) return
  if (e.defaultPrevented) return
  clearSelection()
}
onMounted(() => document.addEventListener('keydown', onDocKeyDown))
onUnmounted(() => document.removeEventListener('keydown', onDocKeyDown))

const zoomIndex = ref(loadZoomIndex())
const pxPerHour = computed(() => ZOOM_LEVELS[zoomIndex.value])

function zoomIn() {
  if (zoomIndex.value >= ZOOM_LEVELS.length - 1) return
  zoomIndex.value++
  localStorage.setItem(ZOOM_STORAGE_KEY, String(ZOOM_LEVELS[zoomIndex.value]))
}

function zoomOut() {
  if (zoomIndex.value <= 0) return
  zoomIndex.value--
  localStorage.setItem(ZOOM_STORAGE_KEY, String(ZOOM_LEVELS[zoomIndex.value]))
}

function customerNameFor(entry) {
  return entry.customer?.name || ''
}

function projectNameFor(entry) {
  return entry.project?.name || ''
}

const customerColorMap = computed(() => assignCustomerColors(props.allCustomers))
const projectColorMap = computed(() => assignProjectColors(props.allProjects))

function entryColorFor(entry) {
  if (auth.user?.calendar_color_mode === 'project') {
    if (entry.project_id == null) return NO_CUSTOMER_COLOR
    return projectColorMap.value.get(entry.project_id) || NO_CUSTOMER_COLOR
  }
  if (entry.customer_id == null) return NO_CUSTOMER_COLOR
  return customerColorMap.value.get(entry.customer_id) || NO_CUSTOMER_COLOR
}

function onSlotClick({ date, startTime, endTime }) {
  if (props.readOnly) return
  openCreateModal({ date, startTime, endTime })
}

function onSlotContextMenu({ x, y, date, startTime }) {
  if (props.readOnly) return
  ctxMenu.value = {
    x, y,
    items: [
      { key: 'create', label: t('timeTracking.new_entry') },
      ...pasteItems(),
    ],
    target: { date, startTime },
  }
}

function onDayContextMenu({ x, y, date }) {
  if (props.readOnly) return
  const hasEntries = entriesOnDay(date).length > 0
  ctxMenu.value = {
    x, y,
    items: [
      { key: 'select-day', label: t('timeTracking.calendar_select_day'), disabled: !hasEntries },
      { key: 'copy-day', label: t('timeTracking.calendar_copy_day'), disabled: !hasEntries },
      { key: 'paste-keep', label: copiedEntries.value.length > 1
        ? `${t('timeTracking.paste_keep_times')} (${copiedEntries.value.length})`
        : t('timeTracking.paste_keep_times'), disabled: !copiedEntries.value.length },
    ],
    target: { date, startTime: null },
  }
}

// Right-clicking a block that is part of a multi-selection acts on the whole
// selection; any other block gets the single-entry menu.
function onBlockContextMenu({ x, y, entry }) {
  if (props.readOnly) return
  const n = selectedIds.value.size
  if (n > 1 && selectedIds.value.has(entry.id)) {
    ctxMenu.value = {
      x, y,
      items: [
        { key: 'copy-selection', label: `${t('common.copy')} (${n})` },
        { key: 'delete-selection', label: `${t('common.delete')} (${n})`, danger: true },
        { key: 'clear-selection', label: t('timeTracking.calendar_clear_selection') },
      ],
      target: entry,
    }
    return
  }
  ctxMenu.value = {
    x, y,
    items: [
      { key: 'edit', label: t('common.edit') },
      { key: 'copy', label: t('common.copy') },
      { key: 'select-day', label: t('timeTracking.calendar_select_day') },
      { key: 'delete', label: t('common.delete'), danger: true },
    ],
    target: entry,
  }
}

function onCtxSelect(key) {
  const target = ctxMenu.value?.target
  ctxMenu.value = null
  if (!target) return
  if (key === 'create') openCreateModal(target)
  else if (key === 'edit') openEditModal(target)
  else if (key === 'delete') emit('delete-entry', target)
  else if (key === 'copy') copyEntries([target])
  else if (key === 'copy-selection') copyEntries(selectedEntries.value)
  else if (key === 'delete-selection') deleteSelection()
  else if (key === 'clear-selection') clearSelection()
  else if (key === 'select-day') onDaySelect({ date: target.date.slice(0, 10), additive: false })
  else if (key === 'copy-day') copyEntries(entriesOnDay(target.date))
  else if (key === 'paste') pasteEntries(target.date, target.startTime)
  else if (key === 'paste-keep') pasteEntries(target.date, null)
}

function pasteEntries(date, startTime) {
  if (props.readOnly || !copiedEntries.value.length) return
  const payloads = buildPastePayloads(copiedEntries.value, date, startTime)
  if (payloads.length === 1) emit('save-entry', payloads[0])
  else if (payloads.length) emit('create-entries', payloads)
}

function openEditModal(entry) {
  if (props.readOnly) return
  modalState.value = { entry, prefill: null }
}

function openCreateModal({ date, startTime, endTime }) {
  const finalEndTime = endTime || fmtWallClock(Math.min(24 * 60 - 1, parseWallClock(startTime) + 60))
  modalState.value = { entry: null, prefill: { date, start_time: startTime, end_time: finalEndTime } }
}

function onModalSave(payload) {
  emit('save-entry', payload)
  modalState.value = null
}

function onModalDelete(entry) {
  emit('delete-entry', entry)
  modalState.value = null
}

// Lets a parent (e.g. a global-search deep link) scroll to and briefly
// highlight a specific entry's block - just showing where it is, the same
// as clicking any other search result takes you to the item rather than
// straight into editing it. Opening the edit form is still a deliberate
// click on the block itself.
let highlightTimer = null
function scrollToEntry(entry) {
  clearTimeout(highlightTimer)
  nextTick(() => {
    const el = document.getElementById(`tt-entry-${entry.id}`)
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' })
    el.classList.add('cal-block-highlight')
    highlightTimer = setTimeout(() => el.classList.remove('cal-block-highlight'), 2500)
  })
}

defineExpose({ scrollToEntry })
</script>

<style scoped>
.tt-calendar {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 420px;
}

.cal-toolbar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  padding: 0 0 6px;
}

.cal-selection {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-right: auto;
  font-size: 13px;
}
.cal-selection-count { color: var(--color-text-muted); margin-right: 4px; }
.cal-selection-delete { color: var(--color-danger); }

.cal-zoom { display: flex; gap: 2px; }

.cal-zoom-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  font-size: 14px;
  line-height: 1;
}
</style>
