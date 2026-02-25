<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { Button } from '@/components/ui/button'
import { Phone, PhoneOff, Mic, MicOff } from 'lucide-vue-next'

const { t } = useI18n()
const store = useCallingStore()

const formattedDuration = computed(() => {
  const m = Math.floor(store.callDuration / 60)
  const s = store.callDuration % 60
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
})

const displayName = computed(() => {
  if (store.isOutgoingCall) {
    return store.outgoingContactName || store.outgoingContactPhone || 'Unknown'
  }
  return store.activeTransfer?.contact?.profile_name || store.activeTransfer?.caller_phone || 'Unknown'
})

const statusText = computed(() => {
  if (store.isOutgoingCall) {
    switch (store.outgoingCallStatus) {
      case 'initiating': return `${t('outgoingCalls.initiating')}...`
      case 'ringing': return `${t('outgoingCalls.ringing')}...`
      case 'answered': return t('outgoingCalls.answered')
      default: return ''
    }
  }
  return t('callTransfers.callConnected')
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="store.isOnCall"
      class="fixed bottom-6 right-6 z-50 bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl p-4 min-w-[260px]"
    >
      <div class="flex items-center gap-3 mb-3">
        <div class="w-8 h-8 rounded-full bg-green-600/20 flex items-center justify-center">
          <Phone class="h-4 w-4 text-green-400" />
        </div>
        <div>
          <p class="text-sm font-medium text-zinc-100">
            {{ displayName }}
          </p>
          <p class="text-xs text-zinc-400">{{ statusText }}</p>
        </div>
      </div>

      <div class="text-center mb-3">
        <span class="text-2xl font-mono text-zinc-200">{{ formattedDuration }}</span>
      </div>

      <div class="flex items-center justify-center gap-3">
        <Button
          size="sm"
          variant="outline"
          class="h-10 w-10 rounded-full p-0"
          :class="store.isMuted ? 'bg-red-900/30 border-red-700' : 'border-zinc-600'"
          @click="store.toggleMute()"
        >
          <MicOff v-if="store.isMuted" class="h-4 w-4 text-red-400" />
          <Mic v-else class="h-4 w-4 text-zinc-300" />
        </Button>

        <Button
          size="sm"
          class="h-10 w-10 rounded-full p-0 bg-red-600 hover:bg-red-500"
          @click="store.endCall()"
        >
          <PhoneOff class="h-4 w-4 text-white" />
        </Button>
      </div>
    </div>
  </Teleport>
</template>
