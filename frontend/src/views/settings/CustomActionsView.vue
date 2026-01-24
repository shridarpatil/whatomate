<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { customActionsService, type CustomAction } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { toast } from 'vue-sonner'
import {
  Plus,
  Trash2,
  Pencil,
  Zap,
  Loader2,
  Globe,
  Webhook,
  Code,
  Ticket,
  User,
  BarChart,
  Link,
  Phone,
  Mail,
  FileText,
  ExternalLink
} from 'lucide-vue-next'

const actions = ref<CustomAction[]>([])
const isLoading = ref(false)
const isSaving = ref(false)

// Create/Edit dialog
const isDialogOpen = ref(false)
const isEditing = ref(false)
const editingActionId = ref<string | null>(null)
const formData = ref({
  name: '',
  icon: 'zap',
  action_type: 'webhook' as 'webhook' | 'url' | 'javascript',
  is_active: true,
  display_order: 0,
  config: {
    url: '',
    method: 'POST',
    headers: {} as Record<string, string>,
    body: '',
    open_in_new_tab: true,
    code: ''
  }
})

// Headers editor
const newHeaderKey = ref('')
const newHeaderValue = ref('')

// Delete confirmation
const isDeleteDialogOpen = ref(false)
const actionToDelete = ref<CustomAction | null>(null)

// Available icons
const iconOptions = [
  { value: 'ticket', label: 'Ticket', icon: Ticket },
  { value: 'user', label: 'User', icon: User },
  { value: 'bar-chart', label: 'Chart', icon: BarChart },
  { value: 'link', label: 'Link', icon: Link },
  { value: 'phone', label: 'Phone', icon: Phone },
  { value: 'mail', label: 'Mail', icon: Mail },
  { value: 'file-text', label: 'Document', icon: FileText },
  { value: 'external-link', label: 'External', icon: ExternalLink },
  { value: 'zap', label: 'Zap', icon: Zap },
  { value: 'globe', label: 'Globe', icon: Globe },
  { value: 'code', label: 'Code', icon: Code }
]

const getIconComponent = (iconName: string) => {
  const iconOption = iconOptions.find(i => i.value === iconName)
  return iconOption?.icon || Zap
}

async function fetchActions() {
  isLoading.value = true
  try {
    const response = await customActionsService.list()
    const data = (response.data as any).data || response.data
    actions.value = data.custom_actions || []
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to load custom actions')
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  isEditing.value = false
  editingActionId.value = null
  formData.value = {
    name: '',
    icon: 'zap',
    action_type: 'webhook',
    is_active: true,
    display_order: actions.value.length,
    config: {
      url: '',
      method: 'POST',
      headers: {},
      body: '',
      open_in_new_tab: true,
      code: ''
    }
  }
  isDialogOpen.value = true
}

function openEditDialog(action: CustomAction) {
  isEditing.value = true
  editingActionId.value = action.id
  formData.value = {
    name: action.name,
    icon: action.icon || 'zap',
    action_type: action.action_type,
    is_active: action.is_active,
    display_order: action.display_order,
    config: {
      url: action.config.url || '',
      method: action.config.method || 'POST',
      headers: { ...(action.config.headers || {}) },
      body: action.config.body || '',
      open_in_new_tab: action.config.open_in_new_tab !== false,
      code: action.config.code || ''
    }
  }
  isDialogOpen.value = true
}

async function saveAction() {
  if (!formData.value.name.trim()) {
    toast.error('Name is required')
    return
  }

  // Validate based on action type
  if (formData.value.action_type === 'webhook' || formData.value.action_type === 'url') {
    if (!formData.value.config.url.trim()) {
      toast.error('URL is required')
      return
    }
  }
  if (formData.value.action_type === 'javascript') {
    if (!formData.value.config.code.trim()) {
      toast.error('JavaScript code is required')
      return
    }
  }

  // Build config based on action type
  let config: Record<string, any> = {}
  switch (formData.value.action_type) {
    case 'webhook':
      config = {
        url: formData.value.config.url.trim(),
        method: formData.value.config.method,
        headers: formData.value.config.headers,
        body: formData.value.config.body.trim()
      }
      break
    case 'url':
      config = {
        url: formData.value.config.url.trim(),
        open_in_new_tab: formData.value.config.open_in_new_tab
      }
      break
    case 'javascript':
      config = {
        code: formData.value.config.code
      }
      break
  }

  isSaving.value = true
  try {
    if (isEditing.value && editingActionId.value) {
      await customActionsService.update(editingActionId.value, {
        name: formData.value.name.trim(),
        icon: formData.value.icon,
        action_type: formData.value.action_type,
        config,
        is_active: formData.value.is_active,
        display_order: formData.value.display_order
      })
      toast.success('Custom action updated successfully')
    } else {
      await customActionsService.create({
        name: formData.value.name.trim(),
        icon: formData.value.icon,
        action_type: formData.value.action_type,
        config,
        is_active: formData.value.is_active,
        display_order: formData.value.display_order
      })
      toast.success('Custom action created successfully')
    }
    isDialogOpen.value = false
    await fetchActions()
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to save custom action')
  } finally {
    isSaving.value = false
  }
}

async function toggleAction(action: CustomAction) {
  try {
    await customActionsService.update(action.id, {
      is_active: !action.is_active
    })
    await fetchActions()
    toast.success(action.is_active ? 'Action disabled' : 'Action enabled')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to update action')
  }
}

async function deleteAction() {
  if (!actionToDelete.value) return

  try {
    await customActionsService.delete(actionToDelete.value.id)
    await fetchActions()
    toast.success('Custom action deleted successfully')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to delete action')
  } finally {
    isDeleteDialogOpen.value = false
    actionToDelete.value = null
  }
}

function addHeader() {
  if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) {
    formData.value.config.headers[newHeaderKey.value.trim()] = newHeaderValue.value.trim()
    newHeaderKey.value = ''
    newHeaderValue.value = ''
  }
}

function removeHeader(key: string) {
  delete formData.value.config.headers[key]
}

function getActionTypeBadge(type: string) {
  switch (type) {
    case 'webhook':
      return { label: 'Webhook', variant: 'default' as const }
    case 'url':
      return { label: 'URL', variant: 'secondary' as const }
    case 'javascript':
      return { label: 'JavaScript', variant: 'outline' as const }
    default:
      return { label: type, variant: 'outline' as const }
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

const defaultBodyTemplate = `{
  "subject": "WhatsApp: {{contact.name}}",
  "phone": "{{contact.phone_number}}",
  "description": "Contact from WhatsApp",
  "user": "{{user.name}}"
}`

onMounted(() => {
  fetchActions()
})
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-yellow-500 to-orange-600 flex items-center justify-center mr-3 shadow-lg shadow-yellow-500/20">
          <Zap class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Custom Actions</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Configure custom action buttons for chat integrations</p>
        </div>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add Action
        </Button>
      </div>
    </header>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Your Custom Actions</CardTitle>
              <CardDescription>
                Custom actions appear as buttons in the chat header for quick integrations.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="w-[40px]"></TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead class="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-if="isLoading">
                    <TableCell colspan="7" class="text-center py-8 text-muted-foreground">
                      Loading...
                    </TableCell>
                  </TableRow>
                  <TableRow v-else-if="actions.length === 0">
                    <TableCell colspan="7" class="text-center py-8 text-muted-foreground">
                      <Zap class="h-8 w-8 mx-auto mb-2 opacity-50" />
                      <p>No custom actions configured</p>
                    </TableCell>
                  </TableRow>
                  <TableRow v-for="action in actions" :key="action.id">
                    <TableCell>
                      <component
                        :is="getIconComponent(action.icon)"
                        class="h-5 w-5 text-muted-foreground"
                      />
                    </TableCell>
                    <TableCell class="font-medium">{{ action.name }}</TableCell>
                    <TableCell>
                      <Badge :variant="getActionTypeBadge(action.action_type).variant">
                        {{ getActionTypeBadge(action.action_type).label }}
                      </Badge>
                    </TableCell>
                    <TableCell class="max-w-[200px] truncate text-muted-foreground">
                      <template v-if="action.action_type === 'webhook' || action.action_type === 'url'">
                        {{ action.config.url }}
                      </template>
                      <template v-else>
                        Custom Script
                      </template>
                    </TableCell>
                    <TableCell>
                      <div class="flex items-center gap-2">
                        <Switch
                          :checked="action.is_active"
                          @update:checked="toggleAction(action)"
                        />
                        <span class="text-sm text-muted-foreground">
                          {{ action.is_active ? 'Active' : 'Inactive' }}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell class="text-muted-foreground">
                      {{ formatDate(action.created_at) }}
                    </TableCell>
                    <TableCell class="text-right">
                      <div class="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          class="h-8 w-8"
                          @click="openEditDialog(action)"
                        >
                          <Pencil class="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          class="h-8 w-8 text-destructive"
                          @click="actionToDelete = action; isDeleteDialogOpen = true"
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
      <DialogContent class="max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? 'Edit Custom Action' : 'Add Custom Action' }}</DialogTitle>
          <DialogDescription>
            Configure an action button that appears in the chat header
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="name">Name</Label>
            <Input
              id="name"
              v-model="formData.name"
              placeholder="Create Support Ticket"
            />
          </div>

          <div class="space-y-2">
            <Label>Icon</Label>
            <div class="flex flex-wrap gap-2">
              <Button
                v-for="iconOpt in iconOptions"
                :key="iconOpt.value"
                variant="outline"
                size="icon"
                class="h-10 w-10"
                :class="{ 'ring-2 ring-primary': formData.icon === iconOpt.value }"
                @click="formData.icon = iconOpt.value"
              >
                <component :is="iconOpt.icon" class="h-5 w-5" />
              </Button>
            </div>
          </div>

          <div class="space-y-2">
            <Label>Action Type</Label>
            <RadioGroup v-model="formData.action_type" class="flex flex-col gap-2">
              <div class="flex items-center space-x-2">
                <RadioGroupItem value="webhook" id="type-webhook" />
                <Label for="type-webhook" class="flex items-center gap-2 cursor-pointer font-normal">
                  <Webhook class="h-4 w-4" />
                  Webhook (Call external API)
                </Label>
              </div>
              <div class="flex items-center space-x-2">
                <RadioGroupItem value="url" id="type-url" />
                <Label for="type-url" class="flex items-center gap-2 cursor-pointer font-normal">
                  <Globe class="h-4 w-4" />
                  Open URL (Open in browser)
                </Label>
              </div>
              <div class="flex items-center space-x-2">
                <RadioGroupItem value="javascript" id="type-javascript" />
                <Label for="type-javascript" class="flex items-center gap-2 cursor-pointer font-normal">
                  <Code class="h-4 w-4" />
                  JavaScript (Run custom code)
                </Label>
              </div>
            </RadioGroup>
          </div>

          <!-- Webhook Configuration -->
          <template v-if="formData.action_type === 'webhook'">
            <div class="border-t pt-4 space-y-4">
              <div class="space-y-2">
                <Label for="url">Webhook URL</Label>
                <Input
                  id="url"
                  v-model="formData.config.url"
                  type="url"
                  placeholder="https://api.helpdesk.com/tickets"
                />
              </div>
              <div class="space-y-2">
                <Label for="method">HTTP Method</Label>
                <Select v-model="formData.config.method">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="POST">POST</SelectItem>
                    <SelectItem value="GET">GET</SelectItem>
                    <SelectItem value="PUT">PUT</SelectItem>
                    <SelectItem value="PATCH">PATCH</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-2">
                <Label>Headers (optional)</Label>
                <div class="space-y-2">
                  <div
                    v-for="(value, key) in formData.config.headers"
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
              <div class="space-y-2">
                <div class="flex items-center justify-between">
                  <Label for="body">Request Body (JSON)</Label>
                  <Button
                    variant="link"
                    size="sm"
                    class="h-auto p-0 text-xs"
                    @click="formData.config.body = defaultBodyTemplate"
                  >
                    Insert template
                  </Button>
                </div>
                <Textarea
                  id="body"
                  v-model="formData.config.body"
                  placeholder='{"subject": "{{contact.name}}"}'
                  class="font-mono text-sm min-h-[120px]"
                />
                <p class="text-xs text-muted-foreground">
                  Available variables: <code class="bg-muted px-1 rounded" v-pre>{{contact.name}}</code>,
                  <code class="bg-muted px-1 rounded" v-pre>{{contact.phone_number}}</code>,
                  <code class="bg-muted px-1 rounded" v-pre>{{user.name}}</code>,
                  <code class="bg-muted px-1 rounded" v-pre>{{user.email}}</code>
                </p>
              </div>
            </div>
          </template>

          <!-- URL Configuration -->
          <template v-if="formData.action_type === 'url'">
            <div class="border-t pt-4 space-y-4">
              <div class="space-y-2">
                <Label for="url">URL</Label>
                <Input
                  id="url"
                  v-model="formData.config.url"
                  type="url"
                  placeholder="https://crm.example.com/contact?phone={{contact.phone_number}}"
                />
                <p class="text-xs text-muted-foreground">
                  Use <code class="bg-muted px-1 rounded" v-pre>{{contact.phone_number}}</code> or other variables in the URL
                </p>
              </div>
              <div class="flex items-center space-x-2">
                <Switch
                  id="new-tab"
                  :checked="formData.config.open_in_new_tab"
                  @update:checked="formData.config.open_in_new_tab = $event"
                />
                <Label for="new-tab" class="cursor-pointer">Open in new tab</Label>
              </div>
            </div>
          </template>

          <!-- JavaScript Configuration -->
          <template v-if="formData.action_type === 'javascript'">
            <div class="border-t pt-4 space-y-4">
              <div class="space-y-2">
                <Label for="code">JavaScript Code</Label>
                <Textarea
                  id="code"
                  v-model="formData.config.code"
                  placeholder="// Available: contact, user, organization
// contact: { id, phone_number, name, profile_name, tags, metadata }
// user: { id, name, email, role }

// Example: Copy phone to clipboard
return { clipboard: contact.phone_number }

// Example: Show toast
return { toast: { message: 'Hello ' + contact.name, type: 'success' } }

// Example: Open URL
return { url: 'https://crm.example.com?phone=' + contact.phone_number }"
                  class="font-mono text-sm min-h-[200px]"
                />
                <p class="text-xs text-muted-foreground">
                  Code runs in browser. Return an object with: <code class="bg-muted px-1 rounded">toast</code>,
                  <code class="bg-muted px-1 rounded">clipboard</code>, or
                  <code class="bg-muted px-1 rounded">url</code>
                </p>
              </div>
            </div>
          </template>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="isDialogOpen = false">Cancel</Button>
          <Button @click="saveAction" :disabled="isSaving">
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
          <AlertDialogTitle>Delete Custom Action</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ actionToDelete?.name }}"?
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            @click="deleteAction"
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
