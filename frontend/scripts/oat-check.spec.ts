import { describe, it, expect } from 'vitest'
// @ts-expect-error plain ESM script without types
import { findViolations } from './oat-check.mjs'

describe('oat-check findViolations', () => {
  it('flags unknown static class', () => {
    const v = findViolations('a.vue', '<div class="flex bg-red-500">', new Set(['flex']))
    expect(v).toHaveLength(1)
    expect(v[0]).toContain('bg-red-500')
  })

  it('allows vue-flow prefix', () => {
    expect(findViolations('a.vue', '<div class="vue-flow__node">', new Set())).toEqual([])
  })

  it('checks string literals in :class', () => {
    const v = findViolations('a.vue', `<div :class="ok ? 'badge' : 'text-muted-foreground'">`, new Set(['badge']))
    expect(v).toHaveLength(1)
    expect(v[0]).toContain('text-muted-foreground')
  })

  it('flags removed imports', () => {
    const src = [
      `import B from '@/components/ui/button'`,
      `import { X } from 'lucide-vue-next'`,
      `import { toast } from 'vue-sonner'`,
      `import a from 'axios'`,
      `import { u } from '@vueuse/core'`,
      `import { r } from 'reka-ui'`,
    ].join('\n')
    expect(findViolations('a.ts', src, new Set())).toHaveLength(6)
  })

  it('flags style blocks and static style attrs', () => {
    expect(findViolations('a.vue', '<style scoped>\n</style>\n<div style="color:red">', new Set())).toHaveLength(2)
    expect(findViolations('a.vue', `<div :style="{ height: h + 'px' }">`, new Set())).toEqual([])
  })
})
