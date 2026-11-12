<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { SUPPORTED_LOCALES, setLocale } from '@/i18n'

const { locale, t } = useI18n()

const currentLocale = computed({
  get: () => locale.value,
  set: (value: string) => setLocale(value)
})
</script>

<template>
  <select v-model="currentLocale" :aria-label="t('userMenu.language')">
    <option v-for="loc in SUPPORTED_LOCALES" :key="loc.code" :value="loc.code">
      {{ loc.nativeName }}
    </option>
  </select>
</template>
