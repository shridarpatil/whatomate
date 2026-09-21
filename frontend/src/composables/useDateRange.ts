import { ref, computed, watch } from 'vue'
import { CalendarDate } from '@internationalized/date'

export type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'

export interface DateRangeResult {
  from: string
  to: string
}

interface UseDateRangeOptions {
  /** Default preset (defaults to 'this_month') */
  defaultPreset?: TimeRangePreset
  /** localStorage key for persisting selection. If omitted, no persistence. */
  storageKey?: string
  /**
   * Compute presets as UTC calendar days. Set this for views backed by Meta's
   * UTC-bucketed analytics; leave it off for our own DB-backed views, where
   * "today" should mean the viewer's local day.
   */
  utc?: boolean
}

export function useDateRange(options: UseDateRangeOptions = {}) {
  const { defaultPreset = 'this_month', storageKey, utc = false } = options

  // Load saved state from localStorage if configured
  const loadSaved = (): { range: TimeRangePreset; customRange: any } => {
    if (!storageKey) return { range: defaultPreset, customRange: { start: undefined, end: undefined } }

    const savedRange = localStorage.getItem(`${storageKey}_range`) as TimeRangePreset | null
    const savedCustom = localStorage.getItem(`${storageKey}_custom`)

    let customRange: any = { start: undefined, end: undefined }
    if (savedCustom) {
      try {
        const parsed = JSON.parse(savedCustom)
        // RangeCalendar requires CalendarDate instances; the JSON-restored
        // POJOs would render an empty calendar (issue: Apply button shown
        // but no grid on second open).
        customRange = {
          start: parsed.start ? new CalendarDate(parsed.start.year, parsed.start.month, parsed.start.day) : undefined,
          end: parsed.end ? new CalendarDate(parsed.end.year, parsed.end.month, parsed.end.day) : undefined,
        }
      } catch {
        // ignore
      }
    }

    return { range: savedRange || defaultPreset, customRange }
  }

  const saved = loadSaved()
  const selectedRange = ref<TimeRangePreset>(saved.range)
  const customDateRange = ref<any>(saved.customRange)
  const isDatePickerOpen = ref(false)

  // Formats a civil (timezone-less) Y/M/D as YYYY-MM-DD. Date.UTC is used only
  // to normalise out-of-range fields (e.g. day - 30); it never shifts the day.
  function formatDay(year: number, month: number, day: number): string {
    const d = new Date(Date.UTC(year, month, day))
    const mm = String(d.getUTCMonth() + 1).padStart(2, '0')
    const dd = String(d.getUTCDate()).padStart(2, '0')
    return `${d.getUTCFullYear()}-${mm}-${dd}`
  }

  const dateRange = computed<DateRangeResult>(() => {
    const now = new Date()
    const year = utc ? now.getUTCFullYear() : now.getFullYear()
    const month = utc ? now.getUTCMonth() : now.getMonth()
    const day = utc ? now.getUTCDate() : now.getDate()
    const today = formatDay(year, month, day)

    switch (selectedRange.value) {
      case 'today':
        return { from: today, to: today }
      case '7days':
        return { from: formatDay(year, month, day - 7), to: today }
      case '30days':
        return { from: formatDay(year, month, day - 30), to: today }
      case 'custom': {
        const { start, end } = customDateRange.value
        if (start && end) {
          return {
            from: formatDay(start.year, start.month - 1, start.day),
            to: formatDay(end.year, end.month - 1, end.day),
          }
        }
        return { from: formatDay(year, month, 1), to: today }
      }
      case 'this_month':
      default:
        return { from: formatDay(year, month, 1), to: today }
    }
  })

  const formatDateRangeDisplay = computed(() => {
    if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
      const s = customDateRange.value.start
      const e = customDateRange.value.end
      return `${s.month}/${s.day}/${s.year} - ${e.month}/${e.day}/${e.year}`
    }
    return ''
  })

  function savePreferences() {
    if (!storageKey) return
    localStorage.setItem(`${storageKey}_range`, selectedRange.value)
    if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
      localStorage.setItem(`${storageKey}_custom`, JSON.stringify({
        start: { year: customDateRange.value.start.year, month: customDateRange.value.start.month, day: customDateRange.value.start.day },
        end: { year: customDateRange.value.end.year, month: customDateRange.value.end.month, day: customDateRange.value.end.day },
      }))
    }
  }

  function applyCustomRange() {
    if (customDateRange.value.start && customDateRange.value.end) {
      isDatePickerOpen.value = false
      savePreferences()
    }
  }

  // Persist on preset change
  watch(selectedRange, () => savePreferences())

  return {
    selectedRange,
    customDateRange,
    isDatePickerOpen,
    dateRange,
    formatDateRangeDisplay,
    applyCustomRange,
    savePreferences,
  }
}
