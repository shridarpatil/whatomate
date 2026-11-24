<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { outgoingCallsService } from '@/services/api'
import { Phone, Clock, X } from '@/lib/icons'
import { toast } from '@/lib/toast'

const props = defineProps<{
  contactId: string
  contactPhone: string
  contactName: string
  whatsappAccount: string
}>()

const { t } = useI18n()
const store = useCallingStore()
const isInitiating = ref(false)

const permission = computed(() => store.getCallPermission(props.contactId))

// Fetch existing permission status on mount
onMounted(async () => {
  if (permission.value) return // already known
  try {
    const resp = await outgoingCallsService.getPermission(props.contactId, props.whatsappAccount)
    const perm = (resp.data as any).data ?? resp.data
    if (perm.status === 'temporary' || perm.status === 'permanent') {
      store.callPermissions.set(props.contactId, {
        status: 'accepted',
        expiresAt: perm.expires_at,
      })
    } else if (perm.status === 'declined') {
      store.callPermissions.set(props.contactId, { status: 'declined' })
    } else if (perm.status === 'pending') {
      store.callPermissions.set(props.contactId, { status: 'pending' })
    }
  } catch {
    // No permission record — leave as default (no badge)
  }
})

const tooltipText = computed(() => {
  if (store.isOnCall) return t('outgoingCalls.callButtonDisabled')
  if (permission.value?.status === 'pending') return t('outgoingCalls.permissionPending')
  if (permission.value?.status === 'accepted') return t('outgoingCalls.callButton')
  if (permission.value?.status === 'declined') return t('outgoingCalls.permissionDeclined')
  return t('outgoingCalls.callButton')
})

async function handleCall() {
  if (store.isOnCall || isInitiating.value) return

  isInitiating.value = true
  try {
    // If permission was already accepted via WebSocket, call directly
    if (permission.value?.status === 'accepted') {
      await store.makeOutgoingCall(props.contactId, props.contactName, props.whatsappAccount)
      return
    }

    // Check if we already have call permission via WhatsApp API
    let hasPermission = false
    try {
      const resp = await outgoingCallsService.getPermission(props.contactId, props.whatsappAccount)
      const perm = (resp.data as any).data ?? resp.data
      hasPermission = perm.status === 'temporary' || perm.status === 'permanent'
      if (hasPermission) {
        store.callPermissions.set(props.contactId, {
          status: 'accepted',
          expiresAt: perm.expires_at,
        })
      }
    } catch {
      // No permission found
    }

    if (hasPermission) {
      await store.makeOutgoingCall(props.contactId, props.contactName, props.whatsappAccount)
    } else {
      // Send permission request and notify the agent
      await outgoingCallsService.requestPermission({
        contact_id: props.contactId,
        whatsapp_account: props.whatsappAccount,
      })
      store.setCallPermissionPending(props.contactId)
      toast.info(t('outgoingCalls.permissionSent'), {
        description: t('outgoingCalls.permissionSentDesc'),
      })
    }
  } catch (err: any) {
    toast.error(t('outgoingCalls.callFailed'), {
      description: err.message || String(err),
    })
  } finally {
    isInitiating.value = false
  }
}
</script>

<template>
  <button
    type="button"
    class="ghost icon"
    :title="tooltipText"
    :aria-label="tooltipText"
    :disabled="store.isOnCall || isInitiating"
    :aria-busy="isInitiating ? 'true' : undefined"
    data-spinner="small"
    @click="handleCall"
  >
    <Phone v-if="!isInitiating" />
    <!-- Permission status (pending/declined only) -->
    <Clock v-if="permission?.status === 'pending'" :size="12" />
    <X v-else-if="permission?.status === 'declined'" :size="12" />
  </button>
</template>
