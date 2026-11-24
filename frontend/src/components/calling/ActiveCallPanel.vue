<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { Phone, PhoneOff, PhoneIncoming, Mic, MicOff, ArrowRightLeft, Pause, Play } from '@/lib/icons'
import CallTransferPicker from '@/components/calling/CallTransferPicker.vue'
import { toast } from '@/lib/toast'

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
  if (store.isOnCall && store.isOnHold) {
    return t('calling.onHold')
  }
  if (store.isOnCall) {
    return t('callTransfers.callConnected')
  }
  if (store.isConnecting) {
    return `${t('callTransfers.connecting')}...`
  }
  if (store.waitingTransfers.length > 0) {
    return t('callTransfers.incomingTransfer')
  }
  return ''
})

const showPanel = computed(() => store.isOnCall || store.isConnecting || store.waitingTransfers.length > 0)

// The first waiting transfer (for single-panel accept button)
const firstWaiting = computed(() => store.waitingTransfers[0] ?? null)

async function handleAccept(id: string) {
  acceptingId.value = id
  try {
    await store.acceptTransfer(id)
    toast.success(t('callTransfers.callConnected'))
  } catch (err: any) {
    const serverMsg = err.response?.data?.message || err.message || ''
    toast.error(t('callTransfers.acceptFailed'), {
      description: serverMsg
    })
  } finally {
    acceptingId.value = null
  }
}
</script>

<template>
  <Teleport to="body">
    <article v-if="showPanel" class="card fixed-bottom-end">
      <!-- Caller info -->
      <div class="hstack items-center gap-2 mb-2">
        <PhoneIncoming v-if="!store.isOnCall && firstWaiting" :size="20" />
        <Phone v-else :size="20" :aria-busy="store.isConnecting ? 'true' : undefined" />
        <div role="status">
          <strong>{{ displayName }}</strong>
          <p class="text-light"><small>{{ statusText }}</small></p>
        </div>
      </div>

      <!-- Timer (only when on active call) -->
      <div v-if="store.isOnCall" class="align-center mb-2">
        <strong>{{ formattedDuration }}</strong>
      </div>

      <!-- Call controls -->
      <div class="hstack items-center justify-center gap-2">
        <!-- Mute (only when on call) -->
        <button
          v-if="store.isOnCall"
          type="button"
          class="icon"
          :data-variant="store.isMuted ? 'danger' : 'secondary'"
          :title="store.isMuted ? t('callTransfers.unmute') : t('callTransfers.mute')"
          :aria-label="store.isMuted ? t('callTransfers.unmute') : t('callTransfers.mute')"
          :aria-pressed="store.isMuted"
          @click="store.toggleMute()"
        >
          <MicOff v-if="store.isMuted" />
          <Mic v-else />
        </button>

        <!-- Hold/Resume (only when on active call) -->
        <button
          v-if="store.isOnCall"
          type="button"
          class="icon"
          :data-variant="store.isOnHold ? undefined : 'secondary'"
          :title="store.isOnHold ? t('calling.resume') : t('calling.hold')"
          :aria-label="store.isOnHold ? t('calling.resume') : t('calling.hold')"
          :aria-pressed="store.isOnHold"
          @click="store.isOnHold ? store.resumeCall() : store.holdCall()"
        >
          <Play v-if="store.isOnHold" />
          <Pause v-else />
        </button>

        <!-- Transfer (only when on active call) -->
        <button
          v-if="store.isOnCall"
          type="button"
          class="icon"
          data-variant="secondary"
          :title="t('callTransfers.transfer')"
          :aria-label="t('callTransfers.transfer')"
          @click="showTransferPicker = true"
        >
          <ArrowRightLeft />
        </button>

        <!-- Accept incoming transfer -->
        <button
          v-if="firstWaiting"
          type="button"
          class="icon"
          :title="t('callTransfers.accept')"
          :aria-label="t('callTransfers.accept')"
          :disabled="acceptingId === firstWaiting.id"
          @click="handleAccept(firstWaiting.id)"
        >
          <Phone />
        </button>

        <!-- Hangup / Decline -->
        <button
          v-if="store.isOnCall || store.isConnecting"
          type="button"
          class="icon"
          data-variant="danger"
          :title="t('callTransfers.hangup')"
          :aria-label="t('callTransfers.hangup')"
          @click="store.endCall()"
        >
          <PhoneOff />
        </button>
      </div>
    </article>

    <CallTransferPicker v-if="showTransferPicker" @close="showTransferPicker = false" />
  </Teleport>
</template>
