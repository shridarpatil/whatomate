<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { IconButton } from '@/components/shared'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Key,
  Trash2,
  Save,
  Plus,
} from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const keywordId = computed(() => route.params.id as string)
const isNew = computed(() => keywordId.value === 'new')
const keyword = ref<any>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('chatbot.keywords', 'write'))
const canDelete = computed(() => authStore.hasPermission('chatbot.keywords', 'delete'))

interface ButtonItem {
  id: string
  title: string
}

const form = ref({
  keywords: '',
  match_type: 'contains' as 'contains' | 'exact' | 'regex',
  response_type: 'text' as 'text' | 'transfer',
  response_content: '',
  buttons: [] as ButtonItem[],
  priority: 0,
  enabled: true,
})

const breadcrumbs = computed(() => [
  { label: t('nav.chatbot', 'Chatbot'), href: '/chatbot' },
  { label: t('nav.keywords', 'Keywords'), href: '/chatbot/keywords' },
  { label: isNew.value ? t('keywords.newKeyword', 'New Keyword') : (keyword.value?.name || form.value.keywords.split(',')[0]?.trim() || '') },
])

async function loadKeyword() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await chatbotService.getKeyword(keywordId.value)
    const data = (response.data as any).data?.rule || (response.data as any).data || response.data
    keyword.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!keyword.value) return
  form.value = {
    keywords: (keyword.value.keywords || []).join(', '),
    match_type: keyword.value.match_type || 'contains',
    response_type: keyword.value.response_type || 'text',
    response_content: keyword.value.response_content?.body || '',
    buttons: [...(keyword.value.response_content?.buttons || [])],
    priority: keyword.value.priority || 0,
    enabled: keyword.value.enabled ?? true,
  }
}

// Track form changes
watch(form, () => {
  hasChanges.value = true
}, { deep: true })

function addButton() {
  if (form.value.buttons.length >= 10) {
    toast.error(t('keywords.maxButtonsError', 'Maximum 10 buttons allowed'))
    return
  }
  form.value.buttons.push({ id: '', title: '' })
}

function removeButton(index: number) {
  form.value.buttons.splice(index, 1)
}

function buildPayload() {
  const validButtons = form.value.buttons.filter(b => b.title.trim())
  return {
    keywords: form.value.keywords.split(',').map(k => k.trim()).filter(Boolean),
    match_type: form.value.match_type,
    response_type: form.value.response_type,
    response_content: {
      body: form.value.response_content,
      buttons: validButtons.length > 0 ? validButtons : undefined,
    },
    priority: form.value.priority,
    enabled: form.value.enabled,
  }
}

async function save() {
  if (!form.value.keywords.trim()) {
    toast.error(t('keywords.enterKeyword', 'Please enter at least one keyword'))
    return
  }

  if (form.value.response_type !== 'transfer' && !form.value.response_content.trim()) {
    toast.error(t('keywords.enterResponse', 'Please enter a response message'))
    return
  }

  isSaving.value = true
  try {
    const data = buildPayload()

    if (isNew.value) {
      const response = await chatbotService.createKeyword(data)
      const created = (response.data as any).data?.rule || (response.data as any).data || response.data
      hasChanges.value = false
      toast.success(t('common.createdSuccess', { resource: t('resources.KeywordRule', 'Keyword Rule') }))
      router.replace(`/chatbot/keywords/${created.id}`)
    } else {
      await chatbotService.updateKeyword(keyword.value!.id, data)
      await loadKeyword()
      hasChanges.value = false
      toast.success(t('common.updatedSuccess', { resource: t('resources.KeywordRule', 'Keyword Rule') }))
    }
  } catch (error: any) {
    toast.error(getErrorMessage(error, isNew.value
      ? t('common.failedSave', { resource: t('resources.keywordRule', 'keyword rule') })
      : t('common.failedSave', { resource: t('resources.keywordRule', 'keyword rule') })
    ))
  } finally {
    isSaving.value = false
  }
}

async function deleteKeyword() {
  if (!keyword.value) return
  try {
    await chatbotService.deleteKeyword(keyword.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.KeywordRule', 'Keyword Rule') }))
    router.push('/chatbot/keywords')
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.keywordRule', 'keyword rule') })))
  }
  deleteDialogOpen.value = false
}

onMounted(async () => {
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadKeyword()
  }
})
</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('keywords.newKeyword', 'New Keyword Rule') : (keyword?.name || form.keywords.split(',')[0]?.trim() || '')"
    :icon="Key"
    icon-gradient="bg-gradient-to-br from-yellow-500 to-orange-600 shadow-yellow-500/20"
    back-link="/chatbot/keywords"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('keywords.notFound', 'Keyword rule not found')"
  >
    <template #actions>
      <div class="flex items-center gap-2">
        <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
          <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </Button>
        <Button v-if="canDelete && !isNew" variant="destructive" size="sm" @click="deleteDialogOpen = true">
          <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
        </Button>
      </div>
    </template>

    <!-- Keyword Details Card -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex items-center justify-between">
          <CardTitle class="text-sm font-medium">{{ $t('keywords.details', 'Details') }}</CardTitle>
          <Badge :variant="(keyword?.enabled ?? form.enabled) ? 'default' : 'secondary'">
            {{ (keyword?.enabled ?? form.enabled) ? $t('keywords.active', 'Active') : $t('keywords.inactive', 'Inactive') }}
          </Badge>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('keywords.keywordsLabel', 'Keywords') }} *</Label>
          <Input
            v-model="form.keywords"
            :placeholder="$t('keywords.keywordsPlaceholder', 'hello, hi, hey')"
            :disabled="!canWrite"
          />
          <p class="text-xs text-muted-foreground">{{ $t('keywords.keywordsHint', 'Comma-separated list of keywords') }}</p>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('keywords.matchTypeLabel', 'Match Type') }}</Label>
          <Select v-model="form.match_type" :disabled="!canWrite">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="contains">{{ $t('keywords.contains', 'Contains') }}</SelectItem>
              <SelectItem value="exact">{{ $t('keywords.exact', 'Exact') }}</SelectItem>
              <SelectItem value="regex">{{ $t('keywords.regex', 'Regex') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('keywords.responseType', 'Response Type') }}</Label>
          <Select v-model="form.response_type" :disabled="!canWrite">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="text">{{ $t('keywords.textResponse', 'Text Response') }}</SelectItem>
              <SelectItem value="transfer">{{ $t('keywords.transferToAgent', 'Transfer to Agent') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">
            {{ form.response_type === 'transfer' ? $t('keywords.transferMessage', 'Transfer Message') : $t('keywords.responseMessage', 'Response Message') }}
            <span v-if="form.response_type !== 'transfer'">*</span>
          </Label>
          <Textarea
            v-model="form.response_content"
            :placeholder="form.response_type === 'transfer' ? $t('keywords.transferPlaceholder', 'Optional message before transfer') + '...' : $t('keywords.responsePlaceholder', 'Enter the response message') + '...'"
            :rows="3"
            :disabled="!canWrite"
          />
          <p v-if="form.response_type === 'transfer'" class="text-xs text-muted-foreground">
            {{ $t('keywords.transferHint', 'Optional message sent before transferring to a human agent') }}
          </p>
        </div>

        <!-- Buttons Section (only for text responses) -->
        <div v-if="form.response_type === 'text'" class="space-y-1.5">
          <div class="flex items-center justify-between">
            <Label class="text-xs">{{ $t('keywords.buttonsOptional', 'Buttons (optional)') }}</Label>
            <Button
              v-if="canWrite"
              type="button"
              variant="outline"
              size="sm"
              class="h-7 text-xs"
              @click="addButton"
              :disabled="form.buttons.length >= 10"
            >
              <Plus class="h-3 w-3 mr-1" />
              {{ $t('keywords.addButton', 'Add Button') }}
            </Button>
          </div>
          <p class="text-xs text-muted-foreground">
            {{ $t('keywords.buttonsHint', 'Add quick-reply buttons to the response message') }}
          </p>
          <div v-if="form.buttons.length > 0" class="space-y-2 mt-2">
            <div
              v-for="(button, index) in form.buttons"
              :key="index"
              class="flex items-center gap-2"
            >
              <Input
                v-model="button.id"
                :placeholder="$t('keywords.buttonId', 'Button ID')"
                class="flex-1"
                :disabled="!canWrite"
              />
              <Input
                v-model="button.title"
                :placeholder="$t('keywords.buttonTitle', 'Button Title')"
                class="flex-1"
                :disabled="!canWrite"
              />
              <IconButton
                v-if="canWrite"
                :icon="Trash2"
                :label="$t('keywords.removeButtonLabel', 'Remove button')"
                class="text-destructive"
                @click="removeButton(index)"
              />
            </div>
          </div>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('keywords.priorityLabel', 'Priority') }}</Label>
          <Input
            v-model.number="form.priority"
            type="number"
            min="1"
            max="100"
            :disabled="!canWrite"
          />
        </div>
        <div class="flex items-center gap-2">
          <Switch :checked="form.enabled" @update:checked="form.enabled = $event" :disabled="!canWrite" />
          <Label class="text-xs">{{ $t('keywords.enabled', 'Enabled') }}</Label>
        </div>
      </CardContent>
    </Card>

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="keyword && !isNew"
      resource-type="keyword_rule"
      :resource-id="keyword.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <MetadataPanel
        :created-at="keyword?.created_at"
        :updated-at="keyword?.updated_at"
        :created-by-name="keyword?.created_by_name"
        :updated-by-name="keyword?.updated_by_name"
      />
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <AlertDialog v-model:open="deleteDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('keywords.deleteRule', 'Delete Keyword Rule') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('keywords.deleteRuleDesc', 'Are you sure? This action cannot be undone.') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="deleteKeyword">{{ $t('common.delete') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
