<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ArrowUp } from '@/lib/icons'

const props = withDefaults(defineProps<{
  target?: string
  threshold?: number
}>(), {
  threshold: 300,
})

const isVisible = ref(false)
let scrollEl: HTMLElement | null = null

function onScroll() {
  if (scrollEl) {
    isVisible.value = scrollEl.scrollTop > props.threshold
  }
}

function scrollToTop() {
  scrollEl?.scrollTo({ top: 0, behavior: 'smooth' })
}

onMounted(() => {
  scrollEl = props.target
    ? document.querySelector(props.target)
    : document.querySelector('main')
  scrollEl?.addEventListener('scroll', onScroll, { passive: true })
})

onUnmounted(() => {
  scrollEl?.removeEventListener('scroll', onScroll)
})
</script>

<template>
  <button
    v-show="isVisible"
    type="button"
    data-variant="secondary"
    class="icon fixed-bottom-end"
    :title="$t('common.scrollToTop')"
    :aria-label="$t('common.scrollToTop')"
    @click="scrollToTop"
  >
    <ArrowUp />
  </button>
</template>
