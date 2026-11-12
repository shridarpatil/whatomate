<script setup lang="ts">
import PageHeader from './PageHeader.vue'
import ErrorState from './ErrorState.vue'
import type { Component } from 'vue'

defineProps<{
  title: string
  description?: string
  icon?: Component
  iconGradient?: string
  backLink: string
  breadcrumbs?: Array<{ label: string; href?: string }>
  isLoading?: boolean
  isNotFound?: boolean
  notFoundTitle?: string
  notFoundDescription?: string
}>()
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="title"
      :description="description"
      :icon="icon"
      :icon-gradient="iconGradient"
      :back-link="backLink"
      :breadcrumbs="breadcrumbs"
    >
      <template #actions>
        <slot name="actions" />
      </template>
    </PageHeader>

    <!-- Loading -->
    <div
      v-if="isLoading"
      class="flex-1 flex items-center justify-center"
      role="status"
      aria-busy="true"
      data-spinner="large"
    />

    <!-- Not found -->
    <ErrorState
      v-else-if="isNotFound"
      :title="notFoundTitle || 'Not found'"
      :description="notFoundDescription || 'The resource you are looking for does not exist.'"
      class="flex-1"
    />

    <!-- Content -->
    <div v-else class="flex-1 scroll-y p-4">
      <div class="container">
        <div class="row">
          <div class="col-8 vstack gap-6">
            <slot />
          </div>
          <div class="col-4 vstack gap-6">
            <slot name="sidebar" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
