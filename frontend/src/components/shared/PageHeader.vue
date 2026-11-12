<script setup lang="ts">
import { ArrowLeft } from '@/lib/icons'
import type { Component } from 'vue'

defineProps<{
  title: string
  description?: string
  icon?: Component
  // Kept for call-site compatibility; Oat's default look has no per-page gradient.
  iconGradient?: string
  backLink?: string
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
      <nav v-if="breadcrumbs?.length" aria-label="Breadcrumb">
        <ol class="unstyled hstack">
          <template v-for="(crumb, index) in breadcrumbs" :key="index">
            <li>
              <RouterLink v-if="crumb.href" :to="crumb.href" class="unstyled">{{ crumb.label }}</RouterLink>
              <span v-else aria-current="page">{{ crumb.label }}</span>
            </li>
            <li v-if="index < breadcrumbs.length - 1" aria-hidden="true">/</li>
          </template>
        </ol>
      </nav>
      <p v-else-if="description" class="text-light">{{ description }}</p>
    </div>
    <slot name="actions" />
  </header>
</template>
