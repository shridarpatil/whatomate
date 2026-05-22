<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { computed } from "vue"
import { Loader2 } from "lucide-vue-next"
import { cn } from "@/lib/utils"

interface SpinnerProps {
  size?: "sm" | "md" | "lg"
  // When true, the spinner fills its nearest positioned ancestor and centers itself.
  overlay?: boolean
  class?: HTMLAttributes["class"]
}

const props = withDefaults(defineProps<SpinnerProps>(), {
  size: "md",
  overlay: false,
})

const sizeClass = computed(() => {
  switch (props.size) {
    case "sm":
      return "h-4 w-4"
    case "lg":
      return "h-8 w-8"
    default:
      return "h-6 w-6"
  }
})
</script>

<template>
  <div
    v-if="overlay"
    class="absolute inset-0 z-20 flex items-center justify-center"
  >
    <Loader2 :class="cn('animate-spin text-white/40 light:text-gray-400', sizeClass, props.class)" />
  </div>
  <Loader2
    v-else
    :class="cn('animate-spin', sizeClass, props.class)"
  />
</template>
