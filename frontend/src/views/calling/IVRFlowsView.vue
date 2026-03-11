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

const { t } = useI18n()
const router = useRouter()
const store = useCallingStore()

const accounts = ref<{ name: string }[]>([])
const showCreateDialog = ref(false)
const showDeleteConfirm = ref(false)
const deletingFlow = ref<IVRFlow | null>(null)
const saving = ref(false)

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
  try {
    await store.deleteIVRFlow(deletingFlow.value.id)
    toast.success(t('calling.flowDeleted'))
    showDeleteConfirm.value = false
    deletingFlow.value = null
  } catch {
    toast.error(t('calling.flowDeleteFailed'))
  }
}

async function toggleActive(flow: IVRFlow) {
  try {
    await store.updateIVRFlow(flow.id, {
      is_active: !flow.is_active,
      is_call_start: flow.is_call_start,
      is_outgoing_end: flow.is_outgoing_end,
    })
    store.fetchIVRFlows()
  } catch {
    toast.error(t('calling.flowSaveFailed'))
  }
}

async function toggleCallStart(flow: IVRFlow) {
  try {
    await store.updateIVRFlow(flow.id, {
      is_active: flow.is_active,
      is_call_start: !flow.is_call_start,
      is_outgoing_end: flow.is_outgoing_end,
    })
    store.fetchIVRFlows()
  } catch {
    toast.error(t('calling.flowSaveFailed'))
  }
}

onMounted(async () => {
  store.fetchIVRFlows()
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
        <Button variant="outline" size="sm" @click="store.fetchIVRFlows()">
          <RefreshCw class="h-4 w-4 mr-2" />
          {{ t('common.refresh') }}
        </Button>
        <Button @click="openCreate">
          <Plus class="h-4 w-4 mr-2" />
          {{ t('calling.createFlow') }}
        </Button>
      </div>
    </div>

    <!-- Flows Table -->
    <Card>
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
                <div>
                  <p class="font-medium">{{ flow.name }}</p>
                  <p v-if="flow.description" class="text-sm text-muted-foreground">{{ flow.description }}</p>
                </div>
              </TableCell>
              <TableCell>{{ flow.whatsapp_account }}</TableCell>
              <TableCell>
                <div class="flex gap-1.5">
                  <Badge
                    :variant="flow.is_active ? 'default' : 'destructive'"
                    class="cursor-pointer"
                    @click="toggleActive(flow)"
                  >
                    {{ flow.is_active ? t('calling.enabled') : t('calling.disabled') }}
                  </Badge>
                  <Badge
                    v-if="flow.is_active"
                    :variant="flow.is_call_start ? 'default' : 'outline'"
                    class="cursor-pointer"
                    @click="toggleCallStart(flow)"
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
                  <Button variant="ghost" size="icon" @click="openEdit(flow)">
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" @click="confirmDelete(flow)">
                    <Trash2 class="h-4 w-4 text-destructive" />
                  </Button>
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
    <Dialog v-model:open="showDeleteConfirm">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ t('calling.deleteFlow') }}</DialogTitle>
          <DialogDescription>
            {{ t('calling.deleteFlowConfirm', { name: deletingFlow?.name }) }}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="showDeleteConfirm = false">{{ t('common.cancel') }}</Button>
          <Button variant="destructive" @click="deleteFlow">{{ t('common.delete') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
