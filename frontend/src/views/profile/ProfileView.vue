<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { toast } from 'vue-sonner'
import { User, Eye, EyeOff, Loader2 } from 'lucide-vue-next'
import { usersService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { PageHeader } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()
const authStore = useAuthStore()
const isChangingPassword = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)

const passwordForm = ref({
  current_password: '',
  new_password: '',
  confirm_password: ''
})

async function changePassword() {
  // Validate passwords match
  if (passwordForm.value.new_password !== passwordForm.value.confirm_password) {
    toast.error(t('profile.passwordMismatch'))
    return
  }

  // Validate password length
  if (passwordForm.value.new_password.length < 6) {
    toast.error(t('profile.passwordTooShort'))
    return
  }

  isChangingPassword.value = true
  try {
    await usersService.changePassword({
      current_password: passwordForm.value.current_password,
      new_password: passwordForm.value.new_password
    })
    toast.success(t('profile.passwordChanged'))
    // Clear the form
    passwordForm.value = {
      current_password: '',
      new_password: '',
      confirm_password: ''
    }
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('profile.passwordChangeFailed')))
  } finally {
    isChangingPassword.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('profile.title')"
      :description="$t('profile.description')"
      :icon="User"
      icon-gradient="bg-gradient-to-br from-gray-500 to-gray-600 shadow-gray-500/20"
    />

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6 max-w-2xl mx-auto">
        <!-- User Info -->
        <Card>
          <CardHeader>
            <CardTitle>{{ $t('profile.accountInfo') }}</CardTitle>
            <CardDescription>{{ $t('profile.accountInfoDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div>
                <Label class="text-muted-foreground">{{ $t('common.name') }}</Label>
                <p class="font-medium">{{ authStore.user?.full_name }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">{{ $t('common.email') }}</Label>
                <p class="font-medium">{{ authStore.user?.email }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">{{ $t('users.role') }}</Label>
                <p class="font-medium capitalize">{{ authStore.user?.role?.name }}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Change Password -->
        <Card>
          <CardHeader>
            <CardTitle>{{ $t('profile.changePassword') }}</CardTitle>
            <CardDescription>{{ $t('profile.changePasswordDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="space-y-2">
              <Label for="current_password">{{ $t('profile.currentPassword') }}</Label>
              <div class="relative">
                <Input
                  id="current_password"
                  v-model="passwordForm.current_password"
                  :type="showCurrentPassword ? 'text' : 'password'"
                  :placeholder="$t('profile.currentPasswordPlaceholder')"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showCurrentPassword = !showCurrentPassword"
                >
                  <Eye v-if="!showCurrentPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="space-y-2">
              <Label for="new_password">{{ $t('profile.newPassword') }}</Label>
              <div class="relative">
                <Input
                  id="new_password"
                  v-model="passwordForm.new_password"
                  :type="showNewPassword ? 'text' : 'password'"
                  :placeholder="$t('profile.newPasswordPlaceholder')"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showNewPassword = !showNewPassword"
                >
                  <Eye v-if="!showNewPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
              <p class="text-xs text-muted-foreground">{{ $t('profile.passwordMinLength') }}</p>
            </div>
            <div class="space-y-2">
              <Label for="confirm_password">{{ $t('profile.confirmNewPassword') }}</Label>
              <div class="relative">
                <Input
                  id="confirm_password"
                  v-model="passwordForm.confirm_password"
                  :type="showConfirmPassword ? 'text' : 'password'"
                  :placeholder="$t('profile.confirmNewPasswordPlaceholder')"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showConfirmPassword = !showConfirmPassword"
                >
                  <Eye v-if="!showConfirmPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="flex justify-end">
              <Button variant="outline" size="sm" @click="changePassword" :disabled="isChangingPassword">
                <Loader2 v-if="isChangingPassword" class="mr-2 h-4 w-4 animate-spin" />
                {{ $t('profile.changePassword') }}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
  </div>
</template>
