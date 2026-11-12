<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { pieArcs } from '@/lib/chart-math'
import ChartLegend from './ChartLegend.vue'
import ChartTooltip from './ChartTooltip.vue'
import ChartDataTable from './ChartDataTable.vue'
import { foldSlices, seriesColor } from './chart-utils'

const props = defineProps<{
  labels: string[]
  values: number[]
  donut?: boolean
  formatValue?: (v: number) => string
  caption?: string
}>()

const { t } = useI18n()
const fmt = (v: number) => (props.formatValue ? props.formatValue(v) : v.toLocaleString())

const SIZE = 240
const R = 100
const active = ref<number | null>(null)

const slices = computed(() => foldSlices(props.labels, props.values, t('common.other')))
const total = computed(() => slices.value.reduce((a, s) => a + s.value, 0))
const empty = computed(() => total.value <= 0)
const arcs = computed(() => pieArcs(slices.value.map(s => s.value), R, props.donut ? R * 0.6 : 0))
const pct = (v: number) => `${total.value > 0 ? Math.round((v / total.value) * 1000) / 10 : 0}%`

const items = computed(() => slices.value.map((s, i) => ({ label: s.label, color: seriesColor(i), kind: 'box' as const })))

function describe(i: number) {
  const s = slices.value[i]
  return `${s.label}: ${fmt(s.value)} (${pct(s.value)})`
}

const tooltip = computed(() => {
  const i = active.value
  if (i === null || i >= arcs.value.length) return null
  const a = arcs.value[i]
  const mid = (a.start + a.end) / 2
  const r = R * (props.donut ? 0.8 : 0.6)
  const cx = a.end - a.start >= Math.PI * 2 - 1e-6 ? 0 : r * Math.sin(mid)
  const cy = a.end - a.start >= Math.PI * 2 - 1e-6 ? -r : -r * Math.cos(mid)
  const left = ((cx + R) / (R * 2)) * 100
  return {
    left,
    top: ((cy + R) / (R * 2)) * 100,
    flip: left > 60,
    title: slices.value[i].label,
    rows: [{ ...items.value[i], value: `${fmt(slices.value[i].value)} (${pct(slices.value[i].value)})` }],
  }
})

const tableCells = computed(() => slices.value.map(s => [fmt(s.value), pct(s.value)]))
const tableColumns = computed(() => [t('common.value'), '%'])
</script>

<template>
  <figure>
    <figcaption v-if="caption || $slots.caption"><slot name="caption">{{ caption }}</slot></figcaption>
    <p v-if="empty" class="text-light">{{ t('common.noData') }}</p>
    <template v-else>
      <div class="relative" :style="{ maxWidth: `${SIZE}px` }">
        <svg :viewBox="`${-R} ${-R} ${R * 2} ${R * 2}`" width="100%" @mouseleave="active = null">
          <template v-for="(a, i) in arcs" :key="i">
            <path
              v-if="a.path"
              :d="a.path"
              :fill="items[i].color"
              fill-rule="evenodd"
              stroke="var(--background)"
              stroke-width="2"
              stroke-linejoin="round"
              :opacity="active === null || active === i ? 1 : 0.6"
              tabindex="0"
              role="graphics-symbol"
              :aria-label="describe(i)"
              @mouseenter="active = i"
              @focus="active = i"
              @blur="active = null"
            />
          </template>
        </svg>
        <ChartTooltip v-if="tooltip" visible :left="tooltip.left" :top="tooltip.top" :flip="tooltip.flip" :title="tooltip.title" :rows="tooltip.rows" />
      </div>
      <ChartLegend :items="items" />
      <ChartDataTable :row-labels="slices.map(s => s.label)" :columns="tableColumns" :cells="tableCells" />
    </template>
  </figure>
</template>
