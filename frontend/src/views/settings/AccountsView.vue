<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { api } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import {
  Plus,
  Pencil,
  Trash2,
  Phone,
  Check,
  X,
  ArrowLeft,
  RefreshCw,
  Loader2,
  Copy,
  ExternalLink,
  AlertCircle,
  CheckCircle2,
  Settings2
} from 'lucide-vue-next'

interface WhatsAppAccount {
  id: string
  name: string
  app_id: string
  phone_id: string
  business_id: string
  webhook_verify_token: string
  api_version: string
  is_default_incoming: boolean
  is_default_outgoing: boolean
  auto_read_receipt: boolean
  status: string
  has_access_token: boolean
  phone_number?: string
  display_name?: string
  created_at: string
  updated_at: string
}

interface TestResult {
  success: boolean
  error?: string
  display_phone_number?: string
  verified_name?: string
  quality_rating?: string
  messaging_limit_tier?: string
}

const organizationsStore = useOrganizationsStore()

const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingAccount = ref<WhatsAppAccount | null>(null)
const testingAccountId = ref<string | null>(null)
const testResults = ref<Record<string, TestResult>>({})
const deleteDialogOpen = ref(false)
const accountToDelete = ref<WhatsAppAccount | null>(null)

const formData = ref({
  name: '',
  app_id: '',
  phone_id: '',
  business_id: '',
  access_token: '',
  webhook_verify_token: '',
  api_version: 'v21.0',
  is_default_incoming: false,
  is_default_outgoing: false,
  auto_read_receipt: false
})

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, () => {
  fetchAccounts()
})

onMounted(async () => {
  await fetchAccounts()
})

async function fetchAccounts() {
  isLoading.value = true
  try {
    const response = await api.get('/accounts')
    accounts.value = response.data.data?.accounts || []
  } catch (error: any) {
    console.error('Failed to fetch accounts:', error)
    toast.error('Failed to load accounts')
    accounts.value = []
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  editingAccount.value = null
  formData.value = {
    name: '',
    app_id: '',
    phone_id: '',
    business_id: '',
    access_token: '',
    webhook_verify_token: '',
    api_version: 'v21.0',
    is_default_incoming: false,
    is_default_outgoing: false,
    auto_read_receipt: false
  }
  isDialogOpen.value = true
}

function openEditDialog(account: WhatsAppAccount) {
  editingAccount.value = account
  formData.value = {
    name: account.name,
    app_id: account.app_id || '',
    phone_id: account.phone_id,
    business_id: account.business_id,
    access_token: '', // Don't show existing token
    webhook_verify_token: account.webhook_verify_token,
    api_version: account.api_version,
    is_default_incoming: account.is_default_incoming,
    is_default_outgoing: account.is_default_outgoing,
    auto_read_receipt: account.auto_read_receipt
  }
  isDialogOpen.value = true
}

async function saveAccount() {
  if (!formData.value.name.trim() || !formData.value.phone_id.trim() || !formData.value.business_id.trim()) {
    toast.error('Please fill in all required fields')
    return
  }

  if (!editingAccount.value && !formData.value.access_token.trim()) {
    toast.error('Access token is required for new accounts')
    return
  }

  isSubmitting.value = true
  try {
    const payload = { ...formData.value }
    // Don't send empty access token when editing
    if (editingAccount.value && !payload.access_token) {
      delete (payload as any).access_token
    }

    if (editingAccount.value) {
      await api.put(`/accounts/${editingAccount.value.id}`, payload)
      toast.success('Account updated successfully')
    } else {
      await api.post('/accounts', payload)
      toast.success('Account created successfully')
    }

    isDialogOpen.value = false
    await fetchAccounts()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to save account'
    toast.error(message)
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(account: WhatsAppAccount) {
  accountToDelete.value = account
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!accountToDelete.value) return

  try {
    await api.delete(`/accounts/${accountToDelete.value.id}`)
    toast.success('Account deleted')
    deleteDialogOpen.value = false
    accountToDelete.value = null
    await fetchAccounts()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete account'
    toast.error(message)
  }
}

async function testConnection(account: WhatsAppAccount) {
  testingAccountId.value = account.id
  try {
    const response = await api.post(`/accounts/${account.id}/test`)
    testResults.value[account.id] = response.data.data

    if (response.data.data.success) {
      toast.success('Connection successful!')
    } else {
      toast.error('Connection failed: ' + (response.data.data.error || 'Unknown error'))
    }
  } catch (error: any) {
    const message = error.response?.data?.message || 'Connection test failed'
    testResults.value[account.id] = { success: false, error: message }
    toast.error(message)
  } finally {
    testingAccountId.value = null
  }
}

function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text)
  toast.success(`${label} copied to clipboard`)
}

function getStatusBadgeClass(status: string) {
  switch (status) {
    case 'active':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
    case 'inactive':
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
    case 'error':
      return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300'
    default:
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300'
  }
}

const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
const webhookUrl = window.location.origin + basePath + '/api/webhook'
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <RouterLink to="/settings">
          <Button variant="ghost" size="icon" class="mr-3">
            <ArrowLeft class="h-5 w-5" />
          </Button>
        </RouterLink>
        <Phone class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">WhatsApp Accounts</h1>
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/settings">Settings</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Accounts</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </div>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add Account
        </Button>
      </div>
    </header>

    <!-- Loading State -->
    <ScrollArea v-if="isLoading" class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Card v-for="i in 3" :key="i">
          <CardContent class="p-6">
            <div class="flex items-start gap-4">
              <Skeleton class="h-12 w-12 rounded-full" />
              <div class="flex-1 space-y-3">
                <Skeleton class="h-5 w-48" />
                <div class="grid grid-cols-2 gap-2">
                  <Skeleton class="h-4 w-32" />
                  <Skeleton class="h-4 w-32" />
                  <Skeleton class="h-4 w-32" />
                  <Skeleton class="h-4 w-32" />
                </div>
                <div class="flex gap-2">
                  <Skeleton class="h-6 w-24 rounded-full" />
                  <Skeleton class="h-6 w-24 rounded-full" />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Accounts List -->
    <ScrollArea v-else class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <!-- Webhook URL Info -->
        <Card class="border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-950">
          <CardContent class="p-4">
            <div class="flex items-start gap-3">
              <AlertCircle class="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
              <div class="flex-1">
                <h4 class="font-medium text-blue-900 dark:text-blue-100">Webhook Configuration</h4>
                <p class="text-sm text-blue-700 dark:text-blue-300 mt-1">
                  Configure this URL in your Meta Developer Console as the webhook callback URL:
                </p>
                <div class="flex items-center gap-2 mt-2">
                  <code class="px-2 py-1 bg-blue-100 dark:bg-blue-900 rounded text-sm font-mono">
                    {{ webhookUrl }}
                  </code>
                  <Button variant="ghost" size="sm" @click="copyToClipboard(webhookUrl, 'Webhook URL')">
                    <Copy class="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Account Cards -->
        <Card v-for="account in accounts" :key="account.id">
          <CardContent class="p-6">
            <div class="flex items-start justify-between">
              <div class="flex items-start gap-4">
                <div class="h-12 w-12 rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center flex-shrink-0">
                  <Phone class="h-6 w-6 text-green-600 dark:text-green-400" />
                </div>
                <div class="min-w-0">
                  <div class="flex items-center gap-2 flex-wrap">
                    <h3 class="font-semibold text-lg">{{ account.name }}</h3>
                    <span :class="['px-2 py-0.5 text-xs font-medium rounded-full', getStatusBadgeClass(account.status)]">
                      {{ account.status }}
                    </span>
                  </div>

                  <!-- Test Result -->
                  <div v-if="testResults[account.id]" class="mt-2">
                    <div v-if="testResults[account.id].success" class="flex items-center gap-2 text-green-600 dark:text-green-400">
                      <CheckCircle2 class="h-4 w-4" />
                      <span class="text-sm font-medium">Connected</span>
                      <span v-if="testResults[account.id].display_phone_number" class="text-sm text-muted-foreground">
                        - {{ testResults[account.id].display_phone_number }}
                      </span>
                    </div>
                    <div v-else class="flex items-center gap-2 text-red-600 dark:text-red-400">
                      <X class="h-4 w-4" />
                      <span class="text-sm">{{ testResults[account.id].error }}</span>
                    </div>
                  </div>

                  <!-- Account Details -->
                  <div class="mt-3 grid grid-cols-2 gap-x-6 gap-y-1 text-sm">
                    <div v-if="account.app_id" class="flex items-center gap-2">
                      <span class="text-muted-foreground">App ID:</span>
                      <code class="text-xs bg-muted px-1 rounded">{{ account.app_id }}</code>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-muted-foreground">Phone ID:</span>
                      <code class="text-xs bg-muted px-1 rounded">{{ account.phone_id }}</code>
                      <Button variant="ghost" size="icon" class="h-6 w-6" @click="copyToClipboard(account.phone_id, 'Phone ID')">
                        <Copy class="h-3 w-3" />
                      </Button>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-muted-foreground">Business ID:</span>
                      <code class="text-xs bg-muted px-1 rounded">{{ account.business_id }}</code>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-muted-foreground">API Version:</span>
                      <span>{{ account.api_version }}</span>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-muted-foreground">Access Token:</span>
                      <Badge
                        variant="outline"
                        :class="account.has_access_token ? 'border-green-600 text-green-600' : 'border-destructive text-destructive'"
                      >
                        {{ account.has_access_token ? 'Configured' : 'Missing' }}
                      </Badge>
                    </div>
                  </div>

                  <!-- Defaults -->
                  <div class="mt-3 flex items-center gap-3 flex-wrap">
                    <Badge v-if="account.is_default_incoming" variant="outline">
                      <Check class="h-3 w-3 mr-1" />
                      Default Incoming
                    </Badge>
                    <Badge v-if="account.is_default_outgoing" variant="outline">
                      <Check class="h-3 w-3 mr-1" />
                      Default Outgoing
                    </Badge>
                    <Badge v-if="account.auto_read_receipt" variant="outline">
                      <Check class="h-3 w-3 mr-1" />
                      Auto Read Receipt
                    </Badge>
                  </div>

                  <!-- Webhook Verify Token -->
                  <div class="mt-3 flex items-center gap-2 text-sm">
                    <span class="text-muted-foreground">Verify Token:</span>
                    <code class="text-xs bg-muted px-2 py-0.5 rounded font-mono truncate max-w-[200px]">
                      {{ account.webhook_verify_token }}
                    </code>
                    <Button variant="ghost" size="icon" class="h-6 w-6" @click="copyToClipboard(account.webhook_verify_token, 'Verify Token')">
                      <Copy class="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  @click="testConnection(account)"
                  :disabled="testingAccountId === account.id"
                >
                  <Loader2 v-if="testingAccountId === account.id" class="h-4 w-4 animate-spin" />
                  <RefreshCw v-else class="h-4 w-4" />
                  <span class="ml-1">Test</span>
                </Button>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openEditDialog(account)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Edit account</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openDeleteDialog(account)">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Delete account</TooltipContent>
                </Tooltip>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Empty State -->
        <Card v-if="accounts.length === 0">
          <CardContent class="py-12 text-center text-muted-foreground">
            <Phone class="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p class="text-lg font-medium">No WhatsApp accounts connected</p>
            <p class="text-sm mb-4">Connect your WhatsApp Business account to start sending and receiving messages.</p>
            <Button variant="outline" size="sm" @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Add Account
            </Button>
          </CardContent>
        </Card>

        <!-- Setup Guide -->
        <Card>
          <CardContent class="p-6">
            <h3 class="font-semibold flex items-center gap-2 mb-4">
              <Settings2 class="h-5 w-5" />
              Setup Guide
            </h3>
            <ol class="list-decimal list-inside space-y-3 text-sm text-muted-foreground">
              <li>
                Go to <a href="https://developers.facebook.com" target="_blank" class="text-primary hover:underline inline-flex items-center gap-1">
                  Meta Developer Console <ExternalLink class="h-3 w-3" />
                </a> and create or select your app
              </li>
              <li>Add WhatsApp product to your app and complete the setup</li>
              <li>In WhatsApp &gt; API Setup, copy your <strong>Phone Number ID</strong> and <strong>WhatsApp Business Account ID</strong></li>
              <li>
                Create a permanent access token in <a href="https://business.facebook.com/settings/system-users" target="_blank" class="text-primary hover:underline inline-flex items-center gap-1">
                  Business Settings &gt; System Users <ExternalLink class="h-3 w-3" />
                </a>
              </li>
              <li>Configure the webhook URL and verify token in your Meta app settings</li>
              <li>Subscribe to messages webhook field</li>
            </ol>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Add/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ editingAccount ? 'Edit' : 'Add' }} WhatsApp Account</DialogTitle>
          <DialogDescription>
            Connect your WhatsApp Business account using the Meta Cloud API.
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="name">Account Name <span class="text-destructive">*</span></Label>
            <Input
              id="name"
              v-model="formData.name"
              placeholder="e.g., Main Business Line"
            />
          </div>

          <Separator />

          <div class="space-y-2">
            <Label for="app_id">Meta App ID</Label>
            <Input
              id="app_id"
              v-model="formData.app_id"
              placeholder="e.g., 123456789012345"
            />
            <p class="text-xs text-muted-foreground">
              Found in Meta Developer Console &gt; App Dashboard
            </p>
          </div>

          <div class="space-y-2">
            <Label for="phone_id">Phone Number ID <span class="text-destructive">*</span></Label>
            <Input
              id="phone_id"
              v-model="formData.phone_id"
              placeholder="e.g., 123456789012345"
            />
            <p class="text-xs text-muted-foreground">
              Found in Meta Developer Console &gt; WhatsApp &gt; API Setup
            </p>
          </div>

          <div class="space-y-2">
            <Label for="business_id">WhatsApp Business Account ID <span class="text-destructive">*</span></Label>
            <Input
              id="business_id"
              v-model="formData.business_id"
              placeholder="e.g., 987654321098765"
            />
          </div>

          <div class="space-y-2">
            <Label for="access_token">
              Access Token
              <span v-if="!editingAccount" class="text-destructive">*</span>
              <span v-else class="text-muted-foreground">(leave blank to keep existing)</span>
            </Label>
            <Input
              id="access_token"
              v-model="formData.access_token"
              type="password"
              placeholder="Permanent access token from System User"
            />
            <p class="text-xs text-muted-foreground">
              Generate in Business Settings &gt; System Users &gt; Generate Token
            </p>
          </div>

          <Separator />

          <div class="space-y-2">
            <Label for="api_version">API Version</Label>
            <Input
              id="api_version"
              v-model="formData.api_version"
              placeholder="v21.0"
            />
          </div>

          <div class="space-y-2">
            <Label for="webhook_verify_token">Webhook Verify Token</Label>
            <Input
              id="webhook_verify_token"
              v-model="formData.webhook_verify_token"
              placeholder="Auto-generated if empty"
            />
            <p class="text-xs text-muted-foreground">
              Used to verify webhook requests from Meta
            </p>
          </div>

          <Separator />

          <div class="space-y-4">
            <Label>Options</Label>
            <div class="flex items-center justify-between">
              <Label for="is_default_incoming" class="font-normal cursor-pointer">
                Default for incoming messages
              </Label>
              <Switch
                id="is_default_incoming"
                :checked="formData.is_default_incoming"
                @update:checked="formData.is_default_incoming = $event"
              />
            </div>
            <div class="flex items-center justify-between">
              <Label for="is_default_outgoing" class="font-normal cursor-pointer">
                Default for outgoing messages
              </Label>
              <Switch
                id="is_default_outgoing"
                :checked="formData.is_default_outgoing"
                @update:checked="formData.is_default_outgoing = $event"
              />
            </div>
            <div class="flex items-center justify-between">
              <Label for="auto_read_receipt" class="font-normal cursor-pointer">
                Automatically send read receipts
              </Label>
              <Switch
                id="auto_read_receipt"
                :checked="formData.auto_read_receipt"
                @update:checked="formData.auto_read_receipt = $event"
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
          <Button size="sm" @click="saveAccount" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ editingAccount ? 'Update' : 'Create' }} Account
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Account</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ accountToDelete?.name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDelete" class="bg-destructive text-destructive-foreground hover:bg-destructive/90">
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
