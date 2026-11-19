<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, useId } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useContactsStore } from '@/stores/contacts'
import { usersService, chatbotService } from '@/services/api'
import Modal from '@/components/oat/Modal.vue'
import { LogOut, User } from '@/lib/icons'
import { toast } from '@/lib/toast'
import { getInitials } from '@/lib/utils'
import ThemeSwitcher from './ThemeSwitcher.vue'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'

const { t } = useI18n()

const emit = defineEmits<{
  logout: []
}>()

const authStore = useAuthStore()
const contactsStore = useContactsStore()
const menuId = useId()
const menu = ref<HTMLElement | null>(null)
const isUpdatingAvailability = ref(false)
const isCheckingTransfers = ref(false)
const showAwayWarning = ref(false)
const awayWarningTransferCount = ref(0)

const handleAvailabilityChange = async (e: Event) => {
  const input = e.target as HTMLInputElement
  await changeAvailability(input.checked)
  // The switch is DOM-controlled; snap it back if the change was cancelled or failed.
  input.checked = authStore.isAvailable
}

const changeAvailability = async (checked: boolean) => {
  if (!checked) {
    isCheckingTransfers.value = true
    try {
      const response = await chatbotService.listTransfers({ status: 'active' })
      const data = response.data.data || response.data
      const transfers = data.transfers || []
      const userId = authStore.user?.id
      const myActiveTransfers = transfers.filter((t: any) => t.agent_id === userId)

      if (myActiveTransfers.length > 0) {
        awayWarningTransferCount.value = myActiveTransfers.length
        showAwayWarning.value = true
        return
      }
    } catch (error) {
      console.error('Failed to check transfers:', error)
    } finally {
      isCheckingTransfers.value = false
    }
  }

  await setAvailability(checked)
}

const confirmGoAway = async () => {
  showAwayWarning.value = false
  await setAvailability(false)
}

const setAvailability = async (checked: boolean) => {
  isUpdatingAvailability.value = true
  try {
    const response = await usersService.updateAvailability(checked)
    const data = response.data.data
    authStore.setAvailability(checked, data.break_started_at)

    if (checked) {
      toast.success(t('userMenu.available'), {
        description: t('userMenu.availableDesc')
      })
    } else {
      const transfersReturned = data.transfers_to_queue || 0
      toast.success(t('userMenu.away'), {
        description: transfersReturned > 0
          ? t('userMenu.transfersReturned', { count: transfersReturned })
          : t('userMenu.awayDesc')
      })

      if (transfersReturned > 0) {
        contactsStore.fetchContacts()
      }
    }
  } catch (error) {
    toast.error(t('common.error'), {
      description: t('userMenu.failedUpdateAvailability')
    })
  } finally {
    isUpdatingAvailability.value = false
  }
}

// Break duration tracking
const breakDuration = ref('')
let breakTimerInterval: ReturnType<typeof setInterval> | null = null

const updateBreakDuration = () => {
  if (!authStore.breakStartedAt) {
    breakDuration.value = ''
    return
  }
  const start = new Date(authStore.breakStartedAt)
  const now = new Date()
  const diffMs = now.getTime() - start.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const hours = Math.floor(diffMins / 60)
  const mins = diffMins % 60

  if (hours > 0) {
    breakDuration.value = `${hours}h ${mins}m`
  } else {
    breakDuration.value = `${mins}m`
  }
}

const stopBreakTimer = () => {
  if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
    breakTimerInterval = null
  }
}

const startBreakTimer = () => {
  stopBreakTimer()
  updateBreakDuration()
  breakTimerInterval = setInterval(updateBreakDuration, 60000)
}

watch(() => authStore.isAvailable, (available) => {
  if (!available && authStore.breakStartedAt) {
    startBreakTimer()
  } else if (breakTimerInterval) {
    stopBreakTimer()
    breakDuration.value = ''
  }
}, { immediate: true })

onMounted(() => {
  authStore.restoreBreakTime()
  if (!authStore.isAvailable && authStore.breakStartedAt) startBreakTimer()
})

onUnmounted(stopBreakTimer)

const handleLogout = () => {
  menu.value?.hidePopover()
  emit('logout')
}

const closeMenu = () => menu.value?.hidePopover()
</script>

<template>
  <ot-dropdown>
    <button type="button" class="ghost w-100" :popovertarget="menuId" :aria-label="$t('userMenu.userMenu')">
      <figure data-variant="avatar" class="small">
        <abbr>{{ getInitials(authStore.user?.full_name || 'U') }}</abbr>
      </figure>
      <span class="vstack min-w-0">
        <span class="truncate">{{ authStore.user?.full_name }}</span>
        <small class="truncate text-light">{{ authStore.user?.email }}</small>
      </span>
    </button>
    <div :id="menuId" ref="menu" popover>
      <div class="vstack gap-2 p-4">
        <small class="text-light">{{ $t('userMenu.myAccount') }}</small>
        <div class="hstack justify-between gap-4">
          <span class="hstack gap-2">
            <span>{{ $t('userMenu.status') }}</span>
            <span class="badge" :data-variant="authStore.isAvailable ? 'success' : 'secondary'">
              {{ authStore.isAvailable ? $t('userMenu.available') : $t('userMenu.away') }}
            </span>
            <small v-if="!authStore.isAvailable && breakDuration" class="text-light">{{ breakDuration }}</small>
          </span>
          <input
            type="checkbox"
            role="switch"
            :checked="authStore.isAvailable"
            :disabled="isUpdatingAvailability || isCheckingTransfers"
            :aria-label="$t('userMenu.toggleAvailability')"
            @change="handleAvailabilityChange"
          />
        </div>
      </div>
      <hr>
      <RouterLink to="/profile" role="menuitem" class="unstyled" @click="closeMenu">
        <User :size="14" aria-hidden="true" />
        <span>{{ $t('userMenu.profile') }}</span>
      </RouterLink>
      <hr>
      <div class="vstack gap-2 p-4">
        <small class="text-light">{{ $t('userMenu.theme') }}</small>
        <ThemeSwitcher />
        <small class="text-light">{{ $t('userMenu.language') }}</small>
        <LanguageSwitcher />
      </div>
      <hr>
      <button type="button" role="menuitem" class="ghost" @click="handleLogout">
        <LogOut :size="14" aria-hidden="true" />
        <span>{{ $t('userMenu.logOut') }}</span>
      </button>
    </div>
  </ot-dropdown>

  <!-- Away Warning Dialog -->
  <Modal
    alert
    :open="showAwayWarning"
    :title="$t('userMenu.awayWarningTitle')"
    :description="$t('userMenu.awayWarningDesc', { count: awayWarningTransferCount })"
    @update:open="showAwayWarning = $event"
  >
    <template #footer>
      <button type="button" class="outline" @click="showAwayWarning = false">{{ $t('common.cancel') }}</button>
      <button type="button" :disabled="isUpdatingAvailability" @click="confirmGoAway">{{ $t('userMenu.goAway') }}</button>
    </template>
  </Modal>
</template>
