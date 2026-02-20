<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
import { cva, type VariantProps } from 'class-variance-authority'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        // Modern opacity-based badges like Linear/Vercel
        default: 'border-transparent bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700',
        secondary: 'border-transparent bg-white/[0.08] text-white/70 light:bg-gray-100 light:text-gray-700',
        destructive: 'border-transparent bg-red-500/20 text-red-400 light:bg-red-100 light:text-red-700',
        outline: 'border-white/20 text-white/70 light:border-gray-200 light:text-gray-700',
        // Status badges with glow effect
        success: 'border-transparent bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700',
        warning: 'border-transparent bg-amber-500/20 text-amber-400 light:bg-amber-100 light:text-amber-700',
        info: 'border-transparent bg-blue-500/20 text-blue-400 light:bg-blue-100 light:text-blue-700'
      }
    },
    defaultVariants: {
      variant: 'default'
    }
  }
)

type BadgeVariants = VariantProps<typeof badgeVariants>

const props = defineProps<{
  variant?: BadgeVariants['variant']
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div :class="cn(badgeVariants({ variant }), props.class)">
    <slot />
  </div>
</template>
