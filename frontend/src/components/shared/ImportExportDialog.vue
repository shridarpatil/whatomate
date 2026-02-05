<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ScrollArea } from '@/components/ui/scroll-area'
import { dataService, type ExportColumn, type ImportResult } from '@/services/api'
import { toast } from 'vue-sonner'
import { Loader2, Upload, Download, FileSpreadsheet, Check, AlertCircle } from 'lucide-vue-next'
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
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ $t('importExport.title', { resource: tableLabel }) }}</DialogTitle>
        <DialogDescription>{{ $t('importExport.description', { resource: tableLabel.toLowerCase() }) }}</DialogDescription>
      </DialogHeader>

      <Tabs v-model="activeTab" class="w-full">
        <TabsList v-if="canExport && canImport" class="grid w-full grid-cols-2">
          <TabsTrigger value="export">
            <Download class="h-4 w-4 mr-2" />
            {{ $t('importExport.export') }}
          </TabsTrigger>
          <TabsTrigger value="import">
            <Upload class="h-4 w-4 mr-2" />
            {{ $t('importExport.import') }}
          </TabsTrigger>
        </TabsList>

        <!-- Export Tab -->
        <TabsContent v-if="canExport" value="export" class="space-y-4 mt-4">
          <div v-if="isLoadingExportConfig" class="flex items-center justify-center py-8">
            <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <template v-else>
            <div class="space-y-2">
              <Label>{{ $t('importExport.selectColumns') }}</Label>
              <ScrollArea class="h-48 border rounded-md p-3">
                <div class="space-y-2">
                  <div v-for="col in exportColumns" :key="col.key" class="flex items-center space-x-2">
                    <Checkbox
                      :id="'col-' + col.key"
                      :checked="isColumnSelected(col.key)"
                      @update:checked="toggleColumn(col.key)"
                    />
                    <Label :for="'col-' + col.key" class="cursor-pointer font-normal">{{ translateColumnLabel(col) }}</Label>
                  </div>
                </div>
              </ScrollArea>
            </div>

            <div class="flex justify-end gap-2 pt-2">
              <Button variant="outline" @click="closeDialog">{{ $t('common.cancel') }}</Button>
              <Button @click="handleExport" :disabled="isExporting || selectedColumns.length === 0">
                <Loader2 v-if="isExporting" class="h-4 w-4 mr-2 animate-spin" />
                <Download v-else class="h-4 w-4 mr-2" />
                {{ $t('importExport.exportCsv') }}
              </Button>
            </div>
          </template>
        </TabsContent>

        <!-- Import Tab -->
        <TabsContent v-if="canImport" value="import" class="space-y-4 mt-4">
          <div v-if="isLoadingImportConfig" class="flex items-center justify-center py-8">
            <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <template v-else>
            <!-- Column Info -->
            <div class="space-y-2">
              <div class="text-sm">
                <span class="font-medium">{{ $t('importExport.requiredColumns') }}:</span>
                <span class="text-muted-foreground ml-1">
                  {{ importRequiredColumns.map(c => translateColumnLabel(c)).join(', ') }}
                </span>
              </div>
              <div v-if="importOptionalColumns.length > 0" class="text-sm">
                <span class="font-medium">{{ $t('importExport.optionalColumns') }}:</span>
                <span class="text-muted-foreground ml-1">
                  {{ importOptionalColumns.map(c => translateColumnLabel(c)).join(', ') }}
                </span>
              </div>
              <Button variant="link" size="sm" class="h-auto p-0 text-xs" @click="downloadSampleCsv">
                <FileSpreadsheet class="h-3 w-3 mr-1" />
                {{ $t('importExport.downloadSample') }}
              </Button>
            </div>

            <!-- File Upload -->
            <div class="space-y-2">
              <Label>{{ $t('importExport.selectCsvFile') }}</Label>
              <Input
                type="file"
                accept=".csv"
                @change="handleFileSelect"
              />
            </div>

            <!-- Update on duplicate -->
            <div v-if="uniqueColumn" class="flex items-center space-x-2">
              <Checkbox
                id="update-dup"
                :checked="updateOnDuplicate"
                @update:checked="updateOnDuplicate = !!$event"
              />
              <Label for="update-dup" class="cursor-pointer font-normal text-sm">
                {{ $t('importExport.updateExisting') }}
              </Label>
            </div>

            <!-- Import Result -->
            <div v-if="importResult" class="rounded-md p-3 space-y-2" :class="importResult.errors > 0 ? 'bg-amber-500/10 border border-amber-500/20' : 'bg-green-500/10 border border-green-500/20'">
              <div class="flex items-center gap-2">
                <Check v-if="importResult.errors === 0" class="h-4 w-4 text-green-500" />
                <AlertCircle v-else class="h-4 w-4 text-amber-500" />
                <span class="font-medium">{{ $t('importExport.importComplete') }}</span>
              </div>
              <div class="text-sm text-muted-foreground space-y-1">
                <p>{{ $t('importExport.created') }}: {{ importResult.created }}</p>
                <p>{{ $t('importExport.updated') }}: {{ importResult.updated }}</p>
                <p v-if="importResult.skipped > 0">{{ $t('importExport.skipped') }}: {{ importResult.skipped }}</p>
                <p v-if="importResult.errors > 0" class="text-amber-500">{{ $t('importExport.errors') }}: {{ importResult.errors }}</p>
              </div>
              <ScrollArea v-if="importResult.messages && importResult.messages.length > 0" class="h-24 text-xs">
                <div v-for="(msg, i) in importResult.messages" :key="i" class="text-amber-500">{{ msg }}</div>
              </ScrollArea>
            </div>

            <div class="flex justify-end gap-2 pt-2">
              <Button variant="outline" @click="closeDialog">{{ $t('common.cancel') }}</Button>
              <Button @click="handleImport" :disabled="isImporting || !importFile">
                <Loader2 v-if="isImporting" class="h-4 w-4 mr-2 animate-spin" />
                <Upload v-else class="h-4 w-4 mr-2" />
                {{ $t('importExport.importCsv') }}
              </Button>
            </div>
          </template>
        </TabsContent>
      </Tabs>
    </DialogContent>
  </Dialog>
</template>
