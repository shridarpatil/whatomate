<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Sun, Moon, Monitor } from 'lucide-vue-next'
import { useColorMode } from '@/composables/useColorMode'

const { colorMode, setColorMode } = useColorMode()
</script>

<template>
  <div class="flex gap-0.5 px-1.5 py-1" role="radiogroup" aria-label="Color theme">
    <Button
      variant="ghost"
      size="icon"
      class="h-7 w-7"
      :class="colorMode === 'light' && 'bg-accent'"
      :aria-checked="colorMode === 'light'"
      aria-label="Light theme"
      role="radio"
      @click="setColorMode('light')"
    >
      <Sun class="h-3.5 w-3.5" aria-hidden="true" />
    </Button>
    <Button
      variant="ghost"
      size="icon"
      class="h-7 w-7"
      :class="colorMode === 'dark' && 'bg-accent'"
      :aria-checked="colorMode === 'dark'"
      aria-label="Dark theme"
      role="radio"
      @click="setColorMode('dark')"
    >
      <Moon class="h-3.5 w-3.5" aria-hidden="true" />
    </Button>
    <Button
      variant="ghost"
      size="icon"
      class="h-7 w-7"
      :class="colorMode === 'system' && 'bg-accent'"
      :aria-checked="colorMode === 'system'"
      aria-label="System theme"
      role="radio"
      @click="setColorMode('system')"
    >
      <Monitor class="h-3.5 w-3.5" aria-hidden="true" />
    </Button>
  </div>
</template>
