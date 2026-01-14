<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
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
import FlowBuilder from '@/components/flow-builder/FlowBuilder.vue'
import { flowsService, accountsService } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Workflow, Play, ExternalLink, Loader2, Archive, RefreshCw, Upload, Copy } from 'lucide-vue-next'

interface WhatsAppFlow {
  id: string
  whatsapp_account: string
  meta_flow_id: string
  name: string
  status: 'DRAFT' | 'PUBLISHED' | 'DEPRECATED'
  category: string
  json_version: string
  flow_json: Record<string, any>
  screens: any[]
  preview_url?: string
  has_local_changes: boolean
  created_at: string
  updated_at: string
}

interface Account {
  id: string
  name: string
}

// Meta allowed flow categories
const flowCategories = [
  { value: 'SIGN_UP', label: 'Sign Up' },
  { value: 'SIGN_IN', label: 'Sign In' },
  { value: 'APPOINTMENT_BOOKING', label: 'Appointment Booking' },
  { value: 'LEAD_GENERATION', label: 'Lead Generation' },
  { value: 'CONTACT_US', label: 'Contact Us' },
  { value: 'CUSTOMER_SUPPORT', label: 'Customer Support' },
  { value: 'SURVEY', label: 'Survey' },
  { value: 'OTHER', label: 'Other' },
]

const flows = ref<WhatsAppFlow[]>([])
const accounts = ref<Account[]>([])
const isLoading = ref(true)
const selectedAccount = ref<string>(localStorage.getItem('flows_selected_account') || 'all')

// Dialog state
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const isCreating = ref(false)
const isUpdating = ref(false)
const isSyncing = ref(false)
const savingToMetaFlowId = ref<string | null>(null)
const publishingFlowId = ref<string | null>(null)
const duplicatingFlowId = ref<string | null>(null)
const deleteDialogOpen = ref(false)
const flowToDelete = ref<WhatsAppFlow | null>(null)
const flowToEdit = ref<WhatsAppFlow | null>(null)

// Form state for create
const formData = ref({
  whatsapp_account: '',
  name: '',
  category: '',
  json_version: '6.0'
})

// Form state for edit
const editFormData = ref({
  name: '',
  category: '',
  json_version: '6.0'
})

// Flow builder state
const flowBuilderData = ref<{ screens: any[] }>({ screens: [] })
const editFlowBuilderData = ref<{ screens: any[] }>({ screens: [] })

onMounted(async () => {
  await fetchAccounts()
  await fetchFlows()
})

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    accounts.value = response.data.data?.accounts || []
    // Validate stored account still exists
    if (selectedAccount.value !== 'all' && !accounts.value.some(a => a.name === selectedAccount.value)) {
      selectedAccount.value = 'all'
      localStorage.setItem('flows_selected_account', 'all')
    }
  } catch (error) {
    console.error('Failed to fetch accounts:', error)
  }
}

function onAccountChange(value: string) {
  localStorage.setItem('flows_selected_account', value)
  fetchFlows()
}

async function fetchFlows() {
  isLoading.value = true
  try {
    const params = selectedAccount.value && selectedAccount.value !== 'all'
      ? `?account=${selectedAccount.value}`
      : ''
    const response = await flowsService.list()
    flows.value = response.data.data?.flows || []

    // Filter by account if selected
    if (selectedAccount.value && selectedAccount.value !== 'all') {
      flows.value = flows.value.filter(f => f.whatsapp_account === selectedAccount.value)
    }
  } catch (error) {
    console.error('Failed to fetch flows:', error)
    flows.value = []
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  formData.value = {
    whatsapp_account: (selectedAccount.value && selectedAccount.value !== 'all')
      ? selectedAccount.value
      : (accounts.value[0]?.name || ''),
    name: '',
    category: '',
    json_version: '6.0'
  }
  flowBuilderData.value = { screens: [] }
  showCreateDialog.value = true
}

async function createFlow() {
  if (!formData.value.name) {
    toast.error('Please enter a flow name')
    return
  }
  if (!formData.value.whatsapp_account) {
    toast.error('Please select a WhatsApp account')
    return
  }

  isCreating.value = true
  try {
    const payload: any = {
      whatsapp_account: formData.value.whatsapp_account,
      name: formData.value.name,
      category: formData.value.category || undefined,
      json_version: formData.value.json_version
    }

    // Build flow_json from the builder (sanitize for Meta API)
    if (flowBuilderData.value.screens.length > 0) {
      const sanitizedScreens = sanitizeScreensForMeta(flowBuilderData.value.screens)
      payload.flow_json = {
        version: formData.value.json_version,
        screens: sanitizedScreens
      }
      payload.screens = sanitizedScreens
    }

    await flowsService.create(payload)
    toast.success('Flow created successfully')
    showCreateDialog.value = false
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to create flow'
    toast.error(message)
  } finally {
    isCreating.value = false
  }
}

function openEditDialog(flow: WhatsAppFlow) {
  flowToEdit.value = flow
  editFormData.value = {
    name: flow.name,
    category: flow.category || '',
    json_version: flow.json_version || '6.0'
  }
  // Handle null/undefined screens - ensure it's always an array
  const screens = Array.isArray(flow.screens) ? flow.screens : []
  editFlowBuilderData.value = { screens }
  showEditDialog.value = true
}

function isFlowDraft(flow: WhatsAppFlow): boolean {
  // Case-insensitive check for DRAFT status
  return flow.status?.toUpperCase() === 'DRAFT'
}

async function updateFlow() {
  if (!flowToEdit.value) return

  if (!editFormData.value.name) {
    toast.error('Please enter a flow name')
    return
  }

  isUpdating.value = true
  try {
    const payload: any = {
      name: editFormData.value.name,
      category: editFormData.value.category || undefined,
      json_version: editFormData.value.json_version
    }

    // Build flow_json from the builder (sanitize for Meta API)
    if (editFlowBuilderData.value.screens.length > 0) {
      const sanitizedScreens = sanitizeScreensForMeta(editFlowBuilderData.value.screens)
      payload.flow_json = {
        version: editFormData.value.json_version,
        screens: sanitizedScreens
      }
      payload.screens = sanitizedScreens
    }

    await flowsService.update(flowToEdit.value.id, payload)
    toast.success('Flow updated successfully')
    showEditDialog.value = false
    flowToEdit.value = null
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to update flow'
    toast.error(message)
  } finally {
    isUpdating.value = false
  }
}

async function saveFlowToMeta(flow: WhatsAppFlow) {
  savingToMetaFlowId.value = flow.id
  try {
    await flowsService.saveToMeta(flow.id)
    toast.success('Flow saved to Meta successfully')
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to save flow to Meta'
    toast.error(message)
  } finally {
    savingToMetaFlowId.value = null
  }
}

async function publishFlow(flow: WhatsAppFlow) {
  publishingFlowId.value = flow.id
  try {
    await flowsService.publish(flow.id)
    toast.success('Flow published successfully')
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to publish flow'
    toast.error(message)
  } finally {
    publishingFlowId.value = null
  }
}

function openDeleteDialog(flow: WhatsAppFlow) {
  flowToDelete.value = flow
  deleteDialogOpen.value = true
}

async function confirmDeleteFlow() {
  if (!flowToDelete.value) return

  try {
    await flowsService.delete(flowToDelete.value.id)
    toast.success('Flow deleted')
    deleteDialogOpen.value = false
    flowToDelete.value = null
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete flow'
    toast.error(message)
  }
}

async function duplicateFlow(flow: WhatsAppFlow) {
  duplicatingFlowId.value = flow.id
  try {
    await flowsService.duplicate(flow.id)
    toast.success('Flow duplicated successfully')
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to duplicate flow'
    toast.error(message)
  } finally {
    duplicatingFlowId.value = null
  }
}

async function syncFlows() {
  if (!selectedAccount.value || selectedAccount.value === 'all') {
    toast.error('Please select a specific WhatsApp account to sync')
    return
  }

  isSyncing.value = true
  try {
    const response = await flowsService.sync(selectedAccount.value)
    const data = response.data.data
    toast.success(`Synced ${data.synced} flows (${data.created} new, ${data.updated} updated)`)
    await fetchFlows()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to sync flows'
    toast.error(message)
  } finally {
    isSyncing.value = false
  }
}

function getStatusClass(status: string): string {
  switch (status) {
    case 'PUBLISHED':
      return 'border-green-600 text-green-600'
    case 'DEPRECATED':
      return 'border-destructive text-destructive'
    default:
      return ''
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

// Components that should NOT have an 'id' property when sent to Meta API
const componentsWithoutId = [
  'TextHeading',
  'TextSubheading',
  'TextBody',
  'TextInput',
  'TextArea',
  'Dropdown',
  'RadioButtonsGroup',
  'CheckboxGroup',
  'DatePicker',
  'Image',
  'Footer'
]

// Sanitize flow screens for Meta API by removing 'id' from components that don't support it
function sanitizeScreensForMeta(screens: any[]): any[] {
  return screens.map(screen => ({
    id: screen.id,
    title: screen.title,
    data: screen.data || {},
    layout: {
      type: screen.layout?.type || 'SingleColumnLayout',
      children: (screen.layout?.children || []).map((comp: any) => {
        // Create a copy without the 'id' if component type doesn't support it
        const { id, ...rest } = comp
        if (componentsWithoutId.includes(comp.type)) {
          return rest
        }
        return comp
      })
    }
  }))
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <Workflow class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">WhatsApp Flows</h1>
          <p class="text-sm text-muted-foreground">Create interactive flows for your customers</p>
        </div>
        <div class="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            @click="syncFlows"
            :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'"
          >
            <RefreshCw :class="['h-4 w-4 mr-2', isSyncing && 'animate-spin']" />
            Sync from Meta
          </Button>
          <Button variant="outline" size="sm" @click="openCreateDialog">
            <Plus class="h-4 w-4 mr-2" />
            Create Flow
          </Button>
        </div>
      </div>
    </header>

    <!-- Filters -->
    <div class="p-4 border-b flex items-center gap-4 flex-wrap">
      <div class="flex items-center gap-2">
        <Label class="text-sm text-muted-foreground">Account:</Label>
        <Select v-model="selectedAccount" @update:model-value="onAccountChange">
          <SelectTrigger class="w-[180px]">
            <SelectValue placeholder="All Accounts" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Accounts</SelectItem>
            <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
              {{ account.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex-1 p-6">
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card v-for="i in 3" :key="i">
          <CardHeader>
            <div class="flex items-center gap-3">
              <Skeleton class="h-10 w-10 rounded-lg" />
              <div class="space-y-2">
                <Skeleton class="h-4 w-32" />
                <Skeleton class="h-3 w-24" />
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <Skeleton class="h-6 w-20" />
          </CardContent>
        </Card>
      </div>
    </div>

    <!-- Flows List -->
    <ScrollArea v-else class="flex-1">
      <div class="p-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card v-for="flow in flows" :key="flow.id">
          <CardHeader>
            <div class="flex items-start justify-between">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-lg bg-indigo-100 dark:bg-indigo-900 flex items-center justify-center">
                  <Workflow class="h-5 w-5 text-indigo-600 dark:text-indigo-400" />
                </div>
                <div>
                  <CardTitle class="text-base">{{ flow.name }}</CardTitle>
                  <p class="text-xs text-muted-foreground">{{ flow.whatsapp_account }}</p>
                </div>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div class="flex flex-wrap gap-2 mb-3">
              <Badge variant="outline" :class="getStatusClass(flow.status)">
                {{ flow.status }}
              </Badge>
              <Badge v-if="flow.category" variant="outline">
                {{ flow.category }}
              </Badge>
            </div>
            <p class="text-xs text-muted-foreground">
              Created {{ formatDate(flow.created_at) }}
            </p>
          </CardContent>
          <div class="px-6 pb-4 flex items-center justify-between border-t pt-4">
            <div class="flex gap-2">
              <Button
                variant="ghost"
                size="icon"
                @click="openEditDialog(flow)"
                title="Edit flow"
              >
                <Pencil class="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                @click="duplicateFlow(flow)"
                :disabled="duplicatingFlowId === flow.id"
                title="Duplicate flow"
              >
                <Loader2 v-if="duplicatingFlowId === flow.id" class="h-4 w-4 animate-spin" />
                <Copy v-else class="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                @click="openDeleteDialog(flow)"
                :disabled="flow.status?.toUpperCase() === 'PUBLISHED'"
                title="Delete flow"
              >
                <Trash2 class="h-4 w-4 text-destructive" />
              </Button>
            </div>
            <div class="flex gap-2">
              <Button
                v-if="flow.preview_url"
                variant="outline"
                size="sm"
                as="a"
                :href="flow.preview_url"
                target="_blank"
              >
                <ExternalLink class="h-4 w-4 mr-1" />
                Preview
              </Button>
              <Button
                v-if="flow.status?.toUpperCase() !== 'DEPRECATED' && (flow.has_local_changes || !flow.meta_flow_id)"
                variant="outline"
                size="sm"
                @click="saveFlowToMeta(flow)"
                :disabled="savingToMetaFlowId === flow.id || publishingFlowId === flow.id"
              >
                <Loader2 v-if="savingToMetaFlowId === flow.id" class="h-4 w-4 mr-1 animate-spin" />
                <Upload v-else class="h-4 w-4 mr-1" />
                {{ flow.meta_flow_id ? 'Update on Meta' : 'Save to Meta' }}
              </Button>
              <Button
                v-if="isFlowDraft(flow) && flow.meta_flow_id"
                size="sm"
                @click="publishFlow(flow)"
                :disabled="savingToMetaFlowId === flow.id || publishingFlowId === flow.id"
              >
                <Loader2 v-if="publishingFlowId === flow.id" class="h-4 w-4 mr-1 animate-spin" />
                <Play v-else class="h-4 w-4 mr-1" />
                Publish
              </Button>
              <Badge v-if="flow.status?.toUpperCase() === 'DEPRECATED'" variant="destructive">
                <Archive class="h-3 w-3 mr-1" />
                Deprecated
              </Badge>
            </div>
          </div>
        </Card>

        <Card v-if="flows.length === 0" class="col-span-full">
          <CardContent class="py-12 text-center text-muted-foreground">
            <Workflow class="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p class="text-lg font-medium">No WhatsApp Flows yet</p>
            <p class="text-sm mb-4">Create interactive flows to engage your customers.</p>
            <Button variant="outline" size="sm" @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Create Flow
            </Button>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Create Flow Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-6xl h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>Create WhatsApp Flow</DialogTitle>
          <DialogDescription>
            Design an interactive flow for WhatsApp using the visual builder.
          </DialogDescription>
        </DialogHeader>

        <!-- Flow Settings -->
        <div class="flex gap-4 py-2 border-b">
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Account:</Label>
            <Select v-model="formData.whatsapp_account" :disabled="isCreating">
              <SelectTrigger class="w-[180px]">
                <SelectValue placeholder="Select an account" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                  {{ account.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Name:</Label>
            <Input
              v-model="formData.name"
              placeholder="Flow name"
              class="w-48"
              :disabled="isCreating"
            />
          </div>
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Category:</Label>
            <Select v-model="formData.category" :disabled="isCreating">
              <SelectTrigger class="w-[180px]">
                <SelectValue placeholder="Select category" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="cat in flowCategories" :key="cat.value" :value="cat.value">
                  {{ cat.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <!-- Flow Builder -->
        <div class="flex-1 overflow-hidden py-4">
          <FlowBuilder v-model="flowBuilderData" />
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="showCreateDialog = false" :disabled="isCreating">
            Cancel
          </Button>
          <Button size="sm" @click="createFlow" :disabled="isCreating">
            <Loader2 v-if="isCreating" class="h-4 w-4 mr-2 animate-spin" />
            Create Flow
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Edit Flow Dialog -->
    <Dialog v-model:open="showEditDialog">
      <DialogContent class="max-w-6xl h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>Edit WhatsApp Flow</DialogTitle>
          <DialogDescription>
            Modify your flow and save changes locally, then push to Meta.
          </DialogDescription>
        </DialogHeader>

        <!-- Flow Settings -->
        <div class="flex gap-4 py-2 border-b">
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Account:</Label>
            <span class="text-sm text-muted-foreground">{{ flowToEdit?.whatsapp_account }}</span>
          </div>
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Name:</Label>
            <Input
              v-model="editFormData.name"
              placeholder="Flow name"
              class="w-48"
              :disabled="isUpdating"
            />
          </div>
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Category:</Label>
            <Select v-model="editFormData.category" :disabled="isUpdating">
              <SelectTrigger class="w-[180px]">
                <SelectValue placeholder="Select category" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="cat in flowCategories" :key="cat.value" :value="cat.value">
                  {{ cat.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="flowToEdit?.meta_flow_id" class="flex items-center gap-2 ml-auto">
            <Badge variant="outline">
              Meta ID: {{ flowToEdit.meta_flow_id }}
            </Badge>
          </div>
        </div>

        <!-- Flow Builder -->
        <div class="flex-1 overflow-hidden py-4">
          <FlowBuilder v-model="editFlowBuilderData" />
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="showEditDialog = false" :disabled="isUpdating">
            Cancel
          </Button>
          <Button size="sm" @click="updateFlow" :disabled="isUpdating">
            <Loader2 v-if="isUpdating" class="h-4 w-4 mr-2 animate-spin" />
            Save Changes
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

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
