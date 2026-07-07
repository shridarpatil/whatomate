<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TagBadge } from '@/components/ui/tag-badge'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { contactsService, accountsService, type Tag } from '@/services/api'
import { useTagsStore } from '@/stores/tags'
import { toast } from 'vue-sonner'
import { Loader2, Check, ChevronsUpDown, X, AlertCircle } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { getTagColorClass } from '@/lib/constants'

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
const tagSelectorOpen = ref(false)
const availableTags = ref<Tag[]>([])
const availableAccounts = ref<{ id: string; name: string; phone_number: string }[]>([])
const showErrors = ref(false)

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    formData.value = { ...defaultFormData }
    showErrors.value = false
    fetchTags()
    fetchAccounts()
  }
})

// ── Validation ──────────────────────────────────────────────────────────────
const errors = computed(() => {
  const e: Record<string, string> = {}
  if (!formData.value.phone_number.trim())
    e.phone_number = 'Telefone é obrigatório'
  if (!formData.value.profile_name.trim())
    e.profile_name = 'Nome é obrigatório'
  if (availableAccounts.value.length > 1 && !formData.value.whatsapp_account)
    e.whatsapp_account = 'Selecione a conta WhatsApp'
  if (formData.value.tags.length === 0)
    e.tags = 'Selecione pelo menos uma etiqueta'
  return e
})

const isFormValid = computed(() => Object.keys(errors.value).length === 0)

async function fetchTags() {
  try {
    const response = await tagsStore.fetchTags({ limit: 100 })
    availableTags.value = response.tags
    // Auto-select account when only one exists
    if (availableAccounts.value.length === 0) await fetchAccounts()
  } catch { /* silent */ }
}

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    const data = response.data as any
    const responseData = data.data || data
    availableAccounts.value = responseData.accounts || []
    // Auto-select if only one account
    if (availableAccounts.value.length === 1) {
      formData.value.whatsapp_account = availableAccounts.value[0].name
    }
  } catch { /* silent */ }
}

async function saveContact() {
  showErrors.value = true
  if (!isFormValid.value) {
    const firstError = Object.values(errors.value)[0]
    toast.error(firstError)
    return
  }
  isSubmitting.value = true
  try {
    const response = await contactsService.create({
      phone_number: formData.value.phone_number.trim(),
      profile_name: formData.value.profile_name.trim(),
      whatsapp_account: formData.value.whatsapp_account || undefined,
      tags: formData.value.tags,
    })
    const contact = response.data?.data || response.data
    toast.success('Contato criado e atribuído a você com sucesso')
    emit('update:open', false)
    emit('created', contact)
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedCreate', { resource: t('resources.contact') })))
  } finally {
    isSubmitting.value = false
  }
}

function toggleTag(tagName: string) {
  const index = formData.value.tags.indexOf(tagName)
  if (index === -1) formData.value.tags.push(tagName)
  else formData.value.tags.splice(index, 1)
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

function closeDialog() {
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ $t('contacts.createTitle') }}</DialogTitle>
        <DialogDescription>
          Preencha todos os campos obrigatórios <span class="text-destructive font-medium">*</span> para cadastrar o contato. Ele será atribuído automaticamente a você.
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">

        <!-- Telefone * -->
        <div class="space-y-1.5">
          <Label>Telefone <span class="text-destructive">*</span></Label>
          <Input
            v-model="formData.phone_number"
            :placeholder="$t('contacts.phonePlaceholder')"
            :class="showErrors && errors.phone_number ? 'border-destructive focus-visible:ring-destructive' : ''"
          />
          <p v-if="showErrors && errors.phone_number" class="text-xs text-destructive flex items-center gap-1">
            <AlertCircle class="h-3 w-3 shrink-0" />{{ errors.phone_number }}
          </p>
          <p v-else class="text-xs text-muted-foreground">{{ $t('contacts.phoneHint') }}</p>
        </div>

        <!-- Nome * -->
        <div class="space-y-1.5">
          <Label>Nome <span class="text-destructive">*</span></Label>
          <Input
            v-model="formData.profile_name"
            :placeholder="$t('contacts.namePlaceholder')"
            :class="showErrors && errors.profile_name ? 'border-destructive focus-visible:ring-destructive' : ''"
          />
          <p v-if="showErrors && errors.profile_name" class="text-xs text-destructive flex items-center gap-1">
            <AlertCircle class="h-3 w-3 shrink-0" />{{ errors.profile_name }}
          </p>
        </div>

        <!-- Conta WhatsApp (obrigatório se houver mais de uma) -->
        <div v-if="availableAccounts.length > 1" class="space-y-1.5">
          <Label>Conta WhatsApp <span class="text-destructive">*</span></Label>
          <Select v-model="formData.whatsapp_account">
            <SelectTrigger :class="showErrors && errors.whatsapp_account ? 'border-destructive' : ''">
              <SelectValue placeholder="Selecione a conta" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="account in availableAccounts" :key="account.id" :value="account.name">
                {{ account.name }} ({{ account.phone_number }})
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="showErrors && errors.whatsapp_account" class="text-xs text-destructive flex items-center gap-1">
            <AlertCircle class="h-3 w-3 shrink-0" />{{ errors.whatsapp_account }}
          </p>
        </div>

        <!-- Etiqueta * -->
        <div class="space-y-1.5">
          <Label>Etiqueta <span class="text-destructive">*</span></Label>
          <Popover v-model:open="tagSelectorOpen">
            <PopoverTrigger as-child>
              <Button
                variant="outline"
                role="combobox"
                class="w-full justify-between"
                :class="showErrors && errors.tags ? 'border-destructive' : ''"
              >
                <span v-if="formData.tags.length === 0" class="text-muted-foreground">
                  Selecione ao menos uma etiqueta
                </span>
                <span v-else class="text-left">
                  {{ formData.tags.length }} etiqueta{{ formData.tags.length > 1 ? 's' : '' }} selecionada{{ formData.tags.length > 1 ? 's' : '' }}
                </span>
                <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-[300px] p-0" @interact-outside="(e) => e.preventDefault()">
              <Command>
                <CommandInput placeholder="Buscar etiqueta..." />
                <CommandList>
                  <CommandEmpty>Nenhuma etiqueta encontrada.</CommandEmpty>
                  <CommandGroup>
                    <CommandItem
                      v-for="tag in availableTags"
                      :key="tag.name"
                      :value="tag.name"
                      class="flex items-center gap-2 cursor-pointer"
                      @select.prevent="toggleTag(tag.name)"
                    >
                      <div class="flex items-center gap-2 flex-1">
                        <span :class="['w-2 h-2 rounded-full shrink-0', getTagColorClass(tag.color).split(' ')[0]]"></span>
                        <span>{{ tag.name }}</span>
                      </div>
                      <Check v-if="isTagSelected(tag.name)" class="h-4 w-4 text-primary shrink-0" />
                    </CommandItem>
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
          <p v-if="showErrors && errors.tags" class="text-xs text-destructive flex items-center gap-1">
            <AlertCircle class="h-3 w-3 shrink-0" />{{ errors.tags }}
          </p>
          <!-- Selected tag chips -->
          <div v-if="formData.tags.length > 0" class="flex flex-wrap gap-1 pt-1">
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

        <!-- Info: auto-assign -->
        <div class="flex items-start gap-2 rounded-lg bg-emerald-500/8 border border-emerald-500/20 px-3 py-2.5">
          <span class="text-emerald-400 text-sm mt-0.5 shrink-0">✓</span>
          <p class="text-xs text-emerald-300/80 leading-relaxed">
            O contato será automaticamente atribuído a você e ficará visível na fila geral para acompanhamento.
          </p>
        </div>

      </div>

      <div class="flex justify-end gap-2 pt-2">
        <Button variant="outline" @click="closeDialog">{{ $t('common.cancel') }}</Button>
        <Button @click="saveContact" :disabled="isSubmitting">
          <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
          {{ isSubmitting ? 'Criando...' : 'Criar contato' }}
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
