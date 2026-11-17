<template>
  <div class="ov-panel">
    <section :aria-label="$t('timeOverview.checks_summary')">
      <h3 class="ov-h3">{{ $t('timeOverview.checks_summary') }}</h3>
      <div class="chips" role="group" :aria-label="$t('timeOverview.filter_by_type')">
        <button type="button" class="ov-chip" :class="{ active: !filterType }" :aria-pressed="!filterType" @click="filterType = ''">
          {{ $t('timeOverview.all_types') }} <strong>{{ data.findings.length }}</strong>
        </button>
        <button
          v-for="ty in typesWithCounts" :key="ty"
          type="button" class="ov-chip" :class="{ active: filterType === ty }"
          :aria-pressed="filterType === ty"
          @click="filterType = filterType === ty ? '' : ty"
        >{{ $t('timeOverview.type_' + ty) }} <strong>{{ data.counts[ty] }}</strong></button>
      </div>
      <p v-if="!data.findings.length" class="ov-muted" role="status">{{ $t('timeOverview.no_findings') }}</p>
    </section>

    <div v-if="data.findings.length" class="sort-row">
      <label for="ov-check-sort">{{ $t('timeOverview.sort_by') }}</label>
      <select id="ov-check-sort" v-model="sortBy" class="ov-select">
        <option value="date">{{ $t('timeOverview.date') }}</option>
        <option value="type">{{ $t('timeOverview.finding_type') }}</option>
      </select>
    </div>

    <div v-if="visible.length" class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.findings_caption', { n: visible.length }) }}</caption>
        <thead><tr>
          <th scope="col">{{ $t('timeOverview.date') }}</th>
          <th scope="col">{{ $t('timeOverview.finding_type') }}</th>
          <th scope="col" class="ov-left">{{ $t('timeOverview.entries_label') }}</th>
          <th scope="col">{{ $t('timeOverview.time_vs_norm') }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="(f, i) in visible" :key="f.type + f.date + i">
            <th scope="row" class="ov-left">{{ f.date ? formatDate(f.date) : '' }}</th>
            <td class="ov-left">{{ $t('timeOverview.type_' + f.type) }}</td>
            <td class="ov-left ov-wrap">{{ (f.labels || []).join('; ') }}</td>
            <td>
              <template v-if="f.minutes || f.norm_minutes">{{ fmtHM(f.minutes || 0) }} / {{ fmtHM(f.norm_minutes || 0) }}</template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="ov-days">{{ $t('timeOverview.days_note', { n: data.total_days }) }}</p>

    <section>
      <h3 class="ov-h3">
        <button type="button" class="ov-link" :aria-expanded="String(showVariants)" aria-controls="ov-variants" @click="showVariants = !showVariants">
          <span aria-hidden="true">{{ showVariants ? '▾' : '▸' }}</span> {{ $t('timeOverview.variants_title') }}
        </button>
      </h3>
      <div v-show="showVariants" id="ov-variants">
        <p class="ov-muted">{{ $t('timeOverview.variants_hint') }}</p>
        <p v-if="!data.description_variants.length" class="ov-muted">{{ $t('timeOverview.no_variants') }}</p>
        <div v-for="p in data.description_variants" :key="p.project_id" class="ov-table-wrap">
          <table class="ov-table">
            <caption>{{ p.project }}<span v-if="p.customer" class="ov-muted"> ({{ p.customer }})</span></caption>
            <thead><tr>
              <th scope="col">{{ $t('timeOverview.description') }}</th>
              <th scope="col">{{ $t('timeOverview.count') }}</th>
              <th scope="col">{{ $t('timeOverview.total_time') }}</th>
            </tr></thead>
            <tbody>
              <tr v-for="v in p.variants" :key="v.description">
                <th scope="row" class="ov-left ov-wrap">{{ v.description || $t('timeOverview.empty_description') }}</th>
                <td>{{ v.count }}</td>
                <td>{{ fmtHM(v.minutes) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useDateFormat } from '@/composables/useDateFormat'
import { fmtHM } from '@/utils/timeOverviewFormat'
import './overview.css'

const props = defineProps({ data: { type: Object, required: true } })
const { formatDate } = useDateFormat()

const filterType = ref('')
const sortBy = ref('date')
const showVariants = ref(false)

const typesWithCounts = computed(() => Object.keys(props.data.counts || {}).filter(k => props.data.counts[k] > 0).sort())
const visible = computed(() => {
  const list = props.data.findings.filter(f => !filterType.value || f.type === filterType.value)
  return [...list].sort((a, b) => sortBy.value === 'type'
    ? a.type.localeCompare(b.type) || (a.date || '').localeCompare(b.date || '')
    : (a.date || '').localeCompare(b.date || '') || a.type.localeCompare(b.type))
})
</script>

<style scoped>
.chips { display: flex; flex-wrap: wrap; gap: 6px; margin: 6px 0; }
.ov-chip { border: 1px solid var(--color-border); background: var(--color-surface); color: var(--color-text); border-radius: 14px; padding: 3px 12px; font-size: 12px; cursor: pointer; }
.ov-chip.active { border-color: var(--color-primary); box-shadow: inset 0 0 0 1px var(--color-primary); font-weight: 600; }
.sort-row { display: flex; gap: 8px; align-items: center; font-size: 13px; color: var(--color-text); }
</style>
