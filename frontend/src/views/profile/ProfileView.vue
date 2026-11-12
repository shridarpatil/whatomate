<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from '@/lib/toast'
import { User, Eye, EyeOff } from '@/lib/icons'
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
    />

    <div class="flex-1 scroll-y p-4">
      <div class="vstack gap-6">
        <!-- User Info -->
        <article class="card">
          <header>
            <h3>{{ $t('profile.accountInfo') }}</h3>
            <p class="text-light">{{ $t('profile.accountInfoDesc') }}</p>
          </header>
          <div class="row">
          <div class="col-4">
            <small class="text-light">{{ $t('common.name') }}</small>
            <p>{{ authStore.user?.full_name }}</p>
          </div>
          <div class="col-4">
            <small class="text-light">{{ $t('common.email') }}</small>
            <p>{{ authStore.user?.email }}</p>
          </div>
          <div class="col-4">
            <small class="text-light">{{ $t('users.role') }}</small>
            <p>{{ authStore.user?.role?.name }}</p>
          </div>
          </div>
        </article>

        <!-- Change Password -->
        <article class="card">
          <header>
            <h3>{{ $t('profile.changePassword') }}</h3>
            <p class="text-light">{{ $t('profile.changePasswordDesc') }}</p>
          </header>
        <div data-field>
          <label for="current_password">{{ $t('profile.currentPassword') }}</label>
          <div class="hstack">
            <input
              id="current_password"
              v-model="passwordForm.current_password"
              class="flex-1"
              :type="showCurrentPassword ? 'text' : 'password'"
              :placeholder="$t('profile.currentPasswordPlaceholder')"
            />
            <button
              type="button"
              class="ghost icon"
              :aria-label="$t('profile.togglePasswordVisibility')"
              @click="showCurrentPassword = !showCurrentPassword"
            >
              <Eye v-if="!showCurrentPassword" />
              <EyeOff v-else />
            </button>
          </div>
        </div>
        <div data-field>
          <label for="new_password">{{ $t('profile.newPassword') }}</label>
          <div class="hstack">
            <input
              id="new_password"
              v-model="passwordForm.new_password"
              class="flex-1"
              :type="showNewPassword ? 'text' : 'password'"
              :placeholder="$t('profile.newPasswordPlaceholder')"
            />
            <button
              type="button"
              class="ghost icon"
              :aria-label="$t('profile.togglePasswordVisibility')"
              @click="showNewPassword = !showNewPassword"
            >
              <Eye v-if="!showNewPassword" />
              <EyeOff v-else />
            </button>
          </div>
          <small data-hint>{{ $t('profile.passwordMinLength') }}</small>
        </div>
        <div data-field>
          <label for="confirm_password">{{ $t('profile.confirmNewPassword') }}</label>
          <div class="hstack">
            <input
              id="confirm_password"
              v-model="passwordForm.confirm_password"
              class="flex-1"
              :type="showConfirmPassword ? 'text' : 'password'"
              :placeholder="$t('profile.confirmNewPasswordPlaceholder')"
            />
            <button
              type="button"
              class="ghost icon"
              :aria-label="$t('profile.togglePasswordVisibility')"
              @click="showConfirmPassword = !showConfirmPassword"
            >
              <Eye v-if="!showConfirmPassword" />
              <EyeOff v-else />
            </button>
          </div>
        </div>
          <footer class="flex justify-end">
            <button type="button" class="outline small" :disabled="isChangingPassword" :aria-busy="isChangingPassword" @click="changePassword">
              {{ $t('profile.changePassword') }}
            </button>
          </footer>
        </article>
      </div>
    </div>
  </div>
</template>
