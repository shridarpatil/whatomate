<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { toast } from 'vue-sonner'
import { MessageSquare, Loader2 } from 'lucide-vue-next'

const { t } = useI18n()

interface SSOProvider {
  provider: string
  name: string
}

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const email = ref('')
const password = ref('')
const isLoading = ref(false)
const ssoProviders = ref<SSOProvider[]>([])

// SSO provider icons (using simple SVG paths)
const providerIcons: Record<string, string> = {
  google: 'M12.545,10.239v3.821h5.445c-0.712,2.315-2.647,3.972-5.445,3.972c-3.332,0-6.033-2.701-6.033-6.032s2.701-6.032,6.033-6.032c1.498,0,2.866,0.549,3.921,1.453l2.814-2.814C17.503,2.988,15.139,2,12.545,2C7.021,2,2.543,6.477,2.543,12s4.478,10,10.002,10c8.396,0,10.249-7.85,9.426-11.748L12.545,10.239z',
  microsoft: 'M11 11H3V3h8v8zm10 0h-8V3h8v8zM11 21H3v-8h8v8zm10 0h-8v-8h8v8z',
  github: 'M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z',
  facebook: 'M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z',
  custom: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z'
}

// Dark-first: default is dark mode, light: prefix for light mode
const providerColors: Record<string, string> = {
  google: 'hover:bg-red-950 border-red-800 light:hover:bg-red-50 light:border-red-200',
  microsoft: 'hover:bg-blue-950 border-blue-800 light:hover:bg-blue-50 light:border-blue-200',
  github: 'hover:bg-gray-800 border-gray-600 light:hover:bg-gray-100 light:border-gray-300',
  facebook: 'hover:bg-blue-950 border-blue-800 light:hover:bg-blue-50 light:border-blue-200',
  custom: 'hover:bg-purple-950 border-purple-800 light:hover:bg-purple-50 light:border-purple-200'
}

onMounted(async () => {
  // Check for SSO error in query params
  const ssoError = route.query.sso_error as string
  if (ssoError) {
    toast.error(decodeURIComponent(ssoError))
    // Clear the error from URL
    router.replace({ query: { ...route.query, sso_error: undefined } })
  }

  // Fetch enabled SSO providers
  try {
    const response = await api.get('/auth/sso/providers')
    ssoProviders.value = response.data.data || []
  } catch {
    ssoProviders.value = []
  }
})

const handleLogin = async () => {
  if (!email.value || !password.value) {
    toast.error(t('auth.enterEmailPassword'))
    return
  }

  isLoading.value = true

  try {
    await authStore.login(email.value, password.value)
    toast.success(t('auth.loginSuccess'))

    const redirect = route.query.redirect as string
    router.push(redirect || '/')
  } catch (error: any) {
    const message = error.response?.data?.message || t('auth.invalidCredentials')
    toast.error(message)
  } finally {
    isLoading.value = false
  }
}

const initiateSSO = (provider: string) => {
  const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
  window.location.href = `${basePath}/api/auth/sso/${provider}/init`
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-[#0a0a0b] light:bg-gradient-to-br light:from-gray-50 light:to-gray-100 p-4">
    <div class="w-full max-w-md rounded-2xl border border-white/[0.08] bg-white/[0.02] backdrop-blur light:bg-white light:border-gray-200 light:shadow-xl">
      <div class="p-8 space-y-1 text-center">
        <div class="flex justify-center mb-4">
          <div class="h-12 w-12 rounded-xl bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
            <MessageSquare class="h-7 w-7 text-white" />
          </div>
        </div>
        <h2 class="text-2xl font-bold text-white light:text-gray-900">{{ $t('auth.welcomeTitle') }}</h2>
        <p class="text-white/50 light:text-gray-500">
          {{ $t('auth.welcomeSubtitle') }}
        </p>
      </div>

      <form @submit.prevent="handleLogin">
        <div class="px-8 pb-4 space-y-4">
          <div class="space-y-2">
            <Label for="email" class="text-white/70 light:text-gray-700">{{ $t('common.email') }}</Label>
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
            <Label for="password" class="text-white/70 light:text-gray-700">{{ $t('auth.password') }}</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              :placeholder="$t('auth.passwordPlaceholder')"
              :disabled="isLoading"
              autocomplete="current-password"
            />
          </div>
          <Button type="submit" class="w-full bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-600 hover:to-green-700 text-white shadow-lg shadow-emerald-500/20" :disabled="isLoading">
            <Loader2 v-if="isLoading" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('auth.signIn') }}
          </Button>
        </div>
      </form>

      <!-- SSO Section -->
      <div v-if="ssoProviders.length > 0" class="px-8 pb-4 space-y-3">
        <div class="relative my-2">
          <Separator class="bg-white/[0.08] light:bg-gray-200" />
          <span class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-[#0a0a0b] light:bg-white px-2 text-xs text-white/40 light:text-gray-500">
            {{ $t('auth.orContinueWith') }}
          </span>
        </div>

        <Button
          v-for="provider in ssoProviders"
          :key="provider.provider"
          variant="outline"
          class="w-full justify-start gap-3 transition-colors bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50"
          :class="providerColors[provider.provider] || providerColors.custom"
          @click="initiateSSO(provider.provider)"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
            <path :d="providerIcons[provider.provider] || providerIcons.custom" />
          </svg>
          {{ provider.name }}
        </Button>
      </div>

      <div class="px-8 pb-8">
        <p class="text-sm text-center text-white/40 light:text-gray-500">
          {{ $t('auth.noAccount') }}
          <RouterLink to="/register" class="text-emerald-400 light:text-emerald-600 hover:underline">
            {{ $t('auth.signUp') }}
          </RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>
