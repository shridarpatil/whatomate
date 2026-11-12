<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { dataService, type ExportColumn, type ImportResult } from '@/services/api'
import { toast } from '@/lib/toast'
import { Upload, Download, FileSpreadsheet, Check, AlertCircle } from '@/lib/icons'
import Modal from '@/components/oat/Modal.vue'
import { getErrorMessage } from '@/lib/api-utils'

const { t, te } = useI18n()

// Translate column label - uses i18n key if available, falls back to backend label
function translateColumnLabel(col: ExportColumn): string {
  const i18nKey = `importExport.columns.${col.key}`
  return te(i18nKey) ? t(i18nKey) : col.label
}

interface Props {
  open: boolean
  table: string
  tableLabel: string
  filters?: Record<string, string>
  canImport?: boolean
  canExport?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  canImport: true,
  canExport: true
})

const emit = defineEmits<{
  'update:open': [value: boolean]
  'imported': [result: ImportResult]
}>()

const activeTab = ref(props.canExport ? 'export' : 'import')

// Export state
const exportColumns = ref<ExportColumn[]>([])
const defaultColumns = ref<string[]>([])
const selectedColumns = ref<string[]>([])
const isExporting = ref(false)
const isLoadingExportConfig = ref(false)

// Import state
const importRequiredColumns = ref<ExportColumn[]>([])
const importOptionalColumns = ref<ExportColumn[]>([])
const uniqueColumn = ref('')
const importFile = ref<File | null>(null)
const updateOnDuplicate = ref(false)
const isImporting = ref(false)
const isLoadingImportConfig = ref(false)
const importResult = ref<ImportResult | null>(null)

watch(() => props.open, async (isOpen) => {
  if (isOpen) {
    importResult.value = null
    importFile.value = null
    if (props.canExport) {
      await loadExportConfig()
    }
    if (props.canImport) {
      await loadImportConfig()
    }
  }
})

async function loadExportConfig() {
  isLoadingExportConfig.value = true
  try {
    const response = await dataService.getExportConfig(props.table)
    const data = (response.data as any)?.data || response.data
    exportColumns.value = data.columns || []
    defaultColumns.value = data.default_columns || []
    selectedColumns.value = [...defaultColumns.value]
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedLoad', { resource: t('common.configuration') })))
  } finally {
    isLoadingExportConfig.value = false
  }
}

async function loadImportConfig() {
  isLoadingImportConfig.value = true
  try {
    const response = await dataService.getImportConfig(props.table)
    const data = (response.data as any)?.data || response.data
    importRequiredColumns.value = data.required_columns || []
    importOptionalColumns.value = data.optional_columns || []
    uniqueColumn.value = data.unique_column || ''
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedLoad', { resource: t('common.configuration') })))
  } finally {
    isLoadingImportConfig.value = false
  }
}

function toggleColumn(key: string) {
  const index = selectedColumns.value.indexOf(key)
  if (index === -1) {
    selectedColumns.value.push(key)
  } else {
    selectedColumns.value.splice(index, 1)
  }
}

function setUpdateOnDuplicate(e: Event) {
  updateOnDuplicate.value = (e.target as HTMLInputElement).checked
}

function isColumnSelected(key: string): boolean {
  return selectedColumns.value.includes(key)
}

async function handleExport() {
  if (selectedColumns.value.length === 0) {
    toast.error(t('importExport.selectAtLeastOneColumn'))
    return
  }

  isExporting.value = true
  try {
    const response = await dataService.exportData(props.table, selectedColumns.value, props.filters)

    // Create download link
    const blob = new Blob([response.data], { type: 'text/csv' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${props.table}_export_${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    toast.success(t('importExport.exportSuccess', { count: 0 }))
    emit('update:open', false)
  } catch (error) {
    toast.error(getErrorMessage(error, t('importExport.exportFailed')))
  } finally {
    isExporting.value = false
  }
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    importFile.value = target.files[0]
    importResult.value = null
  }
}

async function handleImport() {
  if (!importFile.value) {
    toast.error(t('importExport.selectFile'))
    return
  }

  isImporting.value = true
  importResult.value = null
  try {
    const response = await dataService.importData(props.table, importFile.value, updateOnDuplicate.value)
    const result = (response.data as any)?.data || response.data
    importResult.value = result as ImportResult

    if (result.created > 0 || result.updated > 0) {
      toast.success(t('importExport.importSuccess', { created: result.created, updated: result.updated }))
      emit('imported', result)
    }

    if (result.errors > 0) {
      toast.warning(t('importExport.importPartialErrors', { errors: result.errors }))
    }
  } catch (error) {
    toast.error(getErrorMessage(error, t('importExport.importFailed')))
  } finally {
    isImporting.value = false
  }
}

function closeDialog() {
  emit('update:open', false)
}

const allColumns = computed(() => [...importRequiredColumns.value, ...importOptionalColumns.value])

function downloadSampleCsv() {
  const headers = allColumns.value.map(c => translateColumnLabel(c)).join(',')
  const blob = new Blob([headers + '\n'], { type: 'text/csv' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${props.table}_sample.csv`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}
</script>

<template>
  <Modal
    :open="open"
    size="large"
    :title="$t('importExport.title', { resource: tableLabel })"
    :description="$t('importExport.description', { resource: tableLabel.toLowerCase() })"
    @update:open="emit('update:open', $event)"
  >
    <div class="vstack gap-4">
      <!-- State-driven tabs: same roles as ot-tabs, bound to activeTab -->
      <div v-if="canExport && canImport" role="tablist">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'export'"
          @click="activeTab = 'export'"
        >
          <Download :size="16" />
          {{ $t('importExport.export') }}
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'import'"
          @click="activeTab = 'import'"
        >
          <Upload :size="16" />
          {{ $t('importExport.import') }}
        </button>
      </div>

      <!-- Export Tab -->
      <div v-if="canExport" role="tabpanel" :hidden="activeTab !== 'export'">
       <div class="vstack gap-4">
        <div v-if="isLoadingExportConfig" class="flex items-center justify-center p-4" role="status" aria-busy="true" data-spinner="large" />
        <template v-else>
          <fieldset class="scroll-y">
            <legend>{{ $t('importExport.selectColumns') }}</legend>
            <label v-for="col in exportColumns" :key="col.key" :for="'col-' + col.key">
              <input
                :id="'col-' + col.key"
                type="checkbox"
                :checked="isColumnSelected(col.key)"
                @change="toggleColumn(col.key)"
              >
              {{ translateColumnLabel(col) }}
            </label>
          </fieldset>

          <div class="hstack justify-end gap-2">
            <button type="button" class="outline" @click="closeDialog">{{ $t('common.cancel') }}</button>
            <button
              type="button"
              :disabled="isExporting || selectedColumns.length === 0"
              :aria-busy="isExporting ? 'true' : undefined"
              data-spinner="small"
              @click="handleExport"
            >
              <Download v-if="!isExporting" :size="16" />
              {{ $t('importExport.exportCsv') }}
            </button>
          </div>
        </template>
       </div>
      </div>

      <!-- Import Tab -->
      <div v-if="canImport" role="tabpanel" :hidden="activeTab !== 'import'">
       <div class="vstack gap-4">
        <div v-if="isLoadingImportConfig" class="flex items-center justify-center p-4" role="status" aria-busy="true" data-spinner="large" />
        <template v-else>
          <!-- Column Info -->
          <div class="vstack gap-2">
            <div>
              <strong>{{ $t('importExport.requiredColumns') }}:</strong>
              <span class="text-light">
                {{ importRequiredColumns.map(c => translateColumnLabel(c)).join(', ') }}
              </span>
            </div>
            <div v-if="importOptionalColumns.length > 0">
              <strong>{{ $t('importExport.optionalColumns') }}:</strong>
              <span class="text-light">
                {{ importOptionalColumns.map(c => translateColumnLabel(c)).join(', ') }}
              </span>
            </div>
            <div>
              <button type="button" class="ghost small" @click="downloadSampleCsv">
                <FileSpreadsheet :size="12" />
                {{ $t('importExport.downloadSample') }}
              </button>
            </div>
          </div>

          <!-- File Upload -->
          <label data-field>
            {{ $t('importExport.selectCsvFile') }}
            <input type="file" accept=".csv" @change="handleFileSelect">
          </label>

          <!-- Update on duplicate -->
          <label v-if="uniqueColumn" for="update-dup">
            <input
              id="update-dup"
              type="checkbox"
              :checked="updateOnDuplicate"
              @change="setUpdateOnDuplicate"
            >
            {{ $t('importExport.updateExisting') }}
          </label>

          <!-- Import Result -->
          <div v-if="importResult" role="alert" :data-variant="importResult.errors > 0 ? 'warning' : 'success'" class="vstack gap-2">
            <div class="hstack gap-2">
              <Check v-if="importResult.errors === 0" :size="16" />
              <AlertCircle v-else :size="16" />
              <strong>{{ $t('importExport.importComplete') }}</strong>
            </div>
            <div class="vstack gap-1">
              <p>{{ $t('importExport.created') }}: {{ importResult.created }}</p>
              <p>{{ $t('importExport.updated') }}: {{ importResult.updated }}</p>
              <p v-if="importResult.skipped > 0">{{ $t('importExport.skipped') }}: {{ importResult.skipped }}</p>
              <p v-if="importResult.errors > 0">{{ $t('importExport.errors') }}: {{ importResult.errors }}</p>
            </div>
            <div v-if="importResult.messages && importResult.messages.length > 0" class="scroll-y">
              <small v-for="(msg, i) in importResult.messages" :key="i" class="vstack">{{ msg }}</small>
            </div>
          </div>

          <div class="hstack justify-end gap-2">
            <button type="button" class="outline" @click="closeDialog">{{ $t('common.cancel') }}</button>
            <button
              type="button"
              :disabled="isImporting || !importFile"
              :aria-busy="isImporting ? 'true' : undefined"
              data-spinner="small"
              @click="handleImport"
            >
              <Upload v-if="!isImporting" :size="16" />
              {{ $t('importExport.importCsv') }}
            </button>
          </div>
        </template>
       </div>
      </div>
    </div>
  </Modal>
</template>
