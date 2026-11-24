#!/usr/bin/env node
// Verifies that .vue/.ts files only use classes defined by vendored Oat CSS or
// layout.css (or the vue-flow prefix), and none of the removed packages/styles.
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, dirname, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const REMOVED = [
  '@/components/ui/', 'lucide-vue-next', 'vue-sonner', 'axios', '@vueuse/core', 'reka-ui',
  'vaul-vue', 'vee-validate', 'class-variance-authority', 'clsx', 'tailwind-merge', 'chart.js',
  'vue-chartjs', 'grid-layout-plus', '@radix-icons/vue', '@tanstack/vue-query',
  '@internationalized/date', 'vue3-emoji-picker',
]

export function loadAllowed() {
  const files = readdirSync(join(root, 'src/vendor/oat/css'))
    .filter(f => f.endsWith('.css'))
    .map(f => join(root, 'src/vendor/oat/css', f))
  files.push(join(root, 'src/assets/layout.css'))
  const allowed = new Set()
  for (const f of files) {
    const css = readFileSync(f, 'utf8')
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/url\([^)]*\)/g, '')
      .replace(/"[^"]*"|'[^']*'/g, '')
    for (const m of css.matchAll(/\.(-?[_a-zA-Z][\w-]*)/g)) allowed.add(m[1])
  }
  return allowed
}

function classOk(token, allowed) {
  return allowed.has(token) || token.startsWith('vue-flow')
}

export function findViolations(file, source, allowed) {
  const out = []
  const add = reason => out.push(`${file}: ${reason}`)
  const checkTokens = (str, where) => {
    for (const t of str.split(/\s+/)) {
      if (!t || /[${}]/.test(t)) continue
      if (!classOk(t, allowed)) add(`unknown class "${t}" (${where})`)
    }
  }

  for (const m of source.matchAll(/(?<![:\w-])class="([^"]*)"/g)) checkTokens(m[1], 'class')
  for (const m of source.matchAll(/:class="([^"]*)"/g)) {
    for (const s of m[1].matchAll(/'([^']*)'|`([^`]*)`/g)) checkTokens(s[1] ?? s[2], ':class')
  }

  for (const m of source.matchAll(/(?:from|import)\s*\(?\s*['"]([^'"]+)['"]/g)) {
    const spec = m[1]
    const hit = REMOVED.find(p => (p.endsWith('/') ? spec.startsWith(p) : spec === p || spec.startsWith(p + '/')))
    if (hit) add(`removed import "${spec}"`)
  }

  if (/<style[\s>]/.test(source)) add('<style> block')
  if (/(?<![:\w-])style="/.test(source)) add('static style attribute')
  return out
}

function walk(p, acc) {
  const st = statSync(p)
  if (st.isDirectory()) {
    for (const e of readdirSync(p)) walk(join(p, e), acc)
  } else if (/\.(vue|ts)$/.test(p) && !p.includes(`${sep}vendor${sep}`)) {
    acc.push(p)
  }
  return acc
}

function main() {
  const paths = process.argv.slice(2)
  const allowed = loadAllowed()
  const files = paths.flatMap(p => walk(resolve(p), []))
  const all = files.flatMap(f => findViolations(f, readFileSync(f, 'utf8'), allowed))
  if (all.length) {
    console.log(all.join('\n'))
    process.exit(1)
  }
  console.log('oat-check: clean')
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
