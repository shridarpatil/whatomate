<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { contactsService, accountsService, type Tag } from '@/services/api'
import { useTagsStore } from '@/stores/tags'
import { toast } from '@/lib/toast'
import { X } from '@/lib/icons'
import Modal from '@/components/oat/Modal.vue'
import Combobox from '@/components/oat/Combobox.vue'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()
const tagsStore = useTagsStore()

interface Props {
  open: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  'created': [contact: any]
}>()

interface ContactFormData {
  phone_number: string
  profile_name: string
  whatsapp_account: string
  tags: string[]
}

const defaultFormData: ContactFormData = { phone_number: '', profile_name: '', whatsapp_account: '', tags: [] }

const formData = ref<ContactFormData>({ ...defaultFormData })
const isSubmitting = ref(false)
const availableTags = ref<Tag[]>([])
const availableAccounts = ref<{ id: string; name: string; phone_number: string }[]>([])

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    formData.value = { ...defaultFormData }
    fetchTags()
    fetchAccounts()
  }
})

async function fetchTags() {
  try {
    const response = await tagsStore.fetchTags({ limit: 100 })
    availableTags.value = response.tags
  } catch {
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

async function saveContact() {
  if (!formData.value.phone_number.trim()) {
    toast.error(t('contacts.phoneRequired'))
    return
  }
  isSubmitting.value = true
  try {
    const response = await contactsService.create({
      phone_number: formData.value.phone_number.trim(),
      profile_name: formData.value.profile_name.trim() || undefined,
      whatsapp_account: formData.value.whatsapp_account || undefined,
      tags: formData.value.tags.length > 0 ? formData.value.tags : undefined
    })
    const contact = response.data?.data || response.data
    toast.success(t('common.createdSuccess', { resource: t('resources.Contact') }))
    emit('update:open', false)
    emit('created', contact)
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedCreate', { resource: t('resources.contact') })))
  } finally {
    isSubmitting.value = false
  }
}

// Tags already chosen are offered no more; they are removed via their badge.
const tagOptions = computed(() =>
  availableTags.value
    .filter(tag => !formData.value.tags.includes(tag.name))
    .map(tag => ({ value: tag.name, label: tag.name })),
)

function addTag(tagName: string) {
  if (!formData.value.tags.includes(tagName)) formData.value.tags.push(tagName)
}

function removeTag(tagName: string) {
  formData.value.tags = formData.value.tags.filter(t => t !== tagName)
}

function closeDialog() {
  emit('update:open', false)
}
</script>

<template>
  <Modal
    :open="open"
    :title="$t('contacts.createTitle')"
    :description="$t('contacts.createDesc')"
    @update:open="emit('update:open', $event)"
  >
    <div class="vstack gap-4">
      <label data-field>
        {{ $t('contacts.phoneNumber') }} *
        <input v-model="formData.phone_number" :placeholder="$t('contacts.phonePlaceholder')">
        <small data-hint>{{ $t('contacts.phoneHint') }}</small>
      </label>
      <label data-field>
        {{ $t('contacts.profileName') }}
        <input v-model="formData.profile_name" :placeholder="$t('contacts.namePlaceholder')">
      </label>
      <label v-if="availableAccounts.length > 0" data-field>
        {{ $t('contacts.whatsappAccount') }}
        <select v-model="formData.whatsapp_account">
          <option value="" disabled>{{ $t('contacts.selectAccount') }}</option>
          <option v-for="account in availableAccounts" :key="account.id" :value="account.name">
            {{ account.name }} ({{ account.phone_number }})
          </option>
        </select>
      </label>
      <div v-if="availableTags.length > 0" data-field>
        <label for="create-contact-tags-input">{{ $t('contacts.tags') }}</label>
        <Combobox
          id="create-contact-tags-input"
          :model-value="null"
          :options="tagOptions"
          :placeholder="formData.tags.length === 0 ? $t('contacts.selectTags') : `${formData.tags.length} ${$t('contacts.tagsSelected')}`"
          :search-placeholder="$t('contacts.searchTags')"
          :empty-text="$t('contacts.noTagsFound')"
          @update:model-value="addTag"
        />
        <div v-if="formData.tags.length > 0" class="hstack gap-1 flex-wrap">
          <span
            v-for="tagName in formData.tags"
            :key="tagName"
            class="badge"
            data-variant="secondary"
          >
            {{ tagName }}
            <button
              type="button"
              class="ghost small icon"
              :title="$t('common.remove')"
              :aria-label="`${$t('common.remove')} ${tagName}`"
              @click.stop="removeTag(tagName)"
            >
              <X :size="12" />
            </button>
          </span>
        </div>
      </div>
    </div>
    <template #footer>
      <button type="button" class="outline" @click="closeDialog">{{ $t('common.cancel') }}</button>
      <button
        type="button"
        :disabled="isSubmitting"
        :aria-busy="isSubmitting ? 'true' : undefined"
        data-spinner="small"
        @click="saveContact"
      >
        {{ $t('common.create') }}
      </button>
    </template>
  </Modal>
</template>
