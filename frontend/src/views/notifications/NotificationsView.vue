<script setup lang="ts">
import { ref } from 'vue'
import { 
  Bell, 
  Search, 
  Filter, 
  MoreHorizontal, 
  CheckCircle2, 
  AlertCircle, 
  Info,
  Trash2
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface Notification {
  id: string
  title: string
  message: string
  type: 'info' | 'success' | 'warning' | 'error'
  time: string
  read: boolean
}

const notifications = ref<Notification[]>([
  {
    id: '1',
    title: 'New Campaign Started',
    message: 'The "Spring Sale 2026" campaign has been successfully launched.',
    type: 'success',
    time: '2 mins ago',
    read: false
  },
  {
    id: '2',
    title: 'System Update',
    message: 'Scheduled maintenance will occur tomorrow at 02:00 UTC.',
    type: 'info',
    time: '1 hour ago',
    read: false
  },
  {
    id: '3',
    title: 'Account Alert',
    message: 'New login detected from Mumbai, India.',
    type: 'warning',
    time: '3 hours ago',
    read: true
  },
  {
    id: '4',
    title: 'Delivery Failed',
    message: 'Broadcast failed for 12 recipients. View logs for details.',
    type: 'error',
    time: '5 hours ago',
    read: true
  }
])

const searchQuery = ref('')

const getTypeIcon = (type: string) => {
  switch (type) {
    case 'success': return CheckCircle2
    case 'warning': return AlertCircle
    case 'error': return AlertCircle
    default: return Info
  }
}

const getTypeColor = (type: string) => {
  switch (type) {
    case 'success': return 'text-emerald-500 bg-emerald-500/10'
    case 'warning': return 'text-orange-500 bg-orange-500/10'
    case 'error': return 'text-red-500 bg-red-500/10'
    default: return 'text-blue-500 bg-blue-500/10'
  }
}

const markAllRead = () => {
  notifications.value.forEach(n => n.read = true)
}

const deleteNotification = (id: string) => {
  notifications.value = notifications.value.filter(n => n.id !== id)
}
</script>

<template>
  <div class="h-full flex flex-col bg-background/5 p-6 space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="h-12 w-12 rounded-2xl bg-primary/10 flex items-center justify-center border border-primary/20">
          <Bell class="h-6 w-6 text-primary" />
        </div>
        <div>
          <h1 class="text-2xl font-black tracking-tight">Notifications</h1>
          <p class="text-sm text-muted-foreground font-medium">Manage your alerts and system updates</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="sm" class="rounded-xl font-bold h-10 gap-2" @click="markAllRead">
          Mark all as read
        </Button>
        <Button size="icon" variant="ghost" class="rounded-xl h-10 w-10">
          <Trash2 class="h-4 w-4 text-muted-foreground" />
        </Button>
      </div>
    </div>

    <!-- Controls -->
    <div class="flex items-center gap-4">
      <div class="relative flex-1 group">
        <Search class="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground/40 group-focus-within:text-primary transition-colors" />
        <Input 
          v-model="searchQuery"
          placeholder="Search notifications..." 
          class="pl-11 h-11 bg-background border-border rounded-2xl focus-visible:ring-primary/20 transition-all font-medium"
        />
      </div>
      <Button variant="outline" class="h-11 rounded-2xl gap-2 font-bold px-5">
        <Filter class="h-4 w-4" />
        Filter
      </Button>
    </div>

    <!-- List -->
    <div class="flex-1 overflow-auto rounded-3xl border border-border bg-background/50 backdrop-blur-3xl shadow-sm">
      <div v-if="notifications.length > 0" class="divide-y divide-border">
        <div 
          v-for="notify in notifications" 
          :key="notify.id"
          class="group flex items-start gap-4 p-5 hover:bg-muted/30 transition-all cursor-pointer relative overflow-hidden"
          :class="{ 'bg-primary/[0.02]': !notify.read }"
        >
          <!-- Active Indicator -->
          <div v-if="!notify.read" class="absolute left-0 top-0 bottom-0 w-1 bg-primary"></div>

          <div :class="['h-10 w-10 rounded-xl flex items-center justify-center shrink-0 border border-current/10', getTypeColor(notify.type)]">
            <component :is="getTypeIcon(notify.type)" class="h-5 w-5" />
          </div>

          <div class="flex-1 min-w-0 space-y-1">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-black tracking-tight truncate pr-4">{{ notify.title }}</h3>
              <span class="text-[10px] font-black uppercase text-muted-foreground/50 tracking-widest whitespace-nowrap">
                {{ notify.time }}
              </span>
            </div>
            <p class="text-[13px] text-muted-foreground font-medium leading-relaxed max-w-3xl">
              {{ notify.message }}
            </p>
          </div>

          <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button size="icon" variant="ghost" class="h-8 w-8 rounded-lg" @click.stop="deleteNotification(notify.id)">
              <Trash2 class="h-3.5 w-3.5 text-red-400" />
            </Button>
            <Button size="icon" variant="ghost" class="h-8 w-8 rounded-lg">
              <MoreHorizontal class="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="h-full flex flex-col items-center justify-center p-12 space-y-4 text-center">
        <div class="h-20 w-20 rounded-full bg-muted flex items-center justify-center">
          <Bell class="h-10 w-10 text-muted-foreground/30" />
        </div>
        <div>
          <h3 class="text-lg font-bold">All caught up!</h3>
          <p class="text-sm text-muted-foreground">No new notifications to show.</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: rgba(var(--primary), 0.1);
  border-radius: 10px;
}
</style>
