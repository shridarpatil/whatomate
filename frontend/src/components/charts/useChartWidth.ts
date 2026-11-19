import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { VIEW_WIDTH } from './chart-utils'

/** Wrapper width in px (1 viewBox unit = 1px). Falls back to VIEW_WIDTH until measured. */
export function useChartWidth(el: Ref<HTMLElement | null>) {
  const width = ref(VIEW_WIDTH)
  let ro: ResizeObserver | null = null
  const measure = () => {
    const w = el.value?.getBoundingClientRect().width ?? 0
    if (w >= 120) width.value = Math.round(w)
  }
  watch(el, node => {
    ro?.disconnect()
    ro = null
    if (!node) return
    measure()
    if (typeof ResizeObserver !== 'undefined') {
      ro = new ResizeObserver(measure)
      ro.observe(node)
    }
  }, { flush: 'post' })
  onBeforeUnmount(() => ro?.disconnect())
  return width
}
