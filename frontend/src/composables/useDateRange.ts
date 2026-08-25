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
}

export function useDateRange(options: UseDateRangeOptions = {}) {
  const { defaultPreset = 'this_month', storageKey } = options

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

  // Formats a date as YYYY-MM-DD using its UTC calendar fields, so the
  // resulting date always matches the server's (and Meta's) notion of
  // "today" regardless of the browser's local timezone. Using local
  // getters here let users in timezones ahead of UTC pick/compute a date
  // that is still in the future in UTC, which the backend then rejects.
  function formatDateLocal(date: Date): string {
    const year = date.getUTCFullYear()
    const month = String(date.getUTCMonth() + 1).padStart(2, '0')
    const day = String(date.getUTCDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }

  const dateRange = computed<DateRangeResult>(() => {
    const now = new Date()
    let from: Date
    let to: Date = now

    switch (selectedRange.value) {
      case 'today':
        from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
        to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
        break
      case '7days':
        from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - 7))
        to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
        break
      case '30days':
        from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - 30))
        to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
        break
      case 'this_month':
        from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))
        to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
        break
      case 'custom':
        if (customDateRange.value.start && customDateRange.value.end) {
          from = new Date(Date.UTC(customDateRange.value.start.year, customDateRange.value.start.month - 1, customDateRange.value.start.day))
          to = new Date(Date.UTC(customDateRange.value.end.year, customDateRange.value.end.month - 1, customDateRange.value.end.day))
        } else {
          from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))
          to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
        }
        break
      default:
        from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))
        to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
    }

    return { from: formatDateLocal(from), to: formatDateLocal(to) }
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
