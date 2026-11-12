<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { X } from '@/lib/icons'

const props = defineProps<{
  open: boolean
  title?: string
  description?: string
  alert?: boolean
  size?: 'small' | 'large'
}>()

const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const { t } = useI18n()

const dialog = ref<HTMLDialogElement | null>(null)

const widths = { small: 'min(100% - 2rem, 24rem)', large: 'min(100% - 2rem, 48rem)' }

function sync() {
  const el = dialog.value
  if (!el) return
  if (props.open && !el.open) el.showModal()
  else if (!props.open && el.open) el.close()
}

onMounted(sync)
watch(() => props.open, sync)

// Native close/cancel (Escape) fire outside Vue's state; mirror them back to the parent.
function onNativeClose() {
  if (props.open) emit('update:open', false)
}

// The dialog has no padding, so a click whose target is the dialog itself hit the backdrop.
function onClick(e: MouseEvent) {
  if (props.alert || e.target !== dialog.value) return
  emit('update:open', false)
}
</script>

<template>
  <dialog
    ref="dialog"
    :role="alert ? 'alertdialog' : undefined"
    :aria-label="title"
    :style="size ? { width: widths[size] } : undefined"
    @cancel="onNativeClose"
    @close="onNativeClose"
    @click="onClick"
  >
    <header v-if="title || description || !alert">
      <div class="hstack justify-between">
        <h3 v-if="title">{{ title }}</h3>
        <span v-else />
        <button
          v-if="!alert"
          type="button"
          class="ghost small icon"
          :aria-label="t('common.close')"
          @click="emit('update:open', false)"
        >
          <X :size="16" />
        </button>
      </div>
      <p v-if="description">{{ description }}</p>
    </header>
    <div>
      <slot />
    </div>
    <footer v-if="$slots.footer">
      <slot name="footer" />
    </footer>
  </dialog>
</template>
