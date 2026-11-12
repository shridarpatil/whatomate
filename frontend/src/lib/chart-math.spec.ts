import { describe, it, expect } from 'vitest'
import { niceTicks, linearScale, linePath, pieArcs } from './chart-math'

const noNaN = (v: unknown) => {
  if (typeof v === 'number') expect(Number.isNaN(v)).toBe(false)
  else if (typeof v === 'string') expect(v).not.toContain('NaN')
  else if (Array.isArray(v)) v.forEach(noNaN)
  else if (v && typeof v === 'object') Object.values(v).forEach(noNaN)
}

describe('niceTicks', () => {
  it('rounds to 1/2/5 steps', () => {
    expect(niceTicks(0, 87)).toEqual([0, 20, 40, 60, 80, 100])
  })
  it('widens min === max', () => {
    const t = niceTicks(3, 3)
    expect(t[0]).toBe(0)
    expect(t[t.length - 1]).toBeGreaterThanOrEqual(3)
  })
  it('covers negative ranges', () => {
    const t = niceTicks(-7, 12)
    expect(t[0]).toBeLessThanOrEqual(-7)
    expect(t[t.length - 1]).toBeGreaterThanOrEqual(12)
  })
})

describe('linearScale', () => {
  it('maps domain to range', () => {
    expect(linearScale([0, 100], [200, 0])(25)).toBe(150)
  })
  it('maps zero-width domain to the midpoint', () => {
    expect(linearScale([5, 5], [200, 0])(5)).toBe(100)
  })
})

describe('linePath', () => {
  it('builds M/L commands', () => {
    expect(linePath([[0, 1], [2, 3]])).toBe('M0 1L2 3')
  })
  it('handles zero and one point', () => {
    expect(linePath([])).toBe('')
    expect(linePath([[4, 5]])).toBe('M4 5')
  })
})

describe('pieArcs', () => {
  it('splits proportionally and ends at 2π', () => {
    const arcs = pieArcs([1, 1, 2], 50)
    expect(arcs).toHaveLength(3)
    expect(arcs[2].end).toBeCloseTo(Math.PI * 2)
    expect(arcs[0].end).toBeCloseTo(Math.PI / 2)
  })
  it('zero total yields no arcs', () => {
    expect(pieArcs([0, 0], 50)).toEqual([])
  })
  it('single non-zero value draws a full ring from two arcs', () => {
    const arcs = pieArcs([5], 50)
    expect(arcs).toHaveLength(1)
    expect(arcs[0].path.match(/A/g)).toHaveLength(2)
  })
  it('donut paths include the inner radius', () => {
    expect(pieArcs([1, 1], 50, 30)[0].path.match(/A/g)).toHaveLength(2)
  })
})

describe('degenerate inputs produce no NaN', () => {
  it('has no NaN anywhere', () => {
    noNaN(linePath([]))
    noNaN(linePath([[0, 0]]))
    noNaN(pieArcs([0, 0], 50))
    noNaN(pieArcs([5], 50))
    noNaN(pieArcs([5], 50, 30))
    noNaN(pieArcs([], 50))
    noNaN(niceTicks(0, 0))
    const t = niceTicks(0, 0)
    noNaN(t)
    noNaN(linearScale([t[0], t[t.length - 1]], [100, 0])(0))
    noNaN(linearScale([0, 0], [100, 0])(0))
  })
})
