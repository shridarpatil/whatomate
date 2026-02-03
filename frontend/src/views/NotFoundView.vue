<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Home, ArrowLeft } from 'lucide-vue-next'
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-background p-4">
    <div class="text-center">
      <h1 class="text-9xl font-bold text-muted-foreground/20">404</h1>
      <h2 class="text-2xl font-semibold mt-4">{{ $t('notFound.title') }}</h2>
      <p class="text-muted-foreground mt-2 max-w-md">
        {{ $t('notFound.description') }}
      </p>
      <div class="flex items-center justify-center gap-4 mt-8">
        <Button variant="outline" @click="$router.back()">
          <ArrowLeft class="h-4 w-4 mr-2" />
          {{ $t('notFound.goBack') }}
        </Button>
        <RouterLink to="/">
          <Button>
            <Home class="h-4 w-4 mr-2" />
            {{ $t('notFound.goHome') }}
          </Button>
        </RouterLink>
      </div>
    </div>
  </div>
</template>
