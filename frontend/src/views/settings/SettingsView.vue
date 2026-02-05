<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader } from '@/components/shared'
import { toast } from 'vue-sonner'
import { Settings, Bell, Loader2, Globe } from 'lucide-vue-next'
import { usersService, organizationService } from '@/services/api'
import { SUPPORTED_LOCALES, setLocale, getLocale } from '@/i18n'

const { t } = useI18n()

const isSubmitting = ref(false)
const isLoading = ref(true)

// Language Setting
const selectedLocale = ref(getLocale())

function changeLocale(value: unknown) {
  if (typeof value === 'string') {
    selectedLocale.value = value
    setLocale(value)
  }
}

// General Settings
const generalSettings = ref({
  organization_name: 'My Organization',
  default_timezone: 'UTC',
  date_format: 'YYYY-MM-DD',
  mask_phone_numbers: false
})

// Notification Settings
const notificationSettings = ref({
  email_notifications: true,
  new_message_alerts: true,
  campaign_updates: true
})

onMounted(async () => {
  try {
    const [orgResponse, userResponse] = await Promise.all([
      organizationService.getSettings(),
      usersService.me()
    ])

    // Organization settings
    const orgData = orgResponse.data.data || orgResponse.data
    if (orgData) {
      generalSettings.value = {
        organization_name: orgData.name || 'My Organization',
        default_timezone: orgData.settings?.timezone || 'UTC',
        date_format: orgData.settings?.date_format || 'YYYY-MM-DD',
        mask_phone_numbers: orgData.settings?.mask_phone_numbers || false
      }
    }

    // User notification settings
    const user = userResponse.data.data || userResponse.data
    if (user.settings) {
      notificationSettings.value = {
        email_notifications: user.settings.email_notifications ?? true,
        new_message_alerts: user.settings.new_message_alerts ?? true,
        campaign_updates: user.settings.campaign_updates ?? true
      }
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  } finally {
    isLoading.value = false
  }
})

async function saveGeneralSettings() {
  isSubmitting.value = true
  try {
    await organizationService.updateSettings({
      name: generalSettings.value.organization_name,
      timezone: generalSettings.value.default_timezone,
      date_format: generalSettings.value.date_format,
      mask_phone_numbers: generalSettings.value.mask_phone_numbers
    })
    toast.success(t('settings.generalSaved'))
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.settings') }))
  } finally {
    isSubmitting.value = false
  }
}

async function saveNotificationSettings() {
  isSubmitting.value = true
  try {
    await usersService.updateSettings({
      email_notifications: notificationSettings.value.email_notifications,
      new_message_alerts: notificationSettings.value.new_message_alerts,
      campaign_updates: notificationSettings.value.campaign_updates
    })
    toast.success(t('settings.notificationsSaved'))
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.notificationSettings') }))
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('settings.title')" :subtitle="$t('settings.subtitle')" :icon="Settings" icon-gradient="bg-gradient-to-br from-gray-500 to-gray-600 shadow-gray-500/20" />
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Tabs default-value="general" class="w-full">
          <TabsList class="grid w-full grid-cols-2 mb-6 bg-white/[0.04] border border-white/[0.08] light:bg-gray-100 light:border-gray-200">
            <TabsTrigger value="general" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Settings class="h-4 w-4 mr-2" />
              {{ $t('settings.general') }}
            </TabsTrigger>
            <TabsTrigger value="notifications" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Bell class="h-4 w-4 mr-2" />
              {{ $t('settings.notifications') }}
            </TabsTrigger>
          </TabsList>

          <!-- General Settings Tab -->
          <TabsContent value="general">
            <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
              <div class="p-6 pb-3">
                <h3 class="text-lg font-semibold text-white light:text-gray-900">{{ $t('settings.generalSettings') }}</h3>
                <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.generalSettingsDesc') }}</p>
              </div>
              <div class="p-6 pt-3 space-y-4">
                <div class="space-y-2">
                  <Label for="org_name" class="text-white/70 light:text-gray-700">{{ $t('settings.organizationName') }}</Label>
                  <Input
                    id="org_name"
                    v-model="generalSettings.organization_name"
                    :placeholder="$t('settings.organizationPlaceholder')"
                  />
                </div>
                <div class="grid grid-cols-2 gap-4">
                  <div class="space-y-2">
                    <Label for="timezone" class="text-white/70 light:text-gray-700">{{ $t('settings.defaultTimezone') }}</Label>
                    <Select v-model="generalSettings.default_timezone">
                      <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white/70 light:bg-white light:border-gray-200 light:text-gray-700">
                        <SelectValue :placeholder="$t('settings.selectTimezone')" />
                      </SelectTrigger>
                      <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
                        <SelectItem value="UTC" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">UTC</SelectItem>
                        <SelectItem value="America/New_York" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Eastern Time</SelectItem>
                        <SelectItem value="America/Los_Angeles" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Pacific Time</SelectItem>
                        <SelectItem value="Europe/London" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">London</SelectItem>
                        <SelectItem value="Asia/Tokyo" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Tokyo</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div class="space-y-2">
                    <Label for="date_format" class="text-white/70 light:text-gray-700">{{ $t('settings.dateFormat') }}</Label>
                    <Select v-model="generalSettings.date_format">
                      <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white/70 light:bg-white light:border-gray-200 light:text-gray-700">
                        <SelectValue :placeholder="$t('settings.selectFormat')" />
                      </SelectTrigger>
                      <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
                        <SelectItem value="YYYY-MM-DD" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">YYYY-MM-DD</SelectItem>
                        <SelectItem value="DD/MM/YYYY" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">DD/MM/YYYY</SelectItem>
                        <SelectItem value="MM/DD/YYYY" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">MM/DD/YYYY</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div class="space-y-2">
                  <Label class="text-white/70 light:text-gray-700">
                    <Globe class="h-4 w-4 inline mr-1" />
                    {{ $t('settings.language') }}
                  </Label>
                  <Select :model-value="selectedLocale" @update:model-value="changeLocale">
                    <SelectTrigger class="w-full max-w-xs bg-white/[0.04] border-white/[0.1] text-white/70 light:bg-white light:border-gray-200 light:text-gray-700">
                      <SelectValue :placeholder="$t('settings.selectLanguage')" />
                    </SelectTrigger>
                    <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
                      <SelectItem
                        v-for="locale in SUPPORTED_LOCALES"
                        :key="locale.code"
                        :value="locale.code"
                        class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                      >
                        {{ locale.nativeName }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <p class="text-xs text-white/40 light:text-gray-500">{{ $t('settings.languageDesc') }}</p>
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">{{ $t('settings.maskPhoneNumbers') }}</p>
                    <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.maskPhoneNumbersDesc') }}</p>
                  </div>
                  <Switch
                    :checked="generalSettings.mask_phone_numbers"
                    @update:checked="generalSettings.mask_phone_numbers = $event"
                  />
                </div>
                <div class="flex justify-end">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="saveGeneralSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('settings.save') }}
                  </Button>
                </div>
              </div>
            </div>
          </TabsContent>

          <!-- Notification Settings Tab -->
          <TabsContent value="notifications">
            <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
              <div class="p-6 pb-3">
                <h3 class="text-lg font-semibold text-white light:text-gray-900">{{ $t('settings.notifications') }}</h3>
                <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.notificationsDesc') }}</p>
              </div>
              <div class="p-6 pt-3 space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">{{ $t('settings.emailNotifications') }}</p>
                    <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.emailNotificationsDesc') }}</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.email_notifications"
                    @update:checked="notificationSettings.email_notifications = $event"
                  />
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">{{ $t('settings.newMessageAlerts') }}</p>
                    <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.newMessageAlertsDesc') }}</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.new_message_alerts"
                    @update:checked="notificationSettings.new_message_alerts = $event"
                  />
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">{{ $t('settings.campaignUpdates') }}</p>
                    <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.campaignUpdatesDesc') }}</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.campaign_updates"
                    @update:checked="notificationSettings.campaign_updates = $event"
                  />
                </div>
                <div class="flex justify-end pt-4">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="saveNotificationSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('settings.save') }}
                  </Button>
                </div>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </ScrollArea>
  </div>
</template>
