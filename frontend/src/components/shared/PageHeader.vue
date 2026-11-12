<script setup lang="ts">
import { ArrowLeft } from '@/lib/icons'
import type { Component } from 'vue'

defineProps<{
  title: string
  // Kept for call-site compatibility; page subtitles are intentionally not shown (compact headers).
  description?: string
  icon?: Component
  // Kept for call-site compatibility; Oat's default look has no per-page gradient.
  iconGradient?: string
  backLink?: string
  // Kept for call-site compatibility; breadcrumbs are intentionally not shown (the sidebar shows location).
  breadcrumbs?: Array<{ label: string; href?: string }>
}>()
</script>

<template>
  <header class="hstack items-center gap-4 p-4">
    <RouterLink v-if="backLink" :to="backLink" class="button ghost icon" :title="$t('common.back', 'Back')" :aria-label="$t('common.back', 'Back')">
      <ArrowLeft />
    </RouterLink>
    <component :is="icon" v-if="icon" :size="20" />
    <div class="hstack gap-1 flex-1">
      <h1 class="w-100">{{ title }}</h1>
    </div>
    <slot name="actions" />
  </header>
</template>
