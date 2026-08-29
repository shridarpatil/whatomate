<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { useOccurrencesStore } from '@/stores/occurrences'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus } from 'lucide-vue-next'

const props = defineProps<{
  contactId: string
  sourceTransferId?: string
}>()

const { t } = useI18n()
const store = useOccurrencesStore()
const isCreating = ref(false)
const newTitle = ref('')

async function load() {
  if (!props.contactId) return
  // Stages drive the badge colour and rarely change, so fetch them once per
  // session rather than on every contact switch.
  if (store.stages.length === 0) await store.fetchStages()
  await store.fetchContactOccurrences(props.contactId)
}

async function create() {
  if (!newTitle.value.trim()) return
  try {
    await store.createOccurrence({
      contact_id: props.contactId,
      title: newTitle.value.trim(),
      source_transfer_id: props.sourceTransferId,
    })
    newTitle.value = ''
    isCreating.value = false
  } catch (e) {
    toast.error(getErrorMessage(e, t('chat.occurrenceCreateFailed')))
  }
}

onMounted(load)
watch(() => props.contactId, load)
</script>

<template>
  <div id="occurrences-panel" class="w-80 border-l border-white/[0.08] light:border-gray-200 bg-[#111113] light:bg-white flex flex-col h-full min-h-0">
    <div class="flex items-center justify-between px-4 py-3 border-b border-white/[0.08] light:border-gray-200 shrink-0">
      <h3 class="text-sm font-medium text-white light:text-gray-900">{{ $t('chat.occurrences') }}</h3>
      <Button variant="ghost" size="sm" @click="isCreating = !isCreating">
        <Plus class="h-4 w-4 mr-1" />
        {{ $t('chat.newOccurrence') }}
      </Button>
    </div>

    <div v-if="isCreating" class="p-4 border-b border-white/[0.08] light:border-gray-200 space-y-2 shrink-0">
      <Input
        v-model="newTitle"
        :placeholder="$t('chat.occurrenceTitle')"
        @keyup.enter="create"
      />
      <Button size="sm" class="w-full" :disabled="!newTitle.trim()" @click="create">
        {{ $t('chat.newOccurrence') }}
      </Button>
    </div>

    <ScrollArea orientation="vertical" class="flex-1 min-h-0">
      <div class="p-4 space-y-2">
        <RouterLink
          v-for="occ in store.contactOccurrences"
          :key="occ.id"
          :to="`/crm/occurrences/${occ.id}`"
          class="block p-3 rounded-md border border-white/[0.08] light:border-gray-200 hover:bg-white/[0.04] light:hover:bg-gray-50 transition-colors"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="font-mono text-xs text-white/50 light:text-muted-foreground">{{ occ.protocol_number }}</span>
            <Badge
              variant="outline"
              class="shrink-0 text-xs"
              :style="{ borderColor: store.stageColor(occ.stage_id), color: store.stageColor(occ.stage_id) }"
            >{{ occ.stage_name }}</Badge>
          </div>
          <p class="text-sm mt-1 truncate min-w-0 text-white light:text-gray-900">{{ occ.title }}</p>
        </RouterLink>

        <p
          v-if="store.contactOccurrences.length === 0"
          class="text-sm text-white/40 light:text-muted-foreground text-center py-6"
        >
          {{ $t('chat.noOccurrences') }}
        </p>
      </div>
    </ScrollArea>
  </div>
</template>
