<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { linearScale, niceTicks } from '@/lib/chart-math'
import ChartLegend from './ChartLegend.vue'
import ChartTooltip from './ChartTooltip.vue'
import ChartDataTable from './ChartDataTable.vue'
import {
  type ChartSeries, BAR_GAP, VIEW_WIDTH, foldSeries, seriesColor, textWidth, value,
} from './chart-utils'

const props = withDefaults(defineProps<{
  labels: string[]
  series: ChartSeries[]
  height?: number
  stacked?: boolean
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
  kind: 'box' as const,
})))

// Stacked bars only stack non-negative values; negatives are clamped to 0 in the geometry
// (tooltip, aria-label and table still show the real value).
const pos = (s: ChartSeries, i: number) => Math.max(0, value(s, i))

const ticks = computed(() => {
  const idx = props.labels.map((_, i) => i)
  let lo = 0
  let hi = 0
  if (props.stacked) {
    hi = Math.max(0, ...idx.map(i => folded.value.reduce((a, s) => a + pos(s, i), 0)))
  } else {
    const all = folded.value.flatMap(s => idx.map(i => value(s, i)))
    lo = Math.min(0, ...all)
    hi = Math.max(0, ...all)
  }
  return niceTicks(lo, hi)
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
const baseline = computed(() => y.value(0))
const groupW = computed(() => (plot.value.x1 - plot.value.x0) / Math.max(n.value, 1))

interface Bar { x: number; y: number; width: number; height: number; color: string }

const groups = computed(() => props.labels.map((_, i) => {
  const gx = plot.value.x0 + i * groupW.value
  const bars: Bar[] = []
  const k = folded.value.length
  if (props.stacked) {
    const bw = Math.max(groupW.value * 0.6, 1)
    const x = gx + (groupW.value - bw) / 2
    let cum = 0
    folded.value.forEach((s, si) => {
      const v = pos(s, i)
      if (v <= 0) return
      const top = y.value(cum + v)
      const bottom = y.value(cum) - (cum > 0 ? BAR_GAP : 0)
      cum += v
      bars.push({ x, y: top, width: bw, height: Math.max(bottom - top, 0), color: items.value[si].color })
    })
  } else {
    const usable = groupW.value * 0.8
    const bw = Math.max((usable - BAR_GAP * (k - 1)) / k, 1)
    const start = gx + (groupW.value - (bw * k + BAR_GAP * (k - 1))) / 2
    folded.value.forEach((s, si) => {
      const v = value(s, i)
      const yv = y.value(v)
      bars.push({
        x: start + si * (bw + BAR_GAP),
        y: Math.min(yv, baseline.value),
        width: bw,
        height: Math.abs(baseline.value - yv),
        color: items.value[si].color,
      })
    })
  }
  const topY = bars.length ? Math.min(...bars.map(b => b.y)) : baseline.value
  return { gx, bars, topY, cx: gx + groupW.value / 2 }
}))

const labelStep = computed(() => Math.max(1, Math.ceil(n.value / Math.max(1, Math.floor((plot.value.x1 - plot.value.x0) / 60)))))

function describe(i: number) {
  return `${props.labels[i]}: ${folded.value.map(s => `${s.label} ${fmt(value(s, i))}`).join(', ')}`
}

const tooltip = computed(() => {
  const i = active.value
  if (i === null || i >= n.value) return null
  const g = groups.value[i]
  return {
    left: (g.cx / W) * 100,
    top: Math.min(Math.max((g.topY / props.height) * 100, 5), 60),
    flip: g.cx > W * 0.6,
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
              <text v-if="i % labelStep === 0" :x="groups[i].cx" :y="plot.y1 + 18" text-anchor="middle" fill="var(--muted-foreground)">{{ l }}</text>
            </template>
          </g>
          <g v-for="(g, i) in groups" :key="i">
            <rect
              v-for="(b, bi) in g.bars"
              :key="bi"
              :x="b.x"
              :y="b.y"
              :width="b.width"
              :height="b.height"
              :fill="b.color"
              :opacity="active === null || active === i ? 1 : 0.6"
              data-bar
            />
          </g>
          <g>
            <rect
              v-for="(g, i) in groups"
              :key="i"
              :x="g.gx"
              :width="groupW"
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
