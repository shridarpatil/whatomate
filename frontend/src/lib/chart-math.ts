const num = (n: number) => (Number.isFinite(n) ? Math.round(n * 100) / 100 : 0)

/** Ascending ticks in steps of 1/2/5 x 10^n covering [min, max]; `integer` forces whole-number steps. */
export function niceTicks(min: number, max: number, count = 5, opts?: { integer?: boolean }): number[] {
  if (!Number.isFinite(min)) min = 0
  if (!Number.isFinite(max)) max = 0
  if (min > max) [min, max] = [max, min]
  if (min === max) {
    if (min === 0) { min = 0; max = 1 } else if (min > 0) { max = min; min = 0 } else { max = 0 }
  }
  const rough = (max - min) / Math.max(1, count)
  const pow = Math.pow(10, Math.floor(Math.log10(rough)))
  const f = rough / pow
  let step = (f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10) * pow
  if (opts?.integer) step = Math.max(1, Math.round(step))
  const start = Math.floor(min / step + 1e-9)
  const end = Math.ceil(max / step - 1e-9)
  const ticks: number[] = []
  for (let i = start; i <= end; i++) ticks.push(Number((i * step).toPrecision(12)))
  return ticks
}

export function linearScale(domain: [number, number], range: [number, number]): (v: number) => number {
  const [d0, d1] = domain
  const [r0, r1] = range
  const span = d1 - d0
  if (!Number.isFinite(span) || span === 0) return () => (r0 + r1) / 2
  return (v: number) => r0 + ((v - d0) / span) * (r1 - r0)
}

export function linePath(points: Array<[x: number, y: number]>): string {
  return points
    .filter(([x, y]) => Number.isFinite(x) && Number.isFinite(y))
    .map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${num(x)} ${num(y)}`)
    .join('')
}

/**
 * Smooth path through the points (monotone cubic, Fritsch–Carlson). The curve
 * never overshoots between two points, so it can't dip below zero or rise above
 * a real value; flat runs stay flat. Falls back to straight lines for < 3 points.
 */
export function smoothPath(points: Array<[x: number, y: number]>): string {
  const p = points.filter(([x, y]) => Number.isFinite(x) && Number.isFinite(y))
  if (p.length < 3) return linePath(p)
  const n = p.length
  const dx = p.slice(1).map((q, i) => q[0] - p[i][0])
  const slope = p.slice(1).map((q, i) => (dx[i] === 0 ? 0 : (q[1] - p[i][1]) / dx[i]))
  // Tangents: average of neighbouring slopes, zero at peaks/valleys.
  const m = p.map((_, i) => {
    if (i === 0) return slope[0]
    if (i === n - 1) return slope[n - 2]
    return slope[i - 1] * slope[i] <= 0 ? 0 : (slope[i - 1] + slope[i]) / 2
  })
  // Limit tangents so each segment stays monotone.
  for (let i = 0; i < n - 1; i++) {
    if (slope[i] === 0) { m[i] = 0; m[i + 1] = 0; continue }
    const a = m[i] / slope[i]
    const b = m[i + 1] / slope[i]
    const h = a * a + b * b
    if (h > 9) {
      const tau = 3 / Math.sqrt(h)
      m[i] = tau * a * slope[i]
      m[i + 1] = tau * b * slope[i]
    }
  }
  let d = `M${num(p[0][0])} ${num(p[0][1])}`
  for (let i = 0; i < n - 1; i++) {
    const h = dx[i] / 3
    d += `C${num(p[i][0] + h)} ${num(p[i][1] + m[i] * h)} ${num(p[i + 1][0] - h)} ${num(p[i + 1][1] - m[i + 1] * h)} ${num(p[i + 1][0])} ${num(p[i + 1][1])}`
  }
  return d
}

const pt = (r: number, a: number) => `${num(r * Math.sin(a))} ${num(-r * Math.cos(a))}`

/** Arcs centred on the origin, starting at 12 o'clock, clockwise. Angles in radians. One entry per input value (zero values get an empty path). */
export function pieArcs(
  values: number[],
  radius: number,
  innerRadius = 0,
): Array<{ path: string; start: number; end: number; value: number }> {
  const vals = values.map(v => (Number.isFinite(v) && v > 0 ? v : 0))
  const total = vals.reduce((a, b) => a + b, 0)
  if (total <= 0) return []
  const full = Math.PI * 2
  const inner = Math.min(Math.max(innerRadius, 0), radius)
  const out: Array<{ path: string; start: number; end: number; value: number }> = []
  let acc = 0
  vals.forEach(v => {
    const start = (acc / total) * full
    acc += v
    const end = (acc / total) * full
    if (v === 0) {
      out.push({ path: '', start, end: start, value: 0 })
      return
    }
    let path: string
    if (v === total) {
      // Full circle: two half arcs (a single arc with identical endpoints draws nothing).
      path = `M${pt(radius, 0)}A${radius} ${radius} 0 1 1 ${pt(radius, Math.PI)}A${radius} ${radius} 0 1 1 ${pt(radius, 0)}`
      if (inner > 0) {
        path += `M${pt(inner, 0)}A${inner} ${inner} 0 1 0 ${pt(inner, Math.PI)}A${inner} ${inner} 0 1 0 ${pt(inner, 0)}`
      }
      path += 'Z'
    } else {
      const large = end - start > Math.PI ? 1 : 0
      path = `M${pt(radius, start)}A${radius} ${radius} 0 ${large} 1 ${pt(radius, end)}`
      path += inner > 0
        ? `L${pt(inner, end)}A${inner} ${inner} 0 ${large} 0 ${pt(inner, start)}Z`
        : 'L0 0Z'
    }
    out.push({ path, start, end, value: v })
  })
  return out
}
