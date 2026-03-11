<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { Button } from '@/components/ui/button'
import { Phone, PhoneOff, PhoneIncoming, Mic, MicOff, ArrowRightLeft } from 'lucide-vue-next'
import CallTransferPicker from '@/components/calling/CallTransferPicker.vue'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const store = useCallingStore()
const acceptingId = ref<string | null>(null)
const showTransferPicker = ref(false)

const formattedDuration = computed(() => {
  const m = Math.floor(store.callDuration / 60)
  const s = store.callDuration % 60
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
})

const displayName = computed(() => {
  if (store.isOutgoingCall) {
    return store.outgoingContactName || store.outgoingContactPhone || 'Unknown'
  }
  if (store.activeTransfer) {
    return store.activeTransfer.contact?.profile_name || store.activeTransfer.caller_phone || 'Unknown'
  }
  // Show first waiting transfer if not on call yet
  if (store.waitingTransfers.length > 0) {
    const t = store.waitingTransfers[0]
    return t.contact?.profile_name || t.caller_phone || 'Unknown'
  }
  return 'Unknown'
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
  if (store.isOnCall) {
    return t('callTransfers.callConnected')
  }
  if (store.waitingTransfers.length > 0) {
    return t('callTransfers.incomingTransfer')
  }
  return ''
})

const showPanel = computed(() => store.isOnCall || store.waitingTransfers.length > 0)

// The first waiting transfer (for single-panel accept button)
const firstWaiting = computed(() => store.waitingTransfers[0] ?? null)

async function handleAccept(id: string) {
  acceptingId.value = id
  try {
    await store.acceptTransfer(id)
    toast.success(t('callTransfers.callConnected'))
  } catch (err: any) {
    toast.error(t('callTransfers.acceptFailed'), {
      description: err.message || ''
    })
  } finally {
    acceptingId.value = null
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="showPanel"
      class="fixed bottom-6 right-6 z-50 bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl p-4 min-w-[260px]"
    >
      <!-- Caller info -->
      <div class="flex items-center gap-3 mb-3">
        <div class="w-8 h-8 rounded-full flex items-center justify-center"
          :class="store.isOnCall ? 'bg-green-600/20' : 'bg-green-600/20'"
        >
          <PhoneIncoming v-if="!store.isOnCall && firstWaiting" class="h-4 w-4 text-green-400 animate-pulse" />
          <Phone v-else class="h-4 w-4 text-green-400" />
        </div>
        <div>
          <p class="text-sm font-medium text-zinc-100">
            {{ displayName }}
          </p>
          <p class="text-xs text-zinc-400">{{ statusText }}</p>
        </div>
      </div>

      <!-- Timer (only when on active call) -->
      <div v-if="store.isOnCall" class="text-center mb-3">
        <span class="text-2xl font-mono text-zinc-200">{{ formattedDuration }}</span>
      </div>

      <!-- Call controls -->
      <div class="flex items-center justify-center gap-3">
        <!-- Mute (only when on call) -->
        <Button
          v-if="store.isOnCall"
          size="sm"
          variant="ghost"
          class="h-10 w-10 rounded-full p-0 border !text-zinc-300"
          :class="store.isMuted ? '!bg-red-900/30 !border-red-700 hover:!bg-red-900/50' : '!bg-zinc-800 !border-zinc-600 hover:!bg-zinc-700'"
          @click="store.toggleMute()"
        >
          <MicOff v-if="store.isMuted" class="h-4 w-4 !text-red-400" />
          <Mic v-else class="h-4 w-4" />
        </Button>

        <!-- Transfer (only when on active call) -->
        <Button
          v-if="store.isOnCall"
          size="sm"
          variant="ghost"
          class="h-10 w-10 rounded-full p-0 border !bg-zinc-800 !border-zinc-600 !text-zinc-300 hover:!bg-zinc-700"
          :title="t('callTransfers.transfer')"
          @click="showTransferPicker = true"
        >
          <ArrowRightLeft class="h-4 w-4" />
        </Button>

        <!-- Accept incoming transfer (green) -->
        <Button
          v-if="firstWaiting"
          variant="ghost"
          size="sm"
          class="h-10 w-10 rounded-full p-0 !bg-green-600 !text-white hover:!bg-green-500"
          :disabled="acceptingId === firstWaiting.id"
          @click="handleAccept(firstWaiting.id)"
        >
          <Phone class="h-4 w-4" />
        </Button>

        <!-- Hangup / Decline (red) -->
        <Button
          v-if="store.isOnCall"
          variant="ghost"
          size="sm"
          class="h-10 w-10 rounded-full p-0 !bg-red-600 !text-white hover:!bg-red-500"
          @click="store.endCall()"
        >
          <PhoneOff class="h-4 w-4" />
        </Button>
      </div>
    </div>

    <CallTransferPicker v-if="showTransferPicker" @close="showTransferPicker = false" />
  </Teleport>
</template>
