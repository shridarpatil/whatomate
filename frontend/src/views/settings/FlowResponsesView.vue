<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { PageHeader } from '@/components/shared'
import { ArrowLeft, ClipboardList, RefreshCw, Clock, Download, Phone } from 'lucide-vue-next'
import { flowSubmissionsService } from '@/services/api'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()


interface Submission {
  id: string
  phone_number: string
  response_data: Record<string, any>
  created_at: string
  created_at_display: string
}

const submissions = ref<Submission[]>([])
const selectedFlowId = ref<string | null>(null)
const selectedFlowName = ref('')
const isLoading = ref(true)
const isRefreshing = ref(false)
const pollingInterval = ref<ReturnType<typeof setInterval> | null>(null)

onMounted(async () => {
  selectedFlowId.value = route.params.flowId as string
  await fetchSubmissions()
  pollingInterval.value = setInterval(() => {
    fetchSubmissions()
  }, 10000)
})

onUnmounted(() => {
  if (pollingInterval.value) {
    clearInterval(pollingInterval.value)
    pollingInterval.value = null
  }
})



async function fetchSubmissions() {
  if (!selectedFlowId.value) return
  try {
    const resp = await flowSubmissionsService.getSubmissions(selectedFlowId.value)
    const data = (resp.data as any).data

    if (data) {
      selectedFlowName.value = data.flow?.name || ''
      const rawSubmissions = data.submissions || []
      submissions.value = rawSubmissions.map((sub: any) => {
        const d = new Date(sub.created_at)
        const dateStr = d.toLocaleString(undefined, {
          month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit'
        })
        return {
          id: sub.id,
          phone_number: sub.phone_number,
          response_data: sub.response_data || {},
          created_at: sub.created_at,
          created_at_display: dateStr
        }
      })
    }
  } catch (e) {
    console.error('Failed to fetch submissions:', e)
  } finally {
    isLoading.value = false
    isRefreshing.value = false
  }
}

function goBack() {
  router.push('/flows')
}

async function refreshData() {
  isRefreshing.value = true
  await fetchSubmissions()
  isRefreshing.value = false
}

function formatKey(key: string): string {
  return key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())
}



// Dynamic column headers from all submission response data
const responseColumns = computed(() => {
  const cols = new Set<string>()
  for (const sub of submissions.value) {
    if (sub.response_data) {
      for (const key of Object.keys(sub.response_data)) {
        if (key !== 'flow_token') cols.add(key)
      }
    }
  }
  return Array.from(cols)
})


function exportToCsv() {
  if (submissions.value.length === 0) return
  const allKeys = new Set<string>()
  submissions.value.forEach(sub => { Object.keys(sub.response_data).forEach(k => { if (k !== 'flow_token') allKeys.add(k) }) })

  const headers = [t('flowResponses.phoneNumber'), t('flowResponses.submittedAt'), ...Array.from(allKeys).map(k => formatKey(k))]
  const rows = submissions.value.map(sub => {
    const row = [
      sub.phone_number,
      sub.created_at_display,
      ...Array.from(allKeys).map(k => {
        const val = sub.response_data[k]
        return Array.isArray(val) ? val.join(', ') : (val || '')
      })
    ]
    return row.map(cell => `"${String(cell).replace(/"/g, '""')}"`).join(',')
  })

  const csv = [headers.join(','), ...rows].join('\n')
  const blob = new Blob([csv], { type: 'text/csv' })
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${selectedFlowName.value.replace(/\s+/g, '_')}_submissions.csv`
  a.click()
  window.URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="t('flowResponses.title', { name: selectedFlowName })"
      :subtitle="t('flowResponses.submissionsCollected', { count: submissions.length })"
      :icon="ClipboardList"
      icon-gradient="bg-gradient-to-br from-emerald-500 to-teal-600 shadow-emerald-500/20"
    >
      <template #actions>
        <Button variant="outline" size="sm" @click="goBack">
          <ArrowLeft class="h-4 w-4 mr-2" /> {{ t('flowResponses.backToFlows') }}
        </Button>
        <Button variant="outline" size="sm" @click="refreshData" :disabled="isRefreshing">
          <RefreshCw class="h-4 w-4 mr-2" :class="{ 'animate-spin': isRefreshing }" />
          {{ t('flowResponses.refresh') }}
        </Button>
        <div class="flex items-center gap-2 text-sm text-muted-foreground">
          <div class="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
          {{ t('flowResponses.liveUpdating') }}
        </div>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <!-- Loading State -->
          <div v-if="isLoading" class="flex items-center justify-center py-20">
            <div class="flex flex-col items-center gap-4">
              <div class="h-8 w-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" />
              <p class="text-sm text-muted-foreground">{{ t('flowResponses.loading') }}</p>
            </div>
          </div>

          <!-- Submissions View -->
          <template v-if="!isLoading">
            <!-- Table -->
            <Card>
              <CardHeader class="pb-4 border-b">
                <div class="flex items-center justify-between">
                  <div>
                    <CardTitle class="text-xl flex items-center gap-2">
                      <ClipboardList class="h-5 w-5 text-emerald-500" />
                      {{ t('flowResponses.title', { name: selectedFlowName }) }}
                    </CardTitle>
                    <CardDescription class="mt-1">{{ t('flowResponses.responsesCollected', { count: submissions.length }) }}</CardDescription>
                  </div>
                  <div class="flex items-center gap-4">
                    <div class="flex items-center gap-2 px-3 py-1 bg-emerald-500/10 text-emerald-500 rounded-full text-xs font-medium">
                      <div class="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                      {{ t('flowResponses.live') }}
                    </div>
                    <Button variant="outline" size="sm" @click="exportToCsv" :disabled="submissions.length === 0" class="shadow-sm">
                      <Download class="h-4 w-4 mr-2 text-muted-foreground" />
                      {{ t('flowResponses.exportCsv') }}
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent class="p-0">
                <div class="overflow-x-auto">
                  <table class="w-full">
                    <thead>
                      <tr class="border-b">
                        <th class="text-left p-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('flowResponses.phoneNumber') }}</th>
                        <th class="text-left p-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('flowResponses.submittedAt') }}</th>
                        <th
                          v-for="col in responseColumns"
                          :key="col"
                          class="text-left p-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider"
                        >
                          {{ formatKey(col) }}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="sub in submissions"
                        :key="sub.id"
                        class="border-b last:border-0 hover:bg-muted/30 transition-colors"
                      >
                        <td class="p-4 align-top">
                          <div class="flex items-center gap-2">
                            <div class="p-1.5 rounded bg-blue-500/10">
                              <Phone class="h-3.5 w-3.5 text-blue-500" />
                            </div>
                            <span class="font-medium text-sm">{{ sub.phone_number }}</span>
                          </div>
                        </td>
                        <td class="p-4 align-top text-sm text-muted-foreground">
                          <div class="flex items-center gap-1.5">
                            <Clock class="h-3.5 w-3.5 opacity-70" />
                            {{ sub.created_at_display }}
                          </div>
                        </td>
                        <td
                          v-for="col in responseColumns"
                          :key="col"
                          class="p-4 align-top"
                        >
                          <template v-if="Array.isArray(sub.response_data[col])">
                            <div class="flex flex-wrap gap-1">
                              <Badge v-for="(v, i) in sub.response_data[col]" :key="i" variant="outline" class="bg-indigo-500/10 text-indigo-400 border-indigo-500/20">
                                {{ v }}
                              </Badge>
                            </div>
                          </template>
                          <Badge v-else-if="sub.response_data[col] != null" variant="outline" class="bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
                            {{ sub.response_data[col] }}
                          </Badge>
                          <span v-else class="text-muted-foreground text-xs">—</span>
                        </td>
                      </tr>
                      <tr v-if="submissions.length === 0">
                        <td :colspan="2 + responseColumns.length" class="p-12 text-center">
                          <div class="flex flex-col items-center justify-center">
                            <div class="p-4 rounded-full bg-muted mb-4">
                              <ClipboardList class="h-8 w-8 text-muted-foreground/50" />
                            </div>
                            <h3 class="text-lg font-medium">{{ t('flowResponses.noResponsesYet') }}</h3>
                            <p class="text-sm text-muted-foreground max-w-sm mx-auto mt-1">
                              {{ t('flowResponses.noResponsesDesc') }}
                            </p>
                          </div>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </template>
        </div>
      </div>
    </ScrollArea>
  </div>
</template>
