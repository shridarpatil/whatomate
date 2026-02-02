<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { SUPPORTED_LOCALES, setLocale } from '@/i18n'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Globe } from 'lucide-vue-next'

const { locale } = useI18n()

const currentLocale = computed({
  get: () => locale.value,
  set: (value: string) => setLocale(value)
})

const currentLocaleName = computed(() => {
  const found = SUPPORTED_LOCALES.find(l => l.code === locale.value)
  return found?.nativeName || locale.value
})
</script>

<template>
  <Select v-model="currentLocale">
    <SelectTrigger class="w-auto gap-2">
      <Globe class="h-4 w-4" />
      <SelectValue>{{ currentLocaleName }}</SelectValue>
    </SelectTrigger>
    <SelectContent>
      <SelectItem
        v-for="loc in SUPPORTED_LOCALES"
        :key="loc.code"
        :value="loc.code"
      >
        {{ loc.nativeName }}
      </SelectItem>
    </SelectContent>
  </Select>
</template>
