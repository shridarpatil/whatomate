<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { TagBadge } from '@/components/ui/tag-badge'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, DeleteConfirmDialog, CreateContactDialog, ImportExportDialog, type Column } from '@/components/shared'
import { contactsService, tagsService, accountsService, type Tag, type ImportResult } from '@/services/api'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { toast } from 'vue-sonner'
import { Plus, Users, Pencil, Trash2, MessageSquare, Check, ChevronsUpDown, X, Download } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { getTagColorClass } from '@/lib/constants'
import { useDebounceFn } from '@vueuse/core'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const canWriteContacts = authStore.hasPermission('contacts', 'write')
const canImportContacts = authStore.hasPermission('contacts', 'import')
const canExportContacts = authStore.hasPermission('contacts', 'export')

// Import/Export dialog state
const isImportExportOpen = ref(false)

interface Contact {
  id: string
  phone_number: string
  profile_name: string
  name: string
  whatsapp_account: string
  tags: string[]
  metadata: Record<string, any>
  assigned_user_id: string | null
  last_message_at: string | null
  last_message_preview: string
  unread_count: number
  created_at: string
  updated_at: string
}

interface ContactFormData {
  phone_number: string
  profile_name: string
  whatsapp_account: string
  tags: string[]
}

const defaultFormData: ContactFormData = { phone_number: '', profile_name: '', whatsapp_account: '', tags: [] }

const contacts = ref<Contact[]>([])
const availableTags = ref<Tag[]>([])
const availableAccounts = ref<{ id: string; name: string; phone_number: string }[]>([])
const isLoading = ref(false)
const isSubmitting = ref(false)
const isCreateDialogOpen = ref(false)
const isEditDialogOpen = ref(false)
const editingContact = ref<Contact | null>(null)
const deleteDialogOpen = ref(false)
const contactToDelete = ref<Contact | null>(null)
const formData = ref<ContactFormData>({ ...defaultFormData })
const searchQuery = ref('')
const tagSelectorOpen = ref(false)

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

// Sorting state
const sortKey = ref('last_message_at')
const sortDirection = ref<'asc' | 'desc'>('desc')

const columns = computed<Column<Contact>[]>(() => [
  { key: 'profile_name', label: t('contacts.name'), sortable: true },
  { key: 'phone_number', label: t('contacts.phoneNumber'), sortable: true },
  { key: 'tags', label: t('contacts.tags') },
  { key: 'last_message_at', label: t('contacts.lastMessage'), sortable: true },
  { key: 'created_at', label: t('contacts.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

function openCreateDialog() {
  isCreateDialogOpen.value = true
}

function openEditDialog(contact: Contact) {
  editingContact.value = contact
  formData.value = {
    phone_number: contact.phone_number,
    profile_name: contact.profile_name || '',
    whatsapp_account: contact.whatsapp_account || '',
    tags: contact.tags || []
  }
  isEditDialogOpen.value = true
}

function onContactCreated() {
  fetchContacts()
}

function onImported(_result: ImportResult) {
  // Refresh the contacts list but keep dialog open to show import results
  fetchContacts()
  // Dialog stays open so user can see import results
}

function openDeleteDialog(contact: Contact) {
  contactToDelete.value = contact
  deleteDialogOpen.value = true
}

function closeEditDialog() {
  isEditDialogOpen.value = false
  editingContact.value = null
  formData.value = { ...defaultFormData }
}

function closeDeleteDialog() {
  deleteDialogOpen.value = false
  contactToDelete.value = null
}

async function fetchContacts() {
  isLoading.value = true
  try {
    const response = await contactsService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = response.data as any
    const responseData = data.data || data
    contacts.value = responseData.contacts || []
    totalItems.value = responseData.total ?? contacts.value.length
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedLoad', { resource: t('resources.contacts') })))
  } finally {
    isLoading.value = false
  }
}

async function fetchTags() {
  try {
    const response = await tagsService.list({ limit: 100 })
    const data = response.data as any
    const responseData = data.data || data
    availableTags.value = responseData.tags || []
  } catch (error) {
    // Silently fail - tags are optional
  }
}

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    const data = response.data as any
    const responseData = data.data || data
    availableAccounts.value = responseData.accounts || []
  } catch (error) {
    // Silently fail - accounts are optional
  }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchContacts()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchContacts()
}

onMounted(() => {
  fetchContacts()
  fetchTags()
  fetchAccounts()
})

async function updateContact() {
  if (!editingContact.value) return
  isSubmitting.value = true
  try {
    await contactsService.update(editingContact.value.id, {
      profile_name: formData.value.profile_name,
      whatsapp_account: formData.value.whatsapp_account,
      tags: formData.value.tags
    })
    toast.success(t('common.updatedSuccess', { resource: t('resources.Contact') }))
    closeEditDialog()
    await fetchContacts()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedSave', { resource: t('resources.contact') })))
  } finally {
    isSubmitting.value = false
  }
}

async function confirmDelete() {
  if (!contactToDelete.value) return
  try {
    await contactsService.delete(contactToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Contact') }))
    closeDeleteDialog()
    await fetchContacts()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.contact') })))
  }
}

function openChat(contact: Contact) {
  router.push({ name: 'chat-conversation', params: { contactId: contact.id } })
}

function toggleTag(tagName: string) {
  const index = formData.value.tags.indexOf(tagName)
  if (index === -1) {
    formData.value.tags.push(tagName)
  } else {
    formData.value.tags.splice(index, 1)
  }
}

function removeTag(tagName: string) {
  formData.value.tags = formData.value.tags.filter(t => t !== tagName)
}

function isTagSelected(tagName: string): boolean {
  return formData.value.tags.includes(tagName)
}

function getTagDetails(tagName: string): Tag | undefined {
  return availableTags.value.find(t => t.name === tagName)
}

function getDisplayName(contact: Contact): string {
  return contact.profile_name || contact.name || contact.phone_number
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('contacts.title')" :subtitle="$t('contacts.subtitle')" :icon="Users" icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20">
      <template v-if="canWriteContacts || canImportContacts || canExportContacts" #actions>
        <Button v-if="canImportContacts || canExportContacts" variant="outline" size="sm" @click="isImportExportOpen = true">
          <Download class="h-4 w-4 mr-2" />{{ $t('common.import') }}/{{ $t('common.export') }}
        </Button>
        <Button v-if="canWriteContacts" variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('contacts.addContact') }}</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('contacts.allContacts') }}</CardTitle>
                  <CardDescription>{{ $t('contacts.allContactsDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('contacts.searchContacts') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="contacts"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Users"
                :empty-title="searchQuery ? $t('contacts.noMatchingContacts') : $t('contacts.noContactsYet')"
                :empty-description="searchQuery ? $t('contacts.noMatchingContactsDesc') : $t('contacts.noContactsYetDesc')"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="contacts"
                @page-change="handlePageChange"
              >
                <template #cell-profile_name="{ item: contact }">
                  <div class="flex flex-col">
                    <span class="font-medium">{{ getDisplayName(contact) }}</span>
                    <span v-if="contact.last_message_preview" class="text-xs text-muted-foreground truncate max-w-[200px]">{{ contact.last_message_preview }}</span>
                  </div>
                </template>
                <template #cell-phone_number="{ item: contact }">
                  <code class="text-sm">{{ contact.phone_number }}</code>
                </template>
                <template #cell-tags="{ item: contact }">
                  <div class="flex flex-wrap gap-1">
                    <TagBadge v-for="tag in (contact.tags || []).slice(0, 3)" :key="tag" color="gray" class="text-xs">{{ tag }}</TagBadge>
                    <Badge v-if="(contact.tags || []).length > 3" variant="outline" class="text-xs">+{{ contact.tags.length - 3 }}</Badge>
                  </div>
                </template>
                <template #cell-last_message_at="{ item: contact }">
                  <span class="text-muted-foreground">{{ contact.last_message_at ? formatDate(contact.last_message_at) : $t('contacts.never') }}</span>
                </template>
                <template #cell-created_at="{ item: contact }">
                  <span class="text-muted-foreground">{{ formatDate(contact.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: contact }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openChat(contact)" :title="$t('contacts.openChat')">
                      <MessageSquare class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(contact)" :title="$t('common.edit')">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(contact)" :title="$t('common.delete')">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </template>
                <template v-if="canWriteContacts" #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />
                    {{ $t('contacts.addContact') }}
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Create Contact Dialog (shared component) -->
    <CreateContactDialog v-model:open="isCreateDialogOpen" @created="onContactCreated" />

    <!-- Edit Contact Dialog -->
    <CrudFormDialog
      v-model:open="isEditDialogOpen"
      :is-editing="true"
      :is-submitting="isSubmitting"
      :edit-title="$t('contacts.editTitle')"
      :create-title="$t('contacts.createTitle')"
      :edit-description="$t('contacts.editDesc')"
      :create-description="$t('contacts.createDesc')"
      max-width="max-w-md"
      @submit="updateContact"
    >
      <div class="space-y-4">
        <div class="space-y-2">
          <Label>{{ $t('contacts.phoneNumber') }}</Label>
          <Input :model-value="formData.phone_number" disabled />
        </div>
        <div class="space-y-2">
          <Label>{{ $t('contacts.profileName') }}</Label>
          <Input v-model="formData.profile_name" :placeholder="$t('contacts.namePlaceholder')" />
        </div>
        <div v-if="availableAccounts.length > 0" class="space-y-2">
          <Label>{{ $t('contacts.whatsappAccount') }}</Label>
          <Select v-model="formData.whatsapp_account">
            <SelectTrigger>
              <SelectValue :placeholder="$t('contacts.selectAccount')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="account in availableAccounts" :key="account.id" :value="account.name">
                {{ account.name }} ({{ account.phone_number }})
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div v-if="availableTags.length > 0" class="space-y-2">
          <Label>{{ $t('contacts.tags') }}</Label>
          <Popover v-model:open="tagSelectorOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" role="combobox" class="w-full justify-between">
                <span v-if="formData.tags.length === 0" class="text-muted-foreground">{{ $t('contacts.selectTags') }}</span>
                <span v-else>{{ formData.tags.length }} {{ $t('contacts.tagsSelected') }}</span>
                <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-[300px] p-0" @interact-outside="(e) => e.preventDefault()">
              <Command>
                <CommandInput :placeholder="$t('contacts.searchTags')" />
                <CommandList>
                  <CommandEmpty>{{ $t('contacts.noTagsFound') }}</CommandEmpty>
                  <CommandGroup>
                    <CommandItem
                      v-for="tag in availableTags"
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
          <div v-if="formData.tags.length > 0" class="flex flex-wrap gap-1 mt-2">
            <TagBadge
              v-for="tagName in formData.tags"
              :key="tagName"
              :color="getTagDetails(tagName)?.color"
            >
              {{ tagName }}
              <button
                type="button"
                class="ml-1 rounded-full hover:bg-black/10 dark:hover:bg-white/10 p-0.5 transition-colors"
                @click.stop="removeTag(tagName)"
              >
                <X class="h-3 w-3" />
              </button>
            </TagBadge>
          </div>
        </div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('contacts.deleteContact')"
      :item-name="contactToDelete ? getDisplayName(contactToDelete) : ''"
      :description="$t('contacts.deleteWarning')"
      @confirm="confirmDelete"
    />

    <ImportExportDialog
      v-model:open="isImportExportOpen"
      table="contacts"
      :table-label="$t('contacts.title')"
      :filters="searchQuery ? { search: searchQuery } : undefined"
      :can-import="canImportContacts"
      :can-export="canExportContacts"
      @imported="onImported"
    />
  </div>
</template>
