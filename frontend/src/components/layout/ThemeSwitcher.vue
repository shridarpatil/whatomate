<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Sun, Moon, Monitor } from '@/lib/icons'
import { useColorMode, type ColorMode } from '@/composables/useColorMode'

const { t } = useI18n()
const { colorMode, setColorMode } = useColorMode()

const modes: { mode: ColorMode; label: string; icon: typeof Sun }[] = [
  { mode: 'light', label: 'userMenu.themeLight', icon: Sun },
  { mode: 'dark', label: 'userMenu.themeDark', icon: Moon },
  { mode: 'system', label: 'userMenu.themeSystem', icon: Monitor }
]
</script>

<template>
  <div class="hstack gap-1" role="radiogroup" :aria-label="t('userMenu.colorTheme')">
    <!-- The active mode is a filled button; the others are ghost. -->
    <button
      v-for="m in modes"
      :key="m.mode"
      type="button"
      role="radio"
      class="small icon"
      :class="{ ghost: colorMode !== m.mode }"
      :aria-checked="colorMode === m.mode"
      :aria-label="t(m.label)"
      :title="t(m.label)"
      @click="setColorMode(m.mode)"
    >
      <component :is="m.icon" :size="14" aria-hidden="true" />
    </button>
  </div>
</template>
