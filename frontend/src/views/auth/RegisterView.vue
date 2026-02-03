<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { toast } from 'vue-sonner'
import { MessageSquare, Loader2 } from 'lucide-vue-next'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const fullName = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const organizationName = ref('')
const isLoading = ref(false)

const handleRegister = async () => {
  if (!fullName.value || !email.value || !password.value || !organizationName.value) {
    toast.error(t('auth.fillAllFields'))
    return
  }

  if (password.value !== confirmPassword.value) {
    toast.error(t('auth.passwordsMismatch'))
    return
  }

  if (password.value.length < 8) {
    toast.error(t('auth.passwordTooShort'))
    return
  }

  isLoading.value = true

  try {
    await authStore.register({
      full_name: fullName.value,
      email: email.value,
      password: password.value,
      organization_name: organizationName.value
    })
    toast.success(t('auth.registrationSuccess'))
    router.push('/')
  } catch (error: any) {
    const message = error.response?.data?.message || t('auth.registrationFailed')
    toast.error(message)
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 to-gray-800 light:from-violet-50 light:to-violet-100 p-4">
    <Card class="w-full max-w-md">
      <CardHeader class="space-y-1 text-center">
        <div class="flex justify-center mb-4">
          <div class="h-12 w-12 rounded-xl bg-primary flex items-center justify-center">
            <MessageSquare class="h-7 w-7 text-primary-foreground" />
          </div>
        </div>
        <CardTitle class="text-2xl font-bold">{{ $t('auth.createAccount') }}</CardTitle>
        <CardDescription>
          {{ $t('auth.createAccountDesc') }}
        </CardDescription>
      </CardHeader>
      <form @submit.prevent="handleRegister">
        <CardContent class="space-y-4">
          <div class="space-y-2">
            <Label for="fullName">{{ $t('auth.fullName') }}</Label>
            <Input
              id="fullName"
              v-model="fullName"
              type="text"
              :placeholder="$t('auth.fullNamePlaceholder')"
              :disabled="isLoading"
              autocomplete="name"
            />
          </div>
          <div class="space-y-2">
            <Label for="email">{{ $t('common.email') }}</Label>
            <Input
              id="email"
              v-model="email"
              type="email"
              :placeholder="$t('auth.emailPlaceholder')"
              :disabled="isLoading"
              autocomplete="email"
            />
          </div>
          <div class="space-y-2">
            <Label for="organizationName">{{ $t('auth.organizationName') }}</Label>
            <Input
              id="organizationName"
              v-model="organizationName"
              type="text"
              :placeholder="$t('auth.organizationPlaceholder')"
              :disabled="isLoading"
            />
          </div>
          <div class="space-y-2">
            <Label for="password">{{ $t('auth.password') }}</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              :placeholder="$t('auth.passwordMinLength')"
              :disabled="isLoading"
              autocomplete="new-password"
            />
          </div>
          <div class="space-y-2">
            <Label for="confirmPassword">{{ $t('auth.confirmPassword') }}</Label>
            <Input
              id="confirmPassword"
              v-model="confirmPassword"
              type="password"
              :placeholder="$t('auth.confirmPasswordPlaceholder')"
              :disabled="isLoading"
              autocomplete="new-password"
            />
          </div>
        </CardContent>
        <CardFooter class="flex flex-col space-y-4">
          <Button type="submit" class="w-full" :disabled="isLoading">
            <Loader2 v-if="isLoading" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('auth.createAccountBtn') }}
          </Button>
          <p class="text-sm text-center text-muted-foreground">
            {{ $t('auth.alreadyHaveAccount') }}
            <RouterLink to="/login" class="text-primary hover:underline">
              {{ $t('auth.signIn') }}
            </RouterLink>
          </p>
        </CardFooter>
      </form>
    </Card>
  </div>
</template>
