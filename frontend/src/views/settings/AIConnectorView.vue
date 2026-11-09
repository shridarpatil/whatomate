<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { BrainCircuit, Unplug } from 'lucide-vue-next'
import { mcpService, type MCPGrant } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PageHeader, DataTable, ConfirmDialog, ErrorState, RefreshButton, type Column } from '@/components/shared'
import ConnectorSetupSteps from '@/components/ai-connector/ConnectorSetupSteps.vue'
import { getErrorMessage, unwrapResponse } from '@/lib/api-utils'
import { formatDate, formatDateTime } from '@/lib/utils'
import { useViewRefresh } from '@/composables/useViewRefresh'

const { t } = useI18n()
const authStore = useAuthStore()

const connectorUrl = ref('')
const grants = ref<MCPGrant[]>([])
const isLoading = ref(false)
const error = ref<string | null>(null)
const grantToRevoke = ref<MCPGrant | null>(null)
const isRevokeOpen = ref(false)
const isRevoking = ref(false)

const canViewApiKeys = computed(() => authStore.hasPermission('api_keys', 'read'))

const columns = computed<Column<MCPGrant>[]>(() => [
  { key: 'client_name', label: t('aiConnector.app') },
  { key: 'organization_name', label: t('aiConnector.organisation') },
  { key: 'created_at', label: t('aiConnector.connectedOn') },
  { key: 'last_used_at', label: t('aiConnector.lastUsed') },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

async function fetchInfo() {
  try {
    connectorUrl.value = unwrapResponse<{ connector_url?: string }>(await mcpService.info())?.connector_url || ''
  } catch {
    connectorUrl.value = ''
  }
}

async function fetchGrants() {
  isLoading.value = true
  error.value = null
  try {
    grants.value = unwrapResponse<{ grants?: MCPGrant[] }>(await mcpService.listGrants())?.grants || []
  } catch (err) {
    error.value = getErrorMessage(err, t('aiConnector.loadFailed'))
  } finally {
    isLoading.value = false
  }
}

async function refreshAll() {
  await Promise.all([fetchInfo(), fetchGrants()])
}

function askRevoke(grant: MCPGrant) {
  grantToRevoke.value = grant
  isRevokeOpen.value = true
}

async function revoke() {
  if (!grantToRevoke.value) return
  isRevoking.value = true
  try {
    await mcpService.revokeGrant(grantToRevoke.value.id)
    toast.success(t('aiConnector.revokedSuccess'))
    isRevokeOpen.value = false
    grantToRevoke.value = null
    await fetchGrants()
  } catch (err) {
    toast.error(getErrorMessage(err, t('aiConnector.revokeFailed')))
  } finally {
    isRevoking.value = false
  }
}

const { isRefreshing, refreshNow } = useViewRefresh(refreshAll)

onMounted(refreshAll)
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('aiConnector.title')" :subtitle="$t('aiConnector.subtitle')" :icon="BrainCircuit" icon-gradient="bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/20">
      <template #actions>
        <RefreshButton :refreshing="isRefreshing" :label="$t('common.refresh')" @refresh="refreshNow(true)" />
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6">
        <ConnectorSetupSteps :connector-url="connectorUrl" :can-view-api-keys="canViewApiKeys" />

        <ErrorState
          v-if="error && !isLoading"
          :title="$t('common.loadErrorTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchGrants"
        />
        <Card v-else>
          <CardHeader>
            <CardTitle>{{ $t('aiConnector.connectedApps') }}</CardTitle>
            <CardDescription>{{ $t('aiConnector.connectedAppsDesc') }}</CardDescription>
          </CardHeader>
          <CardContent>
            <DataTable :items="grants" :columns="columns" :is-loading="isLoading" :empty-icon="BrainCircuit" :empty-title="$t('aiConnector.noApps')" :empty-description="$t('aiConnector.noAppsDesc')">
              <template #cell-client_name="{ item }"><span class="font-medium" data-testid="grant-app">{{ item.client_name }}</span></template>
              <template #cell-organization_name="{ item }">{{ item.organization_name }}</template>
              <template #cell-created_at="{ item }"><span data-testid="grant-created">{{ formatDate(item.created_at) }}</span></template>
              <template #cell-last_used_at="{ item }">
                <span data-testid="grant-last-used">{{ item.last_used_at ? formatDateTime(item.last_used_at) : $t('aiConnector.never') }}</span>
              </template>
              <template #cell-actions="{ item }">
                <div class="flex justify-end">
                  <Button variant="outline" size="sm" class="text-destructive" data-testid="grant-revoke" @click="askRevoke(item)">
                    <Unplug class="h-4 w-4 mr-2" />{{ $t('aiConnector.revoke') }}
                  </Button>
                </div>
              </template>
            </DataTable>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <ConfirmDialog
      v-model:open="isRevokeOpen"
      :title="$t('aiConnector.revokeTitle')"
      :description="$t('aiConnector.revokeDesc', { app: grantToRevoke?.client_name || '', organisation: grantToRevoke?.organization_name || '' })"
      :confirm-label="$t('aiConnector.revoke')"
      :cancel-label="$t('common.cancel')"
      variant="destructive"
      :is-submitting="isRevoking"
      @confirm="revoke"
    />
  </div>
</template>
