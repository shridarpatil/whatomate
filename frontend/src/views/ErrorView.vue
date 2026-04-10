<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  Search,
  Filter,
  Terminal,
  RefreshCw,
  ChevronRight,
  ShieldAlert,
  Bug,
  Database,
  Activity,
  CheckCircle2,
  Zap
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'

interface ErrorLog {
  id: string
  code: string
  message: string
  service: string
  timestamp: string
  severity: 'critical' | 'warning' | 'error'
  status: 'new' | 'investigating' | 'resolved'
}

const route = useRoute()

const errorLogs = ref<ErrorLog[]>([
  {
    id: '1',
    code: 'API_429',
    message: 'Rate limit exceeded for Meta Graph API.',
    service: 'Messaging Engine',
    timestamp: '2026-04-10 08:45:12',
    severity: 'critical',
    status: 'new'
  },
  {
    id: '2',
    code: 'DB_TIMEOUT',
    message: 'PostgreSQL connection timeout during large analytical query.',
    service: 'Data Analytics',
    timestamp: '2026-04-10 08:30:05',
    severity: 'error',
    status: 'investigating'
  },
  {
    id: '3',
    code: 'WS_DISCONNECT',
    message: 'Socket connection dropped; failed heartbeat after 3 retries.',
    service: 'Realtime Gateway',
    timestamp: '2026-04-10 07:12:44',
    severity: 'warning',
    status: 'resolved'
  },
  {
    id: '4',
    code: 'WEBHOOK_502',
    message: 'External webhook endpoint returned Bad Gateway.',
    service: 'Integration Hub',
    timestamp: '2026-04-10 06:05:11',
    severity: 'error',
    status: 'new'
  }
])

const isRefreshing = ref(false)

const refreshLogs = () => {
  isRefreshing.value = true
  setTimeout(() => {
    isRefreshing.value = false
  }, 1000)
}

onMounted(() => {
  // Check for dynamic error passed via query params
  const { code, message, service } = route.query
  if (code && message) {
    errorLogs.value.unshift({
      id: Date.now().toString(),
      code: code as string,
      message: message as string,
      service: (service as string) || 'Client Browser',
      timestamp: new Date().toLocaleString(),
      severity: 'critical',
      status: 'new'
    })
  }
})

const getSeverityStyles = (severity: string) => {
  switch (severity) {
    case 'critical': return 'bg-red-500/10 text-red-500 border-red-500/20'
    case 'error': return 'bg-orange-500/10 text-orange-500 border-orange-500/20'
    case 'warning': return 'bg-yellow-500/10 text-yellow-500 border-yellow-500/20'
    default: return 'bg-blue-500/10 text-blue-500 border-blue-500/20'
  }
}

const getStatusBadge = (status: string) => {
  switch (status) {
    case 'resolved': return 'bg-emerald-500/10 text-emerald-500'
    case 'investigating': return 'bg-blue-500/10 text-blue-500'
    default: return 'bg-muted text-muted-foreground'
  }
}

const getServiceIcon = (service: string) => {
  if (service.includes('Data')) return Database
  if (service.includes('Gateway')) return ShieldAlert
  if (service.includes('Messaging')) return RefreshCw
  return Bug
}
</script>

<template>
  <div class="h-full flex flex-col bg-background/5 p-6 space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="h-12 w-12 rounded-2xl bg-red-500/10 flex items-center justify-center border border-red-500/20">
          <Terminal class="h-6 w-6 text-red-500" />
        </div>
        <div>
          <h1 class="text-2xl font-black tracking-tight">System Error Logs</h1>
          <p class="text-sm text-muted-foreground font-medium">Monitor platform health and debugging data</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" class="rounded-xl h-10 gap-2 font-bold" @click="refreshLogs" :disabled="isRefreshing">
          <RefreshCw :class="['h-4 w-4', isRefreshing ? 'animate-spin' : '']" />
          Refresh Registry
        </Button>
        <Button variant="outline" class="rounded-xl h-10 px-3">
          <Filter class="h-4 w-4" />
        </Button>
      </div>
    </div>

    <!-- Stats Grid -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div v-for="stat in [
        { label: 'Unresolved Incidents', value: '3', color: 'text-red-500', icon: ShieldAlert, bg: 'bg-red-500/5', border: 'border-red-500/10' },
        { label: 'Platform Availability', value: '99.98%', color: 'text-emerald-500', icon: CheckCircle2, bg: 'bg-emerald-500/5', border: 'border-emerald-500/10' },
        { label: 'Avg API Gateway Latency', value: '42ms', color: 'text-blue-500', icon: Activity, bg: 'bg-blue-500/5', border: 'border-blue-500/10' },
        { label: 'System Load', value: '1.2%', color: 'text-muted-foreground', icon: Zap, bg: 'bg-muted/5', border: 'border-white/5' }
      ]" :key="stat.label" 
        class="p-4 rounded-2xl border bg-black/40 backdrop-blur-3xl flex flex-col justify-between h-32 group transition-all hover:border-primary/20"
        :class="stat.border">
        <div class="flex items-center justify-between">
          <div :class="['p-2 rounded-lg', stat.bg]">
            <component :is="stat.icon" class="h-4 w-4" :class="stat.color" />
          </div>
          <ChevronRight class="h-3 w-3 opacity-0 group-hover:opacity-30 transition-opacity" />
        </div>
        <div>
          <p class="text-[9px] font-black uppercase text-muted-foreground/40 tracking-[0.2em]">{{ stat.label }}</p>
          <p :class="['text-2xl font-black mt-1 tracking-tighter', stat.color]">{{ stat.value }}</p>
        </div>
      </div>
    </div>

    <!-- Search -->
    <div class="relative group max-w-md">
      <Search class="absolute left-3.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/30 group-focus-within:text-primary transition-colors" />
      <Input 
        placeholder="Search incident codes or messages..." 
        class="pl-10 h-10 bg-black/40 border-white/5 rounded-xl font-bold text-xs tracking-tight focus:border-primary/30 transition-all placeholder:opacity-30"
      />
    </div>

    <!-- Error List -->
    <div class="flex-1 overflow-hidden flex flex-col rounded-3xl border border-white/5 bg-black/20 backdrop-blur-3xl shadow-2xl">
      <div class="overflow-auto flex-1 custom-scrollbar">
        <table class="w-full text-left border-collapse">
          <thead>
            <tr class="border-b border-white/5 bg-white/5 sticky top-0 z-10 backdrop-blur-md">
              <th class="px-6 py-4 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/40 w-32">Severity</th>
              <th class="px-6 py-4 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/40">Incident Report</th>
              <th class="px-6 py-4 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/40 w-48">Source Cluster</th>
              <th class="px-6 py-4 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/40 w-48">Detected At</th>
              <th class="px-6 py-4 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/40 w-24"></th>
            </tr>
          </thead>
        <tbody class="divide-y divide-border">
          <tr v-for="log in errorLogs" :key="log.id" class="group hover:bg-muted/30 transition-colors">
            <td class="px-6 py-4">
              <Badge variant="outline" :class="['px-2 py-0.5 rounded-lg font-black text-[9px] uppercase tracking-widest', getSeverityStyles(log.severity)]">
                {{ log.severity }}
              </Badge>
            </td>
            <td class="px-6 py-4">
              <div class="flex flex-col gap-0.5">
                <span class="text-[13px] font-black tracking-tight flex items-center gap-2">
                  <span class="text-xs font-mono text-muted-foreground px-1 border border-border rounded bg-muted/50">{{ log.code }}</span>
                  {{ log.message }}
                </span>
                <span :class="['text-[10px] font-bold px-1.5 py-0.5 rounded-full w-fit mt-1', getStatusBadge(log.status)]">
                  {{ log.status }}
                </span>
              </div>
            </td>
            <td class="px-6 py-4">
              <div class="flex items-center gap-2 text-[12px] font-bold">
                <component :is="getServiceIcon(log.service)" class="h-3.5 w-3.5 text-muted-foreground/50" />
                {{ log.service }}
              </div>
            </td>
            <td class="px-6 py-4">
              <span class="text-[11px] font-medium text-muted-foreground tabular-nums">{{ log.timestamp }}</span>
            </td>
            <td class="px-6 py-4 text-right">
              <Button size="icon" variant="ghost" class="h-8 w-8 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity">
                <ChevronRight class="h-4 w-4" />
              </Button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
  </div>
</template>


<style scoped>
/* Table scroll sync */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-thumb {
  background: rgba(var(--primary), 0.1);
  border-radius: 10px;
}
</style>
