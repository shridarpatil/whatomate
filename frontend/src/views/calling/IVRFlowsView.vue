<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useCallingStore } from '@/stores/calling'
import { accountsService, type IVRFlow, type IVRFlowData } from '@/services/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Plus, Pencil, Trash2, Phone, RefreshCw } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import DeleteConfirmDialog from '@/components/shared/DeleteConfirmDialog.vue'
import IconButton from '@/components/shared/IconButton.vue'
import ErrorState from '@/components/shared/ErrorState.vue'

const { t } = useI18n()
const router = useRouter()
const store = useCallingStore()

const accounts = ref<{ name: string }[]>([])
const showCreateDialog = ref(false)
const showDeleteConfirm = ref(false)
const deletingFlow = ref<IVRFlow | null>(null)
const deleting = ref(false)
const saving = ref(false)
const fetchError = ref(false)

// Toggle confirmation state
const showToggleActiveConfirm = ref(false)
const showToggleCallStartConfirm = ref(false)
const togglingFlow = ref<IVRFlow | null>(null)

// Create form state
const createForm = ref({
  name: '',
  description: '',
  whatsapp_account: '',
})

function resetCreateForm() {
  createForm.value = {
    name: '',
    description: '',
    whatsapp_account: accounts.value[0]?.name || '',
  }
}

function openCreate() {
  resetCreateForm()
  showCreateDialog.value = true
}

function openEdit(flow: IVRFlow) {
  router.push({ name: 'ivr-flow-editor', params: { id: flow.id } })
}

function confirmDelete(flow: IVRFlow) {
  deletingFlow.value = flow
  showDeleteConfirm.value = true
}

async function createFlow() {
  if (!createForm.value.name.trim()) {
    toast.error(t('calling.nameRequired'))
    return
  }
  if (!createForm.value.whatsapp_account) {
    toast.error(t('calling.accountRequired'))
    return
  }

  saving.value = true
  try {
    // Create with empty v2 flow data
    const emptyFlow: IVRFlowData = {
      version: 2,
      nodes: [],
      edges: [],
      entry_node: '',
    }
    const flow = await store.createIVRFlow({
      name: createForm.value.name,
      description: createForm.value.description,
      whatsapp_account: createForm.value.whatsapp_account,
      menu: emptyFlow,
    })
    showCreateDialog.value = false
    // Navigate to the editor
    const created = (flow as any)?.data?.data || (flow as any)?.data || flow
    if (created?.id) {
      router.push({ name: 'ivr-flow-editor', params: { id: created.id } })
    }
    toast.success(t('calling.flowCreated'))
  } catch {
    toast.error(t('calling.flowSaveFailed'))
  } finally {
    saving.value = false
  }
}

async function deleteFlow() {
  if (!deletingFlow.value) return
  deleting.value = true
  try {
    await store.deleteIVRFlow(deletingFlow.value.id)
    toast.success(t('calling.flowDeleted'))
    showDeleteConfirm.value = false
    deletingFlow.value = null
  } catch {
    toast.error(t('calling.flowDeleteFailed'))
  } finally {
    deleting.value = false
  }
}

function confirmToggleActive(flow: IVRFlow) {
  togglingFlow.value = flow
  showToggleActiveConfirm.value = true
}

async function toggleActive() {
  const flow = togglingFlow.value
  if (!flow) return
  try {
    await store.updateIVRFlow(flow.id, {
      is_active: !flow.is_active,
      is_call_start: flow.is_call_start,
      is_outgoing_end: flow.is_outgoing_end,
    })
    store.fetchIVRFlows()
  } catch {
    toast.error(t('calling.flowSaveFailed'))
  } finally {
    showToggleActiveConfirm.value = false
    togglingFlow.value = null
  }
}

function confirmToggleCallStart(flow: IVRFlow) {
  togglingFlow.value = flow
  showToggleCallStartConfirm.value = true
}

async function toggleCallStart() {
  const flow = togglingFlow.value
  if (!flow) return
  try {
    await store.updateIVRFlow(flow.id, {
      is_active: flow.is_active,
      is_call_start: !flow.is_call_start,
      is_outgoing_end: flow.is_outgoing_end,
    })
    store.fetchIVRFlows()
  } catch {
    toast.error(t('calling.flowSaveFailed'))
  } finally {
    showToggleCallStartConfirm.value = false
    togglingFlow.value = null
  }
}

async function loadFlows() {
  fetchError.value = false
  try {
    await store.fetchIVRFlows()
  } catch {
    fetchError.value = true
  }
}

onMounted(async () => {
  loadFlows()
  try {
    const res = await accountsService.list()
    const data = res.data as any
    accounts.value = data.data?.accounts ?? data.accounts ?? []
  } catch {
    // Ignore
  }
})
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">{{ t('calling.ivrFlows') }}</h1>
        <p class="text-muted-foreground">{{ t('calling.ivrFlowsDesc') }}</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" @click="loadFlows()">
          <RefreshCw class="h-4 w-4 mr-2" />
          {{ t('common.refresh') }}
        </Button>
        <Button @click="openCreate">
          <Plus class="h-4 w-4 mr-2" />
          {{ t('calling.createFlow') }}
        </Button>
      </div>
    </div>

    <!-- Fetch Error -->
    <ErrorState
      v-if="fetchError"
      :title="t('common.error')"
      :description="t('common.failedLoad', { resource: t('calling.ivrFlows') })"
      :retry-label="t('common.retry')"
      @retry="loadFlows"
    />

    <!-- Flows Table -->
    <Card v-if="!fetchError">
      <CardContent class="pt-6">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('calling.name') }}</TableHead>
              <TableHead>{{ t('calling.account') }}</TableHead>
              <TableHead>{{ t('calling.status') }}</TableHead>
              <TableHead>{{ t('calling.options') }}</TableHead>
              <TableHead class="text-right">{{ t('calling.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="flow in store.ivrFlows" :key="flow.id">
              <TableCell>
                <div class="cursor-pointer" @click="openEdit(flow)">
                  <p class="font-medium hover:opacity-80">{{ flow.name }}</p>
                  <p v-if="flow.description" class="text-sm text-muted-foreground">{{ flow.description }}</p>
                </div>
              </TableCell>
              <TableCell>{{ flow.whatsapp_account }}</TableCell>
              <TableCell>
                <div class="flex gap-1.5">
                  <Badge
                    :variant="flow.is_active ? 'default' : 'destructive'"
                    class="cursor-pointer"
                    role="button"
                    tabindex="0"
                    :aria-label="flow.is_active ? t('calling.toggleActiveAriaDisable', { name: flow.name }) : t('calling.toggleActiveAriaEnable', { name: flow.name })"
                    @click="confirmToggleActive(flow)"
                    @keydown.enter="confirmToggleActive(flow)"
                  >
                    {{ flow.is_active ? t('calling.enabled') : t('calling.disabled') }}
                  </Badge>
                  <Badge
                    v-if="flow.is_active"
                    :variant="flow.is_call_start ? 'default' : 'outline'"
                    class="cursor-pointer"
                    role="button"
                    tabindex="0"
                    :aria-label="flow.is_call_start ? t('calling.toggleCallStartAriaDisable', { name: flow.name }) : t('calling.toggleCallStartAriaEnable', { name: flow.name })"
                    @click="confirmToggleCallStart(flow)"
                    @keydown.enter="confirmToggleCallStart(flow)"
                  >
                    {{ flow.is_call_start ? t('calling.callStart') : t('calling.secondary') }}
                  </Badge>
                  <Badge
                    v-if="flow.is_active && flow.is_outgoing_end"
                    variant="default"
                  >
                    {{ t('calling.outgoingEnd') }}
                  </Badge>
                </div>
              </TableCell>
              <TableCell>
                {{ flow.menu?.nodes?.length || 0 }} nodes
              </TableCell>
              <TableCell class="text-right">
                <div class="flex justify-end gap-2">
                  <IconButton
                    :icon="Pencil"
                    :label="t('calling.editFlowAriaLabel', { name: flow.name })"
                    @click="openEdit(flow)"
                  />
                  <IconButton
                    :icon="Trash2"
                    :label="t('calling.deleteFlowAriaLabel', { name: flow.name })"
                    class="text-destructive"
                    @click="confirmDelete(flow)"
                  />
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-if="!store.ivrFlowsLoading && store.ivrFlows.length === 0">
              <TableCell :colspan="5" class="text-center py-8">
                <div class="flex flex-col items-center gap-2 text-muted-foreground">
                  <Phone class="h-8 w-8" />
                  <p>{{ t('calling.noIVRFlows') }}</p>
                  <Button variant="outline" size="sm" @click="openCreate">
                    {{ t('calling.createFirstFlow') }}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>

        <div v-if="store.ivrFlowsLoading" class="flex justify-center py-8">
          <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
        </div>
      </CardContent>
    </Card>

    <!-- Create Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('calling.createFlow') }}</DialogTitle>
          <DialogDescription>
            {{ t('calling.flowEditorDesc') }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-2">
            <Label>{{ t('calling.name') }}</Label>
            <Input v-model="createForm.name" :placeholder="t('calling.flowNamePlaceholder')" />
          </div>
          <div class="space-y-2">
            <Label>{{ t('calling.account') }}</Label>
            <Select v-model="createForm.whatsapp_account">
              <SelectTrigger>
                <SelectValue :placeholder="t('calling.selectAccount')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="acc in accounts" :key="acc.name" :value="acc.name">
                  {{ acc.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-2">
            <Label>{{ t('calling.description') }}</Label>
            <Textarea v-model="createForm.description" :placeholder="t('calling.descriptionPlaceholder')" :rows="2" />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="showCreateDialog = false">{{ t('common.cancel') }}</Button>
          <Button :disabled="saving" @click="createFlow">
            <span v-if="saving" class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
            {{ t('common.create') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation -->
    <DeleteConfirmDialog
      v-model:open="showDeleteConfirm"
      :title="t('calling.deleteFlow')"
      :item-name="deletingFlow?.name"
      :description="t('calling.deleteFlowConfirm', { name: deletingFlow?.name })"
      :is-submitting="deleting"
      @confirm="deleteFlow"
    />

    <!-- Toggle Active Confirmation -->
    <ConfirmDialog
      v-model:open="showToggleActiveConfirm"
      :title="t('calling.toggleActiveConfirmTitle')"
      :description="togglingFlow?.is_active ? t('calling.toggleActiveConfirmDisable', { name: togglingFlow?.name }) : t('calling.toggleActiveConfirmEnable', { name: togglingFlow?.name })"
      :confirm-label="t('common.confirm')"
      @confirm="toggleActive"
    />

    <!-- Toggle Call Start Confirmation -->
    <ConfirmDialog
      v-model:open="showToggleCallStartConfirm"
      :title="t('calling.toggleCallStartConfirmTitle')"
      :description="togglingFlow?.is_call_start ? t('calling.toggleCallStartConfirmDisable', { name: togglingFlow?.name }) : t('calling.toggleCallStartConfirmEnable', { name: togglingFlow?.name })"
      :confirm-label="t('common.confirm')"
      @confirm="toggleCallStart"
    />
  </div>
</template>
