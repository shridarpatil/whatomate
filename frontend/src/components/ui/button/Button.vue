<script setup lang="ts">
import type { PrimitiveProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import type { ButtonVariants } from "."
import { Primitive } from "reka-ui"
import { cn } from "@/lib/utils"
import { buttonVariants } from "."
import { Loader2 } from "lucide-vue-next"

interface Props extends PrimitiveProps {
  variant?: ButtonVariants["variant"]
  size?: ButtonVariants["size"]
  class?: HTMLAttributes["class"]
  loading?: boolean
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  as: "button",
  loading: false,
  disabled: false,
})
</script>

<template>
  <Primitive
    :as="as"
    :as-child="asChild"
    :disabled="props.disabled || props.loading"
    :aria-disabled="props.disabled || props.loading"
    :aria-busy="props.loading"
    :class="cn(buttonVariants({ variant, size }), props.class)"
  >
    <Loader2 v-if="props.loading" class="h-4 w-4 animate-spin" />
    <slot v-if="!props.loading" />
    <span v-else class="opacity-0"><slot /></span>
  </Primitive>
</template>
