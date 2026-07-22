<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@/lib/utils"

const props = defineProps<{
  class?: HTMLAttributes["class"]
}>()
</script>

<template>
  <tr :class="cn('border-b transition-colors duration-150 hover:bg-white/[0.03] even:bg-white/[0.01] data-[state=selected]:bg-muted light:hover:bg-gray-50 light:even:bg-gray-50/50', props.class)">
    <slot />
  </tr>
</template>
