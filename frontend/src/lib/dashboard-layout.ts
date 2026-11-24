import type { DashboardWidget } from '@/services/api'

export const GRID_COLS = 12

export interface Placed { id: string; x: number; y: number; w: number; h: number }

/** Saved positions as-is; legacy widgets (grid_w === 0) are auto-placed below them. */
export function placeWidgets(widgets: DashboardWidget[]): Placed[] {
  const layout: Placed[] = []

  const positioned = widgets.filter(w => w.grid_w > 0)
  const legacy = widgets.filter(w => w.grid_w === 0)

  for (const w of positioned) {
    layout.push({ id: w.id, x: w.grid_x, y: w.grid_y, w: w.grid_w, h: w.grid_h })
  }

  if (legacy.length > 0) {
    let nextY = 0
    if (positioned.length > 0) {
      nextY = Math.max(...positioned.map(w => w.grid_y + w.grid_h))
    }

    let curX = 0
    let curY = nextY

    const legacyNumber = legacy.filter(w => !['chart', 'table', 'shortcuts'].includes(w.display_type))
    const legacyLarge = legacy.filter(w => ['chart', 'table', 'shortcuts'].includes(w.display_type))

    for (const w of legacyNumber) {
      const itemW = 3
      const itemH = 3
      if (curX + itemW > GRID_COLS) {
        curX = 0
        curY += itemH
      }
      layout.push({ id: w.id, x: curX, y: curY, w: itemW, h: itemH })
      curX += itemW
    }

    if (legacyNumber.length > 0 && legacyLarge.length > 0) {
      curX = 0
      curY += 3
    }

    for (const w of legacyLarge) {
      let itemW = 6
      let itemH = 5
      if (w.display_type === 'table' || w.display_type === 'shortcuts') {
        itemW = 6
        itemH = 8
      }
      if (curX + itemW > GRID_COLS) {
        curX = 0
        curY += itemH
      }
      layout.push({ id: w.id, x: curX, y: curY, w: itemW, h: itemH })
      curX += itemW
    }
  }

  return layout
}

/** placeWidgets sorted by y, then x, then original index (stable). */
export function orderWidgets(widgets: DashboardWidget[]): Placed[] {
  return placeWidgets(widgets)
    .map((p, index) => ({ p, index }))
    .sort((a, b) => a.p.y - b.p.y || a.p.x - b.p.x || a.index - b.index)
    .map(({ p }) => p)
}

/** Re-flows x/y left to right in list order, wrapping at GRID_COLS. */
function reflow(list: Placed[]): Placed[] {
  let x = 0
  let y = 0
  let rowH = 0
  for (const item of list) {
    if (x > 0 && x + item.w > GRID_COLS) {
      y += rowH
      x = 0
      rowH = 0
    }
    item.x = x
    item.y = y
    x += item.w
    rowH = Math.max(rowH, item.h)
  }
  return list
}

/** Moves one item, then re-flows x/y left to right. Sizes (w/h) are never changed. */
export function reorder(order: Placed[], from: number, to: number): Placed[] {
  const inRange = (i: number) => Number.isInteger(i) && i >= 0 && i < order.length
  const list = order.map(p => ({ ...p }))
  if (inRange(from) && inRange(to) && from !== to) {
    const [moved] = list.splice(from, 1)
    list.splice(to, 0, moved)
  }
  return reflow(list)
}

export const MIN_W = 2
export const MIN_H = 2
export const MAX_H = 20

/** Sets one item's size (rounded, clamped to MIN_W..GRID_COLS and MIN_H..MAX_H), then re-flows. */
export function resize(order: Placed[], index: number, w: number, h: number): Placed[] {
  const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, Math.round(v)))
  const list = order.map(p => ({ ...p }))
  const item = list[index]
  if (item) {
    item.w = clamp(w, MIN_W, GRID_COLS)
    item.h = clamp(h, MIN_H, MAX_H)
  }
  return reflow(list)
}
