<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toast } from 'vue-sonner'
import { Settings, Bell, Loader2 } from 'lucide-vue-next'
import { usersService, organizationService } from '@/services/api'

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
    toast.success('General settings saved')
  } catch (error) {
    toast.error('Failed to save settings')
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
    toast.success('Notification settings saved')
  } catch (error) {
    toast.error('Failed to save notification settings')
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-gray-500 to-gray-600 flex items-center justify-center mr-3 shadow-lg shadow-gray-500/20">
          <Settings class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Settings</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Manage your organization settings</p>
        </div>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Tabs default-value="general" class="w-full">
          <TabsList class="grid w-full grid-cols-2 mb-6 bg-white/[0.04] border border-white/[0.08] light:bg-gray-100 light:border-gray-200">
            <TabsTrigger value="general" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Settings class="h-4 w-4 mr-2" />
              General
            </TabsTrigger>
            <TabsTrigger value="notifications" class="data-[state=active]:bg-white/[0.08] data-[state=active]:text-white text-white/50 light:data-[state=active]:bg-white light:data-[state=active]:text-gray-900 light:text-gray-500">
              <Bell class="h-4 w-4 mr-2" />
              Notifications
            </TabsTrigger>
          </TabsList>

          <!-- General Settings Tab -->
          <TabsContent value="general">
            <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
              <div class="p-6 pb-3">
                <h3 class="text-lg font-semibold text-white light:text-gray-900">General Settings</h3>
                <p class="text-sm text-white/40 light:text-gray-500">Basic organization and display settings</p>
              </div>
              <div class="p-6 pt-3 space-y-4">
                <div class="space-y-2">
                  <Label for="org_name" class="text-white/70 light:text-gray-700">Organization Name</Label>
                  <Input
                    id="org_name"
                    v-model="generalSettings.organization_name"
                    placeholder="Your Organization"
                  />
                </div>
                <div class="grid grid-cols-2 gap-4">
                  <div class="space-y-2">
                    <Label for="timezone" class="text-white/70 light:text-gray-700">Default Timezone</Label>
                    <Select v-model="generalSettings.default_timezone">
                      <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white/70 light:bg-white light:border-gray-200 light:text-gray-700">
                        <SelectValue placeholder="Select timezone" />
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
                    <Label for="date_format" class="text-white/70 light:text-gray-700">Date Format</Label>
                    <Select v-model="generalSettings.date_format">
                      <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white/70 light:bg-white light:border-gray-200 light:text-gray-700">
                        <SelectValue placeholder="Select format" />
                      </SelectTrigger>
                      <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
                        <SelectItem value="YYYY-MM-DD" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">YYYY-MM-DD</SelectItem>
                        <SelectItem value="DD/MM/YYYY" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">DD/MM/YYYY</SelectItem>
                        <SelectItem value="MM/DD/YYYY" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">MM/DD/YYYY</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">Mask Phone Numbers</p>
                    <p class="text-sm text-white/40 light:text-gray-500">Hide phone numbers showing only last 4 digits</p>
                  </div>
                  <Switch
                    :checked="generalSettings.mask_phone_numbers"
                    @update:checked="generalSettings.mask_phone_numbers = $event"
                  />
                </div>
                <div class="flex justify-end">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="saveGeneralSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
                  </Button>
                </div>
              </div>
            </div>
          </TabsContent>

          <!-- Notification Settings Tab -->
          <TabsContent value="notifications">
            <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
              <div class="p-6 pb-3">
                <h3 class="text-lg font-semibold text-white light:text-gray-900">Notifications</h3>
                <p class="text-sm text-white/40 light:text-gray-500">Manage how you receive notifications</p>
              </div>
              <div class="p-6 pt-3 space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">Email Notifications</p>
                    <p class="text-sm text-white/40 light:text-gray-500">Receive important updates via email</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.email_notifications"
                    @update:checked="notificationSettings.email_notifications = $event"
                  />
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">New Message Alerts</p>
                    <p class="text-sm text-white/40 light:text-gray-500">Get notified when new messages arrive</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.new_message_alerts"
                    @update:checked="notificationSettings.new_message_alerts = $event"
                  />
                </div>
                <Separator class="bg-white/[0.08] light:bg-gray-200" />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium text-white light:text-gray-900">Campaign Updates</p>
                    <p class="text-sm text-white/40 light:text-gray-500">Receive campaign status notifications</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.campaign_updates"
                    @update:checked="notificationSettings.campaign_updates = $event"
                  />
                </div>
                <div class="flex justify-end pt-4">
                  <Button variant="outline" size="sm" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50" @click="saveNotificationSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
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
