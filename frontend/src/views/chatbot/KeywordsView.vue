<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger
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
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Key, Search, ArrowLeft } from 'lucide-vue-next'

interface ButtonItem {
  id: string
  title: string
}

interface KeywordRule {
  id: string
  keywords: string[]
  match_type: 'exact' | 'contains' | 'regex'
  response_type: 'text' | 'template' | 'flow' | 'transfer'
  response_content: any
  priority: number
  enabled: boolean
  created_at: string
}

const rules = ref<KeywordRule[]>([])
const isLoading = ref(true)
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const searchQuery = ref('')
const editingRule = ref<KeywordRule | null>(null)
const deleteDialogOpen = ref(false)
const ruleToDelete = ref<KeywordRule | null>(null)

const formData = ref({
  keywords: '',
  match_type: 'contains' as const,
  response_type: 'text' as const,
  response_content: '',
  buttons: [] as ButtonItem[],
  priority: 0,
  enabled: true
})

function addButton() {
  if (formData.value.buttons.length >= 10) {
    toast.error('Maximum 10 buttons allowed')
    return
  }
  formData.value.buttons.push({ id: '', title: '' })
}

function removeButton(index: number) {
  formData.value.buttons.splice(index, 1)
}

onMounted(async () => {
  await fetchRules()
})

async function fetchRules() {
  isLoading.value = true
  try {
    const response = await chatbotService.listKeywords()
    // API response is wrapped in { status: "success", data: { rules: [...] } }
    const data = response.data.data || response.data
    rules.value = data.rules || []
  } catch (error) {
    console.error('Failed to load keyword rules:', error)
    rules.value = []
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  editingRule.value = null
  formData.value = {
    keywords: '',
    match_type: 'contains',
    response_type: 'text',
    response_content: '',
    buttons: [],
    priority: 0,
    enabled: true
  }
  isDialogOpen.value = true
}

function openEditDialog(rule: KeywordRule) {
  editingRule.value = rule
  formData.value = {
    keywords: rule.keywords.join(', '),
    match_type: rule.match_type,
    response_type: rule.response_type,
    response_content: rule.response_content?.body || '',
    buttons: rule.response_content?.buttons || [],
    priority: rule.priority,
    enabled: rule.enabled
  }
  isDialogOpen.value = true
}

async function saveRule() {
  if (!formData.value.keywords.trim()) {
    toast.error('Please enter at least one keyword')
    return
  }

  // Response content is required for text, optional for transfer
  if (formData.value.response_type !== 'transfer' && !formData.value.response_content.trim()) {
    toast.error('Please enter a response message')
    return
  }

  // Filter out empty buttons
  const validButtons = formData.value.buttons.filter(b => b.id.trim() && b.title.trim())

  isSubmitting.value = true
  try {
    const data = {
      keywords: formData.value.keywords.split(',').map(k => k.trim()).filter(Boolean),
      match_type: formData.value.match_type,
      response_type: formData.value.response_type,
      response_content: {
        body: formData.value.response_content,
        buttons: validButtons.length > 0 ? validButtons : undefined
      },
      priority: formData.value.priority,
      enabled: formData.value.enabled
    }

    if (editingRule.value) {
      await chatbotService.updateKeyword(editingRule.value.id, data)
      toast.success('Keyword rule updated')
    } else {
      await chatbotService.createKeyword(data)
      toast.success('Keyword rule created')
    }

    isDialogOpen.value = false
    await fetchRules()
  } catch (error) {
    toast.error('Failed to save keyword rule')
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(rule: KeywordRule) {
  ruleToDelete.value = rule
  deleteDialogOpen.value = true
}

async function confirmDeleteRule() {
  if (!ruleToDelete.value) return

  try {
    await chatbotService.deleteKeyword(ruleToDelete.value.id)
    toast.success('Keyword rule deleted')
    deleteDialogOpen.value = false
    ruleToDelete.value = null
    await fetchRules()
  } catch (error) {
    toast.error('Failed to delete keyword rule')
  }
}

async function toggleRule(rule: KeywordRule) {
  try {
    await chatbotService.updateKeyword(rule.id, { enabled: !rule.enabled })
    rule.enabled = !rule.enabled
    toast.success(rule.enabled ? 'Rule enabled' : 'Rule disabled')
  } catch (error) {
    toast.error('Failed to toggle rule')
  }
}

const filteredRules = ref<KeywordRule[]>([])
filteredRules.value = searchQuery.value
  ? rules.value.filter(r =>
      r.keywords.some(k => k.toLowerCase().includes(searchQuery.value.toLowerCase()))
    )
  : rules.value
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <RouterLink to="/chatbot">
          <Button variant="ghost" size="icon" class="mr-3">
            <ArrowLeft class="h-5 w-5" />
          </Button>
        </RouterLink>
        <Key class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">Keyword Rules</h1>
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/chatbot">Chatbot</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Keywords</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </div>
        <Dialog v-model:open="isDialogOpen">
          <DialogTrigger as-child>
            <Button variant="outline" size="sm" @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Add Rule
            </Button>
          </DialogTrigger>
          <DialogContent class="max-w-md">
            <DialogHeader>
              <DialogTitle>{{ editingRule ? 'Edit' : 'Create' }} Keyword Rule</DialogTitle>
              <DialogDescription>
                Configure keywords that trigger automated responses.
              </DialogDescription>
            </DialogHeader>
            <div class="space-y-4 py-4">
              <div class="space-y-2">
                <Label for="keywords">Keywords (comma-separated)</Label>
                <Input
                  id="keywords"
                  v-model="formData.keywords"
                  placeholder="hello, hi, hey"
                />
              </div>
              <div class="space-y-2">
                <Label for="match_type">Match Type</Label>
                <Select v-model="formData.match_type">
                  <SelectTrigger>
                    <SelectValue placeholder="Select match type" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="contains">Contains</SelectItem>
                    <SelectItem value="exact">Exact Match</SelectItem>
                    <SelectItem value="regex">Regex</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-2">
                <Label for="response_type">Response Type</Label>
                <Select v-model="formData.response_type">
                  <SelectTrigger>
                    <SelectValue placeholder="Select response type" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="text">Text Response</SelectItem>
                    <SelectItem value="transfer">Transfer to Agent</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-2">
                <Label for="response">
                  {{ formData.response_type === 'transfer' ? 'Transfer Message (optional)' : 'Response Message' }}
                </Label>
                <Textarea
                  id="response"
                  v-model="formData.response_content"
                  :placeholder="formData.response_type === 'transfer' ? 'Connecting you with a human agent...' : 'Enter the response message...'"
                  :rows="3"
                />
                <p v-if="formData.response_type === 'transfer'" class="text-xs text-muted-foreground">
                  This message is sent before transferring the conversation to a human agent
                </p>
              </div>

              <!-- Buttons Section (only for text responses) -->
              <div v-if="formData.response_type !== 'transfer'" class="space-y-2">
                <div class="flex items-center justify-between">
                  <Label>Buttons (optional, max 10)</Label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    @click="addButton"
                    :disabled="formData.buttons.length >= 10"
                  >
                    <Plus class="h-3 w-3 mr-1" />
                    Add Button
                  </Button>
                </div>
                <p class="text-xs text-muted-foreground">
                  Add buttons for quick replies. 3 or fewer shows as buttons, more than 3 shows as a list.
                </p>
                <div v-if="formData.buttons.length > 0" class="space-y-2 mt-2">
                  <div
                    v-for="(button, index) in formData.buttons"
                    :key="index"
                    class="flex items-center gap-2"
                  >
                    <Input
                      v-model="button.id"
                      placeholder="Button ID"
                      class="flex-1"
                    />
                    <Input
                      v-model="button.title"
                      placeholder="Button Title"
                      class="flex-1"
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      @click="removeButton(index)"
                    >
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </div>
              </div>

              <div class="space-y-2">
                <Label for="priority">Priority (higher = checked first)</Label>
                <Input
                  id="priority"
                  v-model.number="formData.priority"
                  type="number"
                  min="0"
                />
              </div>
              <div class="flex items-center gap-2">
                <Switch
                  id="enabled"
                  :checked="formData.enabled"
                  @update:checked="formData.enabled = $event"
                />
                <Label for="enabled">Enabled</Label>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
              <Button size="sm" @click="saveRule" :disabled="isSubmitting">
                {{ editingRule ? 'Update' : 'Create' }}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </header>

    <!-- Search -->
    <div class="p-4 border-b">
      <div class="relative max-w-md">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input v-model="searchQuery" placeholder="Search keywords..." class="pl-9" />
      </div>
    </div>

    <!-- Rules List -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4">
        <!-- Loading Skeleton -->
        <template v-if="isLoading">
          <Card v-for="i in 3" :key="i">
            <CardContent class="p-4">
              <div class="flex items-start justify-between">
                <div class="flex-1">
                  <div class="flex items-center gap-2 mb-2">
                    <Skeleton class="h-4 w-4" />
                    <Skeleton class="h-5 w-16" />
                    <Skeleton class="h-5 w-16" />
                    <Skeleton class="h-5 w-14" />
                  </div>
                  <Skeleton class="h-4 w-48 mb-2" />
                  <Skeleton class="h-12 w-full" />
                </div>
                <div class="flex items-center gap-2 ml-4">
                  <Skeleton class="h-8 w-8 rounded" />
                  <Skeleton class="h-8 w-8 rounded" />
                </div>
              </div>
            </CardContent>
          </Card>
        </template>

        <template v-else>
        <Card v-for="rule in rules" :key="rule.id">
          <CardContent class="p-4">
            <div class="flex items-start justify-between">
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-2">
                  <Key class="h-4 w-4 text-muted-foreground" />
                  <div class="flex flex-wrap gap-1">
                    <Badge v-for="keyword in rule.keywords" :key="keyword" variant="secondary">
                      {{ keyword }}
                    </Badge>
                  </div>
                  <Badge v-if="rule.response_type === 'transfer'" variant="outline" class="border-destructive text-destructive">
                    Transfer
                  </Badge>
                  <Badge
                    variant="outline"
                    :class="rule.enabled ? 'border-green-600 text-green-600' : ''"
                  >
                    {{ rule.enabled ? 'Active' : 'Inactive' }}
                  </Badge>
                </div>
                <p class="text-sm text-muted-foreground mb-2">
                  Match: {{ rule.match_type }} | Priority: {{ rule.priority }}
                </p>
                <p class="text-sm bg-muted p-2 rounded">
                  {{ rule.response_type === 'transfer'
                    ? (rule.response_content?.body || 'Transfers to agent')
                    : (rule.response_content?.body || 'No response configured') }}
                </p>
              </div>
              <div class="flex items-center gap-2 ml-4">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openEditDialog(rule)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Edit rule</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openDeleteDialog(rule)">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Delete rule</TooltipContent>
                </Tooltip>
              </div>
            </div>
          </CardContent>
        </Card>

        <div v-if="rules.length === 0" class="text-center py-12 text-muted-foreground">
          <Key class="h-12 w-12 mx-auto mb-4 opacity-50" />
          <p class="text-lg font-medium">No keyword rules yet</p>
          <p class="text-sm">Create your first keyword rule to get started.</p>
        </div>
        </template>
      </div>
    </ScrollArea>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Keyword Rule</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete this keyword rule? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteRule">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
