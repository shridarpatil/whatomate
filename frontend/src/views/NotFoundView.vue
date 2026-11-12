<script setup lang="ts">
import { Home, ArrowLeft } from '@/lib/icons'
</script>

<template>
  <div class="flex flex-col items-center justify-center h-screen p-4 align-center">
    <h1>404</h1>
    <h2>{{ $t('notFound.title') }}</h2>
    <p class="text-light">{{ $t('notFound.description') }}</p>
    <div class="hstack gap-4 mt-8">
      <button type="button" class="outline" @click="$router.back()">
        <ArrowLeft :size="16" />
        {{ $t('notFound.goBack') }}
      </button>
      <RouterLink to="/" class="button">
        <Home :size="16" />
        {{ $t('notFound.goHome') }}
      </RouterLink>
    </div>
  </div>
</template>
