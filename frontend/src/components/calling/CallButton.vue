<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { outgoingCallsService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Phone, Loader2, Clock, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

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
  <Tooltip>
    <TooltipTrigger as-child>
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 relative"
        :class="[
          permission?.status === 'accepted'
            ? 'text-green-500 hover:text-green-400 hover:bg-green-500/10 light:text-green-600 light:hover:text-green-700 light:hover:bg-green-50'
            : 'text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100'
        ]"
        :disabled="store.isOnCall || isInitiating"
        @click="handleCall"
      >
        <Loader2 v-if="isInitiating" class="h-4 w-4 animate-spin" />
        <Phone v-else class="h-4 w-4" />
        <!-- Permission status badge (pending/declined only) -->
        <span
          v-if="permission?.status === 'pending'"
          class="absolute -top-0.5 -right-0.5 flex h-3 w-3 items-center justify-center rounded-full bg-yellow-500 text-white"
        >
          <Clock class="h-2 w-2" />
        </span>
        <span
          v-else-if="permission?.status === 'declined'"
          class="absolute -top-0.5 -right-0.5 flex h-3 w-3 items-center justify-center rounded-full bg-red-500 text-white"
        >
          <X class="h-2 w-2" />
        </span>
      </Button>
    </TooltipTrigger>
    <TooltipContent>{{ tooltipText }}</TooltipContent>
  </Tooltip>
</template>
