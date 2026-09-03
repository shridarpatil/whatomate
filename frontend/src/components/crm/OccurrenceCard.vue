<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import type { Occurrence } from '@/services/api'

defineProps<{ occurrence: Occurrence; disabled?: boolean }>()

// As chaves do i18n são camelCase; não existe `priority_low`.
const PRIORITY_KEY = {
  low: 'occurrences.priorityLow',
  normal: 'occurrences.priorityNormal',
  high: 'occurrences.priorityHigh',
  urgent: 'occurrences.priorityUrgent',
} as const
</script>

<template>
  <div
    data-board-card
    class="rounded-md border border-white/[0.08] light:border-gray-200 bg-white/[0.02] light:bg-white p-3"
    :class="disabled ? 'opacity-50 cursor-wait' : 'cursor-grab'"
  >
    <div class="flex items-center justify-between gap-2">
      <span class="font-mono text-xs text-white/50 light:text-muted-foreground">
        {{ occurrence.protocol_number }}
      </span>
      <!-- Sem cor de etapa aqui: o cabeçalho da coluna já carrega a cor, e
           todos os cartões de uma coluna estão na mesma etapa. Colorir um
           rótulo de prioridade pela etapa não diria nada e ainda sugeriria
           que a cor significa urgência. `normal` é o padrão e fica implícito. -->
      <Badge v-if="occurrence.priority !== 'normal'" variant="outline" class="shrink-0 text-xs">
        {{ $t(PRIORITY_KEY[occurrence.priority]) }}
      </Badge>
    </div>
    <p class="text-sm mt-1 truncate text-white light:text-gray-900">{{ occurrence.title }}</p>
    <p class="text-xs mt-1 truncate text-white/50 light:text-muted-foreground">
      <span class="text-white/30 light:text-gray-400">{{ $t('occurrences.cardContact') }}:</span>
      {{ occurrence.contact_name }}
    </p>
    <p class="text-xs mt-0.5 truncate text-white/40 light:text-muted-foreground">
      <span class="text-white/30 light:text-gray-400">{{ $t('occurrences.cardAssignee') }}:</span>
      {{ occurrence.assigned_user_name || $t('occurrences.unassigned') }}
    </p>
  </div>
</template>
