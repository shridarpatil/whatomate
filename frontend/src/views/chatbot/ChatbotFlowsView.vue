<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Workflow, ArrowLeft, Play, Pause } from 'lucide-vue-next'

interface ChatbotFlow {
  id: string
  name: string
  description: string
  trigger_keywords: string[]
  steps_count: number
  enabled: boolean
  created_at: string
}

const router = useRouter()
const flows = ref<ChatbotFlow[]>([])
const isLoading = ref(true)
const deleteDialogOpen = ref(false)
const flowToDelete = ref<ChatbotFlow | null>(null)

onMounted(async () => {
  await fetchFlows()
})

async function fetchFlows() {
  isLoading.value = true
  try {
    const response = await chatbotService.listFlows()
    const data = response.data.data || response.data
    flows.value = data.flows || []
  } catch (error) {
    console.error('Failed to load flows:', error)
    flows.value = []
  } finally {
    isLoading.value = false
  }
}

function createFlow() {
  router.push('/chatbot/flows/new')
}

function editFlow(flow: ChatbotFlow) {
  router.push(`/chatbot/flows/${flow.id}/edit`)
}

async function toggleFlow(flow: ChatbotFlow) {
  try {
    await chatbotService.updateFlow(flow.id, { enabled: !flow.enabled })
    flow.enabled = !flow.enabled
    toast.success(flow.enabled ? 'Flow enabled' : 'Flow disabled')
  } catch (error) {
    toast.error('Failed to toggle flow')
  }
}

function openDeleteDialog(flow: ChatbotFlow) {
  flowToDelete.value = flow
  deleteDialogOpen.value = true
}

async function confirmDeleteFlow() {
  if (!flowToDelete.value) return

  try {
    await chatbotService.deleteFlow(flowToDelete.value.id)
    toast.success('Flow deleted')
    deleteDialogOpen.value = false
    flowToDelete.value = null
    await fetchFlows()
  } catch (error) {
    toast.error('Failed to delete flow')
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <RouterLink to="/chatbot">
          <Button variant="ghost" size="icon" class="mr-3">
            <ArrowLeft class="h-5 w-5" />
          </Button>
        </RouterLink>
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center mr-3 shadow-lg shadow-purple-500/20">
          <Workflow class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Conversation Flows</h1>
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/chatbot">Chatbot</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Flows</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </div>
        <Button variant="outline" size="sm" @click="createFlow">
          <Plus class="h-4 w-4 mr-2" />
          Create Flow
        </Button>
      </div>
    </header>

    <!-- Flows List -->
    <ScrollArea class="flex-1">
      <div class="p-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <!-- Loading Skeleton -->
        <template v-if="isLoading">
          <div v-for="i in 6" :key="i" class="flex flex-col rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="p-6">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-3">
                  <Skeleton class="h-10 w-10 rounded-lg bg-white/[0.08] light:bg-gray-200" />
                  <div>
                    <Skeleton class="h-5 w-32 mb-2 bg-white/[0.08] light:bg-gray-200" />
                    <Skeleton class="h-5 w-16 bg-white/[0.08] light:bg-gray-200" />
                  </div>
                </div>
              </div>
            </div>
            <div class="px-6 pb-6 flex-1">
              <Skeleton class="h-4 w-full mb-3 bg-white/[0.08] light:bg-gray-200" />
              <div class="flex gap-1 mb-3">
                <Skeleton class="h-5 w-14 bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-5 w-16 bg-white/[0.08] light:bg-gray-200" />
              </div>
              <Skeleton class="h-4 w-20 bg-white/[0.08] light:bg-gray-200" />
            </div>
            <div class="p-4 flex items-center justify-between border-t border-white/[0.08] light:border-gray-200 mt-auto">
              <div class="flex gap-2">
                <Skeleton class="h-8 w-8 rounded bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-8 w-8 rounded bg-white/[0.08] light:bg-gray-200" />
              </div>
              <Skeleton class="h-8 w-20 bg-white/[0.08] light:bg-gray-200" />
            </div>
          </div>
        </template>

        <template v-else>
          <div v-for="flow in flows" :key="flow.id" class="flex flex-col rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="p-6">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-3">
                  <div class="h-10 w-10 rounded-lg bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center shadow-lg shadow-purple-500/20">
                    <Workflow class="h-5 w-5 text-white" />
                  </div>
                  <div>
                    <h3 class="text-base font-semibold text-white light:text-gray-900">{{ flow.name }}</h3>
                    <Badge
                      :class="flow.enabled ? 'bg-emerald-500/20 text-emerald-400 border-transparent mt-1 light:bg-emerald-100 light:text-emerald-700' : 'bg-white/[0.08] text-white/50 border-transparent mt-1 light:bg-gray-100 light:text-gray-500'"
                    >
                      {{ flow.enabled ? 'Active' : 'Inactive' }}
                    </Badge>
                  </div>
                </div>
              </div>
            </div>
            <div class="px-6 pb-6 flex-1">
              <p class="text-sm text-white/50 light:text-gray-500 mb-3">{{ flow.description || 'No description' }}</p>
              <div class="flex flex-wrap gap-1 mb-3" v-if="flow.trigger_keywords?.length">
                <Badge v-for="keyword in flow.trigger_keywords" :key="keyword" variant="outline" class="border-white/20 text-white/70 light:border-gray-200 light:text-gray-600">
                  {{ keyword }}
                </Badge>
              </div>
              <p class="text-xs text-white/40 light:text-gray-400">{{ flow.steps_count }} steps</p>
            </div>
            <div class="p-4 flex items-center justify-between border-t border-white/[0.08] light:border-gray-200 mt-auto">
              <div class="flex gap-2">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="editFlow(flow)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Edit flow</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openDeleteDialog(flow)">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Delete flow</TooltipContent>
                </Tooltip>
              </div>
              <Button
                :variant="flow.enabled ? 'outline' : 'default'"
                size="sm"
                @click="toggleFlow(flow)"
              >
                <component :is="flow.enabled ? Pause : Play" class="h-4 w-4 mr-1" />
                {{ flow.enabled ? 'Disable' : 'Enable' }}
              </Button>
            </div>
          </div>

          <div v-if="flows.length === 0" class="col-span-full rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="py-12 text-center text-white/50 light:text-gray-500">
              <div class="h-16 w-16 rounded-xl bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center mx-auto mb-4 shadow-lg shadow-purple-500/20">
                <Workflow class="h-8 w-8 text-white" />
              </div>
              <p class="text-lg font-medium text-white light:text-gray-900">No conversation flows yet</p>
              <p class="text-sm mb-4">Create your first flow to automate conversations.</p>
              <Button variant="outline" size="sm" @click="createFlow">
                <Plus class="h-4 w-4 mr-2" />
                Create Flow
              </Button>
            </div>
          </div>
        </template>
      </div>
    </ScrollArea>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Flow</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ flowToDelete?.name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteFlow">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
