<script setup lang="ts">
import { ref, computed, nextTick, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Check } from '@/lib/icons'
import { filterOptions, type ComboOption } from './combobox'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  modelValue: string | null
  options: ComboOption[]
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  disabled?: boolean
  id?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const { t } = useI18n()

const uid = useId()
const popoverId = `${uid}-popover`
const listId = `${uid}-list`

const popover = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const search = ref<HTMLInputElement | null>(null)
const query = ref('')

const filtered = computed(() => filterOptions(props.options, query.value))
const selectedLabel = computed(() => props.options.find((o) => o.value === props.modelValue)?.label)

const optionId = (i: number) => `${uid}-opt-${i}`

function focusOption(i: number) {
  document.getElementById(optionId(i))?.focus()
}

function onToggle(e: Event) {
  if ((e as ToggleEvent).newState === 'open') {
    query.value = ''
    nextTick(() => search.value?.focus())
  }
}

function close() {
  popover.value?.hidePopover()
  trigger.value?.focus()
}

function select(opt: ComboOption) {
  emit('update:modelValue', opt.value)
  close()
}

// Focus-based navigation: the search input is position -1, options are 0..n-1.
function onKeydown(e: KeyboardEvent, from: number) {
  const n = filtered.value.length
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
  } else if (e.key === 'ArrowDown' && n) {
    e.preventDefault()
    focusOption(Math.min(from + 1, n - 1))
  } else if (e.key === 'ArrowUp' && n) {
    e.preventDefault()
    if (from <= 0) search.value?.focus()
    else focusOption(from - 1)
  } else if (e.key === 'Enter' && from < 0) {
    e.preventDefault()
    if (filtered.value[0]) select(filtered.value[0])
  }
}
</script>

<template>
  <ot-dropdown>
    <button
      v-bind="$attrs"
      :id="id"
      ref="trigger"
      type="button"
      class="outline"
      :popovertarget="popoverId"
      :disabled="disabled"
      aria-haspopup="listbox"
    >
      <span class="truncate">{{ selectedLabel ?? placeholder ?? t('common.select') }}</span>
      <ChevronDown :size="16" />
    </button>
    <div :id="popoverId" ref="popover" popover @toggle="onToggle">
      <input
        ref="search"
        v-model="query"
        type="search"
        :placeholder="searchPlaceholder ?? t('common.search')"
        :aria-label="searchPlaceholder ?? t('common.search')"
        :aria-controls="listId"
        @keydown="onKeydown($event, -1)"
      />
      <div :id="listId" role="listbox" class="scroll-y">
        <button
          v-for="(opt, i) in filtered"
          :id="optionId(i)"
          :key="opt.value"
          type="button"
          role="option"
          class="ghost"
          tabindex="-1"
          :aria-selected="opt.value === modelValue"
          @click="select(opt)"
          @keydown="onKeydown($event, i)"
        >
          <Check v-if="opt.value === modelValue" :size="16" />
          {{ opt.label }}
          <small v-if="opt.hint" class="text-light">{{ opt.hint }}</small>
        </button>
        <p v-if="!filtered.length" role="status" class="text-light">{{ emptyText ?? t('common.noResults') }}</p>
      </div>
    </div>
  </ot-dropdown>
</template>
