<template>
  <div class="ov-panel">
    <p v-if="data.entries_without_distance > 0" class="ov-warn" role="status">{{ $t('timeOverview.no_distance_warning', { n: data.entries_without_distance }) }}</p>

    <div class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.travel_by_direction') }}</caption>
        <thead><tr>
          <th scope="col">{{ $t('timeOverview.direction') }}</th>
          <th v-for="c in cols" :key="c.key" scope="col">{{ c.label }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="r in directionRows" :key="r.key">
            <th scope="row" class="ov-left">{{ r.label }}</th>
            <td v-for="c in cols" :key="c.key">{{ c.fmt(r.acc[c.key]) }}</td>
          </tr>
        </tbody>
        <tfoot><tr>
          <th scope="row" class="ov-left">{{ $t('timeOverview.total') }}</th>
          <td v-for="c in cols" :key="c.key">{{ c.fmt(c.key === 'days' ? data.total_days : data.total[c.key]) }}</td>
        </tr></tfoot>
      </table>
    </div>

    <div class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.travel_by_month') }}</caption>
        <thead><tr>
          <th scope="col">{{ $t('timeOverview.month') }}</th>
          <th v-for="c in cols" :key="c.key" scope="col">{{ c.label }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="m in data.months" :key="m.month">
            <th scope="row" class="ov-left">{{ monthName(locale, m.month) }}</th>
            <td v-for="c in cols" :key="c.key">{{ c.fmt(m[c.key]) }}</td>
          </tr>
        </tbody>
        <tfoot><tr>
          <th scope="row" class="ov-left">{{ $t('timeOverview.total') }}</th>
          <td v-for="c in cols" :key="c.key">{{ c.fmt(c.key === 'days' ? data.total_days : data.total[c.key]) }}</td>
        </tr></tfoot>
      </table>
    </div>

    <div class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.travel_by_customer') }}</caption>
        <thead><tr>
          <th scope="col">{{ $t('timeOverview.customer') }}</th>
          <th v-for="c in cols" :key="c.key" scope="col">{{ c.label }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="(r, i) in data.by_customer" :key="r.customer_id ?? 'none-' + i">
            <th scope="row" class="ov-left">{{ r.customer || $t('timeOverview.no_customer') }}</th>
            <td v-for="c in cols" :key="c.key">{{ c.fmt(r[c.key]) }}</td>
          </tr>
          <tr v-if="!data.by_customer.length"><td :colspan="cols.length + 1" class="ov-left ov-muted">{{ $t('timeOverview.no_travel') }}</td></tr>
        </tbody>
      </table>
    </div>

    <div class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.travel_by_route') }}</caption>
        <thead><tr>
          <th scope="col">{{ $t('timeOverview.route') }}</th>
          <th scope="col">{{ $t('timeOverview.customer') }}</th>
          <th v-for="c in cols" :key="c.key" scope="col">{{ c.label }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="(r, i) in data.by_route" :key="i">
            <th scope="row" class="ov-left ov-wrap">{{ r.route }}</th>
            <td class="ov-left">{{ r.customer || '' }}</td>
            <td v-for="c in cols" :key="c.key">{{ c.fmt(r[c.key]) }}</td>
          </tr>
          <tr v-if="!data.by_route.length"><td :colspan="cols.length + 2" class="ov-left ov-muted">{{ $t('timeOverview.no_travel') }}</td></tr>
        </tbody>
      </table>
    </div>
    <p class="ov-days">{{ $t('timeOverview.days_note', { n: data.total_days }) }}</p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { fmtHM, fmtKm, monthName } from '@/utils/timeOverviewFormat'
import './overview.css'

const props = defineProps({ data: { type: Object, required: true } })
const { t, locale } = useI18n()

// Per-row days are shown inside a row's own scope only; totals use data.total_days.
const cols = computed(() => [
  { key: 'entries', label: t('timeOverview.entries'), fmt: (v) => v ?? 0 },
  { key: 'days', label: t('timeOverview.days'), fmt: (v) => v ?? 0 },
  { key: 'total_minutes', label: t('timeOverview.total_time'), fmt: fmtHM },
  { key: 'own_minutes', label: t('timeOverview.own_time'), fmt: fmtHM },
  { key: 'declarable_minutes', label: t('timeOverview.declarable'), fmt: fmtHM },
  { key: 'distance', label: t('timeOverview.distance_km'), fmt: fmtKm },
])
const directionRows = computed(() => [
  { key: 'out', label: t('timeOverview.dir_out'), acc: props.data.out },
  { key: 'back', label: t('timeOverview.dir_back'), acc: props.data.back },
  { key: 'none', label: t('timeOverview.dir_none'), acc: props.data.no_direction },
])
</script>
