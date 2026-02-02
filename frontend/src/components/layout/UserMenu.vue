<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useContactsStore } from '@/stores/contacts'
import { usersService, chatbotService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@/components/ui/popover'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { LogOut, User } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { getInitials } from '@/lib/utils'
import ThemeSwitcher from './ThemeSwitcher.vue'

const { t } = useI18n()

defineProps<{
  collapsed?: boolean
}>()

const emit = defineEmits<{
  logout: []
}>()

const authStore = useAuthStore()
const contactsStore = useContactsStore()
const isUserMenuOpen = ref(false)
const isUpdatingAvailability = ref(false)
const isCheckingTransfers = ref(false)
const showAwayWarning = ref(false)
const awayWarningTransferCount = ref(0)

const handleAvailabilityChange = async (checked: boolean) => {
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

watch(() => authStore.isAvailable, (available) => {
  if (!available && authStore.breakStartedAt) {
    updateBreakDuration()
    breakTimerInterval = setInterval(updateBreakDuration, 60000)
  } else if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
    breakTimerInterval = null
    breakDuration.value = ''
  }
}, { immediate: true })

onMounted(() => {
  authStore.restoreBreakTime()
  if (!authStore.isAvailable && authStore.breakStartedAt) {
    updateBreakDuration()
    breakTimerInterval = setInterval(updateBreakDuration, 60000)
  }
})

onUnmounted(() => {
  if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
  }
})

const handleLogout = () => {
  emit('logout')
}
</script>

<template>
  <div class="border-t border-white/[0.08] light:border-gray-200 p-2">
    <Popover v-model:open="isUserMenuOpen">
      <PopoverTrigger as-child>
        <Button
          variant="ghost"
          :class="[
            'flex items-center justify-start w-full h-auto px-2 py-1.5 gap-2 hover:bg-white/[0.04] light:hover:bg-gray-100',
            collapsed && 'md:justify-center'
          ]"
          aria-label="User menu"
        >
          <Avatar class="h-7 w-7 ring-2 ring-white/[0.1] light:ring-gray-200">
            <AvatarImage :src="undefined" />
            <AvatarFallback class="text-xs bg-gradient-to-br from-emerald-500 to-green-600 text-white">
              {{ getInitials(authStore.user?.full_name || 'U') }}
            </AvatarFallback>
          </Avatar>
          <div v-if="!collapsed" class="flex flex-col items-start text-left">
            <span class="text-[13px] font-medium truncate max-w-[140px] text-white light:text-gray-900">
              {{ authStore.user?.full_name }}
            </span>
            <span class="text-[11px] text-white/40 light:text-gray-500 truncate max-w-[140px]">
              {{ authStore.user?.email }}
            </span>
          </div>
        </Button>
      </PopoverTrigger>
      <PopoverContent side="top" align="start" class="w-52 p-1.5 bg-[#141414] light:bg-white border-white/[0.08] light:border-gray-200">
        <div class="text-xs font-medium px-2 py-1 text-white/40 light:text-gray-500">{{ $t('userMenu.myAccount') }}</div>
        <Separator class="my-1 bg-white/[0.08] light:bg-gray-200" />
        <!-- Availability Toggle -->
        <div class="flex items-center justify-between px-2 py-1.5">
          <div class="flex items-center gap-2">
            <span class="text-[13px] text-white/70 light:text-gray-700">{{ $t('userMenu.status') }}</span>
            <Badge
              :class="'text-[10px] px-1.5 py-0 ' + (authStore.isAvailable
                  ? 'bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700'
                  : 'bg-white/[0.08] text-white/50 light:bg-gray-100 light:text-gray-500')"
            >
              {{ authStore.isAvailable ? $t('userMenu.available') : $t('userMenu.away') }}
            </Badge>
            <span v-if="!authStore.isAvailable && breakDuration" class="text-[10px] text-white/40 light:text-gray-400">
              {{ breakDuration }}
            </span>
          </div>
          <Switch
            :checked="authStore.isAvailable"
            :disabled="isUpdatingAvailability || isCheckingTransfers"
            aria-label="Toggle availability status"
            @update:checked="handleAvailabilityChange"
          />
        </div>
        <Separator class="my-1 bg-white/[0.08] light:bg-gray-200" />
        <RouterLink to="/profile">
          <Button
            variant="ghost"
            class="w-full justify-start px-2 py-1 h-auto text-[13px] font-normal text-white/70 hover:text-white hover:bg-white/[0.04] light:text-gray-600 light:hover:text-gray-900 light:hover:bg-gray-100"
            @click="isUserMenuOpen = false"
          >
            <User class="mr-2 h-3.5 w-3.5" aria-hidden="true" />
            <span>{{ $t('userMenu.profile') }}</span>
          </Button>
        </RouterLink>
        <Separator class="my-1 bg-white/[0.08] light:bg-gray-200" />
        <div class="text-xs font-medium px-2 py-1 text-white/40 light:text-gray-500">{{ $t('userMenu.theme') }}</div>
        <ThemeSwitcher />
        <Separator class="my-1 bg-white/[0.08] light:bg-gray-200" />
        <Button
          variant="ghost"
          class="w-full justify-start px-2 py-1 h-auto text-[13px] font-normal text-white/70 hover:text-white hover:bg-white/[0.04] light:text-gray-600 light:hover:text-gray-900 light:hover:bg-gray-100"
          @click="handleLogout"
        >
          <LogOut class="mr-2 h-3.5 w-3.5" aria-hidden="true" />
          <span>{{ $t('userMenu.logOut') }}</span>
        </Button>
      </PopoverContent>
    </Popover>
  </div>

  <!-- Away Warning Dialog -->
  <AlertDialog :open="showAwayWarning">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('userMenu.awayWarningTitle') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('userMenu.awayWarningDesc', { count: awayWarningTransferCount }) }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <Button variant="outline" @click="showAwayWarning = false">{{ $t('common.cancel') }}</Button>
        <Button @click="confirmGoAway" :disabled="isUpdatingAvailability">{{ $t('userMenu.goAway') }}</Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
