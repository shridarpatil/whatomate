<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useTagsStore } from '@/stores/tags'
import { useUsersStore } from '@/stores/users'
import { contactsService, accountsService } from '@/services/api'
import type { Contact } from '@/stores/contacts'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import DeleteConfirmDialog from '@/components/shared/DeleteConfirmDialog.vue'
import Combobox from '@/components/oat/Combobox.vue'
import { filterOptions, type ComboOption } from '@/components/oat/combobox'
import {
  Users,
  Phone,
  Trash2,
  Save,
  MessageSquare,
  X,
} from '@/lib/icons'

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

const agentOptions = computed<ComboOption[]>(() => {
  const opts: ComboOption[] = usersStore.users.map(u => ({ value: u.id, label: u.full_name }))
  if (form.value.assigned_user_id) {
    opts.unshift({ value: '', label: t('contacts.removeAssignment', 'Remove assignment') })
  }
  return opts
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

const tagSearch = ref('')
const filteredTags = computed(() =>
  filterOptions(tagsStore.tags.map(tag => ({ value: tag.name, label: tag.name })), tagSearch.value),
)

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
      back-link="/settings/contacts"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('contacts.notFound', 'Contact not found')"
    >
      <template #actions>
        <div class="hstack gap-2">
          <button v-if="canWrite && hasChanges" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
            <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.save') }}
          </button>
          <button type="button" class="outline small" @click="openChat">
            <MessageSquare :size="16" /> {{ $t('contacts.openChat', 'Open Chat') }}
          </button>
          <button v-if="canDelete" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
            <Trash2 :size="16" /> {{ $t('common.delete') }}
          </button>
        </div>
      </template>

      <article class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('teams.details', 'Details') }}</h3>
          <span v-if="contact?.marketing_opt_out" class="badge" data-variant="secondary">{{ $t('contacts.marketingOptOut', 'Marketing Opt-out') }}</span>
        </header>
        <div class="vstack gap-4">
          <div class="hstack gap-2">
            <Phone :size="20" />
            <div class="min-w-0">
              <p class="truncate"><strong>{{ contact?.profile_name || contact?.name || contact?.phone_number }}</strong></p>
              <small class="text-light truncate">{{ contact?.phone_number }}</small>
            </div>
          </div>

          <label data-field>
            {{ $t('contacts.profileName', 'Profile Name') }}
            <input v-model="form.profile_name" :disabled="!canWrite">
          </label>

          <label data-field>
            {{ $t('contacts.phoneNumber') }}
            <input v-model="form.phone_number" disabled>
          </label>

          <label data-field>
            {{ $t('contacts.whatsappAccount', 'WhatsApp Account') }}
            <select v-model="form.whatsapp_account" :disabled="!canWrite">
              <option value="" disabled>{{ $t('contacts.selectAccount', 'Select account') }}</option>
              <option v-if="form.whatsapp_account && !accounts.some(a => a.name === form.whatsapp_account)" :value="form.whatsapp_account">
                {{ form.whatsapp_account }}
              </option>
              <option v-for="account in accounts" :key="account.id" :value="account.name">
                {{ account.name }} ({{ account.phone_number }})
              </option>
            </select>
          </label>

          <div class="vstack gap-2">
            <span id="contact-tags-label">{{ $t('contacts.tags') }}</span>
            <details v-if="canWrite" aria-labelledby="contact-tags-label">
              <summary>
                <span v-if="form.tags.length === 0" class="text-light">{{ $t('contacts.selectTags') }}</span>
                <span v-else>{{ form.tags.length }} {{ $t('contacts.tagsSelected') }}</span>
              </summary>
              <div class="vstack gap-2">
                <input
                  v-model="tagSearch"
                  type="search"
                  :placeholder="$t('contacts.searchTags')"
                  :aria-label="$t('contacts.searchTags')"
                >
                <div class="vstack gap-2 scroll-y picker-list">
                  <p v-if="filteredTags.length === 0" class="text-light">{{ $t('contacts.noTagsFound') }}</p>
                  <label v-for="tag in filteredTags" :key="tag.value" class="hstack gap-2">
                    <input type="checkbox" :checked="isTagSelected(tag.value)" @change="toggleTag(tag.value)">
                    {{ tag.label }}
                  </label>
                </div>
              </div>
            </details>
            <div v-if="form.tags.length > 0" class="hstack gap-1 flex-wrap">
              <span v-for="tagName in form.tags" :key="tagName" class="badge" data-variant="secondary">
                {{ tagName }}
                <button
                  v-if="canWrite"
                  type="button"
                  class="ghost icon small"
                  :aria-label="$t('common.remove', 'Remove') + ' ' + tagName"
                  @click.stop="removeTag(tagName)"
                >
                  <X :size="12" />
                </button>
              </span>
            </div>
          </div>

          <div class="vstack gap-2">
            <span id="contact-agent-label">{{ $t('contacts.assignedAgent', 'Assigned Agent') }}</span>
            <Combobox
              :model-value="form.assigned_user_id"
              :options="agentOptions"
              :placeholder="$t('contacts.selectAgent', 'Select agent')"
              :search-placeholder="$t('contacts.searchAgents', 'Search agents...')"
              :empty-text="$t('contacts.noAgentsFound', 'No agents found')"
              :disabled="!canWrite"
              aria-labelledby="contact-agent-label"
              @update:model-value="form.assigned_user_id = $event"
            />
          </div>

          <div class="hstack justify-between">
            <span>{{ $t('contacts.marketingOptOut', 'Marketing Opt-out') }}</span>
            <span class="badge" :data-variant="contact?.marketing_opt_out ? 'danger' : 'secondary'">
              {{ contact?.marketing_opt_out ? $t('common.yes', 'Yes') : $t('common.no', 'No') }}
            </span>
          </div>
        </div>
      </article>

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

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('contacts.deleteContact')"
      :item-name="contact?.profile_name || contact?.name || contact?.phone_number || ''"
      :description="$t('contacts.deleteWarning')"
      @confirm="deleteContact"
    />

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
