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
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import { toast } from 'vue-sonner'
import { Settings, Bell, Loader2, Globe, Phone, Upload, Play, Pause, Music, Mail } from 'lucide-vue-next'
import { usersService, organizationService } from '@/services/api'

const { t } = useI18n()

const isSubmitting = ref(false)
const isLoading = ref(true)

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
  campaign_updates: true,
  weekly_report: true,
  audit_logs: true,
  plan_limits: true
})

// Calling Settings
const callingSettings = ref({
  calling_enabled: false,
  max_call_duration: 300,
  transfer_timeout_secs: 120,
  hold_music_file: '',
  ringback_file: ''
})

// SMTP Settings
const smtpSettings = ref({
  enabled: false,
  smtp_host: '',
  smtp_port: 587,
  smtp_user: '',
  smtp_pass: '',
  email_from_address: '',
  email_from_name: '',
  smtp_tls: false
})
const testEmailAddress = ref('')
const isTestingEmail = ref(false)

const isUploadingHoldMusic = ref(false)
const isUploadingRingback = ref(false)
const holdMusicInput = ref<HTMLInputElement | null>(null)
const ringbackInput = ref<HTMLInputElement | null>(null)
const holdMusicAudio = ref<HTMLAudioElement | null>(null)
const ringbackAudio = ref<HTMLAudioElement | null>(null)
const playingHoldMusic = ref(false)
const playingRingback = ref(false)

onMounted(async () => {
  try {
    const [orgResponse, userResponse, smtpResponse] = await Promise.all([
      organizationService.getSettings(),
      usersService.me(),
      organizationService.getEmailSettings().catch(() => ({ data: {} })) // ignore 404/403
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
      callingSettings.value = {
        calling_enabled: orgData.settings?.calling_enabled || false,
        max_call_duration: orgData.settings?.max_call_duration || 300,
        transfer_timeout_secs: orgData.settings?.transfer_timeout_secs || 120,
        hold_music_file: orgData.settings?.hold_music_file || '',
        ringback_file: orgData.settings?.ringback_file || ''
      }
    }

    // SMTP Settings
    const smtpData = smtpResponse?.data?.data || smtpResponse?.data
    if (smtpData && Object.keys(smtpData).length > 0) {
      smtpSettings.value = {
        enabled: smtpData.enabled || false,
        smtp_host: smtpData.smtp_host || '',
        smtp_port: smtpData.smtp_port || 587,
        smtp_user: smtpData.smtp_user || '',
        smtp_pass: smtpData.smtp_pass || '',
        email_from_address: smtpData.email_from_address || '',
        email_from_name: smtpData.email_from_name || '',
        smtp_tls: smtpData.smtp_tls || false
      }
    }

    // User notification settings
    const user = userResponse.data.data || userResponse.data
    if (user.settings) {
      notificationSettings.value = {
        email_notifications: user.settings.email_notifications ?? true,
        new_message_alerts: user.settings.new_message_alerts ?? true,
        campaign_updates: user.settings.campaign_updates ?? true,
        weekly_report: user.settings.weekly_report ?? true,
        audit_logs: user.settings.audit_logs ?? true,
        plan_limits: user.settings.plan_limits ?? true
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
      campaign_updates: notificationSettings.value.campaign_updates,
      weekly_report: notificationSettings.value.weekly_report,
      audit_logs: notificationSettings.value.audit_logs,
      plan_limits: notificationSettings.value.plan_limits
    })
    toast.success(t('settings.notificationsSaved'))
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.notificationSettings') }))
  } finally {
    isSubmitting.value = false
  }
}

async function saveCallingSettings() {
  isSubmitting.value = true
  try {
    await organizationService.updateSettings({
      calling_enabled: callingSettings.value.calling_enabled,
      max_call_duration: callingSettings.value.max_call_duration,
      transfer_timeout_secs: callingSettings.value.transfer_timeout_secs
    })
    toast.success(t('settings.callingSaved'))
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.settings') }))
  } finally {
    isSubmitting.value = false
  }
}

async function saveSmtpSettings() {
  isSubmitting.value = true
  try {
    await organizationService.updateEmailSettings({
      enabled: smtpSettings.value.enabled,
      smtp_host: smtpSettings.value.smtp_host,
      smtp_port: smtpSettings.value.smtp_port,
      smtp_user: smtpSettings.value.smtp_user,
      smtp_pass: smtpSettings.value.smtp_pass,
      email_from_address: smtpSettings.value.email_from_address,
      email_from_name: smtpSettings.value.email_from_name,
      smtp_tls: smtpSettings.value.smtp_tls
    })
    toast.success('SMTP settings saved successfully')
  } catch (error) {
    toast.error('Failed to save SMTP settings')
  } finally {
    isSubmitting.value = false
  }
}

async function testSmtpSettings() {
  if (!testEmailAddress.value) {
    toast.error('Please enter a test email address')
    return
  }
  isTestingEmail.value = true
  try {
    await organizationService.testEmailSettings({ recipient_email: testEmailAddress.value })
    toast.success(`Test email sent to ${testEmailAddress.value}`)
  } catch (error: any) {
    toast.error(error.response?.data?.error || error.response?.data?.message || 'Failed to send test email')
  } finally {
    isTestingEmail.value = false
  }
}

async function uploadAudio(type: 'hold_music' | 'ringback', event: Event) {
  const input = event.target as HTMLInputElement
  const file = input?.files?.[0]
  if (!file) return

  const isHold = type === 'hold_music'
  if (isHold) isUploadingHoldMusic.value = true
  else isUploadingRingback.value = true

  try {
    const response = await organizationService.uploadOrgAudio(file, type)
    const data = response.data.data || response.data
    if (isHold) callingSettings.value.hold_music_file = data.filename
    else callingSettings.value.ringback_file = data.filename
    toast.success(t('settings.audioUploaded'))
  } catch (error) {
    toast.error(t('settings.audioUploadFailed'))
  } finally {
    if (isHold) isUploadingHoldMusic.value = false
    else isUploadingRingback.value = false
    input.value = ''
  }
}

function togglePlayAudio(type: 'hold_music' | 'ringback') {
  const isHold = type === 'hold_music'
  const filename = isHold ? callingSettings.value.hold_music_file : callingSettings.value.ringback_file
  if (!filename) return

  const audioRef = isHold ? holdMusicAudio : ringbackAudio
  const playingRef = isHold ? playingHoldMusic : playingRingback

  if (playingRef.value && audioRef.value) {
    audioRef.value.pause()
    audioRef.value.currentTime = 0
    playingRef.value = false
    return
  }

  const audio = new Audio(`/api/ivr-flows/audio/${filename}`)
  audioRef.value = audio
  playingRef.value = true
  audio.play()
  audio.onended = () => { playingRef.value = false }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('settings.title')" :subtitle="$t('settings.subtitle')" :icon="Settings" icon-gradient="bg-gradient-to-br from-gray-500 to-gray-600 shadow-gray-500/20" />
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Tabs default-value="general" class="w-full">
          <TabsList class="grid w-full grid-cols-4 mb-6 bg-white/[0.04] border border-white/[0.08] light:bg-gray-100 light:border-gray-200">
            <TabsTrigger value="general" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Settings class="h-4 w-4 mr-2" />
              {{ $t('settings.general') }}
            </TabsTrigger>
            <TabsTrigger value="smtp" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Mail class="h-4 w-4 mr-2" />
              SMTP Relay
            </TabsTrigger>
            <TabsTrigger value="notifications" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Bell class="h-4 w-4 mr-2" />
              {{ $t('settings.notifications') }}
            </TabsTrigger>
            <TabsTrigger value="calling" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Phone class="h-4 w-4 mr-2" />
              {{ $t('settings.calling') }}
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
                  <LanguageSwitcher class="max-w-xs" />
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

          <!-- SMTP Relay Tab -->
          <TabsContent value="smtp">
            <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
              <div class="p-6 pb-3">
                <h3 class="text-lg font-semibold text-white light:text-gray-900">Custom SMTP Relay</h3>
                <p class="text-sm text-white/40 light:text-gray-500">Setup your organization's own mail delivery server.</p>
              </div>
              <div class="p-6 pt-3 space-y-6">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">Enable Custom SMTP Relay</p>
                  </div>
                  <Switch
                    :checked="smtpSettings.enabled"
                    @update:checked="smtpSettings.enabled = $event"
                  />
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />

                <div class="grid grid-cols-2 gap-4" :class="{ 'opacity-50 pointer-events-none': !smtpSettings.enabled }">
                  <div class="space-y-2">
                    <Label for="smtp_host" class="text-white/70 light:text-gray-700">SMTP HOST</Label>
                    <Input id="smtp_host" v-model="smtpSettings.smtp_host" placeholder="smtp.example.com" />
                  </div>
                  <div class="space-y-2">
                    <Label for="smtp_port" class="text-white/70 light:text-gray-700">PORT</Label>
                    <Input id="smtp_port" type="number" v-model.number="smtpSettings.smtp_port" placeholder="587" />
                  </div>
                  <div class="space-y-2">
                    <Label for="smtp_user" class="text-white/70 light:text-gray-700">USERNAME</Label>
                    <Input id="smtp_user" v-model="smtpSettings.smtp_user" placeholder="user@example.com" />
                  </div>
                  <div class="space-y-2">
                    <Label for="smtp_pass" class="text-white/70 light:text-gray-700">PASSWORD</Label>
                    <Input id="smtp_pass" type="password" v-model="smtpSettings.smtp_pass" placeholder="••••••••••••" />
                  </div>
                  <div class="space-y-2">
                    <Label for="smtp_from" class="text-white/70 light:text-gray-700">FROM EMAIL</Label>
                    <Input id="smtp_from" v-model="smtpSettings.email_from_address" placeholder="no-reply@example.com" />
                  </div>
                  <div class="space-y-2">
                    <Label for="smtp_name" class="text-white/70 light:text-gray-700">FROM NAME</Label>
                    <Input id="smtp_name" v-model="smtpSettings.email_from_name" placeholder="Acme Inc Notifications" />
                  </div>
                  <div class="space-y-2 col-span-2">
                    <Label for="smtp_tls" class="text-white/70 light:text-gray-700">ENCRYPTION</Label>
                    <Select :model-value="smtpSettings.smtp_tls ? 'true' : 'false'" @update:model-value="(v) => smtpSettings.smtp_tls = (v === 'true')">
                      <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white/70 light:bg-white light:border-gray-200 light:text-gray-700 w-full mb-4">
                        <SelectValue placeholder="Select Encryption Type" />
                      </SelectTrigger>
                      <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
                        <SelectItem value="false" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">STARTTLS (Usually port 587 or 25)</SelectItem>
                        <SelectItem value="true" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Implicit TLS (Usually port 465)</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <div class="flex items-end gap-4" :class="{ 'opacity-50 pointer-events-none': !smtpSettings.enabled }">
                  <div class="flex-1 space-y-2">
                    <Label for="test_email" class="text-white/70 light:text-gray-700">TEST RECIPIENT EMAIL</Label>
                    <Input id="test_email" v-model="testEmailAddress" placeholder="name@example.com" />
                  </div>
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50 h-10" @click="testSmtpSettings" :disabled="isTestingEmail || !testEmailAddress">
                    <Loader2 v-if="isTestingEmail" class="mr-2 h-4 w-4 animate-spin" />
                    <Mail v-else class="mr-2 h-4 w-4" />
                    Send Test Email
                  </Button>
                </div>

                <div class="flex justify-end pt-4 border-t border-white/[0.08] light:border-gray-200">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="saveSmtpSettings" :disabled="isSubmitting">
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
              
              <div class="p-6 pt-0 space-y-6">
                <!-- Notification Items List -->
                <div class="space-y-4">
                  <div class="flex items-center justify-between p-4 rounded-xl border border-white/[0.08] bg-white/[0.01] hover:bg-white/[0.03] transition-all">
                    <div class="space-y-1">
                      <p class="font-medium text-white light:text-gray-900">{{ $t('settings.emailNotifications') }}</p>
                      <p class="text-xs text-white/40 light:text-gray-500">{{ $t('settings.emailNotificationsDesc') }}</p>
                    </div>
                    <Switch
                      :checked="notificationSettings.email_notifications"
                      @update:checked="notificationSettings.email_notifications = $event"
                    />
                  </div>

                  <div class="flex items-center justify-between p-4 rounded-xl border border-white/[0.08] bg-white/[0.01] hover:bg-white/[0.03] transition-all">
                    <div class="space-y-1">
                      <p class="font-medium text-white light:text-gray-900">{{ $t('settings.newMessageAlerts') }}</p>
                      <p class="text-xs text-white/40 light:text-gray-500">{{ $t('settings.newMessageAlertsDesc') }}</p>
                    </div>
                    <Switch
                      :checked="notificationSettings.new_message_alerts"
                      @update:checked="notificationSettings.new_message_alerts = $event"
                    />
                  </div>

                  <div class="flex items-center justify-between p-4 rounded-xl border border-white/[0.08] bg-white/[0.01] hover:bg-white/[0.03] transition-all">
                    <div class="space-y-1">
                      <p class="font-medium text-white light:text-gray-900">{{ $t('settings.campaignUpdates') }}</p>
                      <p class="text-xs text-white/40 light:text-gray-500">{{ $t('settings.campaignUpdatesDesc') }}</p>
                    </div>
                    <Switch
                      :checked="notificationSettings.campaign_updates"
                      @update:checked="notificationSettings.campaign_updates = $event"
                    />
                  </div>

                  <div class="flex items-center justify-between p-4 rounded-xl border border-white/[0.08] bg-white/[0.01] hover:bg-white/[0.03] transition-all">
                    <div class="space-y-1">
                      <p class="font-medium text-white light:text-gray-900 text-emerald-400">Weekly Performance Report</p>
                      <p class="text-xs text-white/40 light:text-gray-500">Receive a weekly summary of your organization's activity.</p>
                    </div>
                    <Switch
                      :checked="notificationSettings.weekly_report"
                      @update:checked="notificationSettings.weekly_report = $event"
                    />
                  </div>

                  <div class="flex items-center justify-between p-4 rounded-xl border border-white/[0.08] bg-white/[0.01] hover:bg-white/[0.03] transition-all">
                    <div class="space-y-1">
                      <p class="font-medium text-white light:text-gray-900 text-amber-400">Security Audit Logs</p>
                      <p class="text-xs text-white/40 light:text-gray-500">Get notified about significant system and administrative actions.</p>
                    </div>
                    <Switch
                      :checked="notificationSettings.audit_logs"
                      @update:checked="notificationSettings.audit_logs = $event"
                    />
                  </div>

                  <div class="flex items-center justify-between p-4 rounded-xl border border-white/[0.08] bg-white/[0.01] hover:bg-white/[0.03] transition-all">
                    <div class="space-y-1">
                      <p class="font-medium text-white light:text-gray-900 text-rose-400">Plan & Quota Limits</p>
                      <p class="text-xs text-white/40 light:text-gray-500">Alerts when you approach or exceed your platform limits.</p>
                    </div>
                    <Switch
                      :checked="notificationSettings.plan_limits"
                      @update:checked="notificationSettings.plan_limits = $event"
                    />
                  </div>
                </div>



                <div class="flex justify-end pt-2">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50 h-10 px-6 font-semibold" @click="saveNotificationSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('settings.save') }}
                  </Button>
                </div>
              </div>
            </div>
          </TabsContent>

          <!-- Calling Settings Tab -->
          <TabsContent value="calling">
            <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
              <div class="p-6 pb-3">
                <h3 class="text-lg font-semibold text-white light:text-gray-900">{{ $t('settings.callingSettings') }}</h3>
                <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.callingSettingsDesc') }}</p>
              </div>
              <div class="p-6 pt-3 space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">{{ $t('settings.callingEnabled') }}</p>
                    <p class="text-sm text-white/40 light:text-gray-500">{{ $t('settings.callingEnabledDesc') }}</p>
                  </div>
                  <Switch
                    :checked="callingSettings.calling_enabled"
                    @update:checked="callingSettings.calling_enabled = $event"
                  />
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="grid grid-cols-2 gap-4" :class="{ 'opacity-50 pointer-events-none': !callingSettings.calling_enabled }">
                  <div class="space-y-2">
                    <Label for="max_call_duration" class="text-white/70 light:text-gray-700">{{ $t('settings.maxCallDuration') }}</Label>
                    <Input
                      id="max_call_duration"
                      type="number"
                      v-model.number="callingSettings.max_call_duration"
                      :min="60"
                      :max="3600"
                    />
                    <p class="text-xs text-white/40 light:text-gray-500">{{ $t('settings.maxCallDurationDesc') }}</p>
                  </div>
                  <div class="space-y-2">
                    <Label for="transfer_timeout" class="text-white/70 light:text-gray-700">{{ $t('settings.transferTimeout') }}</Label>
                    <Input
                      id="transfer_timeout"
                      type="number"
                      v-model.number="callingSettings.transfer_timeout_secs"
                      :min="30"
                      :max="600"
                    />
                    <p class="text-xs text-white/40 light:text-gray-500">{{ $t('settings.transferTimeoutDesc') }}</p>
                  </div>
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <!-- Hold Music Upload -->
                <div class="space-y-3" :class="{ 'opacity-50 pointer-events-none': !callingSettings.calling_enabled }">
                  <div>
                    <Label class="text-white/70 light:text-gray-700 flex items-center gap-2">
                      <Music class="h-4 w-4" />
                      {{ $t('settings.holdMusic') }}
                    </Label>
                    <p class="text-xs text-white/40 light:text-gray-500 mt-1">{{ $t('settings.holdMusicDesc') }}</p>
                  </div>
                  <div class="flex items-center gap-3">
                    <span class="text-sm text-white/50 light:text-gray-500">
                      {{ callingSettings.hold_music_file ? `${$t('settings.currentFile')}: ${callingSettings.hold_music_file}` : $t('settings.noFileUploaded') }}
                    </span>
                    <Button
                      v-if="callingSettings.hold_music_file"
                      variant="ghost"
                      size="sm"
                      class="h-8 w-8 p-0 text-white/50 hover:text-white light:text-gray-500 light:hover:text-gray-900"
                      @click="togglePlayAudio('hold_music')"
                    >
                      <Pause v-if="playingHoldMusic" class="h-4 w-4" />
                      <Play v-else class="h-4 w-4" />
                    </Button>
                  </div>
                  <div class="flex items-center gap-2">
                    <input ref="holdMusicInput" type="file" accept=".ogg,.opus,.mp3,.wav" class="hidden" @change="uploadAudio('hold_music', $event)" />
                    <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="holdMusicInput?.click()" :disabled="isUploadingHoldMusic">
                      <Loader2 v-if="isUploadingHoldMusic" class="mr-2 h-4 w-4 animate-spin" />
                      <Upload v-else class="mr-2 h-4 w-4" />
                      {{ $t('settings.uploadAudio') }}
                    </Button>
                    <span class="text-xs text-white/30 light:text-gray-400">.ogg, .opus, .mp3, .wav (max 5MB)</span>
                  </div>
                </div>
                <!-- Ringback Tone Upload -->
                <div class="space-y-3" :class="{ 'opacity-50 pointer-events-none': !callingSettings.calling_enabled }">
                  <div>
                    <Label class="text-white/70 light:text-gray-700 flex items-center gap-2">
                      <Phone class="h-4 w-4" />
                      {{ $t('settings.ringbackTone') }}
                    </Label>
                    <p class="text-xs text-white/40 light:text-gray-500 mt-1">{{ $t('settings.ringbackToneDesc') }}</p>
                  </div>
                  <div class="flex items-center gap-3">
                    <span class="text-sm text-white/50 light:text-gray-500">
                      {{ callingSettings.ringback_file ? `${$t('settings.currentFile')}: ${callingSettings.ringback_file}` : $t('settings.noFileUploaded') }}
                    </span>
                    <Button
                      v-if="callingSettings.ringback_file"
                      variant="ghost"
                      size="sm"
                      class="h-8 w-8 p-0 text-white/50 hover:text-white light:text-gray-500 light:hover:text-gray-900"
                      @click="togglePlayAudio('ringback')"
                    >
                      <Pause v-if="playingRingback" class="h-4 w-4" />
                      <Play v-else class="h-4 w-4" />
                    </Button>
                  </div>
                  <div class="flex items-center gap-2">
                    <input ref="ringbackInput" type="file" accept=".ogg,.opus,.mp3,.wav" class="hidden" @change="uploadAudio('ringback', $event)" />
                    <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="ringbackInput?.click()" :disabled="isUploadingRingback">
                      <Loader2 v-if="isUploadingRingback" class="mr-2 h-4 w-4 animate-spin" />
                      <Upload v-else class="mr-2 h-4 w-4" />
                      {{ $t('settings.uploadAudio') }}
                    </Button>
                    <span class="text-xs text-white/30 light:text-gray-400">.ogg, .opus, .mp3, .wav (max 5MB)</span>
                  </div>
                </div>
                <div class="flex justify-end pt-4">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="saveCallingSettings" :disabled="isSubmitting">
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
