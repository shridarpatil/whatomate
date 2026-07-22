<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, DataTable, DeleteConfirmDialog, ErrorState, type Column } from '@/components/shared'
import { api } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { Plus, Pencil, Trash2, Phone, Check } from 'lucide-vue-next'

const { t } = useI18n()
const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

interface WhatsAppAccount {
  id: string
  name: string
  app_id: string
  phone_id: string
  business_id: string
  api_version: string
  is_default_incoming: boolean
  is_default_outgoing: boolean
  status: string
  has_access_token: boolean
  has_app_secret: boolean
  created_at: string
}

const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const fetchError = ref(false)
const deleteDialogOpen = ref(false)
const accountToDelete = ref<WhatsAppAccount | null>(null)
const isDeleting = ref(false)

const canWrite = computed(() => authStore.hasPermission('accounts', 'write'))
const canDelete = computed(() => authStore.hasPermission('accounts', 'delete'))
const breadcrumbs = computed(() => [{ label: t('nav.settings'), href: '/settings' }, { label: t('settings.accounts') }])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const columns = computed<Column<WhatsAppAccount>[]>(() => [
  { key: 'account', label: t('accounts.account', 'Account'), width: 'w-[250px]', sortable: true, sortKey: 'name' },
  { key: 'app_id', label: t('accounts.appId', 'App ID') },
  { key: 'phone_id', label: t('accounts.phoneNumberId', 'Phone ID'), sortable: true },
  { key: 'api_version', label: t('accounts.apiVersion', 'Version') },
  { key: 'defaults', label: t('accounts.defaults', 'Defaults') },
  { key: 'status', label: t('accounts.status', 'Status'), sortable: true, sortKey: 'status' },
  { key: 'created', label: t('common.created', 'Created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

watch(() => organizationsStore.selectedOrgId, () => fetchAccounts())
onMounted(() => fetchAccounts())

async function fetchAccounts() {
  isLoading.value = true
  fetchError.value = false
  try {
    const response = await api.get('/accounts')
    accounts.value = response.data.data?.accounts || []
  } catch {
    fetchError.value = true
    toast.error(t('common.failedLoad', { resource: t('resources.accounts') }))
  } finally {
    isLoading.value = false
  }
}

function openDeleteDialog(account: WhatsAppAccount) {
  accountToDelete.value = account
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!accountToDelete.value) return
  isDeleting.value = true
  try {
    await api.delete(`/accounts/${accountToDelete.value.id}`)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Account') }))
    deleteDialogOpen.value = false
    accountToDelete.value = null
    await fetchAccounts()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.account') })))
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('accounts.title')"
      :icon="Phone"
      icon-gradient="bg-gradient-to-br from-emerald-500 to-green-600 shadow-emerald-500/20"
      back-link="/settings"
      :breadcrumbs="breadcrumbs"
    >
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/accounts/new">
          <Button variant="outline" size="sm">
            <Plus class="h-4 w-4 mr-2" />
            {{ $t('accounts.addAccount') }}
          </Button>
        </RouterLink>
      </template>
    </PageHeader>

    <ErrorState
      v-if="fetchError && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      class="flex-1"
    >
      <template #action><Button size="sm" @click="fetchAccounts">{{ $t('common.retry') }}</Button></template>
    </ErrorState>

    <ScrollArea v-else class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div>
                <CardTitle>{{ $t('accounts.yourAccounts', 'WhatsApp Accounts') }}</CardTitle>
                <CardDescription>{{ $t('accounts.yourAccountsDesc', 'Manage your connected WhatsApp Business accounts') }}</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="accounts"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Phone"
                :empty-title="$t('accounts.noAccounts')"
                :empty-description="$t('accounts.noAccountsDesc')"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
                item-name="accounts"
              >
                <template #empty-action>
                  <RouterLink v-if="canWrite" to="/settings/accounts/new">
                    <Button variant="outline" size="sm"><Plus class="h-4 w-4 mr-2" />{{ $t('accounts.addAccount') }}</Button>
                  </RouterLink>
                </template>
                <template #cell-account="{ item: account }">
                  <RouterLink :to="`/settings/accounts/${account.id}`" class="flex items-center gap-3 text-inherit no-underline hover:opacity-80">
                    <div class="h-9 w-9 rounded-full bg-emerald-500/10 flex items-center justify-center flex-shrink-0">
                      <Phone class="h-4 w-4 text-emerald-500" />
                    </div>
                    <p class="font-medium truncate">{{ account.name }}</p>
                  </RouterLink>
                </template>
                <template #cell-app_id="{ item: account }">
                  <code v-if="account.app_id" class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ account.app_id }}</code>
                  <span v-else class="text-muted-foreground">—</span>
                </template>
                <template #cell-phone_id="{ item: account }">
                  <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ account.phone_id }}</code>
                </template>
                <template #cell-api_version="{ item: account }">
                  <span class="text-sm">{{ account.api_version }}</span>
                </template>
                <template #cell-defaults="{ item: account }">
                  <div class="flex items-center gap-1.5 flex-wrap">
                    <Badge v-if="account.is_default_incoming" variant="outline" class="text-[10px]">
                      <Check class="h-2.5 w-2.5 mr-0.5" /> {{ $t('accounts.incoming', 'In') }}
                    </Badge>
                    <Badge v-if="account.is_default_outgoing" variant="outline" class="text-[10px]">
                      <Check class="h-2.5 w-2.5 mr-0.5" /> {{ $t('accounts.outgoing', 'Out') }}
                    </Badge>
                  </div>
                </template>
                <template #cell-status="{ item: account }">
                  <Badge variant="outline" :class="account.status === 'active' ? 'border-green-600 text-green-600' : ''">
                    {{ account.status }}
                  </Badge>
                </template>
                <template #cell-created="{ item: account }">
                  <span class="text-muted-foreground">{{ formatDate(account.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: account }">
                  <div class="flex items-center justify-end gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <RouterLink :to="`/settings/accounts/${account.id}`">
                          <Button variant="ghost" size="icon" class="h-8 w-8"><Pencil class="h-4 w-4" /></Button>
                        </RouterLink>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('common.edit') }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="canDelete">
                      <TooltipTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(account)">
                          <Trash2 class="h-4 w-4 text-destructive" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('common.delete') }}</TooltipContent>
                    </Tooltip>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('accounts.deleteAccount')"
      :item-name="accountToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDelete"
    />
  </div>
</template>
