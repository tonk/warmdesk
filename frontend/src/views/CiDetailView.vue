<template>
  <main class="cd-main">
    <RouterLink to="/cmdb" class="cd-back">← {{ $t('cmdb.back_to_list') }}</RouterLink>

    <div v-if="forbidden" class="cd-empty" role="alert">{{ $t('cmdb.forbidden') }}</div>
    <div v-else-if="notFound" class="cd-empty" role="alert">{{ $t('cmdb.not_found') }}</div>

    <div v-else-if="loading && !ci" class="cd-loading" role="status">
      <div class="spinner" style="width:32px;height:32px;border-width:3px" aria-hidden="true"></div>
      <span class="sr-only">{{ $t('common.loading') }}</span>
    </div>

    <template v-else-if="ci">
      <header class="cd-header">
        <div class="cd-heading">
          <h1 class="cd-title">
            <span class="cd-icon" aria-hidden="true">{{ ci.type?.icon || '▫' }}</span>
            {{ ci.name }}
          </h1>
          <div class="cd-badges">
            <span class="cd-badge">{{ ci.type?.name }}</span>
            <span class="cd-badge" :class="`cd-st-${ci.status}`">{{ $t('common.status') }}: {{ $t(`cmdb.status_${ci.status}`) }}</span>
            <span v-if="ci.environment" class="cd-badge">{{ $t(`cmdb.env_${ci.environment}`) }}</span>
            <span v-if="ci.criticality" class="cd-badge" :class="`cd-crit-${ci.criticality}`">{{ $t('cmdb.criticality') }}: {{ $t(`cmdb.crit_${ci.criticality}`) }}</span>
          </div>
        </div>
        <div class="cd-actions" v-if="!editing">
          <button type="button" class="btn btn-secondary" @click="startEdit">{{ $t('common.edit') }}</button>
          <button type="button" class="btn btn-danger" @click="removeCI">{{ $t('common.delete') }}</button>
        </div>
      </header>

      <!-- Details / edit -->
      <section class="cd-section" aria-labelledby="cd-details-h">
        <h2 id="cd-details-h" class="cd-section-title">{{ editing ? $t('cmdb.edit_ci') : $t('cmdb.details') }}</h2>
        <div v-if="editing" class="cd-card">
          <CiForm ref="formRef" :ci="ci" form-id="cd-edit-form" :saving="saving" @submit="save" @cancel="editing = false" />
        </div>
        <div v-else class="cd-card">
          <dl class="cd-dl">
            <div><dt>{{ $t('cmdb.type') }}</dt><dd>{{ ci.type?.name || '—' }}</dd></div>
            <div><dt>{{ $t('cmdb.customer') }}</dt><dd>
              <RouterLink v-if="ci.customer" :to="`/customers/${ci.customer.id}`" class="cd-link">{{ ci.customer.name }}</RouterLink>
              <span v-else>{{ $t('cmdb.internal') }}</span>
            </dd></div>
            <div><dt>{{ $t('common.status') }}</dt><dd>{{ $t(`cmdb.status_${ci.status}`) }}</dd></div>
            <div><dt>{{ $t('cmdb.environment') }}</dt><dd>{{ $t(`cmdb.env_${ci.environment || 'none'}`) }}</dd></div>
            <div><dt>{{ $t('cmdb.criticality') }}</dt><dd>{{ $t(`cmdb.crit_${ci.criticality || 'none'}`) }}</dd></div>
            <div><dt>{{ $t('cmdb.owner') }}</dt><dd>{{ ci.owner ? (ci.owner.display_name || ci.owner.username) : '—' }}</dd></div>
            <div><dt>{{ $t('cmdb.group') }}</dt><dd>{{ ci.group?.name || '—' }}</dd></div>
            <div><dt>{{ $t('cmdb.updated') }}</dt><dd>{{ formatDateTime(ci.updated_at) }}</dd></div>
          </dl>
          <div class="cd-block">
            <h3 class="cd-sub">{{ $t('cmdb.description') }}</h3>
            <p v-if="ci.description" class="cd-text">{{ ci.description }}</p>
            <p v-else class="cd-muted">{{ $t('cmdb.no_description') }}</p>
          </div>
          <div class="cd-block">
            <h3 class="cd-sub">{{ $t('cmdb.attributes') }}</h3>
            <table v-if="attributeList.length" class="cd-attr-table">
              <caption class="sr-only">{{ $t('cmdb.attributes') }}</caption>
              <thead class="sr-only"><tr><th scope="col">{{ $t('cmdb.attribute_key') }}</th><th scope="col">{{ $t('cmdb.attribute_value') }}</th></tr></thead>
              <tbody>
                <tr v-for="[k, v] in attributeList" :key="k"><th scope="row">{{ k }}</th><td>{{ v }}</td></tr>
              </tbody>
            </table>
            <p v-else class="cd-muted">{{ $t('cmdb.no_attributes') }}</p>
          </div>
        </div>
      </section>

      <!-- Relations -->
      <section class="cd-section" aria-labelledby="cd-rel-h">
        <h2 id="cd-rel-h" class="cd-section-title">{{ $t('cmdb.relations') }}</h2>
        <div class="cd-card">
          <p v-if="!relationGroups.length" class="cd-muted">{{ $t('cmdb.no_relations') }}</p>
          <div v-for="g in relationGroups" :key="g.key" class="cd-rel-group">
            <h3 class="cd-sub" :id="`cd-rel-${g.key}`">{{ g.label }}</h3>
            <ul class="cd-rel-list" :aria-labelledby="`cd-rel-${g.key}`">
              <li v-for="r in g.items" :key="r.id" class="cd-rel-item">
                <span aria-hidden="true">{{ r.ci.type?.icon || '▫' }}</span>
                <RouterLink :to="`/cmdb/${r.ci.id}`" class="cd-link">{{ r.ci.name }}</RouterLink>
                <span class="cd-meta">{{ r.ci.type?.name }} · {{ $t(`cmdb.status_${r.ci.status}`) }}</span>
                <button type="button" class="btn btn-ghost btn-sm cd-rel-remove"
                  :aria-label="$t('cmdb.remove_relation', { relation: `${g.label} ${r.ci.name}` })"
                  :title="$t('cmdb.remove_relation', { relation: `${g.label} ${r.ci.name}` })"
                  @click="removeRelation(r, g.label)">✕</button>
              </li>
            </ul>
          </div>

          <form class="cd-rel-add" @submit.prevent="addRelation" :aria-label="$t('cmdb.add_relation')">
            <h3 class="cd-sub">{{ $t('cmdb.add_relation') }}</h3>
            <div class="cd-rel-add-row">
              <div class="cd-rel-field">
                <label class="form-label" for="cd-rel-kind">{{ $t('cmdb.relation_kind', { name: ci.name }) }}</label>
                <select id="cd-rel-kind" v-model="newRel.kind" class="form-input">
                  <option v-for="k in RELATION_KINDS" :key="k.key" :value="k.key">{{ relationPhrase(k.type, k.direction) }}</option>
                </select>
              </div>
              <div class="cd-rel-field cd-rel-picker">
                <CiPicker v-model="newRel.targetId" :exclude-ids="excludeIds" :label="$t('cmdb.relation_target')" @select="ci => newRel.target = ci" />
              </div>
              <button type="submit" class="btn btn-primary" :disabled="!newRel.target || addingRel">{{ $t('common.add') }}</button>
            </div>
          </form>
        </div>
      </section>

      <!-- Impact analysis -->
      <section class="cd-section" aria-labelledby="cd-impact-h">
        <h2 id="cd-impact-h" class="cd-section-title">{{ $t('cmdb.impact') }}</h2>
        <p class="cd-section-desc">{{ $t('cmdb.impact_desc') }}</p>
        <div class="cd-card">
          <div v-if="impactLoading" role="status" class="cd-muted">{{ $t('common.loading') }}</div>
          <template v-else-if="impact">
            <p v-if="!impact.impacted.length && !impact.hidden" class="cd-muted">{{ $t('cmdb.impact_none') }}</p>
            <div v-for="lvl in impactLevels" :key="lvl.depth" class="cd-impact-level" :style="{ '--depth': lvl.depth - 1 }">
              <h3 class="cd-sub" :id="`cd-impact-${lvl.depth}`">
                {{ lvl.depth === 1 ? $t('cmdb.impact_direct') : $t('cmdb.impact_level', { n: lvl.depth }) }}
                <span class="cd-count">({{ lvl.items.length }})</span>
              </h3>
              <ul class="cd-impact-list" :aria-labelledby="`cd-impact-${lvl.depth}`">
                <li v-for="it in lvl.items" :key="it.ci.id" class="cd-impact-item">
                  <div class="cd-impact-row">
                    <span aria-hidden="true">{{ it.ci.type?.icon || '▫' }}</span>
                    <RouterLink :to="`/cmdb/${it.ci.id}`" class="cd-link">{{ it.ci.name }}</RouterLink>
                    <span class="cd-meta">{{ it.ci.customer?.name || $t('cmdb.internal') }} · {{ viaLabel(it.via, it.depth) }}</span>
                    <span v-if="it.ci.criticality" class="cd-badge" :class="`cd-crit-${it.ci.criticality}`">{{ $t('cmdb.criticality') }}: {{ $t(`cmdb.crit_${it.ci.criticality}`) }}</span>
                    <span class="cd-badge" :class="{ 'cd-badge-warn': it.open_tickets?.length }">{{ $t('cmdb.open_tickets_count', { n: it.open_tickets?.length || 0 }, it.open_tickets?.length || 0) }}</span>
                  </div>
                  <ul v-if="it.open_tickets?.length" class="cd-ticket-list cd-ticket-nested" :aria-label="$t('cmdb.open_tickets_of', { name: it.ci.name })">
                    <li v-for="tk in it.open_tickets" :key="tk.id">
                      <RouterLink :to="ticketLink(tk)" class="cd-link">#{{ tk.id }} {{ tk.title }}</RouterLink>
                      <span class="cd-meta">{{ ticketMeta(tk) }}</span>
                    </li>
                  </ul>
                </li>
              </ul>
            </div>
            <p v-if="impact.hidden" class="cd-muted cd-hidden">{{ $t('cmdb.impact_hidden', { n: impact.hidden }, impact.hidden) }}</p>
            <div class="cd-block">
              <h3 class="cd-sub">{{ $t('cmdb.own_open_tickets') }}</h3>
              <ul v-if="impact.open_tickets?.length" class="cd-ticket-list">
                <li v-for="tk in impact.open_tickets" :key="tk.id">
                  <RouterLink :to="ticketLink(tk)" class="cd-link">#{{ tk.id }} {{ tk.title }}</RouterLink>
                  <span class="cd-meta">{{ ticketMeta(tk) }}</span>
                </li>
              </ul>
              <p v-else class="cd-muted">{{ $t('cmdb.no_open_tickets') }}</p>
            </div>
          </template>
        </div>
      </section>

      <!-- Linked tickets -->
      <section class="cd-section" aria-labelledby="cd-tickets-h">
        <h2 id="cd-tickets-h" class="cd-section-title">{{ $t('cmdb.linked_tickets') }}</h2>
        <div class="cd-card">
          <p v-if="!tickets.length" class="cd-muted">{{ $t('cmdb.no_tickets') }}</p>
          <table v-else class="cd-table">
            <caption class="sr-only">{{ $t('cmdb.linked_tickets') }}</caption>
            <thead>
              <tr>
                <th scope="col">{{ $t('ticket.title') }}</th>
                <th scope="col">{{ $t('cmdb.ticket_type') }}</th>
                <th scope="col">{{ $t('common.status') }}</th>
                <th scope="col">{{ $t('cmdb.priority') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="tk in tickets" :key="tk.id">
                <th scope="row"><RouterLink :to="ticketLink(tk)" class="cd-link">#{{ tk.id }} {{ tk.title }}</RouterLink></th>
                <td>{{ labelOr(`ticket.type_${tk.type}`, tk.type) }}</td>
                <td>{{ labelOr(`ticket.status_${tk.status}`, tk.status) }}</td>
                <td>{{ tk.priority ? labelOr(`ticket.priority_${tk.priority}`, tk.priority) : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </main>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import CiForm from '@/components/cmdb/CiForm.vue'
import CiPicker from '@/components/cmdb/CiPicker.vue'
import { cmdbApi } from '@/api/itsm'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'

// Each selectable relation, read from this CI's point of view. Incoming
// kinds are created on the other CI ("X depends on this CI" = Required by X).
const RELATION_KINDS = [
  { key: 'depends_on:outgoing', type: 'depends_on', direction: 'outgoing' },
  { key: 'depends_on:incoming', type: 'depends_on', direction: 'incoming' },
  { key: 'runs_on:outgoing', type: 'runs_on', direction: 'outgoing' },
  { key: 'runs_on:incoming', type: 'runs_on', direction: 'incoming' },
  { key: 'part_of:outgoing', type: 'part_of', direction: 'outgoing' },
  { key: 'part_of:incoming', type: 'part_of', direction: 'incoming' },
  { key: 'connected_to:outgoing', type: 'connected_to', direction: 'outgoing' },
]

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const ui = useUIStore()
const { formatDateTime } = useDateFormat()

const ci = ref(null)
const relations = ref([])
const tickets = ref([])
const impact = ref(null)
const loading = ref(false)
const impactLoading = ref(false)
const forbidden = ref(false)
const notFound = ref(false)
const editing = ref(false)
const saving = ref(false)
const formRef = ref(null)
const addingRel = ref(false)
const newRel = ref({ kind: 'depends_on:outgoing', targetId: null, target: null })

const ciId = computed(() => Number(route.params.id))

function labelOr(key, fallback) { return te(key) ? t(key) : fallback }

function relationPhrase(type, direction) {
  if (type === 'connected_to') return t('cmdb.rel_connected_to')
  return t(`cmdb.rel_${type}_${direction === 'incoming' ? 'in' : 'out'}`)
}

const relationGroups = computed(() => {
  const groups = []
  for (const k of RELATION_KINDS) {
    const items = relations.value.filter(r => r.type === k.type && (k.type === 'connected_to' || r.direction === k.direction))
    if (items.length) groups.push({ key: k.key.replace(':', '-'), label: relationPhrase(k.type, k.direction), items })
  }
  return groups
})

const excludeIds = computed(() => [ciId.value])

const attributeList = computed(() => Object.entries(ci.value?.attributes || {}).sort((a, b) => a[0].localeCompare(b[0])))

const impactLevels = computed(() => {
  const by = new Map()
  for (const it of impact.value?.impacted || []) {
    if (!by.has(it.depth)) by.set(it.depth, [])
    by.get(it.depth).push(it)
  }
  return [...by.entries()].sort((a, b) => a[0] - b[0]).map(([depth, items]) => ({ depth, items }))
})

function viaLabel(via, depth) {
  const key = `cmdb.via_${via}${depth === 1 ? '_direct' : ''}`
  return te(key) ? t(key) : via
}

function ticketLink(tk) {
  return tk.customer_id ? `/customers/${tk.customer_id}/tickets/${tk.id}` : `/tickets/inbox/${tk.id}`
}
function ticketMeta(tk) {
  const parts = [labelOr(`ticket.type_${tk.type}`, tk.type), labelOr(`ticket.status_${tk.status}`, tk.status)]
  if (tk.priority) parts.push(labelOr(`ticket.priority_${tk.priority}`, tk.priority))
  return parts.join(' · ')
}

async function load() {
  loading.value = true
  forbidden.value = false
  notFound.value = false
  try {
    const { data } = await cmdbApi.get(ciId.value)
    ci.value = data.ci
    relations.value = data.relations || []
    tickets.value = data.tickets || []
  } catch (e) {
    const st = e?.response?.status
    if (st === 403) forbidden.value = true
    else if (st === 404) notFound.value = true
    else ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    loading.value = false
  }
}

async function loadImpact() {
  impactLoading.value = true
  try {
    const { data } = await cmdbApi.impact(ciId.value)
    impact.value = { impacted: data.impacted || [], hidden: data.hidden || 0, open_tickets: data.open_tickets || [] }
  } catch {
    impact.value = null
  } finally {
    impactLoading.value = false
  }
}

function reloadAll() {
  editing.value = false
  ci.value = null
  impact.value = null
  load()
  loadImpact()
}

async function startEdit() {
  editing.value = true
  await nextTick()
  formRef.value?.focus()
}

async function save(payload) {
  saving.value = true
  try {
    const { data } = await cmdbApi.update(ciId.value, payload)
    ci.value = data
    editing.value = false
    ui.success(t('cmdb.saved'))
    loadImpact()
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    saving.value = false
  }
}

async function removeCI() {
  const ok = await ui.confirm(t('cmdb.delete_confirm', { name: ci.value.name }), { destructive: true })
  if (!ok) return
  try {
    await cmdbApi.remove(ciId.value)
    ui.success(t('cmdb.deleted'))
    router.push('/cmdb')
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  }
}

async function addRelation() {
  const target = newRel.value.target
  if (!target) return
  const kind = RELATION_KINDS.find(k => k.key === newRel.value.kind)
  addingRel.value = true
  try {
    if (kind.direction === 'incoming') await cmdbApi.addRelation(target.id, ciId.value, kind.type)
    else await cmdbApi.addRelation(ciId.value, target.id, kind.type)
    ui.success(t('cmdb.relation_added'))
    newRel.value = { kind: newRel.value.kind, targetId: null, target: null }
    const { data } = await cmdbApi.get(ciId.value)
    relations.value = data.relations || []
    loadImpact()
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  } finally {
    addingRel.value = false
  }
}

async function removeRelation(r, label) {
  const ok = await ui.confirm(t('cmdb.remove_relation_confirm', { relation: `${label} ${r.ci.name}` }), { destructive: true, confirmLabel: t('cmdb.remove') })
  if (!ok) return
  try {
    await cmdbApi.removeRelation(r.id)
    relations.value = relations.value.filter(x => x.id !== r.id)
    ui.success(t('cmdb.relation_removed'))
    loadImpact()
  } catch (e) {
    ui.error(e?.response?.data?.error || t('common.error'))
  }
}

watch(() => route.params.id, (v, old) => { if (v && v !== old) reloadAll() })
onMounted(() => { load(); loadImpact() })
</script>

<style scoped>
.cd-main { padding: 24px 32px; max-width: 1100px; margin: 0 auto; }
.cd-back { display: inline-block; font-size: 13px; color: var(--color-text-muted); text-decoration: none; margin-bottom: 12px; }
.cd-back:hover { color: var(--color-primary); text-decoration: underline; }
.cd-back:focus-visible, .cd-link:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.cd-loading { display: flex; justify-content: center; padding: 48px; }
.cd-empty { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }

.cd-header { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 8px; }
.cd-title { font-size: 22px; font-weight: 700; margin: 0 0 8px; overflow-wrap: anywhere; }
.cd-icon { margin-right: 4px; }
.cd-badges { display: flex; flex-wrap: wrap; gap: 6px; }
.cd-actions { display: flex; gap: 8px; }

.cd-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px; white-space: nowrap;
  border: 1px solid var(--color-border); color: var(--color-text); background: var(--color-bg);
}
.cd-st-active { border-color: var(--color-success); }
.cd-st-maintenance { border-color: var(--color-warning); }
.cd-st-planned { border-color: var(--color-primary); }
.cd-st-retired { color: var(--color-text-muted); border-style: dashed; }
.cd-crit-high { border-color: var(--color-danger); }
.cd-crit-medium { border-color: var(--color-warning); }
.cd-badge-warn { border-color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 12%, var(--color-bg)); }

.cd-section { margin-top: 24px; }
.cd-section-title { font-size: 16px; font-weight: 700; margin: 0 0 10px; }
.cd-section-desc { font-size: 12px; color: var(--color-text-muted); margin: -4px 0 10px; }
.cd-card { background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius); padding: 14px 16px; }
.cd-sub { font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; color: var(--color-text-muted); margin: 0 0 6px; }
.cd-block { margin-top: 16px; }
.cd-text { margin: 0; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
.cd-muted { margin: 0; font-size: 13px; color: var(--color-text-muted); }
.cd-meta { font-size: 12px; color: var(--color-text-muted); }
.cd-count { font-weight: 400; }
.cd-link { color: var(--color-text); text-decoration: none; font-weight: 600; }
.cd-link:hover { color: var(--color-primary); text-decoration: underline; }

.cd-dl { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px 20px; margin: 0; }
.cd-dl dt { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.cd-dl dd { margin: 2px 0 0; font-size: 13px; }

.cd-attr-table, .cd-table { border-collapse: collapse; font-size: 13px; }
.cd-attr-table th, .cd-attr-table td { padding: 4px 16px 4px 0; text-align: left; vertical-align: top; border-bottom: 1px solid var(--color-border); }
.cd-attr-table th { font-weight: 600; white-space: nowrap; }
.cd-attr-table td { overflow-wrap: anywhere; }
.cd-table { width: 100%; }
.cd-table th, .cd-table td { padding: 6px 10px; text-align: left; border-bottom: 1px solid var(--color-border); }
.cd-table thead th { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.cd-table tbody th { font-weight: 400; }
.cd-table tbody tr:last-child th, .cd-table tbody tr:last-child td { border-bottom: none; }

.cd-rel-group + .cd-rel-group { margin-top: 12px; }
.cd-rel-list, .cd-impact-list, .cd-ticket-list { list-style: none; margin: 0; padding: 0; }
.cd-rel-item { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 8px; padding: 4px 0; font-size: 13px; }
.cd-rel-remove { margin-left: auto; }
.cd-rel-add { margin-top: 16px; padding-top: 12px; border-top: 1px solid var(--color-border); }
.cd-rel-add-row { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; }
.cd-rel-field { min-width: 180px; }
.cd-rel-picker { flex: 1 1 260px; }

.cd-impact-level { margin-left: calc(var(--depth) * 20px); padding-left: 10px; border-left: 2px solid var(--color-border); }
.cd-impact-level + .cd-impact-level { margin-top: 12px; }
.cd-impact-item { padding: 4px 0; }
.cd-impact-row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 8px; font-size: 13px; }
.cd-ticket-list li { font-size: 13px; padding: 2px 0; display: flex; flex-wrap: wrap; gap: 4px 8px; }
.cd-ticket-nested { margin: 2px 0 0 22px; }
.cd-ticket-nested .cd-link { font-weight: 400; }
.cd-hidden { margin-top: 10px; font-style: italic; }

@media (max-width: 640px) {
  .cd-main { padding: 16px; }
  .cd-impact-level { margin-left: calc(var(--depth) * 8px); }
}
</style>
