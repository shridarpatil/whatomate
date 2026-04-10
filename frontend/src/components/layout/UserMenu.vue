<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useContactsStore } from '@/stores/contacts'
import { usersService, chatbotService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetTrigger,
} from '@/components/ui/sheet'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import {
  LogOut,
  Globe,
  Moon,
  ChevronRight,
  Settings
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { getInitials } from '@/lib/utils'
import ThemeSwitcher from './ThemeSwitcher.vue'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import { accountSections } from './navigation'

const { t } = useI18n()
const router = useRouter()

defineProps<{
  collapsed?: boolean
}>()

defineEmits<{
  logout: []
}>()

const authStore = useAuthStore()
const contactsStore = useContactsStore()
const isUserMenuOpen = ref(false)
const isUpdatingAvailability = ref(false)
const isCheckingTransfers = ref(false)
const showAwayWarning = ref(false)
const awayWarningTransferCount = ref(0)

const filteredAccountSections = computed(() => {
  return accountSections.map(section => ({
    ...section,
    items: section.items.filter(item =>
      !item.permission || authStore.hasPermission(item.permission, 'read')
    )
  })).filter(section => section.items.length > 0)
})

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
      let durationDetail = ''
      if (authStore.breakStartedAt) {
        const start = new Date(authStore.breakStartedAt)
        const now = new Date()
        const diffMs = now.getTime() - start.getTime()
        const diffMins = Math.floor(diffMs / 60000)
        durationDetail = diffMins >= 1 ? ` (Session Time: ${diffMins}m)` : ''
      }

      toast.success(`${t('userMenu.available')}${durationDetail}`, {
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

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}
</script>

<template>
  <Sheet v-model:open="isUserMenuOpen">
    <SheetTrigger as-child>
      <Button variant="ghost"
        class="flex items-center justify-start h-10 px-2 gap-2 hover:bg-zinc-500/10 dark:hover:bg-white/5 rounded-full transition-all group shrink-0"
        aria-label="User menu">
        <div class="relative">
          <Avatar class="h-8 w-8 ring-1 ring-border group-hover:ring-primary/30 shadow-lg">
            <AvatarImage :src="undefined" />
            <AvatarFallback class="text-xs font-bold bg-muted text-foreground">
              {{ getInitials(authStore.user?.full_name || 'U') }}
            </AvatarFallback>
          </Avatar>
        </div>
        <div class="hidden lg:flex flex-col items-start text-left shrink-0">
          <span class="text-[13px] font-bold truncate max-w-[120px] text-foreground leading-none">
            {{ authStore.user?.full_name }}
          </span>
          <span class="text-[10px] text-muted-foreground truncate max-w-[120px] mt-1 opacity-60">
            {{ authStore.isAvailable ? 'Available' : 'On Break' }}
          </span>
        </div>
      </Button>
    </SheetTrigger>
    <SheetContent class="w-80 md:w-96 p-0 border-l border-border bg-background flex flex-col shadow-2xl z-[200]">
      <SheetHeader class="sr-only">
        <SheetTitle>User Account Menu</SheetTitle>
        <SheetDescription>Access your profile, subscription, and platform preferences.</SheetDescription>
      </SheetHeader>

      <div class="flex-1 overflow-y-auto custom-scrollbar">
        <!-- Minimal Profile Header -->
        <div class="p-8 pb-6 flex flex-col items-center text-center space-y-4 relative border-b border-border">
          <Avatar class="h-20 w-20 ring-1 ring-border shadow-2xl mt-4">
            <AvatarFallback class="text-2xl font-bold bg-muted text-foreground">
              {{ getInitials(authStore.user?.full_name || 'U') }}
            </AvatarFallback>
          </Avatar>

          <div class="space-y-0.5">
            <h2 class="text-xl font-bold tracking-tight text-foreground">{{ authStore.user?.full_name }}</h2>
            <p class="text-[11px] text-muted-foreground font-medium opacity-60">{{ authStore.user?.email }}</p>
          </div>

          <Button variant="outline" size="sm"
            class="h-9 w-full max-w-[140px] gap-2 text-foreground bg-muted hover:bg-red-500/10 hover:text-red-500 hover:border-red-500/20 border-border rounded-lg transition-all font-bold text-[11px] mt-2 group/logout"
            @click="handleLogout">
            <LogOut class="h-3.5 w-3.5 group-hover/logout:scale-110 transition-transform" />
            Logout
          </Button>
        </div>

        <div class="p-6 space-y-4">
          <!-- Presence Toggle (Zoho Style) -->
          <div class="px-2 pb-4">
            <div
              class="group relative overflow-hidden p-4 rounded-2xl border border-border bg-muted/30 hover:border-primary/20 transition-all">
              <div class="flex items-center justify-between relative z-10">
                <div class="flex flex-col gap-0.5">
                  <div class="flex items-center gap-2">
                    <div
                      :class="['h-2 w-2 rounded-full', authStore.isAvailable ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]' : 'bg-muted-foreground']">
                    </div>
                    <span class="text-[13px] font-bold tracking-tight text-foreground">{{ authStore.isAvailable ?
                      'Available' : 'On Break' }}</span>
                  </div>
                  <p class="text-[10px] text-muted-foreground font-medium opacity-60">
                    {{ authStore.isAvailable ? 'Visible to customers' : 'Away from desk' }}
                  </p>
                </div>
                <Switch :checked="authStore.isAvailable" :disabled="isUpdatingAvailability || isCheckingTransfers"
                  @update:checked="handleAvailabilityChange" class="scale-90" />
              </div>
            </div>
          </div>

          <!-- Main Items (Flat Flow) -->
          <div class="grid gap-1">
            <template v-for="section in filteredAccountSections" :key="section.label">
              <RouterLink v-for="item in section.items" :key="item.path" :to="item.path"
                class="flex items-center gap-4 p-3 rounded-xl hover:bg-accent transition-all group/item"
                @click="isUserMenuOpen = false">
                <div
                  class="h-9 w-9 rounded-lg bg-muted flex items-center justify-center shrink-0 transition-colors group-hover/item:text-primary">
                  <component :is="item.icon"
                    class="h-4.5 w-4.5 text-muted-foreground group-hover/item:text-primary transition-colors" />
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-[13px] font-bold leading-none tracking-tight text-foreground">{{ t(item.name) }}</p>
                </div>
                <ChevronRight
                  class="h-4 w-4 text-muted-foreground/20 group-hover/item:text-primary group-hover/item:translate-x-0.5 transition-all" />
              </RouterLink>
            </template>
          </div>

          <Separator class="opacity-10 !my-8" />

          <!-- Preferences Section (Modern Cards) -->
          <div class="space-y-3 pb-8 relative z-[100]">
            <div class="px-2 flex items-center justify-between">
              <span class="text-[10px] font-black text-primary uppercase tracking-[0.2em] opacity-60">Portal
                Preferences</span>
              <Settings class="h-3 w-3 text-primary opacity-30" />
            </div>
            <div class="grid gap-2">
              <div
                class="p-4 rounded-2xl border border-border bg-muted/20 flex flex-col gap-4 hover:border-primary/10 transition-colors shadow-sm">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2.5">
                    <div class="h-8 w-8 rounded-lg bg-muted flex items-center justify-center">
                      <Moon class="h-4 w-4 text-muted-foreground" />
                    </div>
                    <span class="text-[12px] font-bold tracking-tight text-foreground">Active Theme</span>
                  </div>
                  <ThemeSwitcher />
                </div>
                <Separator class="opacity-10" />
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2.5">
                    <div class="h-8 w-8 rounded-lg bg-muted flex items-center justify-center">
                      <Globe class="h-4 w-4 text-muted-foreground" />
                    </div>
                    <span class="text-[12px] font-bold tracking-tight text-foreground">Portal Language</span>
                  </div>
                  <LanguageSwitcher />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </SheetContent>
  </Sheet>

  <!-- Away Warning Dialog -->
  <AlertDialog :open="showAwayWarning">
    <AlertDialogContent class="rounded-3xl border-border">
      <AlertDialogHeader>
        <AlertDialogTitle class="text-xl font-black tracking-tight">{{ $t('userMenu.awayWarningTitle') }}
        </AlertDialogTitle>
        <AlertDialogDescription class="text-sm font-medium text-muted-foreground">
          {{ $t('userMenu.awayWarningDesc', { count: awayWarningTransferCount }) }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter class="flex sm:justify-center gap-2 mt-4">
        <Button variant="ghost" @click="showAwayWarning = false" class="rounded-xl font-bold px-6">Continue
          Operation</Button>
        <Button @click="confirmGoAway" :disabled="isUpdatingAvailability"
          class="rounded-xl font-black px-6 bg-red-500 hover:bg-red-600 text-white border-0">Go Away</Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
}

.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
  background: hsl(var(--muted-foreground) / 0.1);
  border-radius: 10px;
}
</style>
