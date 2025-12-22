<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useContactsStore, type Contact, type Message } from '@/stores/contacts'
import { useAuthStore } from '@/stores/auth'
import { useUsersStore } from '@/stores/users'
import { useTransfersStore } from '@/stores/transfers'
import { wsService } from '@/services/websocket'
import { contactsService, chatbotService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'vue-sonner'
import {
  Search,
  Send,
  Paperclip,
  Image,
  FileText,
  Smile,
  MoreVertical,
  Phone,
  Video,
  Check,
  CheckCheck,
  Clock,
  AlertCircle,
  User,
  UserPlus,
  UserMinus,
  UserX,
  Play
} from 'lucide-vue-next'
import { formatTime, getInitials, truncate } from '@/lib/utils'

const route = useRoute()
const router = useRouter()
const contactsStore = useContactsStore()
const authStore = useAuthStore()
const usersStore = useUsersStore()
const transfersStore = useTransfersStore()

const messageInput = ref('')
const messagesEndRef = ref<HTMLElement | null>(null)
const isSending = ref(false)
const isAssignDialogOpen = ref(false)
const isTransferring = ref(false)
const isResuming = ref(false)

const contactId = computed(() => route.params.contactId as string | undefined)

// Get active transfer for current contact from the store (reactive)
const activeTransfer = computed(() => {
  if (!contactsStore.currentContact) return null
  return transfersStore.getActiveTransferForContact(contactsStore.currentContact.id)
})

const activeTransferId = computed(() => activeTransfer.value?.id || null)

// Check if current user can assign contacts (admin or manager only)
const canAssignContacts = computed(() => {
  // Try store first, then fallback to localStorage
  let role = authStore.userRole
  if (!role || role === 'agent') {
    try {
      const storedUser = localStorage.getItem('user')
      if (storedUser) {
        const user = JSON.parse(storedUser)
        role = user.role
      }
    } catch {
      // ignore
    }
  }
  return role === 'admin' || role === 'manager'
})

// Get list of users for assignment
const assignableUsers = computed(() => {
  return usersStore.users.filter(u => u.is_active)
})

// Initialize WebSocket connection
function initWebSocket() {
  const token = localStorage.getItem('auth_token')
  if (token) {
    wsService.connect(token)
  }
}

// Fetch contacts on mount and connect WebSocket
onMounted(async () => {
  // Ensure auth session is restored
  if (!authStore.isAuthenticated) {
    authStore.restoreSession()
  }

  await contactsStore.fetchContacts()
  initWebSocket()

  // Fetch transfers to track active transfers
  transfersStore.fetchTransfers({ status: 'active' })

  // Fetch users if can assign contacts
  if (canAssignContacts.value) {
    usersStore.fetchUsers().catch(() => {
      // Silently fail if user list can't be loaded
    })
  }

  if (contactId.value) {
    await selectContact(contactId.value)
  }
})

onUnmounted(() => {
  wsService.setCurrentContact(null)
  // Clear current contact when leaving chat view so notifications work on other pages
  contactsStore.setCurrentContact(null)
})

// Watch for route changes
watch(contactId, async (newId) => {
  if (newId) {
    await selectContact(newId)
  } else {
    wsService.setCurrentContact(null)
    contactsStore.setCurrentContact(null)
    contactsStore.clearMessages()
  }
})

async function selectContact(id: string) {
  const contact = contactsStore.contacts.find(c => c.id === id)
  if (contact) {
    contactsStore.setCurrentContact(contact)
    await contactsStore.fetchMessages(id)
    scrollToBottom()
    // Tell WebSocket server which contact we're viewing
    wsService.setCurrentContact(id)
  }
}

// Watch for new messages to auto-scroll
watch(() => contactsStore.messages.length, () => {
  scrollToBottom()
})

function handleContactClick(contact: Contact) {
  router.push(`/chat/${contact.id}`)
}

async function sendMessage() {
  if (!messageInput.value.trim() || !contactsStore.currentContact) return

  isSending.value = true
  try {
    await contactsStore.sendMessage(
      contactsStore.currentContact.id,
      'text',
      { body: messageInput.value }
    )
    messageInput.value = ''
    await nextTick()
    scrollToBottom()
  } catch (error) {
    toast.error('Failed to send message')
  } finally {
    isSending.value = false
  }
}

async function assignContactToUser(userId: string | null) {
  if (!contactsStore.currentContact) return

  try {
    await contactsService.assign(contactsStore.currentContact.id, userId)
    toast.success(userId ? 'Contact assigned successfully' : 'Contact unassigned')
    // Refresh contacts list
    await contactsStore.fetchContacts()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to assign contact'
    toast.error(message)
  }
}

async function transferToAgent() {
  if (!contactsStore.currentContact) return

  isTransferring.value = true
  try {
    await chatbotService.createTransfer({
      contact_id: contactsStore.currentContact.id,
      whatsapp_account: contactsStore.currentContact.whatsapp_account,
      source: 'manual'
    })
    toast.success('Contact transferred to agent queue', {
      description: 'Chatbot is now paused for this contact'
    })
    // Refresh transfers store (WebSocket will also update, but this ensures immediate sync)
    await transfersStore.fetchTransfers({ status: 'active' })
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to transfer contact'
    toast.error(message)
  } finally {
    isTransferring.value = false
  }
}

async function resumeChatbot() {
  if (!activeTransferId.value) return

  const currentContactId = contactsStore.currentContact?.id
  isResuming.value = true
  try {
    await chatbotService.resumeTransfer(activeTransferId.value)
    toast.success('Chatbot resumed', {
      description: 'The contact will now receive automated responses'
    })
    // Refresh transfers store to update UI
    await transfersStore.fetchTransfers({ status: 'active' })
    // Refresh contacts list (assignment may have changed)
    await contactsStore.fetchContacts()

    // Check if current contact is still in the list (may have been unassigned)
    if (currentContactId) {
      const stillExists = contactsStore.contacts.some(c => c.id === currentContactId)
      if (!stillExists) {
        // Contact no longer visible to this user, navigate away
        contactsStore.setCurrentContact(null)
        contactsStore.clearMessages()
        router.push('/chat')
      }
    }
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to resume chatbot'
    toast.error(message)
  } finally {
    isResuming.value = false
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (messagesEndRef.value) {
      messagesEndRef.value.scrollIntoView({ behavior: 'smooth' })
    }
  })
}

function getMessageStatusIcon(status: string) {
  switch (status) {
    case 'sent':
      return Check
    case 'delivered':
      return CheckCheck
    case 'read':
      return CheckCheck
    case 'failed':
      return AlertCircle
    default:
      return Clock
  }
}

function getMessageStatusClass(status: string) {
  switch (status) {
    case 'read':
      return 'text-blue-500'
    case 'failed':
      return 'text-destructive'
    default:
      return 'text-muted-foreground'
  }
}

function formatMessageTime(dateStr: string) {
  const date = new Date(dateStr)
  return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}

function formatContactTime(dateStr?: string) {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - date.getTime()) / 86400000)

  if (diffDays === 0) {
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  } else if (diffDays === 1) {
    return 'Yesterday'
  } else if (diffDays < 7) {
    return date.toLocaleDateString('en-US', { weekday: 'short' })
  }
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

function getMessageContent(message: Message): string {
  if (message.message_type === 'text') {
    return message.content?.body || ''
  }
  if (message.message_type === 'interactive') {
    // Interactive messages store body text in content (string) or content.body or interactive_data.body
    if (typeof message.content === 'string') {
      return message.content
    }
    if (message.interactive_data?.body) {
      return message.interactive_data.body
    }
    return message.content?.body || '[Interactive Message]'
  }
  if (message.message_type === 'image') {
    return '[Image]'
  }
  if (message.message_type === 'document') {
    return '[Document]'
  }
  if (message.message_type === 'template') {
    return '[Template Message]'
  }
  return '[Message]'
}

function getInteractiveButtons(message: Message): Array<{ id: string; title: string }> {
  if (message.message_type !== 'interactive' || !message.interactive_data) {
    return []
  }
  // Handle both "buttons" (<=3) and "rows" (>3 list format)
  const items = message.interactive_data.buttons || message.interactive_data.rows
  if (!items || !Array.isArray(items)) {
    return []
  }
  return items.map((btn: any) => ({
    id: btn.reply?.id || btn.id || '',
    title: btn.reply?.title || btn.title || ''
  }))
}
</script>

<template>
  <div class="flex h-full">
    <!-- Contacts List -->
    <div class="w-80 border-r flex flex-col bg-card">
      <!-- Search Header -->
      <div class="p-4 border-b">
        <div class="relative">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            v-model="contactsStore.searchQuery"
            placeholder="Search contacts..."
            class="pl-9"
          />
        </div>
      </div>

      <!-- Contacts -->
      <ScrollArea class="flex-1">
        <div class="py-2">
          <div
            v-for="contact in contactsStore.sortedContacts"
            :key="contact.id"
            :class="[
              'flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-accent transition-colors',
              contactsStore.currentContact?.id === contact.id && 'bg-accent'
            ]"
            @click="handleContactClick(contact)"
          >
            <Avatar class="h-12 w-12">
              <AvatarImage :src="contact.avatar_url" />
              <AvatarFallback>
                {{ getInitials(contact.name || contact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between">
                <p class="font-medium truncate">
                  {{ contact.name || contact.phone_number }}
                </p>
                <span class="text-xs text-muted-foreground">
                  {{ formatContactTime(contact.last_message_at) }}
                </span>
              </div>
              <div class="flex items-center justify-between mt-0.5">
                <p class="text-sm text-muted-foreground truncate">
                  {{ contact.profile_name || contact.phone_number }}
                </p>
                <Badge v-if="contact.unread_count > 0" class="ml-2">
                  {{ contact.unread_count }}
                </Badge>
              </div>
            </div>
          </div>

          <div v-if="contactsStore.sortedContacts.length === 0" class="p-4 text-center text-muted-foreground">
            <User class="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p>No contacts found</p>
          </div>
        </div>
      </ScrollArea>
    </div>

    <!-- Chat Area -->
    <div class="flex-1 flex flex-col">
      <!-- No Contact Selected -->
      <div
        v-if="!contactsStore.currentContact"
        class="flex-1 flex items-center justify-center text-muted-foreground"
      >
        <div class="text-center">
          <div class="h-16 w-16 rounded-full bg-muted flex items-center justify-center mx-auto mb-4">
            <Send class="h-8 w-8" />
          </div>
          <h3 class="font-medium text-lg mb-1">Select a conversation</h3>
          <p class="text-sm">Choose a contact to start chatting</p>
        </div>
      </div>

      <!-- Chat Interface -->
      <template v-else>
        <!-- Chat Header -->
        <div class="h-16 px-4 border-b flex items-center justify-between bg-card">
          <div class="flex items-center gap-3">
            <Avatar class="h-10 w-10">
              <AvatarImage :src="contactsStore.currentContact.avatar_url" />
              <AvatarFallback>
                {{ getInitials(contactsStore.currentContact.name || contactsStore.currentContact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div>
              <div class="flex items-center gap-2">
                <p class="font-medium">
                  {{ contactsStore.currentContact.name || contactsStore.currentContact.phone_number }}
                </p>
                <Badge v-if="activeTransferId" variant="outline" class="text-xs border-orange-500 text-orange-500">
                  Chatbot Paused
                </Badge>
              </div>
              <p class="text-xs text-muted-foreground">
                {{ contactsStore.currentContact.phone_number }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <Tooltip v-if="canAssignContacts">
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon" @click="isAssignDialogOpen = true">
                  <UserPlus class="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Assign to agent</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon">
                  <Phone class="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Voice call</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon">
                  <Video class="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Video call</TooltipContent>
            </Tooltip>
            <Tooltip v-if="activeTransferId">
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon" :disabled="isResuming" @click="resumeChatbot">
                  <Play class="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Resume Chatbot</TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon">
                  <MoreVertical class="h-5 w-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>Contact Options</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem v-if="canAssignContacts" @click="isAssignDialogOpen = true">
                  <UserPlus class="mr-2 h-4 w-4" />
                  <span>Assign to agent</span>
                </DropdownMenuItem>
                <DropdownMenuItem v-if="!activeTransferId" @click="transferToAgent" :disabled="isTransferring">
                  <UserX class="mr-2 h-4 w-4" />
                  <span>Transfer to Agent</span>
                </DropdownMenuItem>
                <DropdownMenuItem v-if="activeTransferId" @click="resumeChatbot" :disabled="isResuming">
                  <Play class="mr-2 h-4 w-4" />
                  <span>Resume Chatbot</span>
                </DropdownMenuItem>
                <DropdownMenuItem disabled>
                  <User class="mr-2 h-4 w-4" />
                  <span>View contact details</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <!-- Messages -->
        <ScrollArea class="flex-1 p-4">
          <div class="space-y-4">
            <div
              v-for="message in contactsStore.messages"
              :key="message.id"
              :class="[
                'flex',
                message.direction === 'outgoing' ? 'justify-end' : 'justify-start'
              ]"
            >
              <div
                :class="[
                  'chat-bubble',
                  message.direction === 'outgoing' ? 'chat-bubble-outgoing' : 'chat-bubble-incoming'
                ]"
              >
                <p class="whitespace-pre-wrap break-words">{{ getMessageContent(message) }}</p>
                <!-- Interactive buttons -->
                <div
                  v-if="getInteractiveButtons(message).length > 0"
                  class="mt-2 space-y-1"
                >
                  <div
                    v-for="btn in getInteractiveButtons(message)"
                    :key="btn.id"
                    class="px-3 py-1.5 text-sm text-center border rounded-lg bg-background/50"
                  >
                    {{ btn.title }}
                  </div>
                </div>
                <div
                  :class="[
                    'chat-bubble-time flex items-center gap-1',
                    message.direction === 'outgoing' ? 'justify-end' : 'justify-start'
                  ]"
                >
                  <span>{{ formatMessageTime(message.created_at) }}</span>
                  <component
                    v-if="message.direction === 'outgoing'"
                    :is="getMessageStatusIcon(message.status)"
                    :class="['h-3 w-3', getMessageStatusClass(message.status)]"
                  />
                </div>
              </div>
            </div>
            <div ref="messagesEndRef" />
          </div>
        </ScrollArea>

        <!-- Message Input -->
        <div class="p-4 border-t bg-card">
          <form @submit.prevent="sendMessage" class="flex items-end gap-2">
            <div class="flex gap-1">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button type="button" variant="ghost" size="icon">
                    <Smile class="h-5 w-5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Emoji</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button type="button" variant="ghost" size="icon">
                    <Paperclip class="h-5 w-5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Attach file</TooltipContent>
              </Tooltip>
            </div>
            <Textarea
              v-model="messageInput"
              placeholder="Type a message..."
              class="flex-1 min-h-[40px] max-h-[120px] resize-none"
              :rows="1"
              @keydown.enter.exact.prevent="sendMessage"
            />
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  type="submit"
                  size="icon"
                  :disabled="!messageInput.trim() || isSending"
                >
                  <Send class="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Send message</TooltipContent>
            </Tooltip>
          </form>
        </div>
      </template>
    </div>

    <!-- Assign Contact Dialog -->
    <Dialog v-model:open="isAssignDialogOpen">
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>Assign Contact</DialogTitle>
          <DialogDescription>
            Select a team member to assign this contact to.
          </DialogDescription>
        </DialogHeader>
        <div class="py-4 space-y-2">
          <Button
            variant="outline"
            class="w-full justify-start"
            @click="assignContactToUser(null); isAssignDialogOpen = false"
          >
            <UserMinus class="mr-2 h-4 w-4" />
            Unassign
          </Button>
          <Separator />
          <Button
            v-for="user in assignableUsers"
            :key="user.id"
            variant="ghost"
            class="w-full justify-start"
            @click="assignContactToUser(user.id); isAssignDialogOpen = false"
          >
            <User class="mr-2 h-4 w-4" />
            <span>{{ user.full_name }}</span>
            <Badge variant="outline" class="ml-auto text-xs">
              {{ user.role }}
            </Badge>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
