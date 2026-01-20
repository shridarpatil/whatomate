<script setup lang="ts">
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: string
}>()
</script>

<template>
  <div
    :class="cn(
      'rounded-xl border border-white/[0.06] bg-white/[0.02] text-card-foreground backdrop-blur-sm shadow-xl shadow-black/20 transition-all duration-300 hover:border-white/[0.15] hover:bg-white/[0.04] hover:shadow-2xl hover:shadow-emerald-500/5 light:bg-white light:border-gray-200 light:shadow-gray-200/50 light:hover:border-gray-300 light:hover:shadow-lg light:hover:shadow-gray-300/50',
      props.class
    )"
  >
    <slot />
  </div>
</template>
