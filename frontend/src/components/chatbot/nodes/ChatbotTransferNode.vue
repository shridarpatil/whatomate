<script setup lang="ts">
import { computed } from 'vue'
import { UserPlus } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const teamLabel = computed(() => {
  const cfg = props.data?.config || {}
  const teamId = cfg.team_id || cfg.transfer_config?.team_id
  const teamName = cfg.team_name || cfg.transfer_config?.team_name
  if (!teamId || teamId === '_general') return 'General Queue'
  if (teamName) return teamName
  return teamId.length > 12 ? teamId.slice(0, 12) + '…' : teamId
})

const notes = computed(() => {
  const cfg = props.data?.config || {}
  return cfg.notes || cfg.transfer_config?.notes || ''
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
