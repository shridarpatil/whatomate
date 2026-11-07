<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { contactsService } from '@/services/api'
import { useContactsStore, type Contact } from '@/stores/contacts'

const MAX_NAME_LENGTH = 255

const props = defineProps<{
  open: boolean
  contact: Contact | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { t } = useI18n()
const contactsStore = useContactsStore()
const name = ref('')
const saving = ref(false)

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) name.value = props.contact?.profile_name || props.contact?.name || ''
  },
  { immediate: true }
)

async function save() {
  if (!props.contact || saving.value) return
  const newName = name.value.trim()
  if (newName.length > MAX_NAME_LENGTH) {
    toast.error(t('chat.renameTooLong', 'Name is too long'))
    return
  }
  saving.value = true
  try {
    const response = await contactsService.update(props.contact.id, { profile_name: newName })
    const updated = response.data?.data ?? response.data
    contactsStore.syncContact({
      ...props.contact,
      name: updated?.name ?? newName,
      profile_name: updated?.profile_name ?? newName,
    })
    toast.success(t('chat.renameSaved', 'Contact name updated'))
    emit('update:open', false)
  } catch (error: any) {
    toast.error(error?.response?.data?.message || t('chat.renameFailed', 'Could not update the name'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="(value) => emit('update:open', value)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('chat.renameContact', 'Rename contact') }}</DialogTitle>
        <DialogDescription>
          {{ t('chat.renameDescription', 'This name is shown in chats and contacts. WhatsApp profile name changes will no longer replace it.') }}
        </DialogDescription>
      </DialogHeader>
      <form class="space-y-2" @submit.prevent="save">
        <Label for="rename-contact-name">{{ t('chat.contactName', 'Name') }}</Label>
        <Input
          id="rename-contact-name"
          v-model="name"
          :maxlength="MAX_NAME_LENGTH"
          :placeholder="contact?.phone_number"
          autofocus
        />
        <p class="text-xs text-muted-foreground">{{ contact?.phone_number }}</p>
        <DialogFooter class="pt-2">
          <Button type="button" variant="outline" :disabled="saving" @click="emit('update:open', false)">
            {{ t('common.cancel', 'Cancel') }}
          </Button>
          <Button type="submit" :disabled="saving">
            {{ saving ? t('common.saving', 'Saving...') : t('common.save', 'Save') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
