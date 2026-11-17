<template>
  <div class="ov-panel">
    <dl class="ov-cards">
      <div class="ov-card"><dt>{{ $t('timeOverview.entitlement') }}</dt><dd>{{ fmtHM(data.entitlement_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.carry_over') }}</dt><dd>{{ fmtHM(data.carry_over_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.available') }}</dt><dd>{{ fmtHM(data.available_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.used') }}</dt><dd>{{ fmtHM(data.used_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.planned_vacation') }}</dt><dd>{{ fmtHM(data.planned_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.remaining') }}</dt><dd :class="diffClass(data.remaining_minutes)">{{ fmtHM(data.remaining_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.remaining_after_planned') }}</dt><dd :class="diffClass(data.remaining_after_planned_minutes)">{{ fmtHM(data.remaining_after_planned_minutes) }}</dd></div>
      <div class="ov-card"><dt>{{ $t('timeOverview.days') }}</dt><dd>{{ data.total_days }}</dd></div>
    </dl>

    <p v-if="!data.vacation_projects_set" class="ov-warn">{{ $t('timeOverview.no_vacation_projects') }}</p>

    <form class="carry-form" @submit.prevent="save">
      <div class="ov-field">
        <label for="ov-carry-over">{{ $t('timeOverview.carry_over_label', { year: data.year }) }}</label>
        <input id="ov-carry-over" v-model="carryHours" class="ov-input" type="number" step="0.25" inputmode="decimal" aria-describedby="ov-carry-hint" />
        <span id="ov-carry-hint" class="ov-muted">{{ $t('timeOverview.carry_over_hint') }}</span>
      </div>
      <div class="carry-actions">
        <button
          v-if="data.suggested_carry_over_minutes != null"
          type="button"
          class="ov-btn"
          data-test="use-suggestion"
          @click="useSuggestion"
        >{{ $t('timeOverview.use_suggestion', { hours: minutesToHours(data.suggested_carry_over_minutes) }) }}</button>
        <button type="submit" class="ov-btn ov-btn-primary" :disabled="saving">{{ $t('common.save') }}</button>
        <button type="button" class="ov-link" @click="$emit('open-settings')">{{ $t('timeOverview.open_settings') }}</button>
      </div>
    </form>

    <div class="ov-table-wrap">
      <table class="ov-table">
        <caption>{{ $t('timeOverview.vacation_caption', { year: data.year }) }}</caption>
        <thead>
          <tr>
            <th scope="col">{{ $t('timeOverview.month') }}</th>
            <th scope="col">{{ $t('timeOverview.vacation') }}</th>
            <th scope="col">{{ $t('timeOverview.days') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in data.months" :key="m.month">
            <th scope="row" class="ov-left">{{ monthName(locale, m.month) }}</th>
            <td>{{ fmtHM(m.minutes) }}</td>
            <td>{{ m.days }}</td>
          </tr>
        </tbody>
        <tfoot>
          <tr>
            <th scope="row" class="ov-left">{{ $t('timeOverview.total') }}</th>
            <td>{{ fmtHM(data.used_minutes + data.planned_minutes) }}</td>
            <td>{{ data.total_days }}</td>
          </tr>
        </tfoot>
      </table>
    </div>
    <p class="ov-days">{{ $t('timeOverview.days_note', { n: data.total_days }) }}</p>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fmtHM, diffClass, monthName, minutesToHours } from '@/utils/timeOverviewFormat'
import './overview.css'

const props = defineProps({
  data: { type: Object, required: true },
  saving: { type: Boolean, default: false },
})
// 'save-carry-over' carries the entered hours (number, or null to clear the year's own value).
const emit = defineEmits(['save-carry-over', 'open-settings'])
const { locale } = useI18n()

const carryHours = ref(minutesToHours(props.data.carry_over_minutes))
watch(() => props.data.carry_over_minutes, (v) => { carryHours.value = minutesToHours(v) })

function useSuggestion() {
  carryHours.value = minutesToHours(props.data.suggested_carry_over_minutes)
  emit('save-carry-over', carryHours.value)
}

function save() {
  const raw = String(carryHours.value ?? '').trim().replace(',', '.')
  const n = parseFloat(raw)
  emit('save-carry-over', raw === '' || !Number.isFinite(n) ? null : n)
}
</script>

<style scoped>
.carry-form { max-width: 460px; }
.carry-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
</style>
