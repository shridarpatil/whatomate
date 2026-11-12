<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useCallingStore } from '@/stores/calling'
import { accountsService, type IVRFlow, type IVRFlowData } from '@/services/api'
import Modal from '@/components/oat/Modal.vue'
import PageHeader from '@/components/shared/PageHeader.vue'
import { Plus, Pencil, Trash2, Phone, RefreshCw } from '@/lib/icons'
import { toast } from '@/lib/toast'
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
  <div class="flex flex-col h-full">
    <PageHeader :title="t('calling.ivrFlows')" :description="t('calling.ivrFlowsDesc')">
      <template #actions>
        <button type="button" class="outline small" @click="loadFlows()">
          <RefreshCw :size="16" />
          {{ t('common.refresh') }}
        </button>
        <button type="button" class="small" @click="openCreate">
          <Plus :size="16" />
          {{ t('calling.createFlow') }}
        </button>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4 vstack gap-4">
      <!-- Fetch Error -->
      <ErrorState
        v-if="fetchError"
        :title="t('common.error')"
        :description="t('common.failedLoad', { resource: t('calling.ivrFlows') })"
        :retry-label="t('common.retry')"
        @retry="loadFlows"
      />

      <!-- Flows Table -->
      <article v-else class="card">
        <div class="table">
          <table>
            <thead>
              <tr>
                <th>{{ t('calling.name') }}</th>
                <th>{{ t('calling.account') }}</th>
                <th>{{ t('calling.status') }}</th>
                <th>{{ t('calling.options') }}</th>
                <th class="align-right">{{ t('calling.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <template v-if="store.ivrFlowsLoading">
                <tr v-for="row in 5" :key="`skeleton-${row}`">
                  <td v-for="col in 5" :key="col"><div role="status" class="skeleton line"></div></td>
                </tr>
              </template>
              <template v-else>
                <tr v-for="flow in store.ivrFlows" :key="flow.id">
                  <td>
                    <RouterLink :to="{ name: 'ivr-flow-editor', params: { id: flow.id } }" class="unstyled">
                      <strong>{{ flow.name }}</strong>
                      <p v-if="flow.description" class="text-light"><small>{{ flow.description }}</small></p>
                    </RouterLink>
                  </td>
                  <td>{{ flow.whatsapp_account }}</td>
                  <td>
                    <div class="hstack gap-2 flex-wrap">
                      <button
                        type="button"
                        class="badge"
                        :data-variant="flow.is_active ? 'success' : 'danger'"
                        :aria-label="flow.is_active ? t('calling.toggleActiveAriaDisable', { name: flow.name }) : t('calling.toggleActiveAriaEnable', { name: flow.name })"
                        @click="confirmToggleActive(flow)"
                      >
                        {{ flow.is_active ? t('calling.enabled') : t('calling.disabled') }}
                      </button>
                      <button
                        v-if="flow.is_active"
                        type="button"
                        class="badge"
                        :data-variant="flow.is_call_start ? 'success' : 'secondary'"
                        :aria-label="flow.is_call_start ? t('calling.toggleCallStartAriaDisable', { name: flow.name }) : t('calling.toggleCallStartAriaEnable', { name: flow.name })"
                        @click="confirmToggleCallStart(flow)"
                      >
                        {{ flow.is_call_start ? t('calling.callStart') : t('calling.secondary') }}
                      </button>
                      <span v-if="flow.is_active && flow.is_outgoing_end" class="badge" data-variant="success">
                        {{ t('calling.outgoingEnd') }}
                      </span>
                    </div>
                  </td>
                  <td>
                    {{ flow.menu?.nodes?.length || 0 }} nodes
                  </td>
                  <td>
                    <div class="hstack gap-2 justify-end">
                      <IconButton
                        :icon="Pencil"
                        :label="t('calling.editFlowAriaLabel', { name: flow.name })"
                        @click="openEdit(flow)"
                      />
                      <IconButton
                        :icon="Trash2"
                        :label="t('calling.deleteFlowAriaLabel', { name: flow.name })"
                        @click="confirmDelete(flow)"
                      />
                    </div>
                  </td>
                </tr>
                <tr v-if="store.ivrFlows.length === 0">
                  <td colspan="5" class="align-center text-light">
                    <Phone :size="32" />
                    <p>{{ t('calling.noIVRFlows') }}</p>
                    <button type="button" class="outline small" @click="openCreate">
                      {{ t('calling.createFirstFlow') }}
                    </button>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </article>
    </div>

    <!-- Create Dialog -->
    <Modal v-model:open="showCreateDialog" :title="t('calling.createFlow')" :description="t('calling.flowEditorDesc')">
      <form id="ivr-create-form" class="vstack gap-4" @submit.prevent="createFlow">
        <label data-field>
          {{ t('calling.name') }}
          <input v-model="createForm.name" type="text" :placeholder="t('calling.flowNamePlaceholder')">
        </label>
        <label data-field>
          {{ t('calling.account') }}
          <select v-model="createForm.whatsapp_account">
            <option value="" disabled>{{ t('calling.selectAccount') }}</option>
            <option v-for="acc in accounts" :key="acc.name" :value="acc.name">{{ acc.name }}</option>
          </select>
        </label>
        <label data-field>
          {{ t('calling.description') }}
          <textarea v-model="createForm.description" :placeholder="t('calling.descriptionPlaceholder')" rows="2" />
        </label>
      </form>
      <template #footer>
        <button type="button" class="outline" @click="showCreateDialog = false">{{ t('common.cancel') }}</button>
        <button type="submit" form="ivr-create-form" :disabled="saving" :aria-busy="saving ? 'true' : undefined" data-spinner="small">
          {{ t('common.create') }}
        </button>
      </template>
    </Modal>

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
