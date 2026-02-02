<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, DeleteConfirmDialog, DataTable, SearchInput, type Column } from '@/components/shared'
import FlowBuilder from '@/components/flow-builder/FlowBuilder.vue'
import { flowsService, accountsService } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Workflow, Play, ExternalLink, Loader2, Archive, RefreshCw, Upload, Copy } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

interface WhatsAppFlow {
  id: string; whatsapp_account: string; meta_flow_id: string; name: string; status: 'DRAFT' | 'PUBLISHED' | 'DEPRECATED'
  category: string; json_version: string; flow_json: Record<string, any>; screens: any[]; preview_url?: string
  has_local_changes: boolean; created_at: string; updated_at: string
}
interface Account { id: string; name: string }

const flowCategories = [
  { value: 'SIGN_UP', label: 'Sign Up' }, { value: 'SIGN_IN', label: 'Sign In' }, { value: 'APPOINTMENT_BOOKING', label: 'Appointment Booking' },
  { value: 'LEAD_GENERATION', label: 'Lead Generation' }, { value: 'CONTACT_US', label: 'Contact Us' }, { value: 'CUSTOMER_SUPPORT', label: 'Customer Support' },
  { value: 'SURVEY', label: 'Survey' }, { value: 'OTHER', label: 'Other' },
]

const flows = ref<WhatsAppFlow[]>([])
const accounts = ref<Account[]>([])
const isLoading = ref(true)
const searchQuery = ref('')
const selectedAccount = ref<string>(localStorage.getItem('flows_selected_account') || 'all')

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

const formData = ref({ whatsapp_account: '', name: '', category: '', json_version: '6.0' })
const editFormData = ref({ name: '', category: '', json_version: '6.0' })
const flowBuilderData = ref<{ screens: any[] }>({ screens: [] })
const editFlowBuilderData = ref<{ screens: any[] }>({ screens: [] })

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns: Column<WhatsAppFlow>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'status', label: 'Status', sortable: true },
  { key: 'category', label: 'Category', sortable: true },
  { key: 'created_at', label: 'Created', sortable: true },
  { key: 'actions', label: 'Actions', align: 'right' },
]

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

onMounted(async () => { await fetchAccounts(); await fetchFlows() })

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    accounts.value = response.data.data?.accounts || []
    if (selectedAccount.value !== 'all' && !accounts.value.some(a => a.name === selectedAccount.value)) {
      selectedAccount.value = 'all'; localStorage.setItem('flows_selected_account', 'all')
    }
  } catch { /* ignore */ }
}

function onAccountChange(value: string | number | bigint | Record<string, any> | null) {
  if (typeof value !== 'string') return
  localStorage.setItem('flows_selected_account', value)
  currentPage.value = 1
  fetchFlows()
}

async function fetchFlows() {
  isLoading.value = true
  try {
    const response = await flowsService.list({
      account: selectedAccount.value !== 'all' ? selectedAccount.value : undefined,
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    flows.value = data.flows || []
    totalItems.value = data.total ?? flows.value.length
  } catch { flows.value = [] }
  finally { isLoading.value = false }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchFlows()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchFlows()
}

function openCreateDialog() {
  formData.value = { whatsapp_account: (selectedAccount.value && selectedAccount.value !== 'all') ? selectedAccount.value : (accounts.value[0]?.name || ''), name: '', category: '', json_version: '6.0' }
  flowBuilderData.value = { screens: [] }; showCreateDialog.value = true
}

async function createFlow() {
  if (!formData.value.name) { toast.error('Please enter a flow name'); return }
  if (!formData.value.whatsapp_account) { toast.error('Please select a WhatsApp account'); return }
  isCreating.value = true
  try {
    const payload: any = { whatsapp_account: formData.value.whatsapp_account, name: formData.value.name, category: formData.value.category || undefined, json_version: formData.value.json_version }
    if (flowBuilderData.value.screens.length > 0) {
      const sanitizedScreens = sanitizeScreensForMeta(flowBuilderData.value.screens)
      payload.flow_json = { version: formData.value.json_version, screens: sanitizedScreens }; payload.screens = sanitizedScreens
    }
    await flowsService.create(payload); toast.success('Flow created successfully'); showCreateDialog.value = false; await fetchFlows()
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to create flow')) }
  finally { isCreating.value = false }
}

function openEditDialog(flow: WhatsAppFlow) {
  flowToEdit.value = flow
  editFormData.value = { name: flow.name, category: flow.category || '', json_version: flow.json_version || '6.0' }
  editFlowBuilderData.value = { screens: Array.isArray(flow.screens) ? flow.screens : [] }; showEditDialog.value = true
}

async function updateFlow() {
  if (!flowToEdit.value) return
  if (!editFormData.value.name) { toast.error('Please enter a flow name'); return }
  isUpdating.value = true
  try {
    const payload: any = { name: editFormData.value.name, category: editFormData.value.category || undefined, json_version: editFormData.value.json_version }
    if (editFlowBuilderData.value.screens.length > 0) {
      const sanitizedScreens = sanitizeScreensForMeta(editFlowBuilderData.value.screens)
      payload.flow_json = { version: editFormData.value.json_version, screens: sanitizedScreens }; payload.screens = sanitizedScreens
    }
    await flowsService.update(flowToEdit.value.id, payload); toast.success('Flow updated successfully'); showEditDialog.value = false; flowToEdit.value = null; await fetchFlows()
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to update flow')) }
  finally { isUpdating.value = false }
}

async function saveFlowToMeta(flow: WhatsAppFlow) {
  savingToMetaFlowId.value = flow.id
  try { await flowsService.saveToMeta(flow.id); toast.success('Flow saved to Meta successfully'); await fetchFlows() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to save flow to Meta')) }
  finally { savingToMetaFlowId.value = null }
}

async function publishFlow(flow: WhatsAppFlow) {
  publishingFlowId.value = flow.id
  try { await flowsService.publish(flow.id); toast.success('Flow published successfully'); await fetchFlows() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to publish flow')) }
  finally { publishingFlowId.value = null }
}

async function confirmDeleteFlow() {
  if (!flowToDelete.value) return
  try { await flowsService.delete(flowToDelete.value.id); toast.success('Flow deleted'); deleteDialogOpen.value = false; flowToDelete.value = null; await fetchFlows() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to delete flow')) }
}

async function duplicateFlow(flow: WhatsAppFlow) {
  duplicatingFlowId.value = flow.id
  try { await flowsService.duplicate(flow.id); toast.success('Flow duplicated successfully'); await fetchFlows() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to duplicate flow')) }
  finally { duplicatingFlowId.value = null }
}

async function syncFlows() {
  if (!selectedAccount.value || selectedAccount.value === 'all') { toast.error('Please select a specific WhatsApp account to sync'); return }
  isSyncing.value = true
  try { const response = await flowsService.sync(selectedAccount.value); const data = response.data.data; toast.success(`Synced ${data.synced} flows (${data.created} new, ${data.updated} updated)`); await fetchFlows() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to sync flows')) }
  finally { isSyncing.value = false }
}

function getStatusClass(status: string): string { return { PUBLISHED: 'border-green-600 text-green-600', DEPRECATED: 'border-destructive text-destructive' }[status] || '' }
function isFlowDraft(flow: WhatsAppFlow): boolean { return flow.status?.toUpperCase() === 'DRAFT' }

const componentsWithoutId = ['TextHeading', 'TextSubheading', 'TextBody', 'TextInput', 'TextArea', 'Dropdown', 'RadioButtonsGroup', 'CheckboxGroup', 'DatePicker', 'Image', 'Footer']
function sanitizeScreensForMeta(screens: any[]): any[] {
  return screens.map(screen => ({
    id: screen.id, title: screen.title, data: screen.data || {},
    layout: { type: screen.layout?.type || 'SingleColumnLayout', children: (screen.layout?.children || []).map((comp: any) => { const { id, ...rest } = comp; return componentsWithoutId.includes(comp.type) ? rest : comp }) }
  }))
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="WhatsApp Flows" subtitle="Create interactive flows for your customers" :icon="Workflow" icon-gradient="bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="syncFlows" :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'"><RefreshCw :class="['h-4 w-4 mr-2', isSyncing && 'animate-spin']" />Sync from Meta</Button>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Create Flow</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>Your Flows</CardTitle>
                  <CardDescription>Interactive WhatsApp flows for your customers.</CardDescription>
                </div>
                <div class="flex items-center gap-2">
                  <Label class="text-sm text-muted-foreground">Account:</Label>
                  <Select v-model="selectedAccount" @update:model-value="onAccountChange">
                    <SelectTrigger class="w-[180px]"><SelectValue placeholder="All Accounts" /></SelectTrigger>
                    <SelectContent><SelectItem value="all">All Accounts</SelectItem><SelectItem v-for="account in accounts" :key="account.id" :value="account.name">{{ account.name }}</SelectItem></SelectContent>
                  </Select>
                  <SearchInput v-model="searchQuery" placeholder="Search flows..." class="w-64" />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="flows"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Workflow"
                :empty-title="searchQuery ? 'No matching flows' : 'No WhatsApp Flows yet'"
                :empty-description="searchQuery ? 'No flows match your search.' : 'Create interactive flows to engage your customers.'"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="flows"
                @page-change="handlePageChange"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
              >
                <template #cell-name="{ item: flow }">
                  <div>
                    <span class="font-medium">{{ flow.name }}</span>
                    <p class="text-xs text-muted-foreground">{{ flow.whatsapp_account }}</p>
                  </div>
                </template>
                <template #cell-status="{ item: flow }">
                  <Badge v-if="flow.status?.toUpperCase() === 'DEPRECATED'" variant="destructive" class="text-xs">
                    <Archive class="h-3 w-3 mr-1" />Deprecated
                  </Badge>
                  <Badge v-else variant="outline" :class="[getStatusClass(flow.status), 'text-xs']">{{ flow.status }}</Badge>
                </template>
                <template #cell-category="{ item: flow }">
                  <Badge v-if="flow.category" variant="outline" class="text-xs">{{ flow.category }}</Badge>
                  <span v-else class="text-muted-foreground">—</span>
                </template>
                <template #cell-created_at="{ item: flow }">
                  <span class="text-muted-foreground text-sm">{{ formatDate(flow.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: flow }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(flow)" title="Edit flow">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="duplicateFlow(flow)" :disabled="duplicatingFlowId === flow.id" title="Duplicate flow">
                      <Loader2 v-if="duplicatingFlowId === flow.id" class="h-4 w-4 animate-spin" /><Copy v-else class="h-4 w-4" />
                    </Button>
                    <Button v-if="flow.preview_url" variant="ghost" size="icon" class="h-8 w-8" as="a" :href="flow.preview_url" target="_blank" title="Preview">
                      <ExternalLink class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="flow.status?.toUpperCase() !== 'DEPRECATED' && (flow.has_local_changes || !flow.meta_flow_id)"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="saveFlowToMeta(flow)"
                      :disabled="savingToMetaFlowId === flow.id || publishingFlowId === flow.id"
                      :title="flow.meta_flow_id ? 'Update on Meta' : 'Save to Meta'"
                    >
                      <Loader2 v-if="savingToMetaFlowId === flow.id" class="h-4 w-4 animate-spin" /><Upload v-else class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="isFlowDraft(flow) && flow.meta_flow_id"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-green-600"
                      @click="publishFlow(flow)"
                      :disabled="savingToMetaFlowId === flow.id || publishingFlowId === flow.id"
                      title="Publish"
                    >
                      <Loader2 v-if="publishingFlowId === flow.id" class="h-4 w-4 animate-spin" /><Play v-else class="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-destructive"
                      @click="flowToDelete = flow; deleteDialogOpen = true"
                      :disabled="flow.status?.toUpperCase() === 'PUBLISHED'"
                      title="Delete flow"
                    >
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button v-if="!searchQuery" variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />Create Flow
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Create Flow Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-6xl h-[85vh] flex flex-col">
        <DialogHeader><DialogTitle>Create WhatsApp Flow</DialogTitle><DialogDescription>Design an interactive flow for WhatsApp using the visual builder.</DialogDescription></DialogHeader>
        <div class="flex gap-4 py-2 border-b">
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Account:</Label>
            <Select v-model="formData.whatsapp_account" :disabled="isCreating"><SelectTrigger class="w-[180px]"><SelectValue placeholder="Select an account" /></SelectTrigger><SelectContent><SelectItem v-for="account in accounts" :key="account.id" :value="account.name">{{ account.name }}</SelectItem></SelectContent></Select>
          </div>
          <div class="flex items-center gap-2"><Label class="text-sm whitespace-nowrap">Name:</Label><Input v-model="formData.name" placeholder="Flow name" class="w-48" :disabled="isCreating" /></div>
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Category:</Label>
            <Select v-model="formData.category" :disabled="isCreating"><SelectTrigger class="w-[180px]"><SelectValue placeholder="Select category" /></SelectTrigger><SelectContent><SelectItem v-for="cat in flowCategories" :key="cat.value" :value="cat.value">{{ cat.label }}</SelectItem></SelectContent></Select>
          </div>
        </div>
        <div class="flex-1 overflow-hidden py-4"><FlowBuilder v-model="flowBuilderData" /></div>
        <DialogFooter><Button variant="outline" size="sm" @click="showCreateDialog = false" :disabled="isCreating">Cancel</Button><Button size="sm" @click="createFlow" :disabled="isCreating"><Loader2 v-if="isCreating" class="h-4 w-4 mr-2 animate-spin" />Create Flow</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Edit Flow Dialog -->
    <Dialog v-model:open="showEditDialog">
      <DialogContent class="max-w-6xl h-[85vh] flex flex-col">
        <DialogHeader><DialogTitle>Edit WhatsApp Flow</DialogTitle><DialogDescription>Modify your flow and save changes locally, then push to Meta.</DialogDescription></DialogHeader>
        <div class="flex gap-4 py-2 border-b">
          <div class="flex items-center gap-2"><Label class="text-sm whitespace-nowrap">Account:</Label><span class="text-sm text-muted-foreground">{{ flowToEdit?.whatsapp_account }}</span></div>
          <div class="flex items-center gap-2"><Label class="text-sm whitespace-nowrap">Name:</Label><Input v-model="editFormData.name" placeholder="Flow name" class="w-48" :disabled="isUpdating" /></div>
          <div class="flex items-center gap-2">
            <Label class="text-sm whitespace-nowrap">Category:</Label>
            <Select v-model="editFormData.category" :disabled="isUpdating"><SelectTrigger class="w-[180px]"><SelectValue placeholder="Select category" /></SelectTrigger><SelectContent><SelectItem v-for="cat in flowCategories" :key="cat.value" :value="cat.value">{{ cat.label }}</SelectItem></SelectContent></Select>
          </div>
          <div v-if="flowToEdit?.meta_flow_id" class="flex items-center gap-2 ml-auto"><Badge variant="outline">Meta ID: {{ flowToEdit.meta_flow_id }}</Badge></div>
        </div>
        <div class="flex-1 overflow-hidden py-4"><FlowBuilder v-model="editFlowBuilderData" /></div>
        <DialogFooter><Button variant="outline" size="sm" @click="showEditDialog = false" :disabled="isUpdating">Cancel</Button><Button size="sm" @click="updateFlow" :disabled="isUpdating"><Loader2 v-if="isUpdating" class="h-4 w-4 mr-2 animate-spin" />Save Changes</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" title="Delete Flow" :item-name="flowToDelete?.name" @confirm="confirmDeleteFlow" />
  </div>
</template>
