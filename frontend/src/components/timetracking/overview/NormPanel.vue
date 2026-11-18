<template>
  <div class="ov-panel">
    <dl class="ov-cards">
      <div class="ov-card"><dt>{{ $t('timeOverview.norm') }}</dt><dd>{{ fmtHM(data.total.norm_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.booked') }}</dt><dd>{{ fmtHM(data.total.booked_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.diff') }}</dt><dd :class="diffClass(data.total.diff_minutes)">{{ fmtSignedHM(data.total.diff_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.norm_to_date') }}</dt><dd>{{ fmtHM(data.norm_to_date_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.booked_to_date') }}</dt><dd>{{ fmtHM(data.booked_to_date_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.diff_to_date') }}</dt><dd :class="diffClass(data.diff_to_date_minutes)">{{ fmtSignedHM(data.diff_to_date_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.planned') }}</dt><dd>{{ fmtHM(data.planned_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.days') }}</dt><dd>{{ data.total_days }}</dd></div>
    </dl>
    <p class="ov-muted">{{ $t('timeOverview.norm_basis', { hours: data.hours_per_week, perDay: fmtHM(data.norm_minutes_per_day), days: minutesToDays(data.total.vacation_minutes, data.norm_minutes_per_day) }) }}</p>

    <div class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.norm_caption', { year: data.year }) }}</caption>
        <thead>
          <tr>
            <th scope="col">{{ $t('timeOverview.month') }}</th>
            <th scope="col">{{ $t('timeOverview.norm_days') }}</th>
            <th scope="col">{{ $t('timeOverview.norm') }}</th>
            <th scope="col">{{ $t('timeOverview.worked') }}</th>
            <th scope="col">{{ $t('timeOverview.of_which_travel') }}</th>
            <th scope="col">{{ $t('timeOverview.sick') }}</th>
            <th scope="col">{{ $t('timeOverview.vacation') }}</th>
            <th scope="col">{{ $t('timeOverview.booked') }}</th>
            <th scope="col">{{ $t('timeOverview.diff') }}</th>
            <th scope="col">{{ $t('timeOverview.booked_days') }}</th>
            <th scope="col">{{ $t('timeOverview.unbooked_norm_days') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in data.months" :key="m.month">
            <th scope="row" class="ov-left">{{ monthName(locale, m.month) }}</th>
            <td>{{ m.norm_days }}</td>
            <td>{{ fmtHM(m.norm_minutes) }}</td>
            <td>{{ fmtHM(m.worked_minutes) }}</td>
            <td>{{ fmtHM(m.travel_minutes) }}</td>
            <td>{{ fmtHM(m.sick_minutes) }}</td>
            <td>{{ fmtHM(m.vacation_minutes) }}</td>
            <td>{{ fmtHM(m.booked_minutes) }}</td>
            <td :class="diffClass(m.diff_minutes)">{{ fmtSignedHM(m.diff_minutes) }}</td>
            <td>{{ m.booked_days }}</td>
            <td>{{ m.unbooked_norm_days }}</td>
          </tr>
        </tbody>
        <tfoot>
          <tr>
            <th scope="row" class="ov-left">{{ $t('timeOverview.total') }}</th>
            <td>{{ data.total.norm_days }}</td>
            <td>{{ fmtHM(data.total.norm_minutes) }}</td>
            <td>{{ fmtHM(data.total.worked_minutes) }}</td>
            <td>{{ fmtHM(data.total.travel_minutes) }}</td>
            <td>{{ fmtHM(data.total.sick_minutes) }}</td>
            <td>{{ fmtHM(data.total.vacation_minutes) }}</td>
            <td>{{ fmtHM(data.total.booked_minutes) }}</td>
            <td :class="diffClass(data.total.diff_minutes)">{{ fmtSignedHM(data.total.diff_minutes) }}</td>
            <td>{{ data.total_days }}</td>
            <td>{{ data.total.unbooked_norm_days }}</td>
          </tr>
        </tfoot>
      </table>
    </div>
    <p class="ov-days">{{ $t('timeOverview.days_note', { n: data.total_days }) }}</p>
    <p v-if="data.total.past_unbooked_norm_days > 0" class="ov-warn">{{ $t('timeOverview.past_unbooked', { n: data.total.past_unbooked_norm_days }) }}</p>

    <section :aria-label="$t('timeOverview.holidays_used')">
      <h3 class="ov-h3">{{ $t('timeOverview.holidays_used') }}</h3>
      <p v-if="!data.holiday_weekdays.length" class="ov-muted">{{ $t('timeOverview.no_holidays') }}</p>
      <ul v-else class="ov-holidays">
        <li v-for="d in data.holiday_weekdays" :key="d">{{ formatDate(d) }}</li>
      </ul>
    </section>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { useDateFormat } from '@/composables/useDateFormat'
import { fmtHM, fmtSignedHM, diffClass, monthName, minutesToDays } from '@/utils/timeOverviewFormat'
import './overview.css'

defineProps({ data: { type: Object, required: true } })
const { locale } = useI18n()
const { formatDate } = useDateFormat()
</script>

<style scoped>
.ov-holidays { display: flex; flex-wrap: wrap; gap: 6px; list-style: none; padding: 0; margin: 6px 0 0; }
.ov-holidays li { background: var(--color-surface); border: 1px solid var(--color-border); border-radius: 12px; padding: 2px 10px; font-size: 12px; color: var(--color-text); }
</style>
