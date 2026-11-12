// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest'
import { createApp, h, nextTick, type Component } from 'vue'
import { createI18n } from 'vue-i18n'
import LineChart from './LineChart.vue'
import BarChart from './BarChart.vue'
import PieChart from './PieChart.vue'
import MeterRow from './MeterRow.vue'
import { foldSeries, foldSlices } from './chart-utils'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: { en: { common: { noData: 'No data', other: 'Other', showData: 'Show data', value: 'Value' } } },
})

let el: HTMLElement | null = null
afterEach(() => { el?.remove(); el = null })

function mount(comp: Component, props: Record<string, unknown>) {
  el = document.createElement('div')
  document.body.appendChild(el)
  createApp({ render: () => h(comp, props) }).use(i18n).mount(el)
  return el
}

const labels = ['a', 'b', 'c']
const two = [
  { label: 'S1', data: [1, 2, 3] },
  { label: 'S2', data: [3, 2, 1] },
]

describe('LineChart', () => {
  it('draws one path per series, a hit target per x and a legend for 2+ series', () => {
    const root = mount(LineChart, { labels, series: two })
    expect(root.querySelectorAll('svg path[stroke-width="2"]').length).toBe(2)
    expect(root.querySelectorAll('rect[tabindex="0"]').length).toBe(3)
    expect(root.querySelectorAll('ul li').length).toBe(2)
    expect(root.querySelector('details table')).toBeTruthy()
    expect(root.innerHTML).not.toContain('NaN')
  })
  it('has no legend for one series', () => {
    const root = mount(LineChart, { labels, series: [two[0]] })
    expect(root.querySelector('ul')).toBeNull()
    // minimal single-series look: no markers, soft area fill, no y tick labels
    expect(root.querySelectorAll('path[stroke="var(--background)"]').length).toBe(0)
    expect(root.querySelector('path[fill-opacity="0.12"]')).toBeTruthy()
    expect(root.querySelectorAll('text[text-anchor="end"]').length).toBe(0)
  })
  it('draws markers, no area and y tick labels for multiple series', () => {
    const root = mount(LineChart, { labels, series: two })
    expect(root.querySelectorAll('path[stroke="var(--background)"]').length).toBeGreaterThanOrEqual(6)
    expect(root.querySelector('path[fill-opacity="0.12"]')).toBeNull()
    expect(root.querySelectorAll('text[text-anchor="end"]').length).toBeGreaterThan(0)
  })
  it('shows a tooltip on focus', async () => {
    const root = mount(LineChart, { labels, series: two })
    root.querySelector('rect[tabindex="0"]')!.dispatchEvent(new Event('focus'))
    await nextTick()
    expect(root.querySelector('article')?.textContent).toContain('S2')
  })
  it.each([
    ['empty', { labels: [], series: [] }],
    ['one point', { labels: ['a'], series: [{ label: 'S', data: [5] }] }],
    ['all zeros', { labels, series: [{ label: 'S', data: [0, 0, 0] }] }],
    ['short data', { labels, series: [{ label: 'S', data: [1] }] }],
  ])('renders %s without NaN', (_, props) => {
    const root = mount(LineChart, props)
    expect(root.innerHTML).not.toContain('NaN')
    if (_ === 'empty') expect(root.textContent).toContain('No data')
  })
})

describe('BarChart', () => {
  it('draws series x labels bars grouped and one per series stacked', () => {
    expect(mount(BarChart, { labels, series: two }).querySelectorAll('path[data-bar]').length).toBe(6)
    el!.remove()
    const root = mount(BarChart, { labels, series: two, stacked: true })
    expect(root.querySelectorAll('path[data-bar]').length).toBe(6)
    expect(root.querySelectorAll('ul li').length).toBe(2)
  })
  it('has no legend for one series', () => {
    expect(mount(BarChart, { labels, series: [two[0]] }).querySelector('ul')).toBeNull()
  })
  it.each([
    ['empty', { labels: [], series: [] }],
    ['one bar', { labels: ['a'], series: [{ label: 'S', data: [5] }] }],
    ['all zeros', { labels, series: [{ label: 'S', data: [0, 0, 0] }], stacked: true }],
    ['negatives', { labels, series: [{ label: 'S', data: [-1, 2, -3] }] }],
  ])('renders %s without NaN', (_, props) => {
    expect(mount(BarChart, props).innerHTML).not.toContain('NaN')
  })
})

describe('PieChart', () => {
  it('draws a slice per non-zero value with a legend', () => {
    const root = mount(PieChart, { labels: ['x', 'y', 'z'], values: [1, 2, 0] })
    expect(root.querySelectorAll('svg path').length).toBe(2)
    expect(root.querySelectorAll('ul li').length).toBe(3)
  })
  it('has no legend for a single slice and draws the full ring', async () => {
    const root = mount(PieChart, { labels: ['x'], values: [4], donut: true })
    expect(root.querySelector('ul')).toBeNull()
    expect(root.querySelectorAll('svg path').length).toBe(1)
    root.querySelector('path')!.dispatchEvent(new Event('focus'))
    await nextTick()
    expect(root.querySelector('article')?.textContent).toContain('100%')
    expect(root.innerHTML).not.toContain('NaN')
  })
  it('shows the empty state for zero total', () => {
    const root = mount(PieChart, { labels: ['x'], values: [0] })
    expect(root.querySelector('svg')).toBeNull()
    expect(root.textContent).toContain('No data')
  })
})

describe('foldSeries', () => {
  it('folds series beyond five into Other in the last slot', () => {
    const many = Array.from({ length: 7 }, (_, i) => ({ label: `s${i}`, data: [1, 1] }))
    const out = foldSeries(many, 'Other')
    expect(out).toHaveLength(5)
    expect(out[4]).toEqual({ label: 'Other', data: [3, 3] })
    expect(out[0].label).toBe('s0')
  })
})

describe('foldSlices', () => {
  it('clamps negatives so percentages match arcs', () => {
    expect(foldSlices(['a', 'b'], [-3, 2], 'Other').map(s => s.value)).toEqual([0, 2])
  })
})

describe('MeterRow', () => {
  it('renders label, meter and formatted value', () => {
    const root = mount(MeterRow, { label: 'Equity', value: 42, high: 85, format: (v: number) => `${v}%` })
    expect(root.querySelector('meter')?.getAttribute('value')).toBe('42')
    expect(root.textContent).toContain('Equity')
    expect(root.textContent).toContain('42%')
  })
})

describe('integer ticks', () => {
  it('line chart with integer data renders no fractional tick label', () => {
    const root = mount(LineChart, { labels, series: [{ label: 'A', data: [0, 1, 2] }, { label: 'B', data: [2, 1, 0] }] })
    const texts = Array.from(root.querySelectorAll('svg text')).map(t => t.textContent?.trim() ?? '')
    expect(texts).toContain('1')
    expect(texts.filter(t => /^\d+\.\d+$/.test(t))).toEqual([])
  })
})
