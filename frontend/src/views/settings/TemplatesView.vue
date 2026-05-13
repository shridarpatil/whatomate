<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, DataTable, IconButton, DeleteConfirmDialog, ErrorState, type Column } from '@/components/shared'
import { api, templatesService } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import { Plus, RefreshCw, FileText, Pencil, Trash2, Loader2, MessageSquare, Image, FileIcon, Video } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()

interface WhatsAppAccount {
  id: string
  name: string
  phone_id: string
}

interface Template {
  id: string
  whatsapp_account: string
  meta_template_id: string
  name: string
  display_name: string
  language: string
  category: string
  status: string
  header_type: string
  header_content: string
  body_content: string
  footer_content: string
  buttons: any[]
  sample_values: any[]
  created_at: string
  updated_at: string
}

const organizationsStore = useOrganizationsStore()

const templates = ref<Template[]>([])
const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const error = ref<string | null>(null)
const isSyncing = ref(false)
const selectedAccount = ref<string>(localStorage.getItem('templates_selected_account') || 'all')

// Delete dialog state
const deleteDialogOpen = ref(false)
const templateToDelete = ref<Template | null>(null)
const isDeleting = ref(false)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchTemplates(),
})

const columns = computed<Column<Template>[]>(() => [
  { key: 'name', label: t('templates.name'), sortable: true },
  { key: 'category', label: t('templates.category'), sortable: true },
  { key: 'status', label: t('templates.status'), sortable: true },
  { key: 'language', label: t('templates.language'), sortable: true },
  { key: 'header_type', label: t('templates.header') },
  { key: 'actions', label: '', align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const languages = [
  { code: 'af', name: 'Afrikaans' },
  { code: 'sq', name: 'Albanian' },
  { code: 'ar', name: 'Arabic' },
  { code: 'az', name: 'Azerbaijani' },
  { code: 'bn', name: 'Bengali' },
  { code: 'bg', name: 'Bulgarian' },
  { code: 'ca', name: 'Catalan' },
  { code: 'zh_CN', name: 'Chinese (CHN)' },
  { code: 'zh_HK', name: 'Chinese (HKG)' },
  { code: 'zh_TW', name: 'Chinese (TAI)' },
  { code: 'hr', name: 'Croatian' },
  { code: 'cs', name: 'Czech' },
  { code: 'da', name: 'Danish' },
  { code: 'nl', name: 'Dutch' },
  { code: 'en', name: 'English' },
  { code: 'en_GB', name: 'English (UK)' },
  { code: 'en_US', name: 'English (US)' },
  { code: 'et', name: 'Estonian' },
  { code: 'fil', name: 'Filipino' },
  { code: 'fi', name: 'Finnish' },
  { code: 'fr', name: 'French' },
  { code: 'ka', name: 'Georgian' },
  { code: 'de', name: 'German' },
  { code: 'el', name: 'Greek' },
  { code: 'gu', name: 'Gujarati' },
  { code: 'ha', name: 'Hausa' },
  { code: 'he', name: 'Hebrew' },
  { code: 'hi', name: 'Hindi' },
  { code: 'hu', name: 'Hungarian' },
  { code: 'id', name: 'Indonesian' },
  { code: 'ga', name: 'Irish' },
  { code: 'it', name: 'Italian' },
  { code: 'ja', name: 'Japanese' },
  { code: 'kn', name: 'Kannada' },
  { code: 'kk', name: 'Kazakh' },
  { code: 'rw_RW', name: 'Kinyarwanda' },
  { code: 'ko', name: 'Korean' },
  { code: 'ky_KG', name: 'Kyrgyz (Kyrgyzstan)' },
  { code: 'lo', name: 'Lao' },
  { code: 'lv', name: 'Latvian' },
  { code: 'lt', name: 'Lithuanian' },
  { code: 'mk', name: 'Macedonian' },
  { code: 'ms', name: 'Malay' },
  { code: 'ml', name: 'Malayalam' },
  { code: 'mr', name: 'Marathi' },
  { code: 'nb', name: 'Norwegian (Bokm\u00e5l)' },
  { code: 'fa', name: 'Persian' },
  { code: 'pl', name: 'Polish' },
  { code: 'pt_BR', name: 'Portuguese (BR)' },
  { code: 'pt_PT', name: 'Portuguese (POR)' },
  { code: 'pa', name: 'Punjabi' },
  { code: 'ro', name: 'Romanian' },
  { code: 'ru', name: 'Russian' },
  { code: 'sr', name: 'Serbian' },
  { code: 'sk', name: 'Slovak' },
  { code: 'sl', name: 'Slovenian' },
  { code: 'es', name: 'Spanish' },
  { code: 'es_AR', name: 'Spanish (ARG)' },
  { code: 'es_MX', name: 'Spanish (MEX)' },
  { code: 'es_ES', name: 'Spanish (SPA)' },
  { code: 'sw', name: 'Swahili' },
  { code: 'sv', name: 'Swedish' },
  { code: 'ta', name: 'Tamil' },
  { code: 'te', name: 'Telugu' },
  { code: 'th', name: 'Thai' },
  { code: 'tr', name: 'Turkish' },
  { code: 'uk', name: 'Ukrainian' },
  { code: 'ur', name: 'Urdu' },
  { code: 'uz', name: 'Uzbek' },
  { code: 'vi', name: 'Vietnamese' },
  { code: 'zu', name: 'Zulu' },
]

function getLanguageName(code: string): string {
  return languages.find(l => l.code === code)?.name || code
}

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, async () => {
  await fetchAccounts()
  await fetchTemplates()
})

onMounted(async () => {
  await fetchAccounts()
  await fetchTemplates()
})

async function fetchAccounts() {
  try {
    const response = await api.get('/accounts')
    accounts.value = response.data.data?.accounts || []
    // Validate stored account still exists, fallback to 'all' if not
    if (selectedAccount.value !== 'all' && !accounts.value.some(a => a.name === selectedAccount.value)) {
      selectedAccount.value = 'all'
      localStorage.setItem('templates_selected_account', 'all')
    }
  } catch (error) {
    console.error('Failed to fetch accounts:', error)
  }
}

function onAccountChange(value: string | number | bigint | Record<string, any> | null) {
  if (typeof value !== 'string') return
  localStorage.setItem('templates_selected_account', value)
  currentPage.value = 1
  fetchTemplates()
}

async function fetchTemplates() {
  isLoading.value = true
  error.value = null
  try {
    const response = await templatesService.list({
      account: selectedAccount.value !== 'all' ? selectedAccount.value : undefined,
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    templates.value = data.templates || []
    totalItems.value = data.total ?? templates.value.length
  } catch (err: any) {
    console.error('Failed to fetch templates:', err)
    error.value = t('templates.errorLoadingTemplates')
    toast.error(t('common.failedLoad', { resource: t('resources.templates') }))
    templates.value = []
  } finally {
    isLoading.value = false
  }
}

async function syncTemplates() {
  if (!selectedAccount.value || selectedAccount.value === 'all') {
    toast.error(t('templates.selectAccountFirst'))
    return
  }

  isSyncing.value = true
  try {
    const response = await api.post('/templates/sync', {
      whatsapp_account: selectedAccount.value
    })
    toast.success(response.data.data.message || t('templates.syncSuccess'))
    await fetchTemplates()
  } catch (error) {
    toast.error(getErrorMessage(error, t('templates.syncFailed')))
  } finally {
    isSyncing.value = false
  }
}

function openDeleteDialog(template: Template) {
  templateToDelete.value = template
  deleteDialogOpen.value = true
}

async function confirmDeleteTemplate() {
  if (!templateToDelete.value) return

  isDeleting.value = true
  try {
    await api.delete(`/templates/${templateToDelete.value.id}`)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Template') }))
    deleteDialogOpen.value = false
    templateToDelete.value = null
    await fetchTemplates()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.template') })))
  } finally {
    isDeleting.value = false
  }
}

// Dark-first: default is dark mode, light: prefix for light mode
function getStatusBadgeClass(status: string) {
  switch (status) {
    case 'APPROVED':
      return 'bg-green-900 text-green-300 light:bg-green-100 light:text-green-800'
    case 'PENDING':
      return 'bg-yellow-900 text-yellow-300 light:bg-yellow-100 light:text-yellow-800'
    case 'REJECTED':
      return 'bg-red-900 text-red-300 light:bg-red-100 light:text-red-800'
    case 'DRAFT':
      return 'bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800'
    default:
      return 'bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800'
  }
}

function getCategoryBadgeClass(category: string) {
  switch (category) {
    case 'UTILITY':
      return 'bg-blue-900 text-blue-300 light:bg-blue-100 light:text-blue-800'
    case 'MARKETING':
      return 'bg-purple-900 text-purple-300 light:bg-purple-100 light:text-purple-800'
    case 'AUTHENTICATION':
      return 'bg-orange-900 text-orange-300 light:bg-orange-100 light:text-orange-800'
    default:
      return 'bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800'
  }
}

function getHeaderIcon(type: string) {
  switch (type) {
    case 'IMAGE':
      return Image
    case 'VIDEO':
      return Video
    case 'DOCUMENT':
      return FileIcon
    default:
      return MessageSquare
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('templates.title')" :subtitle="$t('templates.subtitle')" :icon="FileText" icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="syncTemplates" :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'">
          <Loader2 v-if="isSyncing" class="h-4 w-4 mr-2 animate-spin" />
          <RefreshCw v-else class="h-4 w-4 mr-2" />
          {{ $t('templates.syncFromMeta') }}
        </Button>
        <RouterLink to="/templates/new">
          <Button variant="outline" size="sm">
            <Plus class="h-4 w-4 mr-2" />
            {{ $t('templates.createTemplate') }}
          </Button>
        </RouterLink>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div>
          <ErrorState
            v-if="error && !isLoading"
            :title="$t('common.loadErrorTitle')"
            :description="error"
            :retry-label="$t('common.retry')"
            @retry="fetchTemplates"
          />
          <Card v-else>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('templates.yourTemplates') }}</CardTitle>
                  <CardDescription>{{ $t('templates.yourTemplatesDesc') }}</CardDescription>
                </div>
                <div class="flex items-center gap-4 flex-wrap">
                  <div class="flex items-center gap-2">
                    <Label class="text-sm text-muted-foreground">{{ $t('templates.account') }}:</Label>
                    <Select v-model="selectedAccount" @update:model-value="onAccountChange">
                      <SelectTrigger class="w-[180px]">
                        <SelectValue :placeholder="$t('templates.allAccounts')" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">{{ $t('templates.allAccounts') }}</SelectItem>
                        <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                          {{ account.name }}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <SearchInput v-model="searchQuery" :placeholder="$t('templates.searchTemplates') + '...'" class="w-64" />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="templates"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="FileText"
                :empty-title="$t('templates.noTemplatesFound')"
                :empty-description="$t('templates.noTemplatesFoundDesc')"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="templates"
                @page-change="handlePageChange"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
              >
                <template #cell-name="{ item: template }">
                  <RouterLink :to="`/templates/${template.id}`" class="text-inherit no-underline hover:opacity-80">
                    <span class="font-medium">{{ template.display_name || template.name }}</span>
                    <p class="text-xs font-mono text-muted-foreground">{{ template.name }}</p>
                  </RouterLink>
                </template>
                <template #cell-category="{ item: template }">
                  <Badge :class="getCategoryBadgeClass(template.category)" class="text-xs">
                    {{ template.category }}
                  </Badge>
                </template>
                <template #cell-status="{ item: template }">
                  <Badge :class="getStatusBadgeClass(template.status)" class="text-xs">
                    {{ template.status }}
                  </Badge>
                </template>
                <template #cell-language="{ item: template }">
                  <span class="text-muted-foreground">{{ getLanguageName(template.language) }}</span>
                </template>
                <template #cell-header_type="{ item: template }">
                  <div class="flex items-center gap-1">
                    <component :is="getHeaderIcon(template.header_type)" class="h-4 w-4 text-muted-foreground" />
                    <span class="text-muted-foreground text-sm">{{ template.header_type || 'NONE' }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: template }">
                  <div class="flex items-center justify-end gap-1">
                    <RouterLink :to="`/templates/${template.id}`">
                      <IconButton
                        :icon="Pencil"
                        :label="$t('common.edit')"
                        class="h-8 w-8"
                      />
                    </RouterLink>
                    <IconButton
                      :icon="Trash2"
                      :label="$t('common.delete')"
                      class="h-8 w-8 text-destructive"
                      @click="openDeleteDialog(template)"
                    />
                  </div>
                </template>
                <template #empty-action>
                  <div class="flex items-center justify-center gap-2">
                    <Button variant="outline" size="sm" @click="syncTemplates" :disabled="!selectedAccount || selectedAccount === 'all'">
                      <RefreshCw class="h-4 w-4 mr-2" />
                      {{ $t('templates.syncFromMeta') }}
                    </Button>
                    <RouterLink to="/templates/new">
                      <Button variant="outline" size="sm">
                        <Plus class="h-4 w-4 mr-2" />
                        {{ $t('templates.createTemplate') }}
                      </Button>
                    </RouterLink>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Delete Confirmation Dialog -->
    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('templates.deleteTemplate')"
      :item-name="templateToDelete?.display_name || templateToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteTemplate"
    />
  </div>
</template>
