<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TimeRangePreset } from '@/composables/useDateRange'

interface DayValue { year: number; month: number; day: number }
interface RangeValue { start?: DayValue; end?: DayValue }

// isDatePickerOpen and formatDateRangeDisplay are kept so callers' bindings stay valid;
// the native date inputs are always visible for a custom range, so neither is needed.
const props = defineProps<{
  selectedRange: TimeRangePreset
  customDateRange: RangeValue | null | undefined
  isDatePickerOpen?: boolean
  formatDateRangeDisplay?: string
}>()

const emit = defineEmits<{
  'update:selectedRange': [value: TimeRangePreset]
  'update:customDateRange': [value: RangeValue]
  'update:isDatePickerOpen': [value: boolean]
  'apply-custom': []
}>()

const { t } = useI18n()

const pad = (n: number) => String(n).padStart(2, '0')
const toIso = (d?: DayValue) => (d ? `${d.year}-${pad(d.month)}-${pad(d.day)}` : '')
const fromIso = (s: string): DayValue | undefined => {
  const [y, m, d] = s.split('-').map(Number)
  return s && y && m && d ? { year: y, month: m, day: d } : undefined
}

// Analytics only exist up to today; a future range is always empty.
const maxDate = (() => {
  const n = new Date()
  return `${n.getFullYear()}-${pad(n.getMonth() + 1)}-${pad(n.getDate())}`
})()

const from = computed(() => toIso(props.customDateRange?.start))
const to = computed(() => toIso(props.customDateRange?.end))

function setStart(e: Event) {
  emit('update:customDateRange', { start: fromIso((e.target as HTMLInputElement).value), end: props.customDateRange?.end })
}
function setEnd(e: Event) {
  emit('update:customDateRange', { start: props.customDateRange?.start, end: fromIso((e.target as HTMLInputElement).value) })
}
</script>

<template>
  <select
    :value="selectedRange"
    :aria-label="t('dateRange.selectRange', 'Date Range')"
    @change="emit('update:selectedRange', ($event.target as HTMLSelectElement).value as TimeRangePreset)"
  >
    <option value="today">{{ t('dateRange.today', 'Today') }}</option>
    <option value="7days">{{ t('dateRange.last7Days', 'Last 7 Days') }}</option>
    <option value="30days">{{ t('dateRange.last30Days', 'Last 30 Days') }}</option>
    <option value="this_month">{{ t('dateRange.thisMonth', 'This Month') }}</option>
    <option value="custom">{{ t('dateRange.customRange', 'Custom Range') }}</option>
  </select>
  <template v-if="selectedRange === 'custom'">
    <input
      type="date"
      :value="from"
      :max="to && to < maxDate ? to : maxDate"
      :aria-label="t('dateRange.from', 'From')"
      @change="setStart"
    />
    <input
      type="date"
      :value="to"
      :min="from || undefined"
      :max="maxDate"
      :aria-label="t('dateRange.to', 'To')"
      @change="setEnd"
    />
    <button
      type="button"
      class="small"
      :disabled="!from || !to || from > to"
      @click="emit('apply-custom')"
    >
      {{ t('dateRange.apply', 'Apply') }}
    </button>
  </template>
</template>
