<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useContactsStore } from '@/stores/contacts'
import { usersService, chatbotService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@/components/ui/popover'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import {
  LayoutDashboard,
  MessageSquare,
  Bot,
  FileText,
  Megaphone,
  Settings,
  LogOut,
  ChevronLeft,
  ChevronRight,
  Users,
  Workflow,
  Sparkles,
  Key,
  User,
  UserX,
  MessageSquareText,
  Sun,
  Moon,
  Monitor,
  Webhook,
  BarChart3,
  ShieldCheck,
  Zap,
  Shield
} from 'lucide-vue-next'
import { useColorMode } from '@/composables/useColorMode'
import { toast } from 'vue-sonner'
import { getInitials } from '@/lib/utils'
import { wsService } from '@/services/websocket'
import OrganizationSwitcher from './OrganizationSwitcher.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const contactsStore = useContactsStore()
const isCollapsed = ref(false)
const isUserMenuOpen = ref(false)
const isUpdatingAvailability = ref(false)
const isCheckingTransfers = ref(false)
const showAwayWarning = ref(false)
const awayWarningTransferCount = ref(0)
const { colorMode, isDark, setColorMode } = useColorMode()

const handleAvailabilityChange = async (checked: boolean) => {
  // If going away, check for assigned transfers first
  if (!checked) {
    isCheckingTransfers.value = true
    try {
      // Fetch current user's active transfers from API
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
      // Proceed anyway if check fails
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
      toast.success('Available', {
        description: 'You are now available to receive transfers'
      })
    } else {
      const transfersReturned = data.transfers_to_queue || 0
      toast.success('Away', {
        description: transfersReturned > 0
          ? `${transfersReturned} transfer(s) returned to queue`
          : 'You will not receive new transfer assignments'
      })

      // Refresh contacts list if transfers were returned to queue
      if (transfersReturned > 0) {
        contactsStore.fetchContacts()
      }
    }
  } catch (error) {
    toast.error('Error', {
      description: 'Failed to update availability'
    })
  } finally {
    isUpdatingAvailability.value = false
  }
}

// Calculate break duration for display
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

// Start/stop break timer based on availability
watch(() => authStore.isAvailable, (available) => {
  if (!available && authStore.breakStartedAt) {
    updateBreakDuration()
    breakTimerInterval = setInterval(updateBreakDuration, 60000) // Update every minute
  } else if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
    breakTimerInterval = null
    breakDuration.value = ''
  }
}, { immediate: true })

// Restore break time on mount and connect WebSocket
onMounted(() => {
  authStore.restoreBreakTime()
  if (!authStore.isAvailable && authStore.breakStartedAt) {
    updateBreakDuration()
    breakTimerInterval = setInterval(updateBreakDuration, 60000)
  }

  // Connect WebSocket for real-time updates across all pages
  const token = localStorage.getItem('auth_token')
  if (token) {
    wsService.connect(token)
  }
})

onUnmounted(() => {
  if (breakTimerInterval) {
    clearInterval(breakTimerInterval)
  }
})

// Define all navigation items with permission requirements
// Each item can have a 'permission' (resource name) that is checked for 'read' access
const allNavItems = [
  {
    name: 'Dashboard',
    path: '/',
    icon: LayoutDashboard,
    permission: 'analytics' // Dashboard requires analytics read permission
  },
  {
    name: 'Chat',
    path: '/chat',
    icon: MessageSquare,
    permission: 'chat'
  },
  {
    name: 'Chatbot',
    path: '/chatbot',
    icon: Bot,
    permission: 'settings.chatbot',
    // Show parent if user has any child permission
    childPermissions: ['settings.chatbot', 'chatbot.keywords', 'flows.chatbot', 'chatbot.ai'],
    children: [
      { name: 'Overview', path: '/chatbot', icon: Bot, permission: 'settings.chatbot' },
      { name: 'Keywords', path: '/chatbot/keywords', icon: Key, permission: 'chatbot.keywords' },
      { name: 'Flows', path: '/chatbot/flows', icon: Workflow, permission: 'flows.chatbot' },
      { name: 'AI Contexts', path: '/chatbot/ai', icon: Sparkles, permission: 'chatbot.ai' }
    ]
  },
  {
    name: 'Transfers',
    path: '/chatbot/transfers',
    icon: UserX,
    permission: 'transfers'
  },
  {
    name: 'Agent Analytics',
    path: '/analytics/agents',
    icon: BarChart3,
    permission: 'analytics.agents'
  },
  {
    name: 'Templates',
    path: '/templates',
    icon: FileText,
    permission: 'templates'
  },
  {
    name: 'Flows',
    path: '/flows',
    icon: Workflow,
    permission: 'flows.whatsapp'
  },
  {
    name: 'Campaigns',
    path: '/campaigns',
    icon: Megaphone,
    permission: 'campaigns'
  },
  {
    name: 'Settings',
    path: '/settings',
    icon: Settings,
    permission: 'settings.general',
    // Show parent if user has any child permission
    childPermissions: ['settings.general', 'settings.chatbot', 'accounts', 'canned_responses', 'teams', 'users', 'roles', 'api_keys', 'webhooks', 'custom_actions', 'settings.sso'],
    children: [
      { name: 'General', path: '/settings', icon: Settings, permission: 'settings.general' },
      { name: 'Chatbot', path: '/settings/chatbot', icon: Bot, permission: 'settings.chatbot' },
      { name: 'Accounts', path: '/settings/accounts', icon: Users, permission: 'accounts' },
      { name: 'Canned Responses', path: '/settings/canned-responses', icon: MessageSquareText, permission: 'canned_responses' },
      { name: 'Teams', path: '/settings/teams', icon: Users, permission: 'teams' },
      { name: 'Users', path: '/settings/users', icon: Users, permission: 'users' },
      { name: 'Roles', path: '/settings/roles', icon: Shield, permission: 'roles' },
      { name: 'API Keys', path: '/settings/api-keys', icon: Key, permission: 'api_keys' },
      { name: 'Webhooks', path: '/settings/webhooks', icon: Webhook, permission: 'webhooks' },
      { name: 'Custom Actions', path: '/settings/custom-actions', icon: Zap, permission: 'custom_actions' },
      { name: 'SSO', path: '/settings/sso', icon: ShieldCheck, permission: 'settings.sso' }
    ]
  }
]

// Filter navigation based on user permissions
const navigation = computed(() => {
  return allNavItems
    .filter(item => {
      // If item has childPermissions, show if user has ANY of them
      if (item.childPermissions) {
        return item.childPermissions.some(p => authStore.hasPermission(p, 'read'))
      }
      // Otherwise check the item's own permission
      return !item.permission || authStore.hasPermission(item.permission, 'read')
    })
    .map(item => {
      // Filter children first
      const filteredChildren = item.children?.filter(
        child => !child.permission || authStore.hasPermission(child.permission, 'read')
      )

      // If user doesn't have permission for parent but has childPermissions,
      // use the first accessible child's path as the parent link
      let effectivePath = item.path
      if (item.childPermissions && !authStore.hasPermission(item.permission, 'read') && filteredChildren?.length) {
        effectivePath = filteredChildren[0].path
      }

      // For active state, use original path so children show when on any child route
      const originalPath = item.path
      const isActive = originalPath === '/'
        ? route.name === 'dashboard'
        : originalPath === '/chat'
          ? route.name === 'chat' || route.name === 'chat-conversation'
          : route.path.startsWith(originalPath)

      return {
        ...item,
        path: effectivePath,
        active: isActive,
        children: filteredChildren
      }
    })
})

const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
}

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}
</script>

<template>
  <div class="flex h-screen bg-background">
    <!-- Sidebar -->
    <aside
      :class="[
        'flex flex-col border-r bg-card transition-all duration-300',
        isCollapsed ? 'w-16' : 'w-64'
      ]"
    >
      <!-- Logo -->
      <div class="flex h-12 items-center justify-between px-3 border-b">
        <RouterLink to="/" class="flex items-center gap-2">
          <div class="h-7 w-7 rounded-md bg-primary flex items-center justify-center">
            <MessageSquare class="h-4 w-4 text-primary-foreground" />
          </div>
          <span
            v-if="!isCollapsed"
            class="font-semibold text-sm text-foreground"
          >
            Whatomate
          </span>
        </RouterLink>
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7"
          @click="toggleSidebar"
        >
          <ChevronLeft v-if="!isCollapsed" class="h-3.5 w-3.5" />
          <ChevronRight v-else class="h-3.5 w-3.5" />
        </Button>
      </div>

      <!-- Organization Switcher (Super Admin only) -->
      <OrganizationSwitcher :collapsed="isCollapsed" />

      <!-- Navigation -->
      <ScrollArea class="flex-1 py-2">
        <nav class="space-y-0.5 px-2">
          <template v-for="item in navigation" :key="item.path">
            <RouterLink
              :to="item.path"
              :class="[
                'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                item.active
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                isCollapsed && 'justify-center px-2'
              ]"
            >
              <component :is="item.icon" class="h-4 w-4 shrink-0" />
              <span v-if="!isCollapsed">{{ item.name }}</span>
            </RouterLink>

            <!-- Submenu items -->
            <template v-if="item.children && item.active && !isCollapsed">
              <RouterLink
                v-for="child in item.children"
                :key="child.path"
                :to="child.path"
                :class="[
                  'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition-colors ml-4',
                  route.path === child.path
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                ]"
              >
                <component :is="child.icon" class="h-3.5 w-3.5 shrink-0" />
                <span>{{ child.name }}</span>
              </RouterLink>
            </template>
          </template>
        </nav>
      </ScrollArea>

      <!-- User section -->
      <div class="border-t p-2">
        <Popover v-model:open="isUserMenuOpen">
          <PopoverTrigger as-child>
            <Button
              variant="ghost"
              :class="[
                'flex items-center w-full h-auto px-2 py-1.5 gap-2',
                isCollapsed && 'justify-center'
              ]"
            >
              <Avatar class="h-7 w-7">
                <AvatarImage :src="undefined" />
                <AvatarFallback class="text-xs">
                  {{ getInitials(authStore.user?.full_name || 'U') }}
                </AvatarFallback>
              </Avatar>
              <div v-if="!isCollapsed" class="flex flex-col items-start text-left">
                <span class="text-[13px] font-medium truncate max-w-[140px]">
                  {{ authStore.user?.full_name }}
                </span>
                <span class="text-[11px] text-muted-foreground truncate max-w-[140px]">
                  {{ authStore.user?.email }}
                </span>
              </div>
            </Button>
          </PopoverTrigger>
          <PopoverContent side="top" align="start" class="w-52 p-1.5">
            <div class="text-xs font-medium px-2 py-1 text-muted-foreground">My Account</div>
            <Separator class="my-1" />
            <!-- Availability Toggle -->
            <div class="flex items-center justify-between px-2 py-1.5">
              <div class="flex items-center gap-2">
                <span class="text-[13px]">Status</span>
                <Badge :variant="authStore.isAvailable ? 'default' : 'secondary'" class="text-[10px] px-1.5 py-0">
                  {{ authStore.isAvailable ? 'Available' : 'Away' }}
                </Badge>
                <span v-if="!authStore.isAvailable && breakDuration" class="text-[10px] text-muted-foreground">
                  {{ breakDuration }}
                </span>
              </div>
              <Switch
                :checked="authStore.isAvailable"
                :disabled="isUpdatingAvailability || isCheckingTransfers"
                @update:checked="handleAvailabilityChange"
              />
            </div>
            <Separator class="my-1" />
            <RouterLink to="/profile">
              <Button
                variant="ghost"
                class="w-full justify-start px-2 py-1 h-auto text-[13px] font-normal"
                @click="isUserMenuOpen = false"
              >
                <User class="mr-2 h-3.5 w-3.5" />
                <span>Profile</span>
              </Button>
            </RouterLink>
            <Separator class="my-1" />
            <div class="text-xs font-medium px-2 py-1 text-muted-foreground">Theme</div>
            <div class="flex gap-0.5 px-1.5 py-1">
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                :class="colorMode === 'light' && 'bg-accent'"
                @click="setColorMode('light')"
              >
                <Sun class="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                :class="colorMode === 'dark' && 'bg-accent'"
                @click="setColorMode('dark')"
              >
                <Moon class="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                :class="colorMode === 'system' && 'bg-accent'"
                @click="setColorMode('system')"
              >
                <Monitor class="h-3.5 w-3.5" />
              </Button>
            </div>
            <Separator class="my-1" />
            <Button
              variant="ghost"
              class="w-full justify-start px-2 py-1 h-auto text-[13px] font-normal"
              @click="handleLogout"
            >
              <LogOut class="mr-2 h-3.5 w-3.5" />
              <span>Log out</span>
            </Button>
          </PopoverContent>
        </Popover>
      </div>
    </aside>

    <!-- Main content -->
    <main class="flex-1 overflow-hidden">
      <RouterView />
    </main>

    <!-- Away Warning Dialog -->
    <AlertDialog :open="showAwayWarning">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Active Transfers Will Be Returned to Queue</AlertDialogTitle>
          <AlertDialogDescription>
            You have {{ awayWarningTransferCount }} active transfer(s) assigned to you.
            Setting your status to "Away" will return them to the queue for other agents to pick up.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" @click="showAwayWarning = false">Cancel</Button>
          <Button @click="confirmGoAway" :disabled="isUpdatingAvailability">Go Away</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
