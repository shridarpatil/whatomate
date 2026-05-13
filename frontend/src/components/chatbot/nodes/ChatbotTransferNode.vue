<script setup lang="ts">
import { computed } from 'vue'
import { UserPlus } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const teamLabel = computed(() => {
  const cfg = props.data?.config?.transfer_config
  if (!cfg?.team_id || cfg.team_id === '_general') return 'General Queue'
  // If a team_name was injected, use it
  if (cfg.team_name) return cfg.team_name
  // Fallback: show truncated team ID
  return cfg.team_id.length > 12 ? cfg.team_id.slice(0, 12) + '…' : cfg.team_id
})

const notes = computed(() => {
  return props.data?.config?.transfer_config?.notes || ''
})
</script>

<template>
  <BaseNode :label="data?.label || 'Transfer'" header-class="bg-amber-600" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><UserPlus class="w-4 h-4" /></template>
    <div>
      <p class="font-medium truncate" :title="teamLabel">→ {{ teamLabel }}</p>
      <p v-if="notes" class="truncate text-muted-foreground/70 mt-0.5" :title="notes">{{ notes }}</p>
    </div>
  </BaseNode>
</template>
