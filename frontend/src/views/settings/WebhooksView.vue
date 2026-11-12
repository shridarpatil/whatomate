<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { webhooksService, type Webhook, type WebhookEvent } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import { PageHeader, DataTable, SearchInput, DeleteConfirmDialog, ConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { toast } from '@/lib/toast'
import { Plus, Trash2, Pencil, Webhook as WebhookIcon, Play } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { debounce, formatDate } from '@/lib/utils'

const { t } = useI18n()

const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

const webhooks = ref<Webhook[]>([])
const availableEvents = ref<WebhookEvent[]>([])
const isLoading = ref(false)
const isDeleting = ref(false)
const isTesting = ref<string | null>(null)
const error = ref(false)

const canWrite = computed(() => authStore.hasPermission('webhooks', 'write'))
const canDelete = computed(() => authStore.hasPermission('webhooks', 'delete'))

const isDeleteDialogOpen = ref(false)
const webhookToDelete = ref<Webhook | null>(null)

const isDisableDialogOpen = ref(false)
const webhookToToggle = ref<Webhook | null>(null)
const isToggling = ref(false)

const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns = computed<Column<Webhook>[]>(() => [
  { key: 'name', label: t('webhooks.name'), sortable: true },
  { key: 'url', label: t('webhooks.url'), sortable: true },
  { key: 'events', label: t('webhooks.events') },
  { key: 'status', label: t('webhooks.status'), sortable: true, sortKey: 'is_active' },
  { key: 'created', label: t('webhooks.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const searchQuery = ref('')

async function fetchWebhooks() {
  isLoading.value = true
  error.value = false
  try {
    const response = await webhooksService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    webhooks.value = data.webhooks || []
    availableEvents.value = data.available_events || []
    totalItems.value = data.total ?? webhooks.value.length
  } catch (e) { error.value = true; toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.webhooks') }))) }
  finally { isLoading.value = false }
}

const debouncedSearch = debounce(() => {
  currentPage.value = 1
  fetchWebhooks()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchWebhooks()
}

function handleToggleWebhook(webhook: Webhook) {
  if (webhook.is_active) {
    webhookToToggle.value = webhook
    isDisableDialogOpen.value = true
  } else {
    performToggleWebhook(webhook)
  }
}

async function performToggleWebhook(webhook: Webhook) {
  isToggling.value = true
  try {
    await webhooksService.update(webhook.id, { is_active: !webhook.is_active })
    await fetchWebhooks()
    toast.success(webhook.is_active ? t('common.disabledSuccess', { resource: t('resources.Webhook') }) : t('common.enabledSuccess', { resource: t('resources.Webhook') }))
    isDisableDialogOpen.value = false
    webhookToToggle.value = null
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedToggle', { resource: t('resources.webhook') }))) }
  finally { isToggling.value = false }
}

function confirmDisableWebhook() {
  if (webhookToToggle.value) performToggleWebhook(webhookToToggle.value)
}

async function testWebhook(webhook: Webhook) {
  isTesting.value = webhook.id
  try { await webhooksService.test(webhook.id); toast.success(t('webhooks.testSent')) }
  catch (e) { toast.error(getErrorMessage(e, t('webhooks.testFailed'))) }
  finally { isTesting.value = null }
}

async function deleteWebhook() {
  if (!webhookToDelete.value) return
  isDeleting.value = true
  try { await webhooksService.delete(webhookToDelete.value.id); await fetchWebhooks(); toast.success(t('common.deletedSuccess', { resource: t('resources.Webhook') })); isDeleteDialogOpen.value = false; webhookToDelete.value = null }
  catch (e) { toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.webhook') }))) }
  finally { isDeleting.value = false }
}

function getEventLabel(eventValue: string): string { return availableEvents.value.find(e => e.value === eventValue)?.label || eventValue }

watch(() => organizationsStore.selectedOrgId, () => fetchWebhooks())
onMounted(() => fetchWebhooks())
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('webhooks.title')" :description="$t('webhooks.subtitle')" :icon="WebhookIcon" back-link="/settings">
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/webhooks/new" class="button outline small"><Plus :size="16" />{{ $t('webhooks.addWebhook') }}</RouterLink>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('webhooks.fetchErrorTitle')"
      :description="$t('webhooks.fetchErrorDescription')"
      :retry-label="$t('common.retry')"
      class="flex-1"
      @retry="fetchWebhooks"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <div>
            <h3>{{ $t('webhooks.yourWebhooks') }}</h3>
            <p class="text-light">{{ $t('webhooks.yourWebhooksDesc') }}</p>
          </div>
          <SearchInput v-model="searchQuery" :placeholder="$t('webhooks.searchWebhooks') + '...'" />
        </header>
        <DataTable :items="webhooks" :columns="columns" :is-loading="isLoading" :empty-icon="WebhookIcon" :empty-title="searchQuery ? $t('webhooks.noMatchingWebhooks') : $t('webhooks.noWebhooksYet')" :empty-description="searchQuery ? $t('webhooks.noMatchingWebhooksDesc') : $t('webhooks.noWebhooksYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="webhooks" @page-change="handlePageChange">
          <template #cell-name="{ item: webhook }">
            <RouterLink :to="`/settings/webhooks/${webhook.id}`"><strong>{{ webhook.name }}</strong></RouterLink>
          </template>
          <template #cell-url="{ item: webhook }"><span class="text-light truncate">{{ webhook.url }}</span></template>
          <template #cell-events="{ item: webhook }">
            <div class="hstack gap-1 flex-wrap">
              <span v-for="event in webhook.events.slice(0, 2)" :key="event" class="badge" data-variant="secondary">{{ getEventLabel(event) }}</span>
              <span v-if="webhook.events.length > 2" class="badge outline">+{{ webhook.events.length - 2 }}</span>
            </div>
          </template>
          <template #cell-status="{ item: webhook }">
            <label class="hstack gap-2">
              <input type="checkbox" role="switch" :checked="webhook.is_active" :aria-label="`${webhook.name}: ${webhook.is_active ? $t('common.active') : $t('common.inactive')}`" @click.prevent="handleToggleWebhook(webhook)" />
              <small class="text-light">{{ webhook.is_active ? $t('common.active') : $t('common.inactive') }}</small>
            </label>
          </template>
          <template #cell-created="{ item: webhook }"><span class="text-light">{{ formatDate(webhook.created_at) }}</span></template>
          <template #cell-actions="{ item: webhook }">
            <div class="hstack gap-1 justify-end">
              <IconButton :icon="Play" :label="$t('webhooks.testWebhook')" :disabled="isTesting === webhook.id" :loading="isTesting === webhook.id" @click="testWebhook(webhook)" />
              <RouterLink :to="`/settings/webhooks/${webhook.id}`" class="button ghost icon" :title="$t('common.edit')" :aria-label="$t('common.edit')"><Pencil :size="16" /></RouterLink>
              <IconButton v-if="canDelete" :icon="Trash2" :label="$t('common.delete')" @click="webhookToDelete = webhook; isDeleteDialogOpen = true" />
            </div>
          </template>
          <template #empty-action>
            <RouterLink v-if="canWrite" to="/settings/webhooks/new" class="button outline small"><Plus :size="16" />{{ $t('webhooks.addWebhook') }}</RouterLink>
          </template>
        </DataTable>
      </article>
    </div>

    <ConfirmDialog
      v-model:open="isDisableDialogOpen"
      :title="$t('webhooks.confirmDisableTitle')"
      :description="$t('webhooks.confirmDisableDescription')"
      :confirm-label="$t('common.confirm')"
      variant="destructive"
      :is-submitting="isToggling"
      @confirm="confirmDisableWebhook"
    />

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" :title="$t('webhooks.deleteWebhook')" :item-name="webhookToDelete?.name" :is-submitting="isDeleting" @confirm="deleteWebhook" />
  </div>
</template>
