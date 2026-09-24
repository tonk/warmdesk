<template>
  <div class="board-layout">
    <div class="board-toolbar">
      <div class="board-toolbar-left">
        <img v-if="projectAvatar(projectStore.currentProject)" :src="projectAvatar(projectStore.currentProject)" class="board-project-avatar" alt="" />
        <h1 class="board-project-name">{{ projectStore.currentProject?.name }}</h1>
        <button class="btn btn-ghost btn-sm star-btn" :class="{ starred: sidebarStore.isStarred(slug) }" @click="toggleStar" :title="sidebarStore.isStarred(slug) ? $t('board.unstar') : $t('board.star')" :aria-label="sidebarStore.isStarred(slug) ? 'Unstar project' : 'Star project'">
          {{ sidebarStore.isStarred(slug) ? '★' : '☆' }}
        </button>
        <RouterLink v-if="canManageColumns" :to="`/projects/${slug}/settings`" class="btn btn-ghost btn-sm settings-link" :title="$t('project.settings')" aria-label="Project settings">
          ⚙
        </RouterLink>
      </div>
      <div class="board-toolbar-right">
        <template v-if="projectStore.currentProject?.board_type === 'scrum'">
          <RouterLink :to="`/projects/${slug}/backlog`" class="btn btn-ghost btn-sm">
            📋 {{ $t('sprint.backlog') }}
          </RouterLink>
          <RouterLink :to="`/projects/${slug}/sprint`" class="btn btn-ghost btn-sm">
            🏃 {{ $t('sprint.board') }}
          </RouterLink>
          <RouterLink :to="`/projects/${slug}/epics`" class="btn btn-ghost btn-sm">
            ⚡ {{ $t('epic.title') }}
          </RouterLink>
        </template>
        <RouterLink :to="`/projects/${slug}/charts`" class="btn btn-ghost btn-sm">
          📊 {{ $t('sprint.charts') }}
        </RouterLink>
        <RouterLink :to="`/projects/${slug}/gantt`" class="btn btn-ghost btn-sm">
          📅 {{ $t('gantt.title') }}
        </RouterLink>
        <RouterLink :to="`/projects/${slug}/topics`" class="btn btn-ghost btn-sm">
          💬 {{ $t('topics.title') }}
        </RouterLink>
        <button
          class="btn btn-ghost btn-sm view-toggle-btn"
          :aria-pressed="viewMode === 'table'"
          :title="viewMode === 'table' ? $t('board.view_lanes') : $t('board.view_table')"
          @click="toggleViewMode"
        >{{ viewMode === 'table' ? '🗂' : '☰' }} {{ viewMode === 'table' ? $t('board.view_lanes') : $t('board.view_table') }}</button>
        <button
          :class="['btn btn-sm', showClosed ? 'btn-secondary' : 'btn-warning']"
          @click="toggleShowClosed"
        >{{ showClosed ? $t('board.hide_closed') : $t('board.show_closed') }}<span v-if="!showClosed && closedCardCount > 0" class="closed-count-badge">{{ closedCardCount }}</span></button>
        <button v-if="canManageColumns" class="btn btn-secondary btn-sm" @click="showAddColumn = true">
          + {{ $t('board.add_column') }}
        </button>
      </div>
    </div>

    <div v-if="projectStore.currentProject?.is_closed" class="board-closed-banner">
      {{ $t('board.board_closed') }}
      <RouterLink v-if="canManageColumns" :to="`/projects/${slug}/settings`" class="btn btn-sm btn-secondary" style="margin-left:12px">⚙ {{ $t('project.settings') }}</RouterLink>
    </div>

    <div class="board-body">
      <div v-if="boardStore.loading" class="board-loading">
        <div class="spinner" style="width:40px;height:40px;border-width:3px"></div>
      </div>

      <div v-else-if="viewMode === 'lanes'" class="board-columns-wrap">
        <div class="board-columns" ref="columnsEl">
          <BoardColumn
            v-for="col in boardStore.columns"
            :key="col.id"
            :column="col"
            :data-column-id="col.id"
            :can-manage-columns="canManageColumns"
            :show-closed="showClosed"
            @add-card="openAddCard"
            @open-card="openCardDetail"
            @card-moved="onCardMoved"
            @rename-column="onRenameColumn"
            @delete-column="onDeleteColumn"
            @edit-column="openEditColumn"
          />
        </div>
      </div>

      <div v-else class="board-table-wrap">
        <table class="data-table board-table">
          <thead>
            <tr>
              <th scope="col">{{ $t('board.card_ref') }}</th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('title')">
                  {{ $t('board.card_title') }}
                  <span v-if="tableSortField === 'title'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('column')">
                  {{ $t('board.table_column') }}
                  <span v-if="tableSortField === 'column'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('priority')">
                  {{ $t('board.priority') }}
                  <span v-if="tableSortField === 'priority'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('assignee')">
                  {{ $t('board.assignee') }}
                  <span v-if="tableSortField === 'assignee'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('due_date')">
                  {{ $t('board.due_date') }}
                  <span v-if="tableSortField === 'due_date'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('story_points')">
                  {{ $t('board.story_points') }}
                  <span v-if="tableSortField === 'story_points'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">
                <button type="button" class="th-sort-btn" @click="toggleTableSort('time_spent')">
                  {{ $t('board.time_spent') }}
                  <span v-if="tableSortField === 'time_spent'" aria-hidden="true">{{ tableSortDir === 'asc' ? '▲' : '▼' }}</span>
                </button>
              </th>
              <th scope="col">{{ $t('board.closed') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="card in sortedTableCards" :key="card.id" :class="{ 'row-closed': card.closed }">
              <td>{{ tableCardRef(card) }}</td>
              <td>
                <button type="button" class="row-title-btn" @click="openCardDetail(card)">{{ card.title }}</button>
              </td>
              <td>
                <span class="table-column-dot" :style="{ background: card.columnColor || 'var(--color-text-muted)' }" aria-hidden="true"></span>
                {{ card.columnName }}
              </td>
              <td>
                <span v-if="card.priority !== 'none'" :class="`badge priority-${card.priority}`">{{ $t(`board.priorities.${card.priority}`) }}</span>
              </td>
              <td>{{ card.assignee ? (card.assignee.display_name || card.assignee.username) : '—' }}<span v-if="(card.assignees || []).length > 1">, +{{ (card.assignees || []).length - 1 }}</span></td>
              <td>{{ card.due_date ? formatDate(card.due_date) : '—' }}</td>
              <td>{{ card.story_points ?? '—' }}</td>
              <td>{{ fmtCardTime(card.time_spent_minutes) }}</td>
              <td>
                <span v-if="card.closed" class="badge">{{ $t('board.closed') }}</span>
                <span v-else>—</span>
              </td>
            </tr>
            <tr v-if="!sortedTableCards.length">
              <td colspan="9" class="table-empty">{{ $t('board.table_no_cards') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Add column modal -->
    <BaseModal v-if="showAddColumn" :title="$t('board.add_column')" @close="showAddColumn = false" :resizable="true">
      <form @submit.prevent="submitAddColumn">
        <div class="form-group">
          <label class="form-label">{{ $t('board.column_name') }}</label>
          <input class="form-input" v-model="newColumn.name" required autofocus />
        </div>
        <div class="form-group">
          <label class="form-label">{{ $t('project.color') }}</label>
          <input type="color" class="form-input" v-model="newColumn.color" style="height:40px;padding:4px" aria-label="Column color" />
        </div>
        <div class="form-group">
          <label class="form-label">{{ $t('board.wip_limit') }}</label>
          <input class="form-input" type="text" inputmode="numeric" v-model="newColumn.wip" autocomplete="off" />
          <p class="form-hint">{{ $t('board.wip_limit_hint') }}</p>
        </div>
      </form>
      <template #footer>
        <button class="btn btn-secondary" @click="showAddColumn = false">{{ $t('common.cancel') }}</button>
        <button class="btn btn-primary" @click="submitAddColumn">{{ $t('common.create') }}</button>
      </template>
    </BaseModal>

    <!-- Edit column modal -->
    <BaseModal v-if="showEditColumn" :title="$t('board.edit_column')" @close="showEditColumn = false" :resizable="true">
      <form @submit.prevent="submitEditColumn">
        <div class="form-group">
          <label class="form-label">{{ $t('board.column_name') }}</label>
          <input class="form-input" v-model="editColumn.name" required autofocus />
        </div>
        <div class="form-group">
          <label class="form-label">{{ $t('project.color') }}</label>
          <input type="color" class="form-input" v-model="editColumn.color" style="height:40px;padding:4px" aria-label="Column color" />
        </div>
        <div class="form-group">
          <label class="form-label">{{ $t('board.wip_limit') }}</label>
          <input class="form-input" type="text" inputmode="numeric" v-model="editColumn.wip" autocomplete="off" />
          <p class="form-hint">{{ $t('board.wip_limit_hint') }}</p>
        </div>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showEditColumn = false">{{ $t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" @click="submitEditColumn">{{ $t('common.save') }}</button>
      </template>
    </BaseModal>

    <!-- Card detail -->
    <CardDetail
      v-if="selectedCard"
      :card="selectedCard"
      :labels="projectStore.currentProject?.labels || []"
      :members="projectMembers"
      :project-slug="slug"
      :focus-comment-id="route.query.comment ? Number(route.query.comment) : null"
      @close="selectedCard = null"
      @deleted="selectedCard = null"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Sortable from 'sortablejs'
import BoardColumn from '@/components/board/BoardColumn.vue'
import CardDetail from '@/components/board/CardDetail.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import { useBoardStore } from '@/stores/board'
import { useProjectStore } from '@/stores/project'
import { useUIStore } from '@/stores/ui'
import { useSidebarStore } from '@/stores/sidebar'
import { useAuthStore } from '@/stores/auth'
import { useWebSocket } from '@/composables/useWebSocket'
import { useDateFormat } from '@/composables/useDateFormat'
import { projectsApi } from '@/api/projects'
import { resolveAssetUrl } from '@/api/serverConfig'

const route = useRoute()
const { t } = useI18n()
const slug = computed(() => route.params.slug)

const boardStore = useBoardStore()
const projectStore = useProjectStore()
const ui = useUIStore()
const sidebarStore = useSidebarStore()
const auth = useAuthStore()
const { formatDate } = useDateFormat()

const showAddColumn = ref(false)
const showEditColumn = ref(false)
const editingColumnId = ref(null)
const selectedCard = ref(null)
const newColumn = ref({ name: '', color: '#94a3b8', wip: '' })
const editColumn = ref({ name: '', color: '#94a3b8', wip: '' })
const columnsEl = ref(null)
let columnSortable = null

const showClosed = ref(localStorage.getItem('board_show_closed') !== 'false')
function toggleShowClosed() {
  showClosed.value = !showClosed.value
  localStorage.setItem('board_show_closed', String(showClosed.value))
}
const closedCardCount = computed(() =>
  boardStore.columns.reduce((sum, col) => sum + (col.cards || []).filter(c => c.closed).length, 0)
)

const viewMode = ref(localStorage.getItem('board_view_mode') === 'table' ? 'table' : 'lanes')
function toggleViewMode() {
  viewMode.value = viewMode.value === 'lanes' ? 'table' : 'lanes'
  localStorage.setItem('board_view_mode', viewMode.value)
}

const tableSortField = ref('')
const tableSortDir = ref('asc')
function toggleTableSort(field) {
  if (tableSortField.value === field) {
    tableSortDir.value = tableSortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    tableSortField.value = field
    tableSortDir.value = 'asc'
  }
}

const PRIORITY_ORDER = { none: 0, low: 1, medium: 2, high: 3, critical: 4 }

const allTableCards = computed(() => {
  const rows = []
  for (const col of boardStore.columns) {
    const cards = showClosed.value ? (col.cards || []) : (col.cards || []).filter(c => !c.closed)
    for (const card of cards) rows.push({ ...card, columnName: col.name, columnColor: col.color })
  }
  return rows
})

const sortedTableCards = computed(() => {
  const rows = allTableCards.value
  const field = tableSortField.value
  if (!field) return rows
  return [...rows].sort((a, b) => {
    let av, bv
    if (field === 'due_date') {
      av = a.due_date ? new Date(a.due_date).getTime() : Infinity
      bv = b.due_date ? new Date(b.due_date).getTime() : Infinity
    } else if (field === 'priority') {
      av = PRIORITY_ORDER[a.priority] ?? 0
      bv = PRIORITY_ORDER[b.priority] ?? 0
    } else if (field === 'assignee') {
      av = (a.assignee?.display_name || a.assignee?.username || '').toLowerCase()
      bv = (b.assignee?.display_name || b.assignee?.username || '').toLowerCase()
    } else if (field === 'column') {
      av = (a.columnName || '').toLowerCase()
      bv = (b.columnName || '').toLowerCase()
    } else if (field === 'story_points') {
      av = a.story_points ?? -1
      bv = b.story_points ?? -1
    } else if (field === 'time_spent') {
      av = a.time_spent_minutes ?? 0
      bv = b.time_spent_minutes ?? 0
    } else {
      av = (a.title || '').toLowerCase()
      bv = (b.title || '').toLowerCase()
    }
    if (av < bv) return tableSortDir.value === 'asc' ? -1 : 1
    if (av > bv) return tableSortDir.value === 'asc' ? 1 : -1
    return 0
  })
})

function tableCardRef(card) {
  const prefix = projectStore.currentProject?.key_prefix
  return prefix && card.card_number ? `${prefix}-${card.card_number}` : '—'
}

function fmtCardTime(minutes) {
  if (!minutes) return '0m'
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return h > 0 ? (m > 0 ? `${h}h ${m}m` : `${h}h`) : `${m}m`
}

const projectMembers = ref([])

const { connected, presenceUsers, connect, disconnect, send: wsSend } = useWebSocket(slug)

// Track whether we've had the first successful connection for the current board.
// On subsequent connections (reconnects), silently refresh to catch missed events.
let wsConnectedOnce = false

watch(connected, (isConnected) => {
  if (isConnected) {
    if (wsConnectedOnce) boardStore.silentRefresh(slug.value)
    wsConnectedOnce = true
  }
})

async function loadBoard(projectSlug) {
  wsConnectedOnce = false
  selectedCard.value = null
  await Promise.all([
    boardStore.loadBoard(projectSlug),
    projectStore.fetchProject(projectSlug),
    sidebarStore.fetchStarred()
  ])
  loadMembers()
  connect()
  await nextTick()
  initColumnSortable()
  if (route.query.card) openCardById(Number(route.query.card))
}

async function openCardById(cardId) {
  try {
    const { data } = await projectsApi.getCard(slug.value, cardId)
    selectedCard.value = data
  } catch {}
}

onMounted(() => {
  ui.setHelpContext('board')
  loadBoard(slug.value)
})

watch(slug, async (newSlug) => {
  disconnect()
  boardStore.reset()
  columnSortable?.destroy()
  columnSortable = null
  await loadBoard(newSlug)
})

// Reopen a card when a search result is clicked while already viewing this
// project (e.g. another card, or a different comment on the current card) -
// route.query.card alone wouldn't otherwise re-trigger anything since only
// the project slug is watched above.
watch(() => route.query.card, (val) => {
  if (val) openCardById(Number(val))
})

onUnmounted(() => {
  disconnect()
  boardStore.reset()
  columnSortable?.destroy()
  ui.setHelpContext(null)
})

async function loadMembers() {
  try {
    const { data } = await projectsApi.listMembers(slug.value)
    projectMembers.value = data
  } catch {}
}

// Global admins and project admins/owners can manage columns
const ADMIN_RANKS = { admin: 3, owner: 4 }
const canManageColumns = computed(() => {
  if (auth.user?.global_role === 'admin') return true
  const me = projectMembers.value.find(m => m.user_id === auth.user?.id)
  return me ? (ADMIN_RANKS[me.role] ?? 0) >= 3 : false
})

watch(canManageColumns, (val) => {
  columnSortable?.option('disabled', !val)
})

async function toggleStar() {
  if (!slug.value) return
  if (sidebarStore.isStarred(slug.value)) {
    await sidebarStore.unstarProject(slug.value)
  } else {
    await sidebarStore.starProject(slug.value)
  }
}

function projectAvatar(project) {
  return resolveAssetUrl(project?.avatar || '')
}

function openAddCard(columnId) {
  selectedCard.value = { id: null, column_id: columnId, title: '' }
}

async function submitAddColumn() {
  if (!newColumn.value.name.trim()) return
  try {
    const extra = { color: newColumn.value.color }
    const raw = String(newColumn.value.wip ?? '').trim()
    if (raw !== '') {
      const n = parseInt(raw, 10)
      if (!Number.isFinite(n) || n < 1) {
        ui.error(t('board.wip_limit_invalid'))
        return
      }
      extra.wip_limit = n
    }
    await boardStore.createColumn(newColumn.value.name.trim(), extra)
    showAddColumn.value = false
    newColumn.value = { name: '', color: '#94a3b8', wip: '' }
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to create column')
  }
}

function openEditColumn(column) {
  editingColumnId.value = column.id
  editColumn.value = {
    name: column.name,
    color: column.color || '#94a3b8',
    wip: column.wip_limit != null && column.wip_limit !== undefined ? String(column.wip_limit) : ''
  }
  showEditColumn.value = true
}

async function submitEditColumn() {
  const name = editColumn.value.name.trim()
  if (!name || editingColumnId.value == null) return
  const payload = { name, color: editColumn.value.color || '#94a3b8' }
  const raw = String(editColumn.value.wip ?? '').trim()
  if (raw === '') {
    payload.wip_limit_clear = true
  } else {
    const n = parseInt(raw, 10)
    if (!Number.isFinite(n) || n < 1) {
      ui.error(t('board.wip_limit_invalid'))
      return
    }
    payload.wip_limit = n
  }
  try {
    const { data } = await projectsApi.updateColumn(slug.value, editingColumnId.value, payload)
    boardStore.updateColumn(data)
    showEditColumn.value = false
    editingColumnId.value = null
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update column')
  }
}

async function openCardDetail(card) {
  // Fetch full card with comments
  try {
    const { data } = await projectsApi.getCard(slug.value, card.id)
    selectedCard.value = data
  } catch {
    selectedCard.value = card
  }
}

function initColumnSortable() {
  if (!columnsEl.value) return
  columnSortable = Sortable.create(columnsEl.value, {
    animation: 150,
    handle: '.column-header',
    ghostClass: 'column-ghost',
    dragClass: 'column-drag',
    disabled: !canManageColumns.value,
    onEnd(evt) {
      if (evt.oldIndex === evt.newIndex) return
      onColumnReordered(evt.oldIndex, evt.newIndex)
    }
  })
}

async function onColumnReordered(oldIndex, newIndex) {
  const cols = [...boardStore.columns]
  const [moved] = cols.splice(oldIndex, 1)
  cols.splice(newIndex, 0, moved)

  // Assign new positions
  const updates = cols.map((col, i) => ({ id: col.id, position: (i + 1) * 1000 }))
  boardStore.columns = cols

  try {
    await projectsApi.reorderColumns(slug.value, updates)
  } catch {
    ui.error('Failed to reorder columns')
    await boardStore.loadBoard(slug.value)
  }
}

async function onRenameColumn({ columnId, name }) {
  try {
    await projectsApi.updateColumn(slug.value, columnId, { name })
    const col = boardStore.columns.find(c => c.id === columnId)
    if (col) col.name = name
  } catch (e) {
    ui.error('Failed to rename column')
  }
}

async function onDeleteColumn(columnId) {
  if (!await ui.confirm(t('board.delete_column_confirm'), { destructive: true })) return
  try {
    await projectsApi.deleteColumn(slug.value, columnId)
    boardStore.columns = boardStore.columns.filter(c => c.id !== columnId)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to delete column')
  }
}

async function onCardMoved({ cardId, fromColumnId, toColumnId, newIndex }) {
  const toCol = boardStore.columns.find(c => c.id === toColumnId)
  if (!toCol) return

  const cards = toCol.cards.filter(c => c.id !== cardId)
  let position
  if (newIndex === 0) {
    position = (cards[0]?.position || 1000) / 2
  } else if (newIndex >= cards.length) {
    position = (cards[cards.length - 1]?.position || 0) + 1000
  } else {
    position = ((cards[newIndex - 1]?.position || 0) + (cards[newIndex]?.position || cards[newIndex - 1]?.position + 2000)) / 2
  }

  try {
    await projectsApi.moveCard(slug.value, cardId, { column_id: toColumnId, position })
  } catch (e) {
    ui.error('Failed to move card')
    await boardStore.loadBoard(slug.value) // revert
  }
}
</script>

<style scoped>
.board-layout { display: flex; flex-direction: column; flex: 1; min-height: 0; overflow: hidden; }

.board-toolbar {
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  padding: 8px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.board-toolbar-left, .board-toolbar-right { display: flex; gap: 8px; align-items: center; }
.board-project-avatar {
  width: 22px;
  height: 22px;
  border-radius: 6px;
  object-fit: cover;
  border: 1px solid var(--color-border);
}
.board-project-name { font-size: 15px; font-weight: 600; color: var(--color-text); padding: 0 4px; }
.star-btn { font-size: 18px; line-height: 1; color: var(--color-text-muted); }
.star-btn.starred { color: var(--color-warning); }
.settings-link { font-size: 15px; color: var(--color-text-muted); }

.board-body {
  flex: 1;
  overflow: hidden;
  transition: margin-right .25s;
}

.board-columns-wrap {
  height: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 20px;
}

.board-columns {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  height: 100%;
}

.board-loading { display: flex; justify-content: center; align-items: center; height: 200px; }

.breadcrumb-sep { color: var(--color-text-muted); margin: 0 4px; }
.breadcrumb-link { font-size: 14px; color: var(--color-text-muted); }
.breadcrumb-current { font-size: 14px; font-weight: 600; }

:global(.column-ghost) { opacity: 0.4; background: var(--color-primary) !important; }
:global(.column-drag) { transform: rotate(1deg); box-shadow: var(--shadow-md); }

.form-hint { font-size: 12px; color: var(--color-text-muted); margin-top: 4px; margin-bottom: 0; }

.closed-count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--color-text-muted);
  color: #fff;
  border-radius: 9999px;
  padding: 1px 6px;
  font-size: 11px;
  font-weight: 600;
  margin-left: 6px;
  line-height: 1.4;
}

.board-closed-banner {
  background: var(--color-warning-bg, #fef3c7);
  color: var(--color-warning-text, #92400e);
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 500;
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--color-warning-border, #fde68a);
}

.view-toggle-btn[aria-pressed="true"] { background: var(--color-primary); color: #fff; }

.board-table-wrap { height: 100%; overflow: auto; padding: 20px; }

.board-table { width: 100%; border-collapse: collapse; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); }
.board-table th, .board-table td { padding: 10px 14px; text-align: left; border-bottom: 1px solid var(--color-border); font-size: 13px; vertical-align: middle; white-space: nowrap; }
.board-table th { font-weight: 600; color: var(--color-text-muted); font-size: 12px; background: var(--color-bg); }
.board-table tbody tr:hover { background: var(--color-bg); }
.board-table tr.row-closed { opacity: 0.6; }

.th-sort-btn {
  background: none;
  border: none;
  padding: 0;
  margin: 0;
  font: inherit;
  font-weight: 600;
  color: var(--color-text-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.th-sort-btn:hover { color: var(--color-text); }

.row-title-btn {
  background: none;
  border: none;
  padding: 0;
  margin: 0;
  font: inherit;
  color: var(--color-primary);
  text-align: left;
  cursor: pointer;
  white-space: normal;
}
.row-title-btn:hover { text-decoration: underline; }

.table-column-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
}

.table-empty { text-align: center; color: var(--color-text-muted); padding: 24px; }
</style>
