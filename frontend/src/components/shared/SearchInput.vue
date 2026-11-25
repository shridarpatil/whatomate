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
  <!-- Icon sits inside the field (layout.css .search-field). -->
  <div class="search-field">
    <Search :size="16" aria-hidden="true" class="text-light" />
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
      <X :size="14" />
    </button>
  </div>
</template>
