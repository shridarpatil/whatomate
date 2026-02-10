<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, AlertCircle, CheckCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const status = ref<'loading' | 'success' | 'error'>('loading')
const errorMessage = ref('')

onMounted(async () => {
  try {
    // Cookies were set by the SSO redirect — call /me to get the user object
    const response = await api.get('/me')
    const user = response.data.data

    // Set auth in store (only user — tokens are in httpOnly cookies)
    authStore.setAuth({ user })

    status.value = 'success'
    toast.success(t('auth.ssoLoginSuccess'))

    // Redirect based on role
    setTimeout(() => {
      if (user.role?.name === 'agent') {
        router.push('/analytics/agents')
      } else {
        router.push('/')
      }
    }, 1000)
  } catch (error: any) {
    status.value = 'error'
    errorMessage.value = error.response?.data?.message || t('auth.ssoLoginFailed')
  }
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 to-gray-800 light:from-violet-50 light:to-violet-100 p-4">
    <Card class="w-full max-w-md">
      <CardHeader class="text-center">
        <div class="flex justify-center mb-4">
          <div v-if="status === 'loading'" class="h-12 w-12 rounded-xl bg-primary/10 flex items-center justify-center">
            <Loader2 class="h-7 w-7 text-primary animate-spin" />
          </div>
          <div v-else-if="status === 'success'" class="h-12 w-12 rounded-xl bg-green-900/30 light:bg-green-100 flex items-center justify-center">
            <CheckCircle class="h-7 w-7 text-green-400 light:text-green-600" />
          </div>
          <div v-else class="h-12 w-12 rounded-xl bg-red-900/30 light:bg-red-100 flex items-center justify-center">
            <AlertCircle class="h-7 w-7 text-red-400 light:text-red-600" />
          </div>
        </div>
        <CardTitle class="text-xl">
          <template v-if="status === 'loading'">{{ $t('auth.ssoLoading') }}</template>
          <template v-else-if="status === 'success'">{{ $t('auth.ssoSuccess') }}</template>
          <template v-else>{{ $t('auth.ssoFailed') }}</template>
        </CardTitle>
        <CardDescription>
          <template v-if="status === 'loading'">{{ $t('auth.ssoLoadingDesc') }}</template>
          <template v-else-if="status === 'success'">{{ $t('auth.ssoSuccessDesc') }}</template>
          <template v-else>{{ errorMessage }}</template>
        </CardDescription>
      </CardHeader>
      <CardContent v-if="status === 'error'" class="text-center">
        <RouterLink to="/login" class="text-primary hover:underline">
          {{ $t('auth.returnToLogin') }}
        </RouterLink>
      </CardContent>
    </Card>
  </div>
</template>
