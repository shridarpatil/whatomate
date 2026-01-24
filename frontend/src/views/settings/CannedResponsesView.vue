<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { toast } from 'vue-sonner'
import {
  Plus,
  Search,
  MessageSquareText,
  Pencil,
  Trash2,
  Loader2,
  Copy
} from 'lucide-vue-next'

const cannedResponses = ref<CannedResponse[]>([])
const isLoading = ref(true)
const searchQuery = ref('')
const selectedCategory = ref<string>('all')

// Dialog state
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingResponse = ref<CannedResponse | null>(null)
const deleteDialogOpen = ref(false)
const responseToDelete = ref<CannedResponse | null>(null)

const formData = ref({
  name: '',
  shortcut: '',
  content: '',
  category: '',
  is_active: true
})

const categories = [
  { value: 'greeting', label: 'Greetings' },
  { value: 'support', label: 'Support' },
  { value: 'sales', label: 'Sales' },
  { value: 'closing', label: 'Closing' },
  { value: 'general', label: 'General' },
]

onMounted(async () => {
  await fetchCannedResponses()
})

async function fetchCannedResponses() {
  isLoading.value = true
  try {
    const params: any = {}
    if (selectedCategory.value && selectedCategory.value !== 'all') {
      params.category = selectedCategory.value
    }
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    const response = await cannedResponsesService.list(params)
    cannedResponses.value = response.data.data?.canned_responses || []
  } catch (error: any) {
    toast.error('Failed to load canned responses')
    cannedResponses.value = []
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  editingResponse.value = null
  formData.value = {
    name: '',
    shortcut: '',
    content: '',
    category: '',
    is_active: true
  }
  isDialogOpen.value = true
}

function openEditDialog(response: CannedResponse) {
  editingResponse.value = response
  formData.value = {
    name: response.name,
    shortcut: response.shortcut || '',
    content: response.content,
    category: response.category || '',
    is_active: response.is_active
  }
  isDialogOpen.value = true
}

async function saveResponse() {
  if (!formData.value.name.trim() || !formData.value.content.trim()) {
    toast.error('Name and content are required')
    return
  }

  isSubmitting.value = true
  try {
    if (editingResponse.value) {
      await cannedResponsesService.update(editingResponse.value.id, formData.value)
      toast.success('Canned response updated')
    } else {
      await cannedResponsesService.create(formData.value)
      toast.success('Canned response created')
    }
    isDialogOpen.value = false
    await fetchCannedResponses()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to save'
    toast.error(message)
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(response: CannedResponse) {
  responseToDelete.value = response
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!responseToDelete.value) return
  try {
    await cannedResponsesService.delete(responseToDelete.value.id)
    toast.success('Canned response deleted')
    deleteDialogOpen.value = false
    responseToDelete.value = null
    await fetchCannedResponses()
  } catch (error: any) {
    toast.error('Failed to delete')
  }
}

function copyToClipboard(content: string) {
  navigator.clipboard.writeText(content)
  toast.success('Copied to clipboard')
}

const filteredResponses = computed(() => {
  return cannedResponses.value
})

function getCategoryLabel(category: string): string {
  return categories.find(c => c.value === category)?.label || category || 'Uncategorized'
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-teal-500 to-emerald-600 flex items-center justify-center mr-3 shadow-lg shadow-teal-500/20">
          <MessageSquareText class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Canned Responses</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Pre-defined responses for quick messaging</p>
        </div>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add Response
        </Button>
      </div>
    </header>

    <!-- Filters -->
    <div class="p-4 border-b flex items-center gap-4 flex-wrap">
      <div class="flex items-center gap-2">
        <Label class="text-sm text-muted-foreground">Category:</Label>
        <Select v-model="selectedCategory" @update:model-value="fetchCannedResponses">
          <SelectTrigger class="w-[150px]">
            <SelectValue placeholder="All" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Categories</SelectItem>
            <SelectItem v-for="cat in categories" :key="cat.value" :value="cat.value">
              {{ cat.label }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="relative flex-1 max-w-md">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          v-model="searchQuery"
          placeholder="Search responses..."
          class="pl-9"
          @input="fetchCannedResponses"
        />
      </div>
    </div>

    <!-- Loading -->
    <div v-if="isLoading" class="flex-1 flex items-center justify-center">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Responses Grid -->
    <ScrollArea v-else class="flex-1">
      <div class="p-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card v-for="response in filteredResponses" :key="response.id" class="flex flex-col">
          <CardHeader class="pb-3">
            <div class="flex items-start justify-between">
              <div class="flex-1 min-w-0">
                <CardTitle class="text-base truncate">{{ response.name }}</CardTitle>
                <div class="flex items-center gap-2 mt-2">
                  <Badge variant="outline" class="text-xs">
                    {{ getCategoryLabel(response.category) }}
                  </Badge>
                  <span v-if="response.shortcut" class="text-xs font-mono text-muted-foreground">
                    /{{ response.shortcut }}
                  </span>
                </div>
              </div>
              <Badge v-if="!response.is_active" variant="secondary" class="ml-2">
                Inactive
              </Badge>
            </div>
          </CardHeader>
          <CardContent class="flex-1">
            <p class="text-sm text-muted-foreground line-clamp-3 whitespace-pre-wrap">
              {{ response.content }}
            </p>
            <p class="text-xs text-muted-foreground mt-2">
              Used {{ response.usage_count }} times
            </p>
          </CardContent>
          <div class="px-6 pb-4 flex items-center gap-1 border-t pt-3">
            <Button variant="ghost" size="sm" @click="copyToClipboard(response.content)">
              <Copy class="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="sm" @click="openEditDialog(response)">
              <Pencil class="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="sm" @click="openDeleteDialog(response)">
              <Trash2 class="h-4 w-4 text-destructive" />
            </Button>
          </div>
        </Card>

        <!-- Empty State -->
        <Card v-if="filteredResponses.length === 0" class="col-span-full">
          <CardContent class="py-12 text-center text-muted-foreground">
            <MessageSquareText class="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p class="text-lg font-medium">No canned responses found</p>
            <p class="text-sm mb-4">Create your first canned response to get started.</p>
            <Button variant="outline" size="sm" @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Add Response
            </Button>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Create/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ editingResponse ? 'Edit' : 'Create' }} Canned Response</DialogTitle>
          <DialogDescription>
            {{ editingResponse ? 'Update the response details.' : 'Add a new quick response.' }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label>Name <span class="text-destructive">*</span></Label>
            <Input v-model="formData.name" placeholder="Welcome Message" />
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label>Shortcut</Label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">/</span>
                <Input v-model="formData.shortcut" placeholder="welcome" class="pl-7" />
              </div>
              <p class="text-xs text-muted-foreground">Type /welcome to quickly find</p>
            </div>

            <div class="space-y-2">
              <Label>Category</Label>
              <Select v-model="formData.category">
                <SelectTrigger>
                  <SelectValue placeholder="Select category" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="cat in categories" :key="cat.value" :value="cat.value">
                    {{ cat.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div class="space-y-2">
            <Label>Content <span class="text-destructive">*</span></Label>
            <Textarea
              v-model="formData.content"
              placeholder="Hello {{contact_name}}! Thank you for reaching out. How can I help you today?"
              :rows="5"
            />
            <p class="text-xs text-muted-foreground">
              Placeholders: <code class="bg-muted px-1 rounded" v-pre>{{contact_name}}</code> for name, <code class="bg-muted px-1 rounded" v-pre>{{phone_number}}</code> for phone
            </p>
          </div>

          <div class="flex items-center justify-between" v-if="editingResponse">
            <Label>Active</Label>
            <Switch v-model:checked="formData.is_active" />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="isDialogOpen = false">Cancel</Button>
          <Button @click="saveResponse" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ editingResponse ? 'Update' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Canned Response</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ responseToDelete?.name }}"?
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDelete">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
