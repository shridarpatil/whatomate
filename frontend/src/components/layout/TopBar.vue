<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  MessageSquare,
  Search,
  Bell,
  HelpCircle,
  Smartphone,
  ChevronDown,
  Check,
  FileText,
  Layout,
  ExternalLink,
  X
} from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import OrganizationSwitcher from './OrganizationSwitcher.vue'
import UserMenu from './UserMenu.vue'
import { accountsService } from '@/services/api'
import { useContactsStore } from '@/stores/contacts'

const route = useRoute()
const router = useRouter()
const contactsStore = useContactsStore()
const searchQuery = ref('')
const accounts = ref<any[]>([])
const isAccountMenuOpen = ref(false)

// Setup Mode Logic
const isSettingsMode = computed(() => route.path.startsWith('/settings'))

const fetchAccounts = async () => {
  try {
    const res = await accountsService.list()
    const fetched = res.data.data?.accounts || []
    accounts.value = fetched

    // Auto-select if: 
    // 1. Only one account exists (always select it)
    // 2. Multiple exist but none are currently selected
    if (fetched.length === 1) {
      contactsStore.setAccountFilter(fetched[0].name)
    } else if (fetched.length > 1 && !contactsStore.accountFilter) {
      contactsStore.setAccountFilter(fetched[0].name)
    }
  } catch (error) {
    console.error('Failed to fetch accounts:', error)
  }
}

onMounted(() => {
  contactsStore.init() // Restore persistent account filter
  fetchAccounts()
})

const handleAccountChange = (name: string) => {
  contactsStore.setAccountFilter(name)
  isAccountMenuOpen.value = false
}

const exitSetup = () => {
  router.push('/')
}

const currentAccount = computed(() => contactsStore.accountFilter || 'Select Channel')

</script>

<template>
  <header
    class="h-14 border-b border-white/5 bg-background/95 backdrop-blur-xl flex items-center justify-between px-4 z-50 sticky top-0 transition-all duration-300">
    <!-- Left Segment: Identity & Brand -->
    <div class="flex items-center gap-6 shrink-0 min-w-0">
      <RouterLink to="/" class="flex items-center gap-2.5 group">
        <div
          class="h-9 w-9 rounded-xl flex items-center justify-center transition-all duration-300 relative overflow-hidden"
          :class="isSettingsMode ? 'bg-orange-500/20 border border-orange-500/20' : 'bg-primary/20 border border-primary/20 group-hover:scale-105'">
          <div class="absolute inset-0 bg-gradient-to-br from-white/10 to-transparent opacity-50"></div>
          <MessageSquare v-if="!isSettingsMode" class="h-5 w-5 text-primary relative z-10" />
          <Layout v-else class="h-5 w-5 text-orange-500 relative z-10" />
        </div>
        <div class="flex flex-col hidden sm:flex">
          <span class="font-bold text-[14px] leading-none tracking-tight">Whatomate</span>
        </div>
      </RouterLink>

      <div v-if="!isSettingsMode" class="hidden md:flex items-center flex-1 max-w-[360px]">
        <div class="relative w-full group">
          <Search
            class="absolute left-3.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/40 transition-colors group-focus-within:text-primary" />
          <Input v-model="searchQuery" placeholder="Search commands... (⌘K)"
            class="w-full bg-muted/30 border-border h-9 pl-10 pr-12 focus-visible:ring-primary/20 focus-visible:bg-muted/50 transition-all rounded-xl text-[13px] font-medium" />
          <kbd
            class="absolute right-2.5 top-1/2 -translate-y-1/2 h-4 px-1.5 rounded border border-border bg-background text-[9px] font-black text-muted-foreground/60 pointer-events-none group-focus-within:opacity-100 transition-opacity">
            ⌘K
          </kbd>
        </div>
      </div>
    </div>

    <!-- Right Segment: Context Switchers & Global Actions -->
    <div class="flex items-center gap-2 shrink-0">

      <OrganizationSwitcher />

      <!-- Account Switcher -->
      <DropdownMenu v-if="!isSettingsMode" v-model:open="isAccountMenuOpen">
        <DropdownMenuTrigger as-child>
          <Button variant="ghost" size="sm"
            class="h-9 px-2 bg-muted/30 border border-border hover:bg-muted/50 text-[12px] font-black gap-2.5 rounded-xl transition-all shadow-sm group">
            <div
              class="h-6 w-6 rounded-lg bg-primary/10 flex items-center justify-center text-[10px] font-black text-primary border border-primary/20 shrink-0 group-hover:bg-primary/20 transition-colors uppercase">
              {{ currentAccount.charAt(0).toUpperCase() }}
            </div>
            <span class="truncate max-w-[100px] tracking-tight font-bold">{{ currentAccount }}</span>
            <ChevronDown class="h-3 w-3 opacity-20" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-56 p-1 bg-popover/95 backdrop-blur-xl border-border shadow-3xl">
          <DropdownMenuLabel
            class="text-[9px] font-black text-muted-foreground/50 uppercase tracking-[0.2em] px-3 py-2.5 text-center">
            Active Channel
          </DropdownMenuLabel>
          <DropdownMenuSeparator class="border-border" />
          <DropdownMenuItem v-for="acct in accounts" :key="acct.id"
            class="flex items-center gap-3 px-2 py-2.5 rounded-lg cursor-pointer group transition-colors focus:bg-primary/10"
            :class="contactsStore.accountFilter === acct.name ? 'bg-primary/5' : ''"
            @select="handleAccountChange(acct.name)">
            <div
              class="h-7 w-7 rounded-md bg-muted flex items-center justify-center text-[10px] font-black group-hover:bg-primary/20 group-hover:text-primary transition-colors">
              {{ acct.name.charAt(0).toUpperCase() }}
            </div>
            <span class="flex-1 text-[13px] font-bold tracking-tight">{{ acct.name }}</span>
            <Check v-if="contactsStore.accountFilter === acct.name" class="h-3.5 w-3.5 text-primary" />
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Setup Mode Badge -->
      <Badge v-else variant="outline"
        class="h-7 px-3 bg-orange-500/5 text-orange-500 border-orange-500/10 font-black uppercase text-[9px] tracking-widest">
        Setup Active
      </Badge>

      <!-- Global Actions -->
      <template v-if="!isSettingsMode">
        <div class="flex items-center gap-1">
          <TooltipProvider :delay-duration="300">
            <Tooltip>
              <Popover>
                <TooltipTrigger as-child>
                  <PopoverTrigger as-child>
                    <Button variant="ghost" size="icon"
                      class="h-9 w-9 relative hover:bg-muted rounded-full transition-colors group">
                      <Bell class="h-4.5 w-4.5 text-muted-foreground/60 group-hover:text-primary transition-colors" />
                      <span
                        class="absolute top-2.5 right-2.5 h-2 w-2 bg-primary rounded-full border-2 border-background shadow-[0_0_10px_rgba(16,185,129,0.5)]"></span>
                    </Button>
                  </PopoverTrigger>
                </TooltipTrigger>
                <TooltipContent>Notifications</TooltipContent>
                <PopoverContent align="end" class="w-80 p-0 border-border bg-popover/95 backdrop-blur-xl shadow-3xl">
                  <div class="p-4 border-b border-border flex items-center justify-between">
                    <h3 class="font-black text-[12px] uppercase tracking-widest text-muted-foreground/60">Notifications
                    </h3>
                    <Badge variant="secondary"
                      class="text-[9px] bg-primary/10 text-primary uppercase font-black tracking-widest px-2">Live Feed
                    </Badge>
                  </div>
                  <div class="p-8 text-center bg-muted/10">
                    <p class="text-[11px] text-muted-foreground font-medium italic opacity-40">No new alerts found</p>
                  </div>
                </PopoverContent>
              </Popover>
            </Tooltip>
          </TooltipProvider>

          <Popover>
            <PopoverTrigger as-child>
              <Button variant="ghost" size="icon" class="h-9 w-9 hover:bg-muted rounded-full transition-colors">
                <HelpCircle class="h-4.5 w-4.5 text-muted-foreground/60 group-hover:text-primary transition-colors" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-64 p-2 border-border shadow-3xl bg-popover/95 backdrop-blur-xl">
              <div class="px-3 py-2.5 text-[9px] font-black text-primary uppercase tracking-[0.2em] opacity-60">
                Knowledge Base</div>
              <div class="grid gap-1">
                <Button variant="ghost"
                  class="w-full justify-start h-10 px-3 text-[13px] font-bold rounded-xl group/btn hover:bg-muted transition-all">
                  <FileText class="mr-3 h-4 w-4 text-muted-foreground/40 group-hover/btn:text-primary" /> Documentation
                </Button>
                <Button variant="ghost"
                  class="w-full justify-start h-10 px-3 text-[13px] font-black text-emerald-500/80 hover:text-emerald-500 hover:bg-emerald-500/5 rounded-xl transition-all">
                  <ExternalLink class="mr-3 h-4 w-4" /> Help Center
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <Separator orientation="vertical" class="h-6 mx-2 border-border" />
      </template>

      <!-- Exit Setup Button -->
      <Button v-if="isSettingsMode" variant="ghost" size="sm"
        class="h-9 px-4 gap-2 text-muted-foreground hover:text-primary hover:bg-primary/5 font-black text-[12px] uppercase tracking-widest rounded-xl transition-all"
        @click="exitSetup">
        <X class="h-3.5 w-3.5" /> Exit
      </Button>

      <UserMenu />
    </div>
  </header>
</template>
