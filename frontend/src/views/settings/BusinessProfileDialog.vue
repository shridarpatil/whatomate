<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/services/api'
import { toast } from 'vue-sonner'
import { CrudFormDialog } from '@/components/shared'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Loader2,
  Globe,
  Mail,
  MapPin,
  Image as ImageIcon,
  AlertTriangle,
  Pencil
} from 'lucide-vue-next'

interface Props {
  open: boolean
  accountId: string | null
  accountName: string
}

const props = defineProps<Props>()
const emit = defineEmits(['update:open'])

const { t } = useI18n()

const dialogOpen = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value)
})

interface BusinessProfile {
  messaging_product: string
  address: string
  description: string
  vertical: string
  email: string
  websites: string[]
  profile_picture_url: string
  about: string
}

const isLoading = ref(false)
const isSubmitting = ref(false)
const profile = ref<BusinessProfile>({
  messaging_product: 'whatsapp',
  address: '',
  description: '',
  vertical: '',
  email: '',
  websites: ['', ''],
  profile_picture_url: '',
  about: ''
})

// Categories (Verticals) supported by Meta
const verticals = [
  { value: 'ALCOHOL', label: 'Alcohol' },
  { value: 'APPAREL', label: 'Apparel' },
  { value: 'AUTO', label: 'Automotive' },
  { value: 'BEAUTY', label: 'Beauty & Personal Care' },
  { value: 'EDU', label: 'Education' },
  { value: 'ENTERTAIN', label: 'Entertainment' },
  { value: 'EVENT_PLAN', label: 'Event Planning' },
  { value: 'FINANCE', label: 'Finance & Banking' },
  { value: 'GOVT', label: 'Government & Public Service' },
  { value: 'GROCERY', label: 'Grocery' },
  { value: 'HEALTH', label: 'Health & Wellness' },
  { value: 'HOTEL', label: 'Hotel & Lodging' },
  { value: 'NONPROFIT', label: 'Non-profit' },
  { value: 'ONLINE_GAMBLING', label: 'Online Gambling' },
  { value: 'OTC_DRUGS', label: 'Over-the-counter Drugs' },
  { value: 'OTHER', label: 'Other/Not Listed' },
  { value: 'PHYSICAL_GAMBLING', label: 'Physical Gambling' },
  { value: 'PROF_SERVICES', label: 'Professional Services' },
  { value: 'RETAIL', label: 'Retail' },
  { value: 'TRAVEL', label: 'Travel & Transportation' }
]

const selectedVerticalLabel = computed(() => {
  const found = verticals.find(v => v.value === profile.value.vertical)
  return found?.label || ''
})

watch(() => props.open, async (isOpen) => {
  if (isOpen && props.accountId) {
    await fetchProfile()
  }
})

async function fetchProfile() {
  if (!props.accountId) return

  isLoading.value = true
  try {
    const response = await api.get(`/accounts/${props.accountId}/business_profile`)
    const data = response.data.data

    // Fill the form, ensure arrays have data
    profile.value = {
      messaging_product: data.messaging_product || 'whatsapp',
      address: data.address || '',
      description: data.description || '',
      vertical: data.vertical || '',
      email: data.email || '',
      websites: data.websites && data.websites.length > 0 ? [...data.websites, ''] : ['', ''],
      profile_picture_url: data.profile_picture_url || '',
      about: data.about || ''
    }

    // Ensure at least two slots for websites
    if (profile.value.websites.length < 2) {
      profile.value.websites.push('')
    }
    // Trim to max 2
    profile.value.websites = profile.value.websites.slice(0, 2)

  } catch (error: any) {
    console.error('Failed to fetch business profile:', error)
    toast.error(t('common.failedLoad', { resource: t('resources.businessProfile') }))
  } finally {
    isLoading.value = false
  }
}

async function saveProfile() {
  if (!props.accountId) return

  isSubmitting.value = true
  try {
    // Filter out empty websites
    const websites = profile.value.websites.filter(w => w.trim() !== '')

    const payload = {
      messaging_product: 'whatsapp',
      address: profile.value.address,
      description: profile.value.description,
      vertical: profile.value.vertical,
      email: profile.value.email,
      websites: websites,
      about: profile.value.about
    }

    await api.put(`/accounts/${props.accountId}/business_profile`, payload)
    toast.success(t('common.updatedSuccess', { resource: t('resources.BusinessProfile') }))
    emit('update:open', false)
  } catch (error: any) {
    console.error('Failed to update profile:', error)
    const message = error.response?.data?.message || t('common.failedUpdate', { resource: t('resources.businessProfile') })
    toast.error(message)
  } finally {
    isSubmitting.value = false
  }
}

const fileInput = ref<HTMLInputElement | null>(null)
const isUploading = ref(false)

function triggerFileInput() {
  fileInput.value?.click()
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  if (!input.files || input.files.length === 0) return

  const file = input.files[0]
  // Validate basic type
  if (!file.type.startsWith('image/')) {
    toast.error(t('businessProfile.selectImage'))
    return
  }

  // Validate size (Meta limit is usually 5MB for profile generic, strict on square)
  if (file.size > 5 * 1024 * 1024) {
    toast.error(t('businessProfile.imageTooLarge'))
    return
  }

  isUploading.value = true
  const formData = new FormData()
  formData.append('file', file)

  try {
    await api.post(`/accounts/${props.accountId}/business_profile/photo`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    toast.success(t('common.updatedSuccess', { resource: t('resources.ProfilePhoto') }))
    // Refresh
    await fetchProfile()
  } catch (error: any) {
    console.error('Failed to upload photo:', error)
    toast.error(error.response?.data?.message || t('common.failedUpload', { resource: t('resources.profilePhoto') }))
  } finally {
    isUploading.value = false
    // Reset input
    if (fileInput.value) fileInput.value.value = ''
  }
}
</script>

<template>
  <CrudFormDialog
    v-model:open="dialogOpen"
    :is-editing="true"
    :is-submitting="isSubmitting || isLoading"
    :title="`${$t('businessProfile.title')}: ${accountName}`"
    :description="$t('businessProfile.description')"
    :submit-label="$t('businessProfile.saveChanges')"
    max-width="max-w-2xl"
    @submit="saveProfile"
  >
    <div v-if="isLoading" class="py-12 flex justify-center">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <div v-else class="space-y-6">
      <Alert variant="warning">
        <AlertTriangle class="h-4 w-4" />
        <AlertTitle>{{ $t('businessProfile.profileUpdates') }}</AlertTitle>
        <AlertDescription>
          {{ $t('businessProfile.profileUpdatesDesc') }}
        </AlertDescription>
      </Alert>

      <div class="grid gap-6 md:grid-cols-2">
        <!-- Profile Picture Preview -->
        <div class="md:col-span-2 flex items-center gap-4">
          <div
            class="group relative h-24 w-24 rounded-full bg-secondary flex items-center justify-center overflow-hidden border border-border cursor-pointer transition-all hover:ring-2 hover:ring-emerald-500 hover:ring-offset-2 hover:ring-offset-background"
            @click="triggerFileInput"
          >
            <!-- Loading Overlay -->
            <div v-if="isUploading" class="absolute inset-0 bg-black/50 flex items-center justify-center z-10">
              <Loader2 class="h-6 w-6 text-white animate-spin" />
            </div>

            <!-- Hover Overlay -->
            <div v-if="!isUploading" class="absolute inset-0 bg-black/40 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity z-10">
              <Pencil class="h-6 w-6 text-white" />
            </div>

            <img v-if="profile.profile_picture_url" :src="profile.profile_picture_url" alt="Profile" class="h-full w-full object-cover" />
            <ImageIcon v-else class="h-10 w-10 text-muted-foreground" />

            <input
              ref="fileInput"
              type="file"
              accept="image/png, image/jpeg"
              class="hidden"
              @change="handleFileChange"
            />
          </div>
          <div class="flex-1">
            <Label>{{ $t('businessProfile.profilePicture') }}</Label>
            <p class="text-xs text-muted-foreground mt-1">
              {{ $t('businessProfile.profilePictureHint') }}
            </p>
          </div>
        </div>

        <!-- About -->
        <div class="md:col-span-2 space-y-2">
          <Label for="about">{{ $t('businessProfile.about') }}</Label>
          <Input id="about" v-model="profile.about" :placeholder="$t('businessProfile.aboutPlaceholder')" maxlength="139" />
          <p class="text-xs text-muted-foreground text-right">{{ profile.about.length }}/139</p>
        </div>

        <!-- Description -->
        <div class="md:col-span-2 space-y-2">
          <Label for="description">{{ $t('businessProfile.businessDescription') }}</Label>
          <Textarea id="description" v-model="profile.description" :placeholder="$t('businessProfile.descriptionPlaceholder')" rows="3" maxlength="512" />
          <p class="text-xs text-muted-foreground text-right">{{ profile.description.length }}/512</p>
        </div>

        <!-- Vertical (Category) -->
        <div class="space-y-2">
          <Label for="vertical">{{ $t('businessProfile.industry') }}</Label>
          <Select v-model="profile.vertical">
            <SelectTrigger>
              <SelectValue :placeholder="$t('businessProfile.selectCategory')">
                <template v-if="profile.vertical">{{ selectedVerticalLabel }}</template>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="v in verticals" :key="v.value" :value="v.value">
                {{ v.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- Email -->
        <div class="space-y-2">
          <Label for="email">{{ $t('businessProfile.contactEmail') }}</Label>
          <div class="relative">
            <Mail class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input id="email" v-model="profile.email" type="email" class="pl-9" :placeholder="$t('businessProfile.emailPlaceholder')" maxlength="128" />
          </div>
        </div>

        <!-- Address -->
        <div class="md:col-span-2 space-y-2">
          <Label for="address">{{ $t('businessProfile.businessAddress') }}</Label>
          <div class="relative">
            <MapPin class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input id="address" v-model="profile.address" class="pl-9" :placeholder="$t('businessProfile.addressPlaceholder')" maxlength="256" />
          </div>
        </div>

        <!-- Websites -->
        <div class="md:col-span-2 space-y-3">
          <Label>{{ $t('businessProfile.websites') }}</Label>
          <div v-for="(_, index) in profile.websites" :key="index" class="relative">
            <Globe class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input v-model="profile.websites[index]" class="pl-9" :placeholder="$t('businessProfile.websitePlaceholder')" maxlength="256" />
          </div>
        </div>
      </div>
    </div>
  </CrudFormDialog>
</template>
