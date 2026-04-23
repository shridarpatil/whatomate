<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useTagsStore } from '@/stores/tags'
import { useUsersStore } from '@/stores/users'
import { contactsService, accountsService, type Tag } from '@/services/api'
import type { Contact } from '@/stores/contacts'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { getTagColorClass } from '@/lib/constants'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { TagBadge } from '@/components/ui/tag-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
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
  Users,
  Phone,
  User,
  Trash2,
  Save,
  MessageSquare,
  Check,
  ChevronsUpDown,
  X,
} from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()
const tagsStore = useTagsStore()
const usersStore = useUsersStore()

const contactId = computed(() => route.params.id as string)
const contact = ref<Contact | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)
const tagSelectorOpen = ref(false)
const agentSelectorOpen = ref(false)

const accounts = ref<{ id: string; name: string; phone_number: string }[]>([])

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('contacts', 'write'))
const canDelete = computed(() => authStore.hasPermission('contacts', 'delete'))

const form = ref({
  profile_name: '',
  phone_number: '',
  whatsapp_account: '',
  tags: [] as string[],
  assigned_user_id: '' as string,
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('contacts.title'), href: '/settings/contacts' },
  { label: contact.value?.profile_name || contact.value?.name || contact.value?.phone_number || '' },
])

const assignedUserName = computed(() => {
  if (!form.value.assigned_user_id) return null
  const user = usersStore.users.find(u => u.id === form.value.assigned_user_id)
  return user?.full_name || null
})

async function loadContact() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await contactsService.get(contactId.value)
    const data = (response.data as any).data || response.data
    contact.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!contact.value) return
  form.value = {
    profile_name: contact.value.profile_name || '',
    phone_number: contact.value.phone_number,
    whatsapp_account: contact.value.whatsapp_account || '',
    tags: contact.value.tags ? [...contact.value.tags] : [],
    assigned_user_id: contact.value.assigned_user_id || '',
  }
}

watch(form, () => {
  if (!contact.value) return
  hasChanges.value = true
}, { deep: true })

async function save() {
  if (!contact.value) return

  isSaving.value = true
  try {
    const payload: Record<string, any> = {
      profile_name: form.value.profile_name,
      whatsapp_account: form.value.whatsapp_account,
      tags: form.value.tags,
    }
    if (form.value.assigned_user_id) {
      payload.assigned_user_id = form.value.assigned_user_id
    } else {
      payload.clear_assigned_agent = true
    }
    await contactsService.update(contact.value.id, payload)
    toast.success(t('common.updatedSuccess', { resource: t('resources.Contact') }))
    await loadContact()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.contact') })))
  } finally {
    isSaving.value = false
  }
}

async function deleteContact() {
  if (!contact.value) return
  try {
    await contactsService.delete(contact.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Contact') }))
    router.push('/settings/contacts')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.contact') })))
  }
  deleteDialogOpen.value = false
}

function openChat() {
  if (!contact.value) return
  router.push({ name: 'chat-conversation', params: { contactId: contact.value.id } })
}

function toggleTag(tagName: string) {
  const index = form.value.tags.indexOf(tagName)
  if (index === -1) {
    form.value.tags.push(tagName)
  } else {
    form.value.tags.splice(index, 1)
  }
}

function removeTag(tagName: string) {
  form.value.tags = form.value.tags.filter(t => t !== tagName)
}

function isTagSelected(tagName: string): boolean {
  return form.value.tags.includes(tagName)
}

function getTagDetails(tagName: string): Tag | undefined {
  return tagsStore.getTagByName(tagName)
}

function selectAgent(userId: string | null) {
  form.value.assigned_user_id = userId || ''
  agentSelectorOpen.value = false
}

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    const data = (response.data as any).data || response.data
    accounts.value = data.accounts || []
  } catch {
    // accounts are optional
  }
}

onMounted(async () => {
  await Promise.all([
    loadContact(),
    fetchAccounts(),
    tagsStore.fetchTags().catch(() => {}),
    usersStore.fetchUsers().catch(() => {}),
  ])
})
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="contact?.profile_name || contact?.name || contact?.phone_number || ''"
      :icon="Users"
      icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20"
      back-link="/settings/contacts"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('contacts.notFound', 'Contact not found')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button v-if="canWrite && hasChanges" size="sm" @click="save" :disabled="isSaving">
            <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.save') }}
          </Button>
          <Button variant="outline" size="sm" @click="openChat">
            <MessageSquare class="h-4 w-4 mr-1" /> {{ $t('contacts.openChat', 'Open Chat') }}
          </Button>
          <Button
            v-if="canDelete"
            variant="destructive"
            size="sm"
            @click="deleteDialogOpen = true"
          >
            <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
          </Button>
        </div>
      </template>

      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <div class="flex items-center gap-2">
              <Badge v-if="contact?.marketing_opt_out" variant="secondary">{{ $t('contacts.marketingOptOut', 'Marketing Opt-out') }}</Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-center gap-3 p-3 rounded-lg bg-muted/50">
            <div class="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
              <Phone class="h-5 w-5 text-primary" />
            </div>
            <div class="min-w-0">
              <p class="font-medium truncate">{{ contact?.profile_name || contact?.name || contact?.phone_number }}</p>
              <p class="text-sm text-muted-foreground truncate">{{ contact?.phone_number }}</p>
            </div>
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('contacts.profileName', 'Profile Name') }}</Label>
            <Input v-model="form.profile_name" :disabled="!canWrite" />
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('contacts.phoneNumber') }}</Label>
            <Input v-model="form.phone_number" disabled />
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('contacts.whatsappAccount', 'WhatsApp Account') }}</Label>
            <Select v-model="form.whatsapp_account" :disabled="!canWrite">
              <SelectTrigger>
                <SelectValue :placeholder="$t('contacts.selectAccount', 'Select account')">
                  <template v-if="form.whatsapp_account">
                    {{ form.whatsapp_account }}
                  </template>
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                  {{ account.name }} ({{ account.phone_number }})
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('contacts.tags') }}</Label>
            <Popover v-model:open="tagSelectorOpen">
              <PopoverTrigger as-child>
                <Button variant="outline" role="combobox" class="w-full justify-between" :disabled="!canWrite">
                  <span v-if="form.tags.length === 0" class="text-muted-foreground">{{ $t('contacts.selectTags') }}</span>
                  <span v-else>{{ form.tags.length }} {{ $t('contacts.tagsSelected') }}</span>
                  <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-[300px] p-0" @interact-outside="(e: Event) => e.preventDefault()">
                <Command>
                  <CommandInput :placeholder="$t('contacts.searchTags')" />
                  <CommandList>
                    <CommandEmpty>{{ $t('contacts.noTagsFound') }}</CommandEmpty>
                    <CommandGroup>
                      <CommandItem
                        v-for="tag in tagsStore.tags"
                        :key="tag.name"
                        :value="tag.name"
                        class="flex items-center gap-2 cursor-pointer"
                        @select.prevent="toggleTag(tag.name)"
                      >
                        <div class="flex items-center gap-2 flex-1">
                          <span :class="['w-2 h-2 rounded-full', getTagColorClass(tag.color).split(' ')[0]]"></span>
                          <span>{{ tag.name }}</span>
                        </div>
                        <Check v-if="isTagSelected(tag.name)" class="h-4 w-4 text-primary" />
                      </CommandItem>
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
            <div v-if="form.tags.length > 0" class="flex flex-wrap gap-1 mt-2">
              <TagBadge
                v-for="tagName in form.tags"
                :key="tagName"
                :color="getTagDetails(tagName)?.color"
              >
                {{ tagName }}
                <button
                  v-if="canWrite"
                  type="button"
                  class="ml-1 rounded-full hover:bg-black/10 dark:hover:bg-white/10 p-0.5 transition-colors"
                  @click.stop="removeTag(tagName)"
                >
                  <X class="h-3 w-3" />
                </button>
              </TagBadge>
            </div>
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('contacts.assignedAgent', 'Assigned Agent') }}</Label>
            <Popover v-model:open="agentSelectorOpen">
              <PopoverTrigger as-child>
                <Button variant="outline" role="combobox" class="w-full justify-between" :disabled="!canWrite">
                  <span v-if="!assignedUserName" class="text-muted-foreground">{{ $t('contacts.selectAgent', 'Select agent') }}</span>
                  <span v-else class="flex items-center gap-2">
                    <User class="h-3.5 w-3.5" />
                    {{ assignedUserName }}
                  </span>
                  <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-[300px] p-0" @interact-outside="(e: Event) => e.preventDefault()">
                <Command>
                  <CommandInput :placeholder="$t('contacts.searchAgents', 'Search agents...')" />
                  <CommandList>
                    <CommandEmpty>{{ $t('contacts.noAgentsFound', 'No agents found') }}</CommandEmpty>
                    <CommandGroup>
                      <CommandItem
                        v-if="form.assigned_user_id"
                        value="__unassign__"
                        class="flex items-center gap-2 text-muted-foreground"
                        @select="selectAgent(null)"
                      >
                        <X class="h-3.5 w-3.5" />
                        <span>{{ $t('contacts.removeAssignment', 'Remove assignment') }}</span>
                      </CommandItem>
                      <CommandItem
                        v-for="u in usersStore.users"
                        :key="u.id"
                        :value="u.full_name"
                        class="flex items-center gap-2"
                        @select="selectAgent(u.id)"
                      >
                        <User class="h-3.5 w-3.5" />
                        <span class="flex-1">{{ u.full_name }}</span>
                        <Check v-if="form.assigned_user_id === u.id" class="h-4 w-4 text-primary" />
                      </CommandItem>
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </div>

          <div class="flex items-center justify-between">
            <Label class="text-xs font-normal">{{ $t('contacts.marketingOptOut', 'Marketing Opt-out') }}</Label>
            <Badge :variant="contact?.marketing_opt_out ? 'destructive' : 'secondary'">
              {{ contact?.marketing_opt_out ? $t('common.yes', 'Yes') : $t('common.no', 'No') }}
            </Badge>
          </div>
        </CardContent>
      </Card>

      <AuditLogPanel
        v-if="contact"
        resource-type="contact"
        :resource-id="contact.id"
      />

      <template #sidebar>
        <MetadataPanel
          :created-at="contact?.created_at"
          :updated-at="contact?.updated_at"
        />
      </template>
    </DetailPageLayout>

    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('contacts.deleteContact') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('contacts.deleteWarning') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteContact">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
