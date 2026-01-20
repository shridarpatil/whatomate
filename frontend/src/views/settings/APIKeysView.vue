<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiKeysService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
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
import { Plus, Trash2, Copy, Key, AlertTriangle } from 'lucide-vue-next'

interface APIKey {
  id: string
  name: string
  key_prefix: string
  last_used_at: string | null
  expires_at: string | null
  is_active: boolean
  created_at: string
}

interface NewAPIKeyResponse {
  id: string
  name: string
  key: string
  key_prefix: string
  expires_at: string | null
  created_at: string
}

const apiKeys = ref<APIKey[]>([])
const isLoading = ref(false)
const isCreating = ref(false)

// Create dialog
const isCreateDialogOpen = ref(false)
const newKeyName = ref('')
const newKeyExpiry = ref('')

// Key display dialog (shown after creation)
const isKeyDisplayOpen = ref(false)
const newlyCreatedKey = ref<NewAPIKeyResponse | null>(null)

// Delete confirmation
const isDeleteDialogOpen = ref(false)
const keyToDelete = ref<APIKey | null>(null)

async function fetchAPIKeys() {
  isLoading.value = true
  try {
    const response = await apiKeysService.list()
    apiKeys.value = response.data.data || []
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to load API keys')
  } finally {
    isLoading.value = false
  }
}

async function createAPIKey() {
  if (!newKeyName.value.trim()) {
    toast.error('Name is required')
    return
  }

  isCreating.value = true
  try {
    const payload: { name: string; expires_at?: string } = {
      name: newKeyName.value.trim()
    }
    if (newKeyExpiry.value) {
      payload.expires_at = new Date(newKeyExpiry.value).toISOString()
    }

    const response = await apiKeysService.create(payload)
    newlyCreatedKey.value = response.data.data
    isCreateDialogOpen.value = false
    isKeyDisplayOpen.value = true
    newKeyName.value = ''
    newKeyExpiry.value = ''
    await fetchAPIKeys()
    toast.success('API key created successfully')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to create API key')
  } finally {
    isCreating.value = false
  }
}

async function deleteAPIKey() {
  if (!keyToDelete.value) return

  try {
    await apiKeysService.delete(keyToDelete.value.id)
    await fetchAPIKeys()
    toast.success('API key deleted successfully')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to delete API key')
  } finally {
    isDeleteDialogOpen.value = false
    keyToDelete.value = null
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
  toast.success('Copied to clipboard')
}

function formatDate(dateStr: string | null) {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function isExpired(expiresAt: string | null) {
  if (!expiresAt) return false
  return new Date(expiresAt) < new Date()
}

onMounted(() => {
  fetchAPIKeys()
})
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center mr-3 shadow-lg shadow-amber-500/20">
          <Key class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">API Keys</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Manage API keys for programmatic access</p>
        </div>
        <Button variant="outline" size="sm" @click="isCreateDialogOpen = true">
          <Plus class="h-4 w-4 mr-2" />
          Create API Key
        </Button>
      </div>
    </header>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>Your API Keys</CardTitle>
            <CardDescription>
              API keys allow external applications to access your account. Keep them secure.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Last Used</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead class="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-if="isLoading">
                  <TableCell colspan="6" class="text-center py-8 text-muted-foreground">
                    Loading...
                  </TableCell>
                </TableRow>
                <TableRow v-else-if="apiKeys.length === 0">
                  <TableCell colspan="6" class="text-center py-8 text-muted-foreground">
                    <Key class="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>No API keys yet</p>
                  </TableCell>
                </TableRow>
                <TableRow v-for="key in apiKeys" :key="key.id">
                  <TableCell class="font-medium">{{ key.name }}</TableCell>
                  <TableCell>
                    <code class="bg-muted px-2 py-1 rounded text-sm">
                      whm_{{ key.key_prefix }}...
                    </code>
                  </TableCell>
                  <TableCell>{{ formatDate(key.last_used_at) }}</TableCell>
                  <TableCell>{{ formatDate(key.expires_at) }}</TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      :class="isExpired(key.expires_at) ? 'border-destructive text-destructive' : key.is_active ? 'border-green-600 text-green-600' : ''"
                    >
                      {{ isExpired(key.expires_at) ? 'Expired' : key.is_active ? 'Active' : 'Inactive' }}
                    </Badge>
                  </TableCell>
                  <TableCell class="text-right">
                    <Button
                      variant="ghost"
                      size="icon"
                      @click="keyToDelete = key; isDeleteDialogOpen = true"
                    >
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Create API Key Dialog -->
    <Dialog v-model:open="isCreateDialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create API Key</DialogTitle>
          <DialogDescription>
            Create a new API key for programmatic access to your account.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="name">Name</Label>
            <Input
              id="name"
              v-model="newKeyName"
              placeholder="e.g., Production Integration"
            />
          </div>
          <div class="space-y-2">
            <Label for="expiry">Expiration (optional)</Label>
            <Input
              id="expiry"
              v-model="newKeyExpiry"
              type="datetime-local"
            />
            <p class="text-xs text-muted-foreground">
              Leave empty for no expiration
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" @click="isCreateDialogOpen = false">
            Cancel
          </Button>
          <Button size="sm" @click="createAPIKey" :disabled="isCreating">
            {{ isCreating ? 'Creating...' : 'Create Key' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- New Key Display Dialog -->
    <Dialog v-model:open="isKeyDisplayOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>API Key Created</DialogTitle>
          <DialogDescription>
            <div class="flex items-center gap-2 text-amber-600 mt-2">
              <AlertTriangle class="h-4 w-4" />
              <span>Make sure to copy your API key now. You won't be able to see it again!</span>
            </div>
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label>Your API Key</Label>
            <div class="flex gap-2">
              <Input
                :model-value="newlyCreatedKey?.key"
                readonly
                class="font-mono text-sm"
              />
              <Button
                variant="outline"
                size="icon"
                @click="copyToClipboard(newlyCreatedKey?.key || '')"
              >
                <Copy class="h-4 w-4" />
              </Button>
            </div>
          </div>
          <div class="bg-muted p-3 rounded-lg text-sm">
            <p class="font-medium mb-1">Usage:</p>
            <code class="text-xs">
              curl -H "X-API-Key: {{ newlyCreatedKey?.key }}" https://your-api.com/api/contacts
            </code>
          </div>
        </div>
        <DialogFooter>
          <Button size="sm" @click="isKeyDisplayOpen = false">
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="isDeleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete API Key</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ keyToDelete?.name }}"?
            This action cannot be undone and any applications using this key will stop working.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="deleteAPIKey" class="bg-destructive text-destructive-foreground hover:bg-destructive/90">
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
