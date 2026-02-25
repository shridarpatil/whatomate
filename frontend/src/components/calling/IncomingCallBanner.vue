<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Phone, PhoneIncoming } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const store = useCallingStore()

async function handleAccept(id: string) {
  try {
    await store.acceptTransfer(id)
    toast.success(t('callTransfers.callConnected'))
  } catch (err: any) {
    toast.error(t('callTransfers.acceptFailed'), {
      description: err.message || ''
    })
  }
}
</script>

<template>
  <div
    v-if="store.waitingTransfers.length > 0"
    class="bg-green-950/80 border-b border-green-800/50 px-4 py-2.5"
  >
    <div class="flex items-center justify-between max-w-screen-xl mx-auto">
      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2 text-green-400">
          <PhoneIncoming class="h-4 w-4 animate-pulse" />
          <span class="text-sm font-medium">{{ t('callTransfers.incomingTransfer') }}</span>
        </div>
        <Badge variant="secondary" class="bg-green-800/50 text-green-300">
          {{ store.waitingTransfers.length }}
        </Badge>
      </div>

      <div class="flex items-center gap-2">
        <div
          v-for="transfer in store.waitingTransfers.slice(0, 3)"
          :key="transfer.id"
          class="flex items-center gap-2 bg-green-900/50 rounded-lg px-3 py-1.5"
        >
          <Phone class="h-3.5 w-3.5 text-green-400" />
          <span class="text-sm text-green-200">
            {{ transfer.contact?.profile_name || transfer.caller_phone }}
          </span>
          <span v-if="transfer.team?.name" class="text-xs text-green-400/70">
            ({{ transfer.team.name }})
          </span>
          <Button
            size="sm"
            variant="default"
            class="h-7 bg-green-600 hover:bg-green-500 text-white text-xs"
            @click="handleAccept(transfer.id)"
          >
            {{ t('callTransfers.accept') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
