<script setup lang="ts">
import { ArrowLeft } from '@/lib/icons'
import type { Component } from 'vue'

defineProps<{
  title: string
  description?: string
  icon?: Component
  // Kept for call-site compatibility; every page uses the classic emerald icon tile.
  iconGradient?: string
  backLink?: string
  // Kept for call-site compatibility; breadcrumbs are intentionally not shown (the sidebar shows location).
  breadcrumbs?: Array<{ label: string; href?: string }>
}>()
</script>

<template>
  <header class="hstack items-center gap-4" data-page-header>
    <RouterLink v-if="backLink" :to="backLink" class="button ghost icon" :title="$t('common.back', 'Back')" :aria-label="$t('common.back', 'Back')">
      <ArrowLeft />
    </RouterLink>
    <span v-if="icon" data-page-icon aria-hidden="true"><component :is="icon" :size="16" /></span>
    <div class="flex-1 min-w-0">
      <h1>{{ title }}</h1>
      <p v-if="description" data-page-description>{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="hstack items-center" data-page-actions>
      <slot name="actions" />
    </div>
  </header>
</template>
