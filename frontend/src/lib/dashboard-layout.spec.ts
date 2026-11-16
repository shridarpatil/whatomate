import { describe, it, expect } from 'vitest'
import type { DashboardWidget } from '@/services/api'
import { orderWidgets, placeWidgets, reorder, resize, type Placed } from './dashboard-layout'

const widget = (id: string, p: Partial<DashboardWidget> = {}): DashboardWidget => ({
  id, name: id, description: '', data_source: 'messages', metric: 'count', field: '', filters: [],
  display_type: 'number', chart_type: '', group_by_field: '', show_change: true, color: 'blue', size: 'small',
  display_order: 0, grid_x: 0, grid_y: 0, grid_w: 0, grid_h: 0, config: {}, is_shared: false,
  is_default: false, is_owner: true, created_by: '', created_at: '', updated_at: '', ...p,
})

const at = (id: string, x: number, y: number, w: number, h: number) => widget(id, { grid_x: x, grid_y: y, grid_w: w, grid_h: h })

describe('placeWidgets', () => {
  it('keeps saved positions and auto-places legacy widgets below', () => {
    const placed = placeWidgets([at('A', 0, 0, 6, 5), widget('L')])
    expect(placed[0]).toEqual({ id: 'A', x: 0, y: 0, w: 6, h: 5 })
    expect(placed[1]).toEqual({ id: 'L', x: 0, y: 5, w: 3, h: 3 })
  })
  it('sizes legacy chart 6x5 and table/shortcuts 6x8', () => {
    const placed = placeWidgets([widget('C', { display_type: 'chart' }), widget('T', { display_type: 'table' })])
    expect(placed.find(p => p.id === 'C')).toMatchObject({ w: 6, h: 5 })
    expect(placed.find(p => p.id === 'T')).toMatchObject({ w: 6, h: 8 })
  })
})

describe('orderWidgets', () => {
  it('sorts by y, then x, with legacy widgets last', () => {
    const ids = orderWidgets([at('B', 6, 0, 6, 5), at('A', 0, 0, 6, 5), widget('L')]).map(p => p.id)
    expect(ids).toEqual(['A', 'B', 'L'])
  })
  it('overlapping saved positions keep stable order', () => {
    const ids = orderWidgets([at('first', 0, 0, 6, 5), at('second', 0, 0, 6, 5)]).map(p => p.id)
    expect(ids).toEqual(['first', 'second'])
  })
})

describe('reorder', () => {
  const order: Placed[] = [
    { id: 'A', x: 0, y: 0, w: 3, h: 3 },
    { id: 'B', x: 3, y: 0, w: 6, h: 5 },
    { id: 'C', x: 9, y: 0, w: 4, h: 8 },
  ]
  it('reorder preserves sizes', () => {
    const out = reorder(order, 2, 0)
    expect(out.map(p => p.id)).toEqual(['C', 'A', 'B'])
    for (const p of out) {
      const src = order.find(o => o.id === p.id)!
      expect([p.w, p.h]).toEqual([src.w, src.h])
      expect(p.x + p.w).toBeLessThanOrEqual(12)
    }
  })
  it('does not mutate its input', () => {
    const copy = JSON.parse(JSON.stringify(order))
    reorder(order, 0, 2)
    expect(order).toEqual(copy)
  })
  it('reorder flows to new row', () => {
    const six: Placed[] = ['A', 'B', 'C'].map((id, i) => ({ id, x: 0, y: i, w: 6, h: 5 }))
    expect(reorder(six, 0, 1).map(p => p.y)).toEqual([0, 0, 5])
  })
  it('uses the tallest item as row height', () => {
    const out = reorder([
      { id: 'A', x: 0, y: 0, w: 6, h: 3 }, { id: 'B', x: 6, y: 0, w: 6, h: 8 }, { id: 'C', x: 0, y: 8, w: 6, h: 2 },
    ], 2, 2)
    expect(out.map(p => p.y)).toEqual([0, 0, 8])
  })
  it('returns the same order for out-of-range moves', () => {
    expect(reorder(order, 0, 9).map(p => p.id)).toEqual(['A', 'B', 'C'])
  })
})

describe('resize', () => {
  const order: Placed[] = [
    { id: 'A', x: 0, y: 0, w: 6, h: 3 },
    { id: 'B', x: 6, y: 0, w: 6, h: 3 },
  ]
  it('widening an item wraps the next one to a new row', () => {
    const out = resize(order, 0, 8, 3)
    expect(out[0]).toEqual({ id: 'A', x: 0, y: 0, w: 8, h: 3 })
    expect(out[1]).toEqual({ id: 'B', x: 0, y: 3, w: 6, h: 3 })
  })
  it('row height follows the tallest item', () => {
    const out = resize(order, 0, 4, 7)
    expect(out[1]).toMatchObject({ x: 4, y: 0 })
    expect(out[0]).toMatchObject({ w: 4, h: 7 })
  })
  it('rounds and clamps sizes', () => {
    expect(resize(order, 0, 0.4, 0)[0]).toMatchObject({ w: 2, h: 2 })
    expect(resize(order, 0, 15.6, 99)[0]).toMatchObject({ w: 12, h: 20 })
    expect(resize(order, 0, 5.6, 3.4)[0]).toMatchObject({ w: 6, h: 3 })
  })
  it('does not mutate the input and ignores a bad index', () => {
    const out = resize(order, 5, 3, 3)
    expect(out).toEqual(order)
    expect(out[0]).not.toBe(order[0])
  })
})
