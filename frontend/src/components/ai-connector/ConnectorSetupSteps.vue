<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Copy } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const props = defineProps<{
  connectorUrl: string
  canViewApiKeys: boolean
}>()

const { t } = useI18n()

const claudeAiSteps = ['claudeAiStep1', 'claudeAiStep2', 'claudeAiStep3', 'claudeAiStep4', 'claudeAiStep5']

const claudeCodeCommand = computed(
  () => `claude mcp add --transport http whatomate ${props.connectorUrl || '<connector-url>'} --header "X-API-Key: whm_..."`,
)

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('common.copied'))
  } catch {
    toast.error(t('common.clipboardFailed'))
  }
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ $t('aiConnector.connectorUrl') }}</CardTitle>
      <CardDescription>{{ $t('aiConnector.connectorUrlDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <div v-if="connectorUrl" class="flex items-center gap-2">
        <code class="flex-1 min-w-0 truncate bg-muted px-3 py-2 rounded text-sm" data-testid="connector-url">{{ connectorUrl }}</code>
        <Button variant="outline" size="sm" @click="copy(connectorUrl)">
          <Copy class="h-4 w-4 mr-2" />{{ $t('common.copy') }}
        </Button>
      </div>
      <p v-else class="text-sm text-muted-foreground">{{ $t('aiConnector.urlUnavailable') }}</p>
    </CardContent>
  </Card>

  <div class="grid gap-6 lg:grid-cols-2">
    <Card>
      <CardHeader>
        <CardTitle>{{ $t('aiConnector.claudeAiTitle') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <ol class="list-decimal pl-5 space-y-1.5 text-sm">
          <li v-for="step in claudeAiSteps" :key="step">{{ $t(`aiConnector.${step}`) }}</li>
        </ol>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('aiConnector.claudeCodeTitle') }}</CardTitle>
        <CardDescription>{{ $t('aiConnector.claudeCodeDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="space-y-3">
        <code class="block bg-muted px-3 py-2 rounded text-xs break-all" data-testid="claude-code-command">{{ claudeCodeCommand }}</code>
        <div class="flex flex-wrap items-center gap-3">
          <Button variant="outline" size="sm" @click="copy(claudeCodeCommand)">
            <Copy class="h-4 w-4 mr-2" />{{ $t('aiConnector.copyCommand') }}
          </Button>
          <RouterLink v-if="canViewApiKeys" to="/settings/api-keys" class="text-sm text-emerald-500 hover:underline">
            {{ $t('aiConnector.apiKeysLink') }}
          </RouterLink>
          <span v-else class="text-sm text-muted-foreground">{{ $t('aiConnector.apiKeysAskAdmin') }}</span>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
