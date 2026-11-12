<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { PageHeader, SearchInput, DataTable, IconButton, DeleteConfirmDialog, ErrorState, type Column } from '@/components/shared'
import { api, templatesService } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from '@/lib/toast'
import { Plus, RefreshCw, FileText, Pencil, Trash2, MessageSquare, Image, FileIcon, Video } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { useSearchPagination } from '@/composables/useSearchPagination'
import { getQualityBadgeVariant, getQualityRatingLabel } from '@/lib/utils'

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
  quality_rating?: string
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
  { key: 'quality_rating', label: t('templates.qualityRating'), sortable: true },
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

function onAccountChange() {
  localStorage.setItem('templates_selected_account', selectedAccount.value)
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

function getStatusBadgeVariant(status: string): string | undefined {
  switch (status) {
    case 'APPROVED':
      return 'success'
    case 'PENDING':
      return 'warning'
    case 'REJECTED':
      return 'danger'
    default:
      return 'secondary'
  }
}

function getCategoryBadgeVariant(category: string): string | undefined {
  return category === 'UTILITY' ? 'secondary' : undefined
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
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('templates.title')" :description="$t('templates.subtitle')" :icon="FileText">
      <template #actions>
        <button type="button" class="outline small" :aria-busy="isSyncing ? 'true' : undefined" :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'" @click="syncTemplates">
          <RefreshCw v-if="!isSyncing" :size="16" />
          {{ $t('templates.syncFromMeta') }}
        </button>
        <RouterLink to="/templates/new" class="button outline small">
          <Plus :size="16" />
          {{ $t('templates.createTemplate') }}
        </RouterLink>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4">
      <ErrorState
        v-if="error && !isLoading"
        :title="$t('common.loadErrorTitle')"
        :description="error"
        :retry-label="$t('common.retry')"
        @retry="fetchTemplates"
      />
      <article v-else class="card">
        <header class="hstack justify-between items-center flex-wrap gap-4">
          <h3>{{ $t('templates.yourTemplates') }}</h3>
          <div class="hstack items-center gap-4 flex-wrap">
            <label class="hstack items-center gap-2">
              <small class="text-light">{{ $t('templates.account') }}:</small>
              <select v-model="selectedAccount" @change="onAccountChange">
                <option value="all">{{ $t('templates.allAccounts') }}</option>
                <option v-for="account in accounts" :key="account.id" :value="account.name">{{ account.name }}</option>
              </select>
            </label>
            <SearchInput v-model="searchQuery" :placeholder="$t('templates.searchTemplates') + '...'" />
          </div>
        </header>
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
            <RouterLink :to="`/templates/${template.id}`">
              <strong>{{ template.display_name || template.name }}</strong>
              <br />
              <small class="text-light"><code>{{ template.name }}</code></small>
            </RouterLink>
          </template>
          <template #cell-category="{ item: template }">
            <span class="badge" :data-variant="getCategoryBadgeVariant(template.category)">{{ template.category }}</span>
          </template>
          <template #cell-status="{ item: template }">
            <span class="badge" :data-variant="getStatusBadgeVariant(template.status)">{{ template.status }}</span>
          </template>
          <template #cell-quality_rating="{ item: template }">
            <span v-if="template.quality_rating" class="badge" :data-variant="getQualityBadgeVariant(template.quality_rating)">
              {{ getQualityRatingLabel(template.quality_rating, t) }}
            </span>
            <small v-else class="text-light">—</small>
          </template>
          <template #cell-language="{ item: template }">
            <span class="text-light">{{ getLanguageName(template.language) }}</span>
          </template>
          <template #cell-header_type="{ item: template }">
            <div class="hstack items-center gap-1">
              <component :is="getHeaderIcon(template.header_type)" :size="16" />
              <small class="text-light">{{ template.header_type || 'NONE' }}</small>
            </div>
          </template>
          <template #cell-actions="{ item: template }">
            <div class="hstack items-center justify-end gap-1">
              <RouterLink :to="`/templates/${template.id}`" class="button ghost icon" :title="$t('common.edit')" :aria-label="$t('common.edit')"><Pencil :size="16" /></RouterLink>
              <IconButton
                :icon="Trash2"
                :label="$t('common.delete')"
                @click="openDeleteDialog(template)"
              />
            </div>
          </template>
          <template #empty-action>
            <div class="hstack items-center justify-center gap-2">
              <button type="button" class="outline small" :disabled="!selectedAccount || selectedAccount === 'all'" @click="syncTemplates">
                <RefreshCw :size="16" />
                {{ $t('templates.syncFromMeta') }}
              </button>
              <RouterLink to="/templates/new" class="button outline small">
                <Plus :size="16" />
                {{ $t('templates.createTemplate') }}
              </RouterLink>
            </div>
          </template>
        </DataTable>
      </article>
    </div>

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
