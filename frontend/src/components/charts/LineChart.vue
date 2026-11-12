<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { linePath, linearScale, niceTicks } from '@/lib/chart-math'
import ChartMarker from './ChartMarker.vue'
import ChartLegend from './ChartLegend.vue'
import ChartTooltip from './ChartTooltip.vue'
import ChartDataTable from './ChartDataTable.vue'
import {
  type ChartSeries, MARKER_SIZE, SERIES_DASHES, SERIES_MARKERS, VIEW_WIDTH,
  foldSeries, seriesColor, textWidth, value,
} from './chart-utils'

const props = withDefaults(defineProps<{
  labels: string[]
  series: ChartSeries[]
  height?: number
  fill?: boolean
  formatValue?: (v: number) => string
  caption?: string
}>(), { height: 240 })

const { t } = useI18n()
const fmt = (v: number) => (props.formatValue ? props.formatValue(v) : v.toLocaleString())

const W = VIEW_WIDTH
const active = ref<number | null>(null)

const folded = computed(() => foldSeries(props.series, t('common.other')))
const n = computed(() => props.labels.length)
const empty = computed(() => n.value === 0 || folded.value.length === 0)

const items = computed(() => folded.value.map((s, i) => ({
  label: s.label,
  color: seriesColor(i, i === 4 && props.series.length > 5 ? undefined : s.color),
  dash: SERIES_DASHES[Math.min(i, 4)],
  marker: SERIES_MARKERS[Math.min(i, 4)],
  kind: 'line' as const,
})))

const ticks = computed(() => {
  const all = folded.value.flatMap(s => props.labels.map((_, i) => value(s, i)))
  return niceTicks(Math.min(0, ...all, 0), Math.max(0, ...all, 0))
})
const tickLabels = computed(() => ticks.value.map(fmt))

const margin = computed(() => ({
  top: 12,
  right: 16,
  bottom: 28,
  left: Math.max(40, Math.max(...tickLabels.value.map(textWidth)) + 14),
}))
const plot = computed(() => ({
  x0: margin.value.left,
  x1: W - margin.value.right,
  y0: margin.value.top,
  y1: props.height - margin.value.bottom,
}))

const y = computed(() => linearScale([ticks.value[0], ticks.value[ticks.value.length - 1]], [plot.value.y1, plot.value.y0]))
const xs = computed(() => props.labels.map((_, i) =>
  n.value === 1 ? (plot.value.x0 + plot.value.x1) / 2 : plot.value.x0 + (i / (n.value - 1)) * (plot.value.x1 - plot.value.x0)))

const lines = computed(() => folded.value.map((s, si) => {
  const pts = props.labels.map((_, i) => [xs.value[i], y.value(value(s, i))] as [number, number])
  const d = linePath(pts)
  const base = y.value(0)
  const area = pts.length > 1 ? `${d}L${pts[pts.length - 1][0]} ${base}L${pts[0][0]} ${base}Z` : ''
  return { ...items.value[si], pts, d, area }
}))

const labelStep = computed(() => Math.max(1, Math.ceil(n.value / Math.max(1, Math.floor((plot.value.x1 - plot.value.x0) / 60)))))
const markerStep = computed(() => Math.max(1, Math.ceil(n.value / 12)))

function bounds(i: number) {
  const a = i === 0 ? plot.value.x0 : (xs.value[i - 1] + xs.value[i]) / 2
  const b = i === n.value - 1 ? plot.value.x1 : (xs.value[i] + xs.value[i + 1]) / 2
  return { x: a, width: Math.max(b - a, 1) }
}

function describe(i: number) {
  return `${props.labels[i]}: ${folded.value.map(s => `${s.label} ${fmt(value(s, i))}`).join(', ')}`
}

const tooltip = computed(() => {
  const i = active.value
  if (i === null || i >= n.value) return null
  const top = Math.min(...folded.value.map(s => y.value(value(s, i))))
  return {
    left: (xs.value[i] / W) * 100,
    top: Math.min(Math.max((top / props.height) * 100, 5), 60),
    flip: xs.value[i] > W * 0.6,
    title: props.labels[i],
    rows: folded.value.map((s, si) => ({ ...items.value[si], label: s.label, value: fmt(value(s, i)) })),
  }
})

const tableCells = computed(() => props.labels.map((_, i) => folded.value.map(s => fmt(value(s, i)))))
</script>

<template>
  <figure>
    <figcaption v-if="caption || $slots.caption"><slot name="caption">{{ caption }}</slot></figcaption>
    <p v-if="empty" class="text-light">{{ t('common.noData') }}</p>
    <template v-else>
      <ChartLegend :items="items" />
      <div class="relative">
        <svg :viewBox="`0 0 ${W} ${height}`" width="100%" font-size="11" @mouseleave="active = null">
          <g>
            <template v-for="(tk, i) in ticks" :key="i">
              <line :x1="plot.x0" :x2="plot.x1" :y1="y(tk)" :y2="y(tk)" stroke="var(--border)" stroke-width="1" />
              <text :x="plot.x0 - 6" :y="y(tk)" text-anchor="end" dominant-baseline="middle" fill="var(--muted-foreground)">{{ tickLabels[i] }}</text>
            </template>
          </g>
          <g>
            <template v-for="(l, i) in labels" :key="i">
              <text v-if="i % labelStep === 0" :x="xs[i]" :y="plot.y1 + 18" text-anchor="middle" fill="var(--muted-foreground)">{{ l }}</text>
            </template>
          </g>
          <line v-if="active !== null" :x1="xs[active]" :x2="xs[active]" :y1="plot.y0" :y2="plot.y1" stroke="var(--muted-foreground)" stroke-width="1" />
          <g v-for="(l, si) in lines" :key="si">
            <path v-if="fill && l.area" :d="l.area" :fill="l.color" fill-opacity="0.12" stroke="none" />
            <path :d="l.d" fill="none" :stroke="l.color" stroke-width="2" stroke-linejoin="round" :stroke-dasharray="l.dash || undefined" />
            <template v-for="(p, i) in l.pts" :key="i">
              <ChartMarker v-if="i % markerStep === 0 || i === active" :shape="l.marker" :x="p[0]" :y="p[1]" :size="MARKER_SIZE" :color="l.color" />
            </template>
          </g>
          <g>
            <rect
              v-for="(_, i) in labels"
              :key="i"
              :x="bounds(i).x"
              :width="bounds(i).width"
              :y="plot.y0"
              :height="plot.y1 - plot.y0"
              fill="transparent"
              tabindex="0"
              role="graphics-symbol"
              :aria-label="describe(i)"
              @mouseenter="active = i"
              @focus="active = i"
              @blur="active = null"
            />
          </g>
        </svg>
        <ChartTooltip v-if="tooltip" visible :left="tooltip.left" :top="tooltip.top" :flip="tooltip.flip" :title="tooltip.title" :rows="tooltip.rows" />
      </div>
      <ChartDataTable :row-labels="labels" :columns="folded.map(s => s.label)" :cells="tableCells" />
    </template>
  </figure>
</template>
