<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from '@/components/ui/dialog'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import { Plus, Pencil, Trash2, Sparkles, ArrowLeft, FileText, Globe } from 'lucide-vue-next'

interface ApiConfig {
  url: string
  method: string
  headers: Record<string, string>
  body: string
  response_path: string
}

interface AIContext {
  id: string
  name: string
  context_type: string
  trigger_keywords: string[]
  static_content: string
  api_config: ApiConfig
  priority: number
  enabled: boolean
  created_at: string
}

const contexts = ref<AIContext[]>([])
const isLoading = ref(true)
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingContext = ref<AIContext | null>(null)
const deleteDialogOpen = ref(false)
const contextToDelete = ref<AIContext | null>(null)

const formData = ref({
  name: '',
  context_type: 'static',
  trigger_keywords: '',
  static_content: '',
  api_url: '',
  api_method: 'GET',
  api_headers: '',
  api_response_path: '',
  priority: 10,
  enabled: true
})

// Helper to display variable placeholders without Vue parsing issues
const variableExample = (name: string) => `{{${name}}}`

onMounted(async () => {
  await fetchContexts()
})

async function fetchContexts() {
  isLoading.value = true
  try {
    const response = await chatbotService.listAIContexts()
    // API response is wrapped in { status: "success", data: { contexts: [...] } }
    const data = response.data.data || response.data
    contexts.value = data.contexts || []
  } catch (error) {
    console.error('Failed to load AI contexts:', error)
    contexts.value = []
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  editingContext.value = null
  formData.value = {
    name: '',
    context_type: 'static',
    trigger_keywords: '',
    static_content: '',
    api_url: '',
    api_method: 'GET',
    api_headers: '',
    api_response_path: '',
    priority: 10,
    enabled: true
  }
  isDialogOpen.value = true
}

function openEditDialog(context: AIContext) {
  editingContext.value = context
  const apiConfig = context.api_config || {} as ApiConfig
  formData.value = {
    name: context.name,
    context_type: context.context_type || 'static',
    trigger_keywords: (context.trigger_keywords || []).join(', '),
    static_content: context.static_content || '',
    api_url: apiConfig.url || '',
    api_method: apiConfig.method || 'GET',
    api_headers: apiConfig.headers ? JSON.stringify(apiConfig.headers, null, 2) : '',
    api_response_path: apiConfig.response_path || '',
    priority: context.priority || 10,
    enabled: context.enabled
  }
  isDialogOpen.value = true
}

async function saveContext() {
  if (!formData.value.name.trim()) {
    toast.error('Please enter a name')
    return
  }

  if (formData.value.context_type === 'api' && !formData.value.api_url.trim()) {
    toast.error('Please enter an API URL')
    return
  }

  isSubmitting.value = true
  try {
    // Parse headers JSON if provided
    let headers = {}
    if (formData.value.api_headers.trim()) {
      try {
        headers = JSON.parse(formData.value.api_headers)
      } catch (e) {
        toast.error('Invalid JSON format for headers')
        isSubmitting.value = false
        return
      }
    }

    const data: any = {
      name: formData.value.name,
      context_type: formData.value.context_type,
      trigger_keywords: formData.value.trigger_keywords.split(',').map(k => k.trim()).filter(Boolean),
      static_content: formData.value.static_content,
      api_config: formData.value.context_type === 'api' ? {
        url: formData.value.api_url,
        method: formData.value.api_method,
        headers: headers,
        response_path: formData.value.api_response_path
      } : null,
      priority: formData.value.priority,
      enabled: formData.value.enabled
    }

    if (editingContext.value) {
      await chatbotService.updateAIContext(editingContext.value.id, data)
      toast.success('AI context updated')
    } else {
      await chatbotService.createAIContext(data)
      toast.success('AI context created')
    }

    isDialogOpen.value = false
    await fetchContexts()
  } catch (error) {
    toast.error('Failed to save AI context')
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(context: AIContext) {
  contextToDelete.value = context
  deleteDialogOpen.value = true
}

async function confirmDeleteContext() {
  if (!contextToDelete.value) return

  try {
    await chatbotService.deleteAIContext(contextToDelete.value.id)
    toast.success('AI context deleted')
    deleteDialogOpen.value = false
    contextToDelete.value = null
    await fetchContexts()
  } catch (error) {
    toast.error('Failed to delete AI context')
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
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-orange-500 to-amber-600 flex items-center justify-center mr-3 shadow-lg shadow-orange-500/20">
          <Sparkles class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">AI Contexts</h1>
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/chatbot">Chatbot</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>AI Contexts</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </div>
        <Dialog v-model:open="isDialogOpen">
          <DialogTrigger as-child>
            <Button variant="outline" size="sm" @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Add Context
            </Button>
          </DialogTrigger>
          <DialogContent class="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{{ editingContext ? 'Edit' : 'Create' }} AI Context</DialogTitle>
              <DialogDescription>
                Add knowledge context that the AI can use when responding to messages.
              </DialogDescription>
            </DialogHeader>
            <div class="grid gap-4 py-4 max-h-[60vh] overflow-y-auto">
              <div class="grid grid-cols-2 gap-4">
                <div class="space-y-2">
                  <Label for="name">Name *</Label>
                  <Input
                    id="name"
                    v-model="formData.name"
                    placeholder="Product FAQ"
                  />
                </div>
                <div class="space-y-2">
                  <Label for="context_type">Type</Label>
                  <Select v-model="formData.context_type">
                    <SelectTrigger>
                      <SelectValue placeholder="Select type" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="static">Static Content</SelectItem>
                      <SelectItem value="api">API Fetch</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div class="space-y-2">
                <Label for="trigger_keywords">Trigger Keywords (comma-separated, optional)</Label>
                <Input
                  id="trigger_keywords"
                  v-model="formData.trigger_keywords"
                  placeholder="faq, help, info"
                />
                <p class="text-xs text-muted-foreground">
                  Leave empty to always include this context, or specify keywords to include only when mentioned.
                </p>
              </div>

              <!-- Content/Prompt Field - always shown -->
              <div class="space-y-2">
                <Label for="static_content">Content / Prompt</Label>
                <Textarea
                  id="static_content"
                  v-model="formData.static_content"
                  placeholder="Enter knowledge content or prompt for the AI..."
                  :rows="6"
                />
                <p class="text-xs text-muted-foreground">
                  This content will be provided to the AI as context for generating responses.
                </p>
              </div>

              <!-- API Configuration Fields - shown only for API type -->
              <div v-if="formData.context_type === 'api'" class="space-y-4 border-t pt-4">
                <p class="text-sm font-medium">API Configuration</p>
                <p class="text-xs text-muted-foreground">
                  Data fetched from this API will be combined with the content above.
                </p>

                <div class="grid grid-cols-4 gap-4">
                  <div class="col-span-1 space-y-2">
                    <Label for="api_method">Method</Label>
                    <Select v-model="formData.api_method">
                      <SelectTrigger>
                        <SelectValue placeholder="Method" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="GET">GET</SelectItem>
                        <SelectItem value="POST">POST</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div class="col-span-3 space-y-2">
                    <Label for="api_url">API URL *</Label>
                    <Input
                      id="api_url"
                      v-model="formData.api_url"
                      placeholder="https://api.example.com/context"
                    />
                  </div>
                </div>
                <p class="text-xs text-muted-foreground">
                  Variables: <code class="bg-muted px-1 rounded">{{ variableExample('phone_number') }}</code>, <code class="bg-muted px-1 rounded">{{ variableExample('user_message') }}</code>
                </p>

                <div class="space-y-2">
                  <Label for="api_headers">Headers (JSON, optional)</Label>
                  <Textarea
                    id="api_headers"
                    v-model="formData.api_headers"
                    placeholder='{"Authorization": "Bearer xxx"}'
                    :rows="2"
                  />
                </div>

                <div class="space-y-2">
                  <Label for="api_response_path">Response Path (optional)</Label>
                  <Input
                    id="api_response_path"
                    v-model="formData.api_response_path"
                    placeholder="data.context"
                  />
                  <p class="text-xs text-muted-foreground">
                    Dot-notation path to extract from JSON response.
                  </p>
                </div>
              </div>

              <div class="grid grid-cols-2 gap-4">
                <div class="space-y-2">
                  <Label for="priority">Priority</Label>
                  <Input
                    id="priority"
                    v-model.number="formData.priority"
                    type="number"
                    min="1"
                    max="100"
                  />
                  <p class="text-xs text-muted-foreground">Higher priority contexts are used first</p>
                </div>
                <div class="flex items-center gap-2 pt-8">
                  <Switch
                    id="enabled"
                    :checked="formData.enabled"
                    @update:checked="formData.enabled = $event"
                  />
                  <Label for="enabled">Enabled</Label>
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
              <Button size="sm" @click="saveContext" :disabled="isSubmitting">
                {{ editingContext ? 'Update' : 'Create' }}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </header>

    <!-- Contexts List -->
    <ScrollArea class="flex-1">
      <div class="p-6 grid gap-4 md:grid-cols-2">
        <!-- Loading Skeleton -->
        <template v-if="isLoading">
          <div v-for="i in 4" :key="i" class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="p-6">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-3">
                  <Skeleton class="h-10 w-10 rounded-lg bg-white/[0.08] light:bg-gray-200" />
                  <div>
                    <Skeleton class="h-5 w-32 mb-1 bg-white/[0.08] light:bg-gray-200" />
                    <Skeleton class="h-4 w-24 bg-white/[0.08] light:bg-gray-200" />
                  </div>
                </div>
                <Skeleton class="h-5 w-16 bg-white/[0.08] light:bg-gray-200" />
              </div>
            </div>
            <div class="px-6 pb-6">
              <div class="flex flex-wrap gap-1 mb-3">
                <Skeleton class="h-5 w-12 bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-5 w-16 bg-white/[0.08] light:bg-gray-200" />
              </div>
              <Skeleton class="h-5 w-24 mb-3 bg-white/[0.08] light:bg-gray-200" />
              <div class="flex gap-2">
                <Skeleton class="h-8 w-8 rounded bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-8 w-8 rounded bg-white/[0.08] light:bg-gray-200" />
              </div>
            </div>
          </div>
        </template>

        <template v-else>
        <div v-for="context in contexts" :key="context.id" class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
          <div class="p-6">
            <div class="flex items-start justify-between">
              <div class="flex items-center gap-3">
                <div
                  class="h-10 w-10 rounded-lg flex items-center justify-center shadow-lg"
                  :class="context.context_type === 'api' ? 'bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20' : 'bg-gradient-to-br from-orange-500 to-amber-600 shadow-orange-500/20'"
                >
                  <Globe v-if="context.context_type === 'api'" class="h-5 w-5 text-white" />
                  <FileText v-else class="h-5 w-5 text-white" />
                </div>
                <div>
                  <h3 class="text-base font-semibold text-white light:text-gray-900">{{ context.name }}</h3>
                  <p class="text-sm text-white/50 light:text-gray-500">{{ context.context_type === 'api' ? 'API Fetch' : 'Static Content' }}</p>
                </div>
              </div>
              <Badge
                :class="context.enabled ? 'bg-emerald-500/20 text-emerald-400 border-transparent light:bg-emerald-100 light:text-emerald-700' : 'bg-white/[0.08] text-white/50 border-transparent light:bg-gray-100 light:text-gray-500'"
              >
                {{ context.enabled ? 'Active' : 'Inactive' }}
              </Badge>
            </div>
          </div>
          <div class="px-6 pb-6">
            <div class="flex flex-wrap gap-1 mb-3" v-if="context.trigger_keywords?.length">
              <Badge v-for="kw in context.trigger_keywords" :key="kw" variant="outline" class="text-xs border-white/20 text-white/70 light:border-gray-200 light:text-gray-600">
                {{ kw }}
              </Badge>
            </div>
            <div class="flex flex-wrap gap-2 mb-3">
              <Badge variant="secondary">Priority: {{ context.priority }}</Badge>
            </div>
            <div class="flex items-center justify-between">
              <div class="flex gap-2">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openEditDialog(context)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Edit context</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openDeleteDialog(context)">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Delete context</TooltipContent>
                </Tooltip>
              </div>
            </div>
          </div>
        </div>

        <div v-if="contexts.length === 0" class="col-span-full rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
          <div class="py-12 text-center text-white/50 light:text-gray-500">
            <div class="h-16 w-16 rounded-xl bg-gradient-to-br from-orange-500 to-amber-600 flex items-center justify-center mx-auto mb-4 shadow-lg shadow-orange-500/20">
              <Sparkles class="h-8 w-8 text-white" />
            </div>
            <p class="text-lg font-medium text-white light:text-gray-900">No AI contexts yet</p>
            <p class="text-sm mb-4">Create knowledge contexts that the AI can use to answer questions.</p>
            <Button variant="outline" size="sm" @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Create Context
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
          <AlertDialogTitle>Delete AI Context</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ contextToDelete?.name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteContext">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
