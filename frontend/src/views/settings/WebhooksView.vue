<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { webhooksService, type Webhook, type WebhookEvent } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { toast } from 'vue-sonner'
import { Plus, Trash2, Pencil, Webhook as WebhookIcon, Play, Loader2 } from 'lucide-vue-next'

const organizationsStore = useOrganizationsStore()

const webhooks = ref<Webhook[]>([])
const availableEvents = ref<WebhookEvent[]>([])
const isLoading = ref(false)
const isSaving = ref(false)
const isTesting = ref<string | null>(null)

// Create/Edit dialog
const isDialogOpen = ref(false)
const isEditing = ref(false)
const editingWebhookId = ref<string | null>(null)
const formData = ref({
  name: '',
  url: '',
  events: [] as string[],
  secret: '',
  headers: {} as Record<string, string>
})

// Headers editor
const newHeaderKey = ref('')
const newHeaderValue = ref('')

// Delete confirmation
const isDeleteDialogOpen = ref(false)
const webhookToDelete = ref<Webhook | null>(null)

async function fetchWebhooks() {
  isLoading.value = true
  try {
    const response = await webhooksService.list()
    const data = (response.data as any).data || response.data
    webhooks.value = data.webhooks || []
    availableEvents.value = data.available_events || []
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to load webhooks')
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  isEditing.value = false
  editingWebhookId.value = null
  formData.value = {
    name: '',
    url: '',
    events: [],
    secret: '',
    headers: {}
  }
  isDialogOpen.value = true
}

function openEditDialog(webhook: Webhook) {
  isEditing.value = true
  editingWebhookId.value = webhook.id
  formData.value = {
    name: webhook.name,
    url: webhook.url,
    events: [...webhook.events],
    secret: '',
    headers: { ...webhook.headers }
  }
  isDialogOpen.value = true
}

async function saveWebhook() {
  if (!formData.value.name.trim()) {
    toast.error('Name is required')
    return
  }
  if (!formData.value.url.trim()) {
    toast.error('URL is required')
    return
  }
  if (formData.value.events.length === 0) {
    toast.error('At least one event must be selected')
    return
  }

  isSaving.value = true
  try {
    if (isEditing.value && editingWebhookId.value) {
      await webhooksService.update(editingWebhookId.value, {
        name: formData.value.name.trim(),
        url: formData.value.url.trim(),
        events: formData.value.events,
        headers: formData.value.headers,
        secret: formData.value.secret || undefined,
        is_active: true
      })
      toast.success('Webhook updated successfully')
    } else {
      await webhooksService.create({
        name: formData.value.name.trim(),
        url: formData.value.url.trim(),
        events: formData.value.events,
        headers: formData.value.headers,
        secret: formData.value.secret || undefined
      })
      toast.success('Webhook created successfully')
    }
    isDialogOpen.value = false
    await fetchWebhooks()
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to save webhook')
  } finally {
    isSaving.value = false
  }
}

async function toggleWebhook(webhook: Webhook) {
  try {
    await webhooksService.update(webhook.id, {
      is_active: !webhook.is_active
    })
    await fetchWebhooks()
    toast.success(webhook.is_active ? 'Webhook disabled' : 'Webhook enabled')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to update webhook')
  }
}

async function testWebhook(webhook: Webhook) {
  isTesting.value = webhook.id
  try {
    await webhooksService.test(webhook.id)
    toast.success('Test webhook sent successfully')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Webhook test failed')
  } finally {
    isTesting.value = null
  }
}

async function deleteWebhook() {
  if (!webhookToDelete.value) return

  try {
    await webhooksService.delete(webhookToDelete.value.id)
    await fetchWebhooks()
    toast.success('Webhook deleted successfully')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to delete webhook')
  } finally {
    isDeleteDialogOpen.value = false
    webhookToDelete.value = null
  }
}

function addHeader() {
  if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) {
    formData.value.headers[newHeaderKey.value.trim()] = newHeaderValue.value.trim()
    newHeaderKey.value = ''
    newHeaderValue.value = ''
  }
}

function removeHeader(key: string) {
  delete formData.value.headers[key]
}

function toggleEvent(eventValue: string, checked: boolean | 'indeterminate') {
  if (checked === true) {
    if (!formData.value.events.includes(eventValue)) {
      formData.value.events.push(eventValue)
    }
  } else {
    const index = formData.value.events.indexOf(eventValue)
    if (index > -1) {
      formData.value.events.splice(index, 1)
    }
  }
}

function getEventLabel(eventValue: string): string {
  const event = availableEvents.value.find(e => e.value === eventValue)
  return event?.label || eventValue
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, () => {
  fetchWebhooks()
})

onMounted(() => {
  fetchWebhooks()
})
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center mr-3 shadow-lg shadow-indigo-500/20">
          <WebhookIcon class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Webhooks</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Configure webhooks to send events to external systems</p>
        </div>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add Webhook
        </Button>
      </div>
    </header>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Your Webhooks</CardTitle>
              <CardDescription>
                Webhooks allow you to send real-time events to external systems like helpdesks.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>URL</TableHead>
                    <TableHead>Events</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead class="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-if="isLoading">
                    <TableCell colspan="6" class="text-center py-8 text-muted-foreground">
                      Loading...
                    </TableCell>
                  </TableRow>
                  <TableRow v-else-if="webhooks.length === 0">
                    <TableCell colspan="6" class="text-center py-8 text-muted-foreground">
                      <WebhookIcon class="h-8 w-8 mx-auto mb-2 opacity-50" />
                      <p>No webhooks configured</p>
                    </TableCell>
                  </TableRow>
                  <TableRow v-for="webhook in webhooks" :key="webhook.id">
                    <TableCell class="font-medium">{{ webhook.name }}</TableCell>
                    <TableCell class="max-w-[200px] truncate text-muted-foreground">
                      {{ webhook.url }}
                    </TableCell>
                    <TableCell>
                      <div class="flex flex-wrap gap-1">
                        <Badge
                          v-for="event in webhook.events.slice(0, 2)"
                          :key="event"
                          variant="secondary"
                          class="text-xs"
                        >
                          {{ getEventLabel(event) }}
                        </Badge>
                        <Badge
                          v-if="webhook.events.length > 2"
                          variant="outline"
                          class="text-xs"
                        >
                          +{{ webhook.events.length - 2 }}
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div class="flex items-center gap-2">
                        <Switch
                          :checked="webhook.is_active"
                          @update:checked="toggleWebhook(webhook)"
                        />
                        <span class="text-sm text-muted-foreground">
                          {{ webhook.is_active ? 'Active' : 'Inactive' }}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell class="text-muted-foreground">
                      {{ formatDate(webhook.created_at) }}
                    </TableCell>
                    <TableCell class="text-right">
                      <div class="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          class="h-8 w-8"
                          :disabled="isTesting === webhook.id"
                          @click="testWebhook(webhook)"
                        >
                          <Loader2 v-if="isTesting === webhook.id" class="h-4 w-4 animate-spin" />
                          <Play v-else class="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          class="h-8 w-8"
                          @click="openEditDialog(webhook)"
                        >
                          <Pencil class="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          class="h-8 w-8 text-destructive"
                          @click="webhookToDelete = webhook; isDeleteDialogOpen = true"
                        >
                          <Trash2 class="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Create/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? 'Edit Webhook' : 'Add Webhook' }}</DialogTitle>
          <DialogDescription>
            Configure a webhook to receive events from Whatomate
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="name">Name</Label>
            <Input
              id="name"
              v-model="formData.name"
              placeholder="My Helpdesk Integration"
            />
          </div>
          <div class="space-y-2">
            <Label for="url">Webhook URL</Label>
            <Input
              id="url"
              v-model="formData.url"
              type="url"
              placeholder="https://example.com/webhook"
            />
          </div>
          <div class="space-y-2">
            <Label>Events</Label>
            <div class="grid grid-cols-1 gap-2 border rounded-lg p-3">
              <div
                v-for="event in availableEvents"
                :key="event.value"
                class="flex items-start gap-2"
              >
                <Checkbox
                  :id="event.value"
                  :checked="formData.events.includes(event.value)"
                  @update:checked="(checked) => toggleEvent(event.value, checked)"
                />
                <div class="grid gap-0.5">
                  <Label :for="event.value" class="cursor-pointer">{{ event.label }}</Label>
                  <p class="text-xs text-muted-foreground">{{ event.description }}</p>
                </div>
              </div>
            </div>
          </div>
          <div class="space-y-2">
            <Label for="secret">Secret (optional)</Label>
            <Input
              id="secret"
              v-model="formData.secret"
              type="password"
              placeholder="Used for HMAC signature verification"
            />
            <p class="text-xs text-muted-foreground">
              If set, requests will include X-Webhook-Signature header
            </p>
          </div>
          <div class="space-y-2">
            <Label>Custom Headers (optional)</Label>
            <div class="space-y-2">
              <div
                v-for="(value, key) in formData.headers"
                :key="key"
                class="flex items-center gap-2"
              >
                <Badge variant="secondary" class="flex-shrink-0">{{ key }}</Badge>
                <span class="text-sm truncate flex-1">{{ value }}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6 flex-shrink-0"
                  @click="removeHeader(key as string)"
                >
                  <Trash2 class="h-3 w-3" />
                </Button>
              </div>
              <div class="flex gap-2">
                <Input
                  v-model="newHeaderKey"
                  placeholder="Header name"
                  class="flex-1"
                />
                <Input
                  v-model="newHeaderValue"
                  placeholder="Value"
                  class="flex-1"
                />
                <Button variant="outline" size="sm" @click="addHeader">Add</Button>
              </div>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="isDialogOpen = false">Cancel</Button>
          <Button @click="saveWebhook" :disabled="isSaving">
            <Loader2 v-if="isSaving" class="h-4 w-4 mr-2 animate-spin" />
            {{ isEditing ? 'Update' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation -->
    <AlertDialog v-model:open="isDeleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Webhook</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ webhookToDelete?.name }}"?
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            @click="deleteWebhook"
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
