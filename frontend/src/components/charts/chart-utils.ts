export type MarkerShape = 'circle' | 'square' | 'triangle' | 'diamond' | 'cross'

export interface ChartSeries {
  label: string
  data: number[]
  color?: string
}

/** Fixed slot order; a series keeps the slot of its index. */
export const SERIES_COLORS = [
  'var(--primary)',
  'var(--success)',
  'var(--danger)',
  'var(--warning)',
  'var(--muted-foreground)',
]
/** Secondary encodings so identity never relies on colour alone (slot 0 is solid). */
export const SERIES_DASHES = ['', '8 4', '2 3', '10 3 2 3', '4 2 1 2']
export const SERIES_MARKERS: MarkerShape[] = ['circle', 'square', 'triangle', 'diamond', 'cross']

/** Fallback chart width (px) before the wrapper is measured. */
export const VIEW_WIDTH = 600
export const MARKER_SIZE = 10
export const BAR_GAP = 2

export function seriesColor(index: number, explicit?: string): string {
  return explicit || SERIES_COLORS[Math.min(index, SERIES_COLORS.length - 1)]
}

const finite = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : 0)

/** Keep the first 4 series and fold the rest into one "Other" series in the last slot. */
export function foldSeries(series: ChartSeries[], otherLabel: string): ChartSeries[] {
  if (series.length <= SERIES_COLORS.length) return series
  const keep = series.slice(0, SERIES_COLORS.length - 1)
  const rest = series.slice(SERIES_COLORS.length - 1)
  const len = Math.max(...rest.map(s => s.data.length))
  const data = Array.from({ length: len }, (_, i) => rest.reduce((a, s) => a + finite(s.data[i]), 0))
  return [...keep, { label: otherLabel, data }]
}

export function foldSlices(labels: string[], values: number[], otherLabel: string) {
  const n = Math.max(labels.length, values.length)
  const all = Array.from({ length: n }, (_, i) => ({ label: labels[i] ?? '', value: Math.max(0, finite(values[i])) }))
  if (all.length <= SERIES_COLORS.length) return all
  const keep = all.slice(0, SERIES_COLORS.length - 1)
  const other = all.slice(SERIES_COLORS.length - 1).reduce((a, s) => a + s.value, 0)
  return [...keep, { label: otherLabel, value: other }]
}

export function value(series: ChartSeries, i: number): number {
  return finite(series.data[i])
}

/** Approximate label width in viewBox units (font-size 11). */
export function textWidth(s: string): number {
  return s.length * 6.5
}
