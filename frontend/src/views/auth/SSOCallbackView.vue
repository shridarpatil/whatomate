<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'
import { AlertCircle, CheckCircle } from '@/lib/icons'
import { toast } from '@/lib/toast'

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
  <main class="flex items-center justify-center h-screen scroll-y p-4">
    <div class="auth-panel">
        <article class="card">
          <header class="vstack items-center align-center">
            <div v-if="status === 'loading'" role="status" aria-busy="true" data-spinner="large" :aria-label="$t('auth.ssoLoading')"></div>
            <CheckCircle v-else-if="status === 'success'" :size="32" />
            <AlertCircle v-else :size="32" />
            <h3>
              <template v-if="status === 'loading'">{{ $t('auth.ssoLoading') }}</template>
              <template v-else-if="status === 'success'">{{ $t('auth.ssoSuccess') }}</template>
              <template v-else>{{ $t('auth.ssoFailed') }}</template>
            </h3>
            <p class="text-light">
              <template v-if="status === 'loading'">{{ $t('auth.ssoLoadingDesc') }}</template>
              <template v-else-if="status === 'success'">{{ $t('auth.ssoSuccessDesc') }}</template>
              <template v-else>{{ errorMessage }}</template>
            </p>
          </header>
          <footer v-if="status === 'error'" class="align-center">
            <RouterLink to="/login">{{ $t('auth.returnToLogin') }}</RouterLink>
          </footer>
        </article>
    </div>
  </main>
</template>
