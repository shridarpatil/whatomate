<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, AuditLogPanel } from '@/components/shared'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import { toast } from '@/lib/toast'
import { Settings, Bell, Globe, Phone, Upload, Play, Pause, Music } from '@/lib/icons'
import { usersService, organizationService } from '@/services/api'
import { useAuthStore, type CallRingtone } from '@/stores/auth'
import { previewRingtone } from '@/lib/ringtone'

const { t } = useI18n()
const authStore = useAuthStore()

// The active org may be overridden by the X-Organization-ID header
// (localStorage.selected_organization_id) when a super admin switches orgs.
// That override is what the backend uses for scoping, so we must read it here
// too — otherwise the activity log panel would query the user's default org
// instead of the currently-active one.
const orgID = computed(
  () => localStorage.getItem('selected_organization_id') || authStore.organizationId,
)
const userID = computed(() => authStore.user?.id || '')
const canWriteAccounts = computed(() => authStore.hasPermission('accounts', 'write'))

const isSubmitting = ref(false)
const isLoading = ref(true)

// General Settings
const generalSettings = ref({
  organization_name: 'My Organization',
  default_timezone: 'UTC',
  date_format: 'YYYY-MM-DD',
  mask_phone_numbers: false,
  meta_app_id: '',
  meta_config_id: '',
  meta_app_secret: '',
  has_meta_app_secret: false
})

// Notification Settings
const notificationSettings = ref({
  email_notifications: true,
  new_message_alerts: true,
  campaign_updates: true,
  call_ringtone: 'ring' as CallRingtone
})

// Calling Settings
const callingSettings = ref({
  calling_enabled: false,
  max_call_duration: 300,
  transfer_timeout_secs: 120,
  hold_music_file: '',
  ringback_file: ''
})

const isUploadingHoldMusic = ref(false)
const isUploadingRingback = ref(false)
const holdMusicInput = ref<HTMLInputElement | null>(null)
const ringbackInput = ref<HTMLInputElement | null>(null)
const holdMusicAudio = ref<HTMLAudioElement | null>(null)
const ringbackAudio = ref<HTMLAudioElement | null>(null)
const playingHoldMusic = ref(false)
const playingRingback = ref(false)

// Bump these keys to force the AuditLogPanel to remount and refetch after a save.
// The backend writes audit entries asynchronously in a goroutine, so we delay
// the remount slightly to give the write time to hit the DB before refetching.
const generalLogKey = ref(0)
const notificationLogKey = ref(0)
const callingLogKey = ref(0)

function refreshActivityLog(key: typeof generalLogKey) {
  setTimeout(() => { key.value++ }, 500)
}

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
        mask_phone_numbers: orgData.settings?.mask_phone_numbers || false,
        meta_app_id: orgData.settings?.meta_app_id || '',
        meta_config_id: orgData.settings?.meta_config_id || '',
        meta_app_secret: '',
        has_meta_app_secret: orgData.settings?.has_meta_app_secret || false
      }
      callingSettings.value = {
        calling_enabled: orgData.settings?.calling_enabled || false,
        max_call_duration: orgData.settings?.max_call_duration || 300,
        transfer_timeout_secs: orgData.settings?.transfer_timeout_secs || 120,
        hold_music_file: orgData.settings?.hold_music_file || '',
        ringback_file: orgData.settings?.ringback_file || ''
      }
    }

    // User notification settings
    const user = userResponse.data.data || userResponse.data
    if (user.settings) {
      notificationSettings.value = {
        email_notifications: user.settings.email_notifications ?? true,
        new_message_alerts: user.settings.new_message_alerts ?? true,
        campaign_updates: user.settings.campaign_updates ?? true,
        call_ringtone: user.settings.call_ringtone ?? 'ring'
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
    const payload: any = {
      name: generalSettings.value.organization_name,
      timezone: generalSettings.value.default_timezone,
      date_format: generalSettings.value.date_format,
      mask_phone_numbers: generalSettings.value.mask_phone_numbers
    }
    if (canWriteAccounts.value) {
      payload.meta_app_id = generalSettings.value.meta_app_id
      payload.meta_config_id = generalSettings.value.meta_config_id
      if (generalSettings.value.meta_app_secret) {
        payload.meta_app_secret = generalSettings.value.meta_app_secret
      }
    }
    await organizationService.updateSettings(payload)
    toast.success(t('settings.generalSaved'))
    // Clear secret input after save
    generalSettings.value.meta_app_secret = ''
    // Refresh organization settings to update has_meta_app_secret status
    const orgResponse = await organizationService.getSettings()
    const orgData = orgResponse.data.data || orgResponse.data
    if (orgData) {
      generalSettings.value.has_meta_app_secret = orgData.settings?.has_meta_app_secret || false
    }
    refreshActivityLog(generalLogKey)
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
      call_ringtone: notificationSettings.value.call_ringtone
    })
    // Pull the saved settings back into the auth store so the ringtone picker
    // takes effect on the next call without a reload.
    await authStore.refreshUserData()
    toast.success(t('settings.notificationsSaved'))
    refreshActivityLog(notificationLogKey)
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
    refreshActivityLog(callingLogKey)
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.settings') }))
  } finally {
    isSubmitting.value = false
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
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('settings.title')" :description="$t('settings.subtitle')" :icon="Settings" />
    <div class="flex-1 scroll-y p-4">
      <div class="container">
        <ot-tabs>
          <div role="tablist">
            <button type="button" role="tab">
              <Settings :size="16" />
              {{ $t('settings.general') }}
            </button>
            <button type="button" role="tab">
              <Bell :size="16" />
              {{ $t('settings.notifications') }}
            </button>
            <button type="button" role="tab">
              <Phone :size="16" />
              {{ $t('settings.calling') }}
            </button>
          </div>

          <!-- General Settings Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('settings.generalSettings') }}</h3>
                <p class="text-light">{{ $t('settings.generalSettingsDesc') }}</p>
              </header>
              <label data-field>
                {{ $t('settings.organizationName') }}
                <input id="org_name" v-model="generalSettings.organization_name" :placeholder="$t('settings.organizationPlaceholder')" />
              </label>
              <div class="row">
                <div class="col-6">
                  <label data-field>
                    {{ $t('settings.defaultTimezone') }}
                    <select id="timezone" v-model="generalSettings.default_timezone">
                      <option value="UTC">UTC</option>
                      <option value="America/New_York">Eastern Time</option>
                      <option value="America/Los_Angeles">Pacific Time</option>
                      <option value="Europe/London">London</option>
                      <option value="Asia/Tokyo">Tokyo</option>
                    </select>
                  </label>
                </div>
                <div class="col-6">
                  <label data-field>
                    {{ $t('settings.dateFormat') }}
                    <select id="date_format" v-model="generalSettings.date_format">
                      <option value="YYYY-MM-DD">YYYY-MM-DD</option>
                      <option value="DD/MM/YYYY">DD/MM/YYYY</option>
                      <option value="MM/DD/YYYY">MM/DD/YYYY</option>
                    </select>
                  </label>
                </div>
              </div>
              <div data-field>
                <label class="hstack gap-1"><Globe :size="16" /> {{ $t('settings.language') }}</label>
                <LanguageSwitcher />
                <small data-hint>{{ $t('settings.languageDesc') }}</small>
              </div>
              <hr>
            <label data-field>
              <span class="hstack justify-between">
                <span>{{ $t('settings.maskPhoneNumbers') }}</span>
                <input type="checkbox" role="switch" v-model="generalSettings.mask_phone_numbers" />
              </span>
              <small data-hint>{{ $t('settings.maskPhoneNumbersDesc') }}</small>
            </label>
            <div class="hstack justify-end">
              <button type="button" class="outline small" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveGeneralSettings">
                {{ $t('settings.save') }}
              </button>
            </div>
            </article>

            <!-- Meta App Credentials Card (Gated on canWriteAccounts) -->
            <article v-if="canWriteAccounts" class="card mt-6">
              <header>
                <h3>{{ $t('settings.metaAppCredentials') }}</h3>
                <p class="text-light">{{ $t('settings.metaAppCredentialsDesc') }}</p>
              </header>
              <div class="row">
                <div class="col-6">
                  <label data-field>
                    {{ $t('settings.metaAppId') }}
                    <input id="meta_app_id" v-model="generalSettings.meta_app_id" placeholder="e.g. 123456789012345" />
                  </label>
                </div>
                <div class="col-6">
                  <label data-field>
                    {{ $t('settings.metaConfigId') }}
                    <input id="meta_config_id" v-model="generalSettings.meta_config_id" placeholder="e.g. 987654321098765" />
                  </label>
                </div>
              </div>
              <label data-field>
                {{ $t('settings.metaAppSecret') }}
                <input
                  id="meta_app_secret"
                  type="password"
                  v-model="generalSettings.meta_app_secret"
                  :placeholder="generalSettings.has_meta_app_secret ? '••••••••••••' : 'Enter Meta App Secret'"
                />
              </label>
            <div class="hstack justify-end">
              <button type="button" class="outline small" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveGeneralSettings">
                {{ $t('settings.save') }}
              </button>
            </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="generalLogKey" resource-type="settings.general" :resource-id="orgID" />
            </div>
          </div>

          <!-- Notification Settings Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('settings.notifications') }}</h3>
                <p class="text-light">{{ $t('settings.notificationsDesc') }}</p>
              </header>
            <label data-field>
              <span class="hstack justify-between">
                <span>{{ $t('settings.emailNotifications') }}</span>
                <input type="checkbox" role="switch" v-model="notificationSettings.email_notifications" />
              </span>
              <small data-hint>{{ $t('settings.emailNotificationsDesc') }}</small>
            </label>
              <hr>
            <label data-field>
              <span class="hstack justify-between">
                <span>{{ $t('settings.newMessageAlerts') }}</span>
                <input type="checkbox" role="switch" v-model="notificationSettings.new_message_alerts" />
              </span>
              <small data-hint>{{ $t('settings.newMessageAlertsDesc') }}</small>
            </label>
              <hr>
            <label data-field>
              <span class="hstack justify-between">
                <span>{{ $t('settings.campaignUpdates') }}</span>
                <input type="checkbox" role="switch" v-model="notificationSettings.campaign_updates" />
              </span>
              <small data-hint>{{ $t('settings.campaignUpdatesDesc') }}</small>
            </label>
              <hr>
              <div data-field>
                <label for="call_ringtone">{{ $t('settings.callRingtone') }}</label>
                <div class="hstack gap-2">
                  <select id="call_ringtone" v-model="notificationSettings.call_ringtone">
                    <option value="ring">{{ $t('settings.ringtoneRing') }}</option>
                    <option value="beep">{{ $t('settings.ringtoneBeep') }}</option>
                    <option value="none">{{ $t('settings.ringtoneNone') }}</option>
                  </select>
                  <button
                    type="button"
                    class="outline icon"
                    :aria-label="$t('settings.previewRingtone')"
                    :disabled="notificationSettings.call_ringtone === 'none'"
                    @click="previewRingtone(notificationSettings.call_ringtone)"
                  >
                    <Play :size="16" />
                  </button>
                </div>
                <small data-hint>{{ $t('settings.callRingtoneDesc') }}</small>
              </div>
            <div class="hstack justify-end">
              <button type="button" class="outline small" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveNotificationSettings">
                {{ $t('settings.save') }}
              </button>
            </div>
            </article>
            <div v-if="userID" class="mt-4">
              <AuditLogPanel :key="notificationLogKey" resource-type="settings.notification" :resource-id="userID" />
            </div>
          </div>

          <!-- Calling Settings Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('settings.callingSettings') }}</h3>
                <p class="text-light">{{ $t('settings.callingSettingsDesc') }}</p>
              </header>
            <label data-field>
              <span class="hstack justify-between">
                <span>{{ $t('settings.callingEnabled') }}</span>
                <input type="checkbox" role="switch" v-model="callingSettings.calling_enabled" />
              </span>
              <small data-hint>{{ $t('settings.callingEnabledDesc') }}</small>
            </label>
              <hr>
              <fieldset class="vstack gap-4" :disabled="!callingSettings.calling_enabled">
                <div class="row">
                  <div class="col-6">
                    <label data-field>
                      {{ $t('settings.maxCallDuration') }}
                      <input id="max_call_duration" type="number" v-model.number="callingSettings.max_call_duration" min="60" max="3600" />
                      <small data-hint>{{ $t('settings.maxCallDurationDesc') }}</small>
                    </label>
                  </div>
                  <div class="col-6">
                    <label data-field>
                      {{ $t('settings.transferTimeout') }}
                      <input id="transfer_timeout" type="number" v-model.number="callingSettings.transfer_timeout_secs" min="30" max="600" />
                      <small data-hint>{{ $t('settings.transferTimeoutDesc') }}</small>
                    </label>
                  </div>
                </div>
                <hr>
            <div class="vstack gap-2">
              <strong class="hstack gap-2"><Music :size="16" /> {{ $t('settings.holdMusic') }}</strong>
              <small class="text-light">{{ $t('settings.holdMusicDesc') }}</small>
              <div class="hstack gap-2">
                <span class="text-light">
                  {{ callingSettings.hold_music_file ? `${$t('settings.currentFile')}: ${callingSettings.hold_music_file}` : $t('settings.noFileUploaded') }}
                </span>
                <button
                  v-if="callingSettings.hold_music_file"
                  type="button"
                  class="ghost icon small"
                  :aria-label="playingHoldMusic ? $t('settings.pauseAudio') : $t('settings.playAudio')"
                  @click="togglePlayAudio('hold_music')"
                >
                  <Pause v-if="playingHoldMusic" :size="16" />
                  <Play v-else :size="16" />
                </button>
              </div>
              <div class="hstack gap-2">
                <input ref="holdMusicInput" type="file" accept=".ogg,.opus,.mp3,.wav" hidden @change="uploadAudio('hold_music', $event)" />
                <button type="button" class="outline small" :aria-busy="isUploadingHoldMusic" :disabled="isUploadingHoldMusic" @click="holdMusicInput?.click()">
                  <Upload v-if="!isUploadingHoldMusic" :size="16" />
                  {{ $t('settings.uploadAudio') }}
                </button>
                <small class="text-light">.ogg, .opus, .mp3, .wav (max 5MB)</small>
              </div>
            </div>
            <div class="vstack gap-2">
              <strong class="hstack gap-2"><Phone :size="16" /> {{ $t('settings.ringbackTone') }}</strong>
              <small class="text-light">{{ $t('settings.ringbackToneDesc') }}</small>
              <div class="hstack gap-2">
                <span class="text-light">
                  {{ callingSettings.ringback_file ? `${$t('settings.currentFile')}: ${callingSettings.ringback_file}` : $t('settings.noFileUploaded') }}
                </span>
                <button
                  v-if="callingSettings.ringback_file"
                  type="button"
                  class="ghost icon small"
                  :aria-label="playingRingback ? $t('settings.pauseAudio') : $t('settings.playAudio')"
                  @click="togglePlayAudio('ringback')"
                >
                  <Pause v-if="playingRingback" :size="16" />
                  <Play v-else :size="16" />
                </button>
              </div>
              <div class="hstack gap-2">
                <input ref="ringbackInput" type="file" accept=".ogg,.opus,.mp3,.wav" hidden @change="uploadAudio('ringback', $event)" />
                <button type="button" class="outline small" :aria-busy="isUploadingRingback" :disabled="isUploadingRingback" @click="ringbackInput?.click()">
                  <Upload v-if="!isUploadingRingback" :size="16" />
                  {{ $t('settings.uploadAudio') }}
                </button>
                <small class="text-light">.ogg, .opus, .mp3, .wav (max 5MB)</small>
              </div>
            </div>
              </fieldset>
            <div class="hstack justify-end">
              <button type="button" class="outline small" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveCallingSettings">
                {{ $t('settings.save') }}
              </button>
            </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="callingLogKey" resource-type="settings.calling" :resource-id="orgID" />
            </div>
          </div>
        </ot-tabs>
      </div>
    </div>
  </div>
</template>
