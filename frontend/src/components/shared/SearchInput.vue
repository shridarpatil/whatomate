<script setup lang="ts">
import { Search, X } from '@/lib/icons'

const model = defineModel<string>({ default: '' })

withDefaults(defineProps<{
  placeholder?: string
  // Kept for call-site compatibility (Tailwind width classes no longer apply).
  class?: string
}>(), {
  placeholder: 'Search...',
})

function clear() {
  model.value = ''
}
</script>

<template>
  <div class="hstack gap-1">
    <Search />
    <input
      v-model="model"
      type="text"
      role="searchbox"
      :placeholder="placeholder"
      :aria-label="placeholder"
    >
    <button
      v-if="model"
      type="button"
      class="ghost small icon"
      :title="$t('common.clear')"
      :aria-label="$t('common.clear')"
      @click="clear"
    >
      <X />
    </button>
  </div>
</template>
