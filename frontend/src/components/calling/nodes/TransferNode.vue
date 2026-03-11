<script setup lang="ts">
import { computed } from 'vue'
import { Users } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'
import { useTeamsStore } from '@/stores/teams'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any> }>()
const teamsStore = useTeamsStore()

const summary = computed(() => {
  const teamId = props.data?.config?.team_id
  if (!teamId) return 'No team'
  const team = teamsStore.teams.find(t => t.id === teamId)
  return team?.name || 'No team'
})
</script>

<template>
  <BaseNode :label="data?.label || 'Transfer'" header-class="bg-amber-600" :output-handles="[{ id: 'completed', label: 'completed' }, { id: 'no_answer', label: 'no answer' }]" :has-input="!data?.isEntryNode">
    <template #icon><Users class="w-3 h-3" /></template>
    <p class="truncate">{{ summary }}</p>
  </BaseNode>
</template>
