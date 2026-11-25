<script setup lang="ts">
import { ref, useId } from 'vue'
import { MoreHorizontal } from '@/lib/icons'

/**
 * The "more actions" menu at the end of a list row. Put secondary and
 * destructive actions here as `<button role="menuitem" class="ghost">`;
 * keep the row's one main action visible beside it.
 */
withDefaults(defineProps<{ label?: string }>(), { label: undefined })

const id = `row-menu-${useId()}`
const menu = ref<HTMLElement | null>(null)

/** Close the popover (call after an item runs). */
function close() {
  menu.value?.hidePopover?.()
}
defineExpose({ close })
</script>

<template>
  <ot-dropdown>
    <button
      type="button"
      class="ghost icon small"
      :popovertarget="id"
      aria-haspopup="menu"
      :title="label ?? $t('common.moreActions')"
      :aria-label="label ?? $t('common.moreActions')"
    >
      <MoreHorizontal :size="16" />
    </button>
    <div :id="id" ref="menu" popover data-row-menu role="menu" @click="close">
      <slot />
    </div>
  </ot-dropdown>
</template>
