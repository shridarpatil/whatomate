<script setup lang="ts">
import { AlertTriangle, ExternalLink } from 'lucide-vue-next'

// Where the authorization code goes, plus the phishing warning and the
// acknowledgement an unrecognised (non-Claude) app requires before approval.
defineProps<{
  host: string
  verified: boolean
  disabled?: boolean
}>()

const acknowledged = defineModel<boolean>('acknowledged', { default: false })
</script>

<template>
  <div class="space-y-3">
    <p class="flex items-center justify-center gap-1.5 text-sm text-white/60 light:text-gray-600" data-testid="consent-destination">
      <ExternalLink class="h-3.5 w-3.5 shrink-0" />
      <span>{{ $t('oauthConsent.sendsTo', { host }) }}</span>
    </p>

    <div
      v-if="!verified"
      class="rounded-lg border border-amber-500/40 bg-amber-500/10 p-4 text-left space-y-3 light:border-amber-300 light:bg-amber-50"
      data-testid="consent-unverified"
      role="alert"
    >
      <div class="flex gap-2">
        <AlertTriangle class="h-5 w-5 shrink-0 text-amber-400 light:text-amber-600" />
        <p class="text-sm font-medium text-amber-200 light:text-amber-900">
          {{ $t('oauthConsent.unverifiedWarning', { host }) }}
        </p>
      </div>
      <label class="flex items-start gap-2 text-sm text-white/80 light:text-gray-800 cursor-pointer">
        <input
          v-model="acknowledged"
          type="checkbox"
          class="mt-0.5 h-4 w-4 shrink-0 accent-amber-500"
          :disabled="disabled"
          data-testid="consent-acknowledge"
        />
        <span>{{ $t('oauthConsent.unverifiedConfirm', { host }) }}</span>
      </label>
    </div>
  </div>
</template>
