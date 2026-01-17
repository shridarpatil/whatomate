<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useContactsStore, type Contact, type Message } from '@/stores/contacts'
import { useAuthStore } from '@/stores/auth'
import { useUsersStore } from '@/stores/users'
import { useTransfersStore } from '@/stores/transfers'
import { wsService } from '@/services/websocket'
import { contactsService, chatbotService, messagesService, customActionsService, type CustomAction, type ActionResult } from '@/services/api'
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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import EmojiPicker from 'vue3-emoji-picker'
import 'vue3-emoji-picker/css'
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
  Check,
  CheckCheck,
  Clock,
  AlertCircle,
  User,
  UserPlus,
  UserMinus,
  UserX,
  Play,
  Reply,
  X,
  SmilePlus,
  MapPin,
  ExternalLink,
  Loader2,
  Zap,
  Ticket,
  BarChart,
  Link,
  Mail,
  Globe,
  Code,
  RotateCw
} from 'lucide-vue-next'
import { formatTime, getInitials, truncate } from '@/lib/utils'
import { useColorMode } from '@/composables/useColorMode'
import CannedResponsePicker from '@/components/chat/CannedResponsePicker.vue'
import ContactInfoPanel from '@/components/chat/ContactInfoPanel.vue'
import { Info } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const contactsStore = useContactsStore()
const authStore = useAuthStore()
const usersStore = useUsersStore()
const transfersStore = useTransfersStore()
const { isDark } = useColorMode()

const messageInput = ref('')
const messagesEndRef = ref<HTMLElement | null>(null)
const messagesScrollAreaRef = ref<InstanceType<typeof ScrollArea> | null>(null)
const messageInputRef = ref<InstanceType<typeof Textarea> | null>(null)
const isSending = ref(false)
const isAssignDialogOpen = ref(false)
const isTransferring = ref(false)
const isResuming = ref(false)
const isInfoPanelOpen = ref(false)
const contactSessionData = ref<any>(null)

// File upload state
const fileInputRef = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const filePreviewUrl = ref<string | null>(null)
const isMediaDialogOpen = ref(false)
const mediaCaption = ref('')
const isUploadingMedia = ref(false)

// Cache for media blob URLs (message_id -> blob URL)
const mediaBlobUrls = ref<Record<string, string>>({})
const mediaLoadingStates = ref<Record<string, boolean>>({})

// Canned responses slash command state
const cannedPickerOpen = ref(false)
const cannedSearchQuery = ref('')

// Sticky date header state
const stickyDate = ref('')
const showStickyDate = ref(false)
let stickyDateTimeout: ReturnType<typeof setTimeout> | null = null

// Emoji picker state
const emojiPickerOpen = ref(false)

// Custom actions state
const customActions = ref<CustomAction[]>([])
const executingActionId = ref<string | null>(null)

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
        role = user.role?.name || user.role // Support both old and new format
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

// Icon mapping for custom actions
const actionIconMap: Record<string, any> = {
  'ticket': Ticket,
  'user': User,
  'bar-chart': BarChart,
  'link': Link,
  'phone': Phone,
  'mail': Mail,
  'file-text': FileText,
  'external-link': ExternalLink,
  'zap': Zap,
  'globe': Globe,
  'code': Code
}

function getActionIcon(iconName: string) {
  return actionIconMap[iconName] || Zap
}

async function fetchCustomActions() {
  try {
    const response = await customActionsService.list()
    const data = response.data.data || response.data
    customActions.value = (data.custom_actions || []).filter((a: CustomAction) => a.is_active)
  } catch (error) {
    // Silently fail - custom actions are optional
    console.error('Failed to fetch custom actions:', error)
  }
}

async function executeCustomAction(action: CustomAction) {
  if (!contactsStore.currentContact || executingActionId.value) return

  executingActionId.value = action.id
  try {
    const response = await customActionsService.execute(action.id, contactsStore.currentContact.id)
    let result: ActionResult = response.data.data || response.data

    // Handle JavaScript action - execute code in frontend
    if (result.data?.code && result.data?.context) {
      try {
        // Create a function from the code and execute with context
        const context = result.data.context
        const code = result.data.code
        // The code should return an object like: { toast: {...}, clipboard: '...', url: '...' }
        const fn = new Function('context', 'contact', 'user', 'organization', code)
        const jsResult = fn(context, context.contact, context.user, context.organization)

        // Merge JS result into action result
        if (jsResult) {
          if (jsResult.toast) result.toast = jsResult.toast
          if (jsResult.clipboard) result.clipboard = jsResult.clipboard
          if (jsResult.url) result.redirect_url = jsResult.url
          if (jsResult.message) result.message = jsResult.message
        }
      } catch (jsError: any) {
        console.error('JavaScript action error:', jsError)
        toast.error('JavaScript error: ' + jsError.message)
        return
      }
    }

    // Handle different result types
    if (result.redirect_url) {
      // Open URL action result - prepend base path for relative URLs
      let redirectUrl = result.redirect_url
      if (redirectUrl.startsWith('/api/')) {
        const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
        redirectUrl = basePath + redirectUrl
      }
      window.open(redirectUrl, '_blank')
    }

    if (result.clipboard) {
      // Copy to clipboard
      await navigator.clipboard.writeText(result.clipboard)
      toast.success('Copied to clipboard')
    }

    if (result.toast) {
      // Show toast notification
      if (result.toast.type === 'success') {
        toast.success(result.toast.message)
      } else if (result.toast.type === 'error') {
        toast.error(result.toast.message)
      } else {
        toast.info(result.toast.message)
      }
    } else if (result.success && !result.redirect_url && !result.clipboard) {
      // Default success message
      toast.success(result.message || 'Action executed successfully')
    } else if (!result.success) {
      toast.error(result.message || 'Action failed')
    }
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to execute action'
    toast.error(message)
  } finally {
    executingActionId.value = null
  }
}

// Search state for assignment dialog
const assignSearchQuery = ref('')

// Filtered users for assignment dialog
const filteredAssignableUsers = computed(() => {
  const query = assignSearchQuery.value.toLowerCase().trim()
  if (!query) return assignableUsers.value
  return assignableUsers.value.filter(u =>
    u.full_name.toLowerCase().includes(query) ||
    u.email.toLowerCase().includes(query)
  )
})

// Fetch contacts on mount (WebSocket is connected in AppLayout)
onMounted(async () => {
  // Ensure auth session is restored
  if (!authStore.isAuthenticated) {
    authStore.restoreSession()
  }

  await contactsStore.fetchContacts()

  // Fetch transfers to track active transfers
  transfersStore.fetchTransfers({ status: 'active' })

  // Fetch users if can assign contacts
  if (canAssignContacts.value) {
    usersStore.fetchUsers().catch(() => {
      // Silently fail if user list can't be loaded
    })
  }

  // Fetch custom actions for admins/managers
  if (canAssignContacts.value) {
    fetchCustomActions()
  }

  if (contactId.value) {
    await selectContact(contactId.value)
  }
})

onUnmounted(() => {
  wsService.setCurrentContact(null)
  // Clear current contact when leaving chat view so notifications work on other pages
  contactsStore.setCurrentContact(null)
  // Clean up blob URLs to prevent memory leaks
  Object.values(mediaBlobUrls.value).forEach(url => {
    URL.revokeObjectURL(url)
  })
  mediaBlobUrls.value = {}
  // Remove scroll listener
  removeScrollListener()
  // Clear sticky date timeout
  if (stickyDateTimeout) clearTimeout(stickyDateTimeout)
})

// Infinite scroll for loading older messages
let scrollViewport: HTMLElement | null = null

function setupScrollListener() {
  // Get the viewport element from ScrollArea
  const scrollArea = messagesScrollAreaRef.value?.$el
  if (scrollArea) {
    scrollViewport = scrollArea.querySelector('[data-radix-scroll-area-viewport]')
    if (scrollViewport) {
      scrollViewport.addEventListener('scroll', handleScroll)
    }
  }
}

function removeScrollListener() {
  if (scrollViewport) {
    scrollViewport.removeEventListener('scroll', handleScroll)
    scrollViewport = null
  }
}

async function handleScroll(event: Event) {
  const target = event.target as HTMLElement

  // Update sticky date header
  updateStickyDate(target)

  // Trigger load when scrolled near top (within 100px)
  if (target.scrollTop < 100 && contactsStore.hasMoreMessages && !contactsStore.isLoadingOlderMessages) {
    const currentScrollHeight = target.scrollHeight
    const currentScrollTop = target.scrollTop

    await contactsStore.fetchOlderMessages(contactsStore.currentContact!.id)

    // Preserve scroll position after prepending messages
    await nextTick()
    const newScrollHeight = target.scrollHeight
    target.scrollTop = newScrollHeight - currentScrollHeight + currentScrollTop

    // Load media for any new messages
    try {
      loadMediaForMessages()
    } catch (e) {
      console.error('Error loading media:', e)
    }
  }
}

function updateStickyDate(scrollContainer: HTMLElement) {
  // Find all date separator elements
  const dateSeparators = scrollContainer.querySelectorAll('[data-date-separator]')
  if (dateSeparators.length === 0) return

  const containerRect = scrollContainer.getBoundingClientRect()
  const containerTop = containerRect.top + 60 // Offset for sticky header position

  // Find the last date separator that's above the viewport top
  let currentDate = ''
  for (const separator of dateSeparators) {
    const rect = separator.getBoundingClientRect()
    if (rect.top < containerTop) {
      currentDate = separator.getAttribute('data-date-separator') || ''
    } else {
      break
    }
  }

  // Show sticky date if we have scrolled past at least one date separator
  if (currentDate && scrollContainer.scrollTop > 50) {
    stickyDate.value = currentDate
    showStickyDate.value = true

    // Hide after scrolling stops
    if (stickyDateTimeout) clearTimeout(stickyDateTimeout)
    stickyDateTimeout = setTimeout(() => {
      showStickyDate.value = false
    }, 1500)
  } else {
    showStickyDate.value = false
  }
}

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
    // Remove old scroll listener before switching contacts
    removeScrollListener()

    contactsStore.setCurrentContact(contact)
    await contactsStore.fetchMessages(id)
    // Tell WebSocket server which contact we're viewing
    wsService.setCurrentContact(id)
    // Wait for DOM to render messages before scrolling
    await nextTick()
    // Load media for messages after messages are fetched
    try {
      loadMediaForMessages()
    } catch (e) {
      console.error('Error loading media:', e)
    }
    // Scroll after a brief delay to ensure content is rendered (instant on initial load)
    setTimeout(() => {
      scrollToBottom(true)
      // Setup scroll listener for infinite scroll after initial scroll
      setupScrollListener()
    }, 50)

    // Fetch session data and auto-open panel if configured
    try {
      const response = await contactsService.getSessionData(id)
      contactSessionData.value = response.data.data || response.data
      if (contactSessionData.value?.panel_config?.sections?.length > 0) {
        isInfoPanelOpen.value = true
      }
    } catch {
      contactSessionData.value = null
    }
  }
}

// Watch for new messages to auto-scroll and load media
watch(() => contactsStore.messages.length, () => {
  scrollToBottom()
  try {
    loadMediaForMessages()
  } catch (e) {
    console.error('Error loading media:', e)
  }
})

// Watch for messages changes to load media
watch(() => contactsStore.messages, () => {
  try {
    loadMediaForMessages()
  } catch (e) {
    console.error('Error loading media:', e)
  }
}, { deep: true })

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
      { body: messageInput.value },
      contactsStore.replyingTo?.id
    )
    messageInput.value = ''
    contactsStore.clearReplyingTo()
    resetTextareaHeight()
    await nextTick()
    scrollToBottom()
  } catch (error) {
    toast.error('Failed to send message')
  } finally {
    isSending.value = false
  }
}

const retryingMessageId = ref<string | null>(null)

async function retryMessage(message: Message) {
  if (!contactsStore.currentContact || retryingMessageId.value) return

  retryingMessageId.value = message.id
  try {
    // Get the message content based on type
    const content = message.content || {}

    await contactsStore.sendMessage(
      contactsStore.currentContact.id,
      message.message_type,
      content
    )

    // Remove the failed message from the list after successful retry
    const messages = contactsStore.messages.get(contactsStore.currentContact.id)
    if (messages) {
      const index = messages.findIndex(m => m.id === message.id)
      if (index !== -1) {
        messages.splice(index, 1)
      }
    }

    toast.success('Message sent successfully')
  } catch (error) {
    toast.error('Failed to retry message')
  } finally {
    retryingMessageId.value = null
  }
}

function autoResizeTextarea() {
  const textarea = messageInputRef.value?.$el as HTMLTextAreaElement | undefined
  if (!textarea) return
  textarea.style.height = 'auto'
  textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px'
}

function resetTextareaHeight() {
  const textarea = messageInputRef.value?.$el as HTMLTextAreaElement | undefined
  if (!textarea) return
  textarea.style.height = 'auto'
}

function getReplyPreviewContent(message: Message): string {
  if (!message.reply_to_message) return ''
  const reply = message.reply_to_message
  if (reply.message_type === 'text') {
    const body = reply.content?.body || ''
    return body.length > 50 ? body.substring(0, 50) + '...' : body
  }
  if (reply.message_type === 'button_reply') {
    const body = typeof reply.content === 'string' ? reply.content : (reply.content?.body || '')
    return body.length > 50 ? body.substring(0, 50) + '...' : body
  }
  if (reply.message_type === 'interactive') {
    const body = typeof reply.content === 'string' ? reply.content : (reply.interactive_data?.body || reply.content?.body || '')
    return body.length > 50 ? body.substring(0, 50) + '...' : body
  }
  if (reply.message_type === 'template') {
    const body = reply.content?.body || ''
    return body.length > 50 ? body.substring(0, 50) + '...' : body
  }
  if (reply.message_type === 'image') return '[Photo]'
  if (reply.message_type === 'video') return '[Video]'
  if (reply.message_type === 'audio') return '[Audio]'
  if (reply.message_type === 'document') return '[Document]'
  if (reply.message_type === 'location') return '[Location]'
  if (reply.message_type === 'contacts') return '[Contact]'
  if (reply.message_type === 'sticker') return '[Sticker]'
  return '[Message]'
}

function scrollToMessage(messageId: string | undefined) {
  if (!messageId) return
  const messageEl = document.getElementById(`message-${messageId}`)
  if (messageEl) {
    messageEl.scrollIntoView({ behavior: 'smooth', block: 'center' })
    messageEl.classList.add('highlight-message')
    setTimeout(() => messageEl.classList.remove('highlight-message'), 2000)
  }
}

function insertCannedResponse(content: string) {
  messageInput.value = content
  cannedPickerOpen.value = false
  cannedSearchQuery.value = ''
}

function closeCannedPicker() {
  cannedPickerOpen.value = false
  cannedSearchQuery.value = ''
}

function insertEmoji(emoji: string) {
  messageInput.value += emoji
  emojiPickerOpen.value = false
}

// Reaction handling
const reactionPickerMessageId = ref<string | null>(null)
const quickReactionEmojis = ['👍', '❤️', '😂', '😮', '😢', '🙏']

async function sendReaction(messageId: string, emoji: string) {
  if (!contactsStore.currentContact) return

  try {
    const response = await messagesService.sendReaction(
      contactsStore.currentContact.id,
      messageId,
      emoji
    )
    // Update will come via WebSocket, but we can update locally for immediate feedback
    const data = response.data.data || response.data
    contactsStore.updateMessageReactions(messageId, data.reactions)
  } catch (error) {
    toast.error('Failed to send reaction')
  }
  reactionPickerMessageId.value = null
}

function toggleReactionPicker(messageId: string) {
  if (reactionPickerMessageId.value === messageId) {
    reactionPickerMessageId.value = null
  } else {
    reactionPickerMessageId.value = messageId
  }
}

function replyToMessage(message: Message) {
  contactsStore.setReplyingTo(message)
  nextTick(() => {
    messageInputRef.value?.$el?.focus()
  })
}

// Watch for slash commands in message input
watch(messageInput, (val) => {
  if (val.startsWith('/')) {
    const query = val.slice(1) // Remove the leading /
    cannedSearchQuery.value = query
    cannedPickerOpen.value = true
  } else if (cannedPickerOpen.value) {
    // Close picker if user removes the /
    cannedPickerOpen.value = false
    cannedSearchQuery.value = ''
  }
})

async function assignContactToUser(userId: string | null) {
  if (!contactsStore.currentContact) return

  try {
    await contactsService.assign(contactsStore.currentContact.id, userId)
    toast.success(userId ? 'Contact assigned successfully' : 'Contact unassigned')
    // Update current contact with new assignment
    contactsStore.currentContact = {
      ...contactsStore.currentContact,
      assigned_user_id: userId || undefined
    }
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

function scrollToBottom(instant = false) {
  nextTick(() => {
    if (messagesEndRef.value) {
      messagesEndRef.value.scrollIntoView({
        behavior: instant ? 'instant' : 'smooth',
        block: 'end'
      })
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
      return 'text-blue-400' // Bright blue for read
    case 'failed':
      return 'text-destructive'
    default:
      return 'text-muted-foreground' // Gray for sent/delivered
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

function getDateLabel(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const messageDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const diffDays = Math.floor((today.getTime() - messageDate.getTime()) / 86400000)

  if (diffDays === 0) {
    return 'Today'
  } else if (diffDays === 1) {
    return 'Yesterday'
  }
  return date.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' })
}

function shouldShowDateSeparator(index: number): boolean {
  const messages = contactsStore.messages
  if (index === 0) return true

  const currentDate = new Date(messages[index].created_at)
  const prevDate = new Date(messages[index - 1].created_at)

  return currentDate.toDateString() !== prevDate.toDateString()
}

function getMessageContent(message: Message): string {
  if (message.message_type === 'text') {
    return message.content?.body || ''
  }
  if (message.message_type === 'button_reply') {
    // Button reply stores the selected button title in content
    if (typeof message.content === 'string') {
      return message.content
    }
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
  // For media messages, return caption if available (media is displayed inline)
  if (message.message_type === 'image' || message.message_type === 'video' || message.message_type === 'sticker') {
    return message.content?.body || ''
  }
  if (message.message_type === 'audio') {
    return '' // Audio doesn't have captions
  }
  if (message.message_type === 'document') {
    return message.content?.body || ''
  }
  if (message.message_type === 'template') {
    // Show actual content if available (campaign messages), otherwise fallback
    return message.content?.body || '[Template Message]'
  }
  if (message.message_type === 'location') {
    return '' // Location is displayed as a map/card, not text
  }
  if (message.message_type === 'contacts') {
    return '' // Contacts are displayed as a card, not text
  }
  if (message.message_type === 'unsupported') {
    return '' // Displayed as a visual card, not text
  }
  return '[Message]'
}

interface LocationData {
  latitude: number
  longitude: number
  name?: string
  address?: string
}

interface ContactData {
  name: string
  phones?: string[]
}

function getLocationData(message: Message): LocationData | null {
  if (message.message_type !== 'location') return null
  try {
    // Content is stored as JSON string in body
    const body = message.content?.body || message.content
    if (typeof body === 'string') {
      return JSON.parse(body)
    }
    return body as LocationData
  } catch {
    return null
  }
}

function getContactsData(message: Message): ContactData[] {
  if (message.message_type !== 'contacts') return []
  try {
    // Content is stored as JSON string in body
    const body = message.content?.body || message.content
    if (typeof body === 'string') {
      return JSON.parse(body)
    }
    return body as ContactData[]
  } catch {
    return []
  }
}

function getGoogleMapsUrl(location: LocationData): string {
  return `https://www.google.com/maps?q=${location.latitude},${location.longitude}`
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

function isMediaMessage(message: Message): boolean {
  return ['image', 'video', 'audio', 'document'].includes(message.message_type)
}

function getMediaBlobUrl(message: Message): string {
  return mediaBlobUrls.value[message.id] || ''
}

function isMediaLoading(message: Message): boolean {
  return mediaLoadingStates.value[message.id] || false
}

async function loadMediaForMessage(message: Message) {
  if (!message.media_url || mediaBlobUrls.value[message.id] || mediaLoadingStates.value[message.id]) {
    return
  }

  mediaLoadingStates.value[message.id] = true

  try {
    const token = authStore.token
    if (!token) {
      console.error('No auth token available')
      return
    }

    const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
    const response = await fetch(`${basePath}/api/media/${message.id}`, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })

    if (!response.ok) {
      throw new Error(`Failed to load media: ${response.status}`)
    }

    const blob = await response.blob()
    const blobUrl = URL.createObjectURL(blob)
    mediaBlobUrls.value[message.id] = blobUrl
  } catch (error) {
    console.error('Failed to load media:', error, 'message_id:', message.id)
  } finally {
    mediaLoadingStates.value[message.id] = false
  }
}

// Load media for all messages that have media_url
function loadMediaForMessages() {
  try {
    for (const message of contactsStore.messages) {
      if (message.media_url && !mediaBlobUrls.value[message.id]) {
        // Fire and forget - errors are handled inside loadMediaForMessage
        loadMediaForMessage(message).catch(() => {})
      }
    }
  } catch (e) {
    console.error('Error in loadMediaForMessages:', e)
  }
}

function openMediaPreview(message: Message) {
  const url = getMediaBlobUrl(message)
  if (url) {
    window.open(url, '_blank')
  }
}

function handleImageError(event: Event) {
  const img = event.target as HTMLImageElement
  img.style.display = 'none'
}

function handleMediaError(event: Event, mediaType: string) {
  console.error(`Failed to load ${mediaType}:`, event)
}

// File upload functions
function openFilePicker() {
  fileInputRef.value?.click()
}

function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  // Validate file type
  const allowedTypes = ['image/', 'video/', 'audio/', 'application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument']
  const isAllowed = allowedTypes.some(type => file.type.startsWith(type))
  if (!isAllowed) {
    toast.error('Unsupported file type', {
      description: 'Please select an image, video, audio, or document file'
    })
    return
  }

  // Validate file size (16MB limit for WhatsApp)
  const maxSize = 16 * 1024 * 1024
  if (file.size > maxSize) {
    toast.error('File too large', {
      description: 'Maximum file size is 16MB'
    })
    return
  }

  selectedFile.value = file
  mediaCaption.value = ''

  // Create preview URL for images and videos
  if (file.type.startsWith('image/') || file.type.startsWith('video/')) {
    filePreviewUrl.value = URL.createObjectURL(file)
  } else {
    filePreviewUrl.value = null
  }

  isMediaDialogOpen.value = true

  // Reset input so same file can be selected again
  input.value = ''
}

function closeMediaDialog() {
  isMediaDialogOpen.value = false
  if (filePreviewUrl.value) {
    URL.revokeObjectURL(filePreviewUrl.value)
    filePreviewUrl.value = null
  }
  selectedFile.value = null
  mediaCaption.value = ''
}

function getMediaType(mimeType: string): string {
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  return 'document'
}

async function sendMediaMessage() {
  if (!selectedFile.value || !contactsStore.currentContact) return

  isUploadingMedia.value = true
  try {
    const formData = new FormData()
    formData.append('file', selectedFile.value)
    formData.append('contact_id', contactsStore.currentContact.id)
    formData.append('type', getMediaType(selectedFile.value.type))
    if (mediaCaption.value.trim()) {
      formData.append('caption', mediaCaption.value.trim())
    }

    const token = authStore.token
    if (!token) {
      toast.error('Authentication required')
      return
    }

    const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
    const response = await fetch(`${basePath}/api/messages/media`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`
      },
      body: formData
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.message || 'Failed to send media')
    }

    const result = await response.json()

    // Add the message to the store (addMessage has duplicate checking for WebSocket)
    if (result.data) {
      contactsStore.addMessage(result.data)
      scrollToBottom()
      // Load media for the new message
      await nextTick()
      if (result.data.media_url) {
        loadMediaForMessage(result.data)
      }
    }

    toast.success('Media sent successfully')
    closeMediaDialog()
  } catch (error: any) {
    toast.error('Failed to send media', {
      description: error.message || 'Please try again'
    })
  } finally {
    isUploadingMedia.value = false
  }
}
</script>

<template>
  <div class="flex h-full">
    <!-- Contacts List -->
    <div class="w-80 border-r flex flex-col bg-card">
      <!-- Search Header -->
      <div class="p-2 border-b">
        <div class="relative">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            v-model="contactsStore.searchQuery"
            placeholder="Search contacts..."
            class="pl-8 h-8 text-sm"
          />
        </div>
      </div>

      <!-- Contacts -->
      <ScrollArea class="flex-1">
        <div class="py-1">
          <div
            v-for="contact in contactsStore.sortedContacts"
            :key="contact.id"
            :class="[
              'flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-accent transition-colors',
              contactsStore.currentContact?.id === contact.id && 'bg-accent'
            ]"
            @click="handleContactClick(contact)"
          >
            <Avatar class="h-9 w-9">
              <AvatarImage :src="contact.avatar_url" />
              <AvatarFallback class="text-xs">
                {{ getInitials(contact.name || contact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between">
                <p class="text-sm font-medium truncate">
                  {{ contact.name || contact.phone_number }}
                </p>
                <span class="text-[11px] text-muted-foreground">
                  {{ formatContactTime(contact.last_message_at) }}
                </span>
              </div>
              <div class="flex items-center justify-between">
                <p class="text-xs text-muted-foreground truncate">
                  {{ contact.phone_number }}
                </p>
                <Badge v-if="contact.unread_count > 0" class="ml-2 h-5 text-[10px]">
                  {{ contact.unread_count }}
                </Badge>
              </div>
            </div>
          </div>

          <!-- Load more indicator -->
          <div v-if="contactsStore.hasMoreContacts" class="p-3 text-center">
            <Button
              v-if="!contactsStore.isLoadingMoreContacts"
              variant="ghost"
              size="sm"
              @click="contactsStore.loadMoreContacts()"
            >
              Load more ({{ contactsStore.sortedContacts.length }} of {{ contactsStore.contactsTotal }})
            </Button>
            <Loader2 v-else class="h-5 w-5 mx-auto animate-spin text-muted-foreground" />
          </div>

          <div v-if="contactsStore.sortedContacts.length === 0" class="p-3 text-center text-muted-foreground">
            <User class="h-6 w-6 mx-auto mb-1.5 opacity-50" />
            <p class="text-sm">No contacts found</p>
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
        <div class="h-12 px-3 border-b flex items-center justify-between bg-card">
          <div class="flex items-center gap-2">
            <Avatar class="h-8 w-8">
              <AvatarImage :src="contactsStore.currentContact.avatar_url" />
              <AvatarFallback class="text-xs">
                {{ getInitials(contactsStore.currentContact.name || contactsStore.currentContact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div>
              <div class="flex items-center gap-1.5">
                <p class="text-sm font-medium">
                  {{ contactsStore.currentContact.name || contactsStore.currentContact.phone_number }}
                </p>
                <Badge v-if="activeTransferId" variant="outline" class="text-[10px] h-5 border-orange-500 text-orange-500">
                  Paused
                </Badge>
              </div>
              <p class="text-[11px] text-muted-foreground">
                {{ contactsStore.currentContact.phone_number }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-1">
            <Tooltip v-if="canAssignContacts">
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon" class="h-8 w-8" @click="isAssignDialogOpen = true">
                  <UserPlus class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Assign to agent</TooltipContent>
            </Tooltip>
            <Tooltip v-if="activeTransferId">
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon" class="h-8 w-8" :disabled="isResuming" @click="resumeChatbot">
                  <Play class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Resume Chatbot</TooltipContent>
            </Tooltip>
            <!-- Custom Action Buttons -->
            <Tooltip v-for="action in customActions" :key="action.id">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8"
                  :disabled="executingActionId === action.id"
                  @click="executeCustomAction(action)"
                >
                  <Loader2 v-if="executingActionId === action.id" class="h-4 w-4 animate-spin" />
                  <component v-else :is="getActionIcon(action.icon)" class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ action.name }}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8"
                  :class="isInfoPanelOpen && 'bg-accent'"
                  @click="isInfoPanelOpen = !isInfoPanelOpen"
                >
                  <Info class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Contact Info</TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon" class="h-8 w-8">
                  <MoreVertical class="h-4 w-4" />
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
                <DropdownMenuItem @click="isInfoPanelOpen = !isInfoPanelOpen">
                  <Info class="mr-2 h-4 w-4" />
                  <span>{{ isInfoPanelOpen ? 'Hide contact details' : 'View contact details' }}</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <!-- Messages -->
        <div class="relative flex-1 min-h-0 overflow-hidden">
          <!-- Sticky date header -->
          <Transition name="sticky-date">
            <div
              v-if="showStickyDate"
              class="absolute top-2 left-1/2 -translate-x-1/2 z-10 px-3 py-1 bg-muted/95 backdrop-blur-sm rounded-full text-xs text-muted-foreground font-medium shadow-sm"
            >
              {{ stickyDate }}
            </div>
          </Transition>

          <ScrollArea ref="messagesScrollAreaRef" class="h-full p-3 chat-background">
            <div class="space-y-2">
              <!-- Loading indicator for older messages -->
              <div v-if="contactsStore.isLoadingOlderMessages" class="flex justify-center py-2">
                <div class="flex items-center gap-2 text-muted-foreground text-sm">
                  <div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                  <span>Loading older messages...</span>
                </div>
              </div>
              <template
                v-for="(message, index) in contactsStore.messages"
                :key="message.id"
              >
                <!-- Date separator -->
                <div
                  v-if="shouldShowDateSeparator(index)"
                  class="flex items-center justify-center my-4"
                  :data-date-separator="getDateLabel(message.created_at)"
                >
                  <div class="px-3 py-1 bg-muted rounded-full text-xs text-muted-foreground font-medium">
                  {{ getDateLabel(message.created_at) }}
                </div>
              </div>

              <!-- Message bubble -->
              <div
                :id="`message-${message.id}`"
                :class="[
                  'flex group',
                  message.direction === 'outgoing' ? 'justify-end' : 'justify-start'
                ]"
              >
              <div
                :class="[
                  'chat-bubble',
                  message.direction === 'outgoing' ? 'chat-bubble-outgoing' : 'chat-bubble-incoming'
                ]"
              >
                <!-- Reply preview (if this message is replying to another) -->
                <div
                  v-if="message.is_reply && message.reply_to_message"
                  class="reply-preview mb-2 p-2 rounded-lg cursor-pointer text-xs"
                  @click="scrollToMessage(message.reply_to_message_id)"
                >
                  <p class="font-medium opacity-70">
                    {{ message.reply_to_message.direction === 'incoming' ? (contactsStore.currentContact?.profile_name || contactsStore.currentContact?.name || 'Customer') : 'You' }}
                  </p>
                  <p class="opacity-60 truncate">
                    {{ getReplyPreviewContent(message) }}
                  </p>
                </div>
                <!-- Image message -->
                <div v-if="message.message_type === 'image' && message.media_url" class="mb-2">
                  <div v-if="isMediaLoading(message)" class="w-[200px] h-[150px] bg-muted rounded-lg animate-pulse flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">Loading...</span>
                  </div>
                  <img
                    v-else-if="getMediaBlobUrl(message)"
                    :src="getMediaBlobUrl(message)"
                    :alt="message.content?.body || 'Image'"
                    class="max-w-[280px] max-h-[300px] rounded-lg cursor-pointer object-cover"
                    @click="openMediaPreview(message)"
                    @error="handleImageError($event)"
                  />
                  <div v-else class="w-[200px] h-[150px] bg-muted rounded-lg flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">[Image]</span>
                  </div>
                </div>
                <!-- Sticker message -->
                <div v-else-if="message.message_type === 'sticker' && message.media_url" class="mb-2">
                  <div v-if="isMediaLoading(message)" class="w-[128px] h-[128px] bg-muted rounded-lg animate-pulse flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">Loading...</span>
                  </div>
                  <img
                    v-else-if="getMediaBlobUrl(message)"
                    :src="getMediaBlobUrl(message)"
                    alt="Sticker"
                    class="max-w-[128px] max-h-[128px] cursor-pointer"
                    @click="openMediaPreview(message)"
                    @error="handleImageError($event)"
                  />
                  <div v-else class="w-[128px] h-[128px] bg-muted rounded-lg flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">[Sticker]</span>
                  </div>
                </div>
                <!-- Video message -->
                <div v-else-if="message.message_type === 'video' && message.media_url" class="mb-2">
                  <div v-if="isMediaLoading(message)" class="w-[200px] h-[150px] bg-muted rounded-lg animate-pulse flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">Loading...</span>
                  </div>
                  <video
                    v-else-if="getMediaBlobUrl(message)"
                    :src="getMediaBlobUrl(message)"
                    controls
                    class="max-w-[280px] max-h-[300px] rounded-lg"
                    @error="handleMediaError($event, 'video')"
                  />
                  <div v-else class="w-[200px] h-[150px] bg-muted rounded-lg flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">[Video]</span>
                  </div>
                </div>
                <!-- Audio message -->
                <div v-else-if="message.message_type === 'audio' && message.media_url" class="mb-2">
                  <div v-if="isMediaLoading(message)" class="w-[200px] h-[40px] bg-muted rounded-lg animate-pulse"></div>
                  <audio
                    v-else-if="getMediaBlobUrl(message)"
                    :src="getMediaBlobUrl(message)"
                    controls
                    class="max-w-[280px]"
                    @error="handleMediaError($event, 'audio')"
                  />
                  <div v-else class="text-muted-foreground text-sm">[Audio]</div>
                </div>
                <!-- Document message -->
                <div v-else-if="message.message_type === 'document' && message.media_url" class="mb-2">
                  <a
                    v-if="getMediaBlobUrl(message)"
                    :href="getMediaBlobUrl(message)"
                    :download="message.media_filename || 'document'"
                    class="flex items-center gap-2 px-3 py-2 bg-background/50 rounded-lg hover:bg-background/80 transition-colors"
                  >
                    <FileText class="h-5 w-5 text-muted-foreground" />
                    <span class="text-sm truncate max-w-[200px]">
                      {{ message.media_filename || 'Document' }}
                    </span>
                  </a>
                  <div v-else-if="isMediaLoading(message)" class="flex items-center gap-2 px-3 py-2 bg-background/50 rounded-lg">
                    <FileText class="h-5 w-5 text-muted-foreground" />
                    <span class="text-sm text-muted-foreground">Loading...</span>
                  </div>
                  <div v-else class="flex items-center gap-2 px-3 py-2 bg-background/50 rounded-lg">
                    <FileText class="h-5 w-5 text-muted-foreground" />
                    <span class="text-sm text-muted-foreground">[Document]</span>
                  </div>
                </div>
                <!-- Location message -->
                <div v-else-if="message.message_type === 'location' && getLocationData(message)" class="mb-2">
                  <a
                    :href="getGoogleMapsUrl(getLocationData(message)!)"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="flex items-center gap-3 px-3 py-3 bg-background/50 rounded-lg hover:bg-background/80 transition-colors"
                  >
                    <div class="h-10 w-10 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center shrink-0">
                      <MapPin class="h-5 w-5 text-red-500" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <p v-if="getLocationData(message)?.name" class="text-sm font-medium truncate">
                        {{ getLocationData(message)?.name }}
                      </p>
                      <p v-else class="text-sm font-medium">Location</p>
                      <p v-if="getLocationData(message)?.address" class="text-xs text-muted-foreground truncate">
                        {{ getLocationData(message)?.address }}
                      </p>
                      <p class="text-xs text-muted-foreground">
                        {{ getLocationData(message)?.latitude.toFixed(6) }}, {{ getLocationData(message)?.longitude.toFixed(6) }}
                      </p>
                    </div>
                    <ExternalLink class="h-4 w-4 text-muted-foreground shrink-0" />
                  </a>
                </div>
                <!-- Contacts message -->
                <div v-else-if="message.message_type === 'contacts' && getContactsData(message).length > 0" class="mb-2 space-y-2">
                  <div
                    v-for="(contact, idx) in getContactsData(message)"
                    :key="idx"
                    class="flex items-center gap-3 px-3 py-2 bg-background/50 rounded-lg"
                  >
                    <div class="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
                      <User class="h-5 w-5 text-primary" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="text-sm font-medium truncate">{{ contact.name }}</p>
                      <div v-if="contact.phones?.length" class="flex items-center gap-1 text-xs text-muted-foreground">
                        <Phone class="h-3 w-3" />
                        <span class="truncate">{{ contact.phones.join(', ') }}</span>
                      </div>
                    </div>
                  </div>
                </div>
                <!-- Unsupported message -->
                <div v-else-if="message.message_type === 'unsupported'" class="mb-2">
                  <div class="flex items-center gap-2 px-3 py-2 bg-muted/50 rounded-lg text-muted-foreground">
                    <AlertCircle class="h-4 w-4 shrink-0" />
                    <span class="text-sm italic">This message type is not supported</span>
                  </div>
                </div>
                <!-- Button reply - WhatsApp style -->
                <div v-if="message.message_type === 'button_reply'" class="button-reply-bubble">
                  <span class="whitespace-pre-wrap break-words">{{ getMessageContent(message) }}</span>
                  <span class="chat-bubble-time"><span>{{ formatMessageTime(message.created_at) }}</span></span>
                </div>
                <!-- Text content (for text messages or captions) -->
                <span v-else-if="getMessageContent(message)" class="whitespace-pre-wrap break-words">{{ getMessageContent(message) }}<span class="chat-bubble-time"><span>{{ formatMessageTime(message.created_at) }}</span><component v-if="message.direction === 'outgoing'" :is="getMessageStatusIcon(message.status)" :class="['h-5 w-5 status-icon', getMessageStatusClass(message.status)]" /></span></span>
                <!-- Fallback for media without URL -->
                <span v-else-if="isMediaMessage(message) && !message.media_url" class="text-muted-foreground italic">[{{ message.message_type.charAt(0).toUpperCase() + message.message_type.slice(1) }}]<span class="chat-bubble-time"><span>{{ formatMessageTime(message.created_at) }}</span><component v-if="message.direction === 'outgoing'" :is="getMessageStatusIcon(message.status)" :class="['h-5 w-5 status-icon', getMessageStatusClass(message.status)]" /></span></span>
                <!-- Interactive buttons - WhatsApp style -->
                <div
                  v-if="getInteractiveButtons(message).length > 0"
                  class="interactive-buttons mt-2 -mx-2 -mb-1.5 border-t border-black/10"
                >
                  <div
                    v-for="(btn, index) in getInteractiveButtons(message)"
                    :key="btn.id"
                    :class="[
                      'py-2 text-sm text-center text-[#00a5f4] font-medium cursor-pointer hover:bg-black/5',
                      index > 0 && 'border-t border-black/10'
                    ]"
                  >
                    {{ btn.title }}
                  </div>
                </div>
                <!-- Time for messages without text content -->
                <span v-if="!getMessageContent(message) && !(isMediaMessage(message) && !message.media_url)" class="chat-bubble-time block clear-both">
                  <span>{{ formatMessageTime(message.created_at) }}</span>
                  <component
                    v-if="message.direction === 'outgoing'"
                    :is="getMessageStatusIcon(message.status)"
                    :class="['h-5 w-5 status-icon', getMessageStatusClass(message.status)]"
                  />
                </span>
                <!-- Reactions display -->
                <div
                  v-if="message.reactions && message.reactions.length > 0"
                  class="reactions-display flex flex-wrap gap-1 mt-1"
                >
                  <span
                    v-for="(reaction, idx) in message.reactions"
                    :key="idx"
                    class="reaction-badge"
                    :title="reaction.from_phone || reaction.from_user || ''"
                  >
                    {{ reaction.emoji }}
                  </span>
                </div>
                <!-- Failed message retry indicator (not for template messages) -->
                <button
                  v-if="message.status === 'failed' && message.direction === 'outgoing' && message.message_type !== 'template'"
                  class="flex items-center gap-1 mt-1 text-xs text-destructive hover:underline cursor-pointer"
                  :disabled="retryingMessageId === message.id"
                  @click="retryMessage(message)"
                >
                  <Loader2 v-if="retryingMessageId === message.id" class="h-3 w-3 animate-spin" />
                  <RotateCw v-else class="h-3 w-3" />
                  <span>{{ retryingMessageId === message.id ? 'Retrying...' : 'Failed - Tap to retry' }}</span>
                </button>
                <!-- Failed template message indicator (no retry) -->
                <span
                  v-if="message.status === 'failed' && message.direction === 'outgoing' && message.message_type === 'template'"
                  class="flex items-center gap-1 mt-1 text-xs text-destructive"
                >
                  <AlertCircle class="h-3 w-3" />
                  <span>Failed to send</span>
                </span>
              </div>
              <!-- Action buttons for incoming messages -->
              <div v-if="message.direction === 'incoming'" class="flex flex-col gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity self-center ml-1">
                <Popover :open="reactionPickerMessageId === message.id" @update:open="(open: boolean) => reactionPickerMessageId = open ? message.id : null">
                  <PopoverTrigger as-child>
                    <Button variant="ghost" size="icon" class="h-6 w-6">
                      <SmilePlus class="h-3 w-3" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent side="top" class="w-auto p-2">
                    <div class="flex gap-1">
                      <button
                        v-for="emoji in quickReactionEmojis"
                        :key="emoji"
                        class="text-lg hover:bg-muted p-1 rounded cursor-pointer"
                        @click="sendReaction(message.id, emoji)"
                      >
                        {{ emoji }}
                      </button>
                    </div>
                  </PopoverContent>
                </Popover>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6"
                  @click="replyToMessage(message)"
                >
                  <Reply class="h-3 w-3" />
                </Button>
              </div>
              <!-- Reply button for outgoing messages (shown on hover) -->
              <div v-if="message.direction === 'outgoing'" class="flex flex-col gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity self-center ml-1">
                <Popover :open="reactionPickerMessageId === message.id" @update:open="(open: boolean) => reactionPickerMessageId = open ? message.id : null">
                  <PopoverTrigger as-child>
                    <Button variant="ghost" size="icon" class="h-6 w-6">
                      <SmilePlus class="h-3 w-3" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent side="top" class="w-auto p-2">
                    <div class="flex gap-1">
                      <button
                        v-for="emoji in quickReactionEmojis"
                        :key="emoji"
                        class="text-lg hover:bg-muted p-1 rounded cursor-pointer"
                        @click="sendReaction(message.id, emoji)"
                      >
                        {{ emoji }}
                      </button>
                    </div>
                  </PopoverContent>
                </Popover>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6"
                  @click="replyToMessage(message)"
                >
                  <Reply class="h-3 w-3" />
                </Button>
                <Button
                  v-if="message.status === 'failed' && message.message_type !== 'template'"
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6 text-destructive hover:text-destructive"
                  :disabled="retryingMessageId === message.id"
                  @click="retryMessage(message)"
                  title="Retry sending"
                >
                  <Loader2 v-if="retryingMessageId === message.id" class="h-3 w-3 animate-spin" />
                  <RotateCw v-else class="h-3 w-3" />
                </Button>
              </div>
            </div>
            </template>
            <div ref="messagesEndRef" />
          </div>
        </ScrollArea>
        </div>

        <!-- Reply indicator -->
        <div
          v-if="contactsStore.replyingTo"
          class="px-3 py-2 border-t bg-muted/50 flex items-center justify-between"
        >
          <div class="flex-1 min-w-0">
            <p class="text-xs font-medium text-muted-foreground">
              Replying to {{ contactsStore.replyingTo.direction === 'incoming' ? (contactsStore.currentContact?.profile_name || contactsStore.currentContact?.name || 'Customer') : 'Yourself' }}
            </p>
            <p class="text-sm truncate">
              {{ getMessageContent(contactsStore.replyingTo) || '[Media]' }}
            </p>
          </div>
          <Button variant="ghost" size="icon" class="h-6 w-6 shrink-0" @click="contactsStore.clearReplyingTo">
            <X class="h-4 w-4" />
          </Button>
        </div>

        <!-- Message Input -->
        <div class="px-3 py-2 border-t bg-card">
          <form @submit.prevent="sendMessage" class="flex items-end gap-1.5">
            <div class="flex">
              <Tooltip>
                <TooltipTrigger as-child>
                  <span>
                    <Popover v-model:open="emojiPickerOpen">
                      <PopoverTrigger as-child>
                        <Button type="button" variant="ghost" size="icon" class="h-8 w-8">
                          <Smile class="h-4 w-4" />
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent side="top" align="start" class="w-auto p-0">
                        <EmojiPicker
                          :native="true"
                          :disable-skin-tones="true"
                          :theme="isDark ? 'dark' : 'light'"
                          @select="insertEmoji($event.i)"
                        />
                      </PopoverContent>
                    </Popover>
                  </span>
                </TooltipTrigger>
                <TooltipContent>Emoji</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <span>
                    <CannedResponsePicker
                      :contact="contactsStore.currentContact"
                      :external-open="cannedPickerOpen"
                      :external-search="cannedSearchQuery"
                      @select="insertCannedResponse"
                      @close="closeCannedPicker"
                    />
                  </span>
                </TooltipTrigger>
                <TooltipContent>Canned Responses</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button type="button" variant="ghost" size="icon" class="h-8 w-8" @click="openFilePicker">
                    <Paperclip class="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Attach file</TooltipContent>
              </Tooltip>
              <input
                ref="fileInputRef"
                type="file"
                accept="image/*,video/*,audio/*,.pdf,.doc,.docx"
                class="hidden"
                @change="handleFileSelect"
              />
            </div>
            <Textarea
              ref="messageInputRef"
              v-model="messageInput"
              placeholder="Type a message..."
              class="flex-1 min-h-[36px] max-h-[120px] resize-none text-sm overflow-y-auto"
              :rows="1"
              @keydown.enter.exact.prevent="sendMessage"
              @input="autoResizeTextarea"
            />
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  type="submit"
                  size="icon"
                  class="h-8 w-8"
                  :disabled="!messageInput.trim() || isSending"
                >
                  <Send class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Send message</TooltipContent>
            </Tooltip>
          </form>
        </div>
      </template>
    </div>

    <!-- Contact Info Panel -->
    <ContactInfoPanel
      v-if="contactsStore.currentContact && isInfoPanelOpen"
      :contact="contactsStore.currentContact"
      :session-data="contactSessionData"
      @close="isInfoPanelOpen = false"
    />

    <!-- Assign Contact Dialog -->
    <Dialog v-model:open="isAssignDialogOpen" @update:open="(open) => !open && (assignSearchQuery = '')">
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>Assign Contact</DialogTitle>
          <DialogDescription>
            Select a team member to assign this contact to.
          </DialogDescription>
        </DialogHeader>
        <div class="py-4 space-y-3">
          <!-- Search input -->
          <div class="relative">
            <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              v-model="assignSearchQuery"
              placeholder="Search users..."
              class="pl-9 h-9"
            />
          </div>
          <Button
            v-if="contactsStore.currentContact?.assigned_user_id"
            variant="outline"
            class="w-full justify-start"
            @click="assignContactToUser(null); isAssignDialogOpen = false"
          >
            <UserMinus class="mr-2 h-4 w-4" />
            Unassign
          </Button>
          <Separator />
          <ScrollArea class="max-h-[280px]">
            <div class="space-y-1">
              <Button
                v-for="user in filteredAssignableUsers"
                :key="user.id"
                :variant="contactsStore.currentContact?.assigned_user_id === user.id ? 'secondary' : 'ghost'"
                class="w-full justify-start"
                @click="assignContactToUser(user.id); isAssignDialogOpen = false"
              >
                <User class="mr-2 h-4 w-4" />
                <span>{{ user.full_name }}</span>
                <Check
                  v-if="contactsStore.currentContact?.assigned_user_id === user.id"
                  class="ml-auto h-4 w-4 text-primary"
                />
                <Badge v-else variant="outline" class="ml-auto text-xs">
                  {{ user.role?.name }}
                </Badge>
              </Button>
              <p v-if="filteredAssignableUsers.length === 0" class="text-sm text-muted-foreground text-center py-4">
                No users found
              </p>
            </div>
          </ScrollArea>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Media Preview Dialog -->
    <Dialog v-model:open="isMediaDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>Send Media</DialogTitle>
          <DialogDescription>
            {{ selectedFile?.name }}
          </DialogDescription>
        </DialogHeader>
        <div class="py-4 space-y-4">
          <!-- Image preview -->
          <div v-if="selectedFile?.type.startsWith('image/') && filePreviewUrl" class="flex justify-center">
            <img
              :src="filePreviewUrl"
              :alt="selectedFile.name"
              class="max-w-full max-h-[300px] rounded-lg object-contain"
            />
          </div>
          <!-- Video preview -->
          <div v-else-if="selectedFile?.type.startsWith('video/') && filePreviewUrl" class="flex justify-center">
            <video
              :src="filePreviewUrl"
              controls
              class="max-w-full max-h-[300px] rounded-lg"
            />
          </div>
          <!-- Audio preview -->
          <div v-else-if="selectedFile?.type.startsWith('audio/')" class="flex justify-center">
            <div class="flex items-center gap-3 px-4 py-3 bg-muted rounded-lg">
              <div class="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                <Paperclip class="h-5 w-5 text-primary" />
              </div>
              <div>
                <p class="font-medium text-sm">{{ selectedFile.name }}</p>
                <p class="text-xs text-muted-foreground">Audio file</p>
              </div>
            </div>
          </div>
          <!-- Document preview -->
          <div v-else-if="selectedFile" class="flex justify-center">
            <div class="flex items-center gap-3 px-4 py-3 bg-muted rounded-lg">
              <div class="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                <FileText class="h-5 w-5 text-primary" />
              </div>
              <div>
                <p class="font-medium text-sm truncate max-w-[200px]">{{ selectedFile.name }}</p>
                <p class="text-xs text-muted-foreground">
                  {{ (selectedFile.size / 1024).toFixed(1) }} KB
                </p>
              </div>
            </div>
          </div>

          <!-- Caption input (not for audio) -->
          <div v-if="selectedFile && !selectedFile.type.startsWith('audio/')">
            <Textarea
              v-model="mediaCaption"
              placeholder="Add a caption..."
              class="min-h-[60px] max-h-[100px] resize-none"
              :rows="2"
            />
          </div>

          <!-- Actions -->
          <div class="flex justify-end gap-2">
            <Button variant="outline" @click="closeMediaDialog" :disabled="isUploadingMedia">
              Cancel
            </Button>
            <Button @click="sendMediaMessage" :disabled="isUploadingMedia">
              <Send v-if="!isUploadingMedia" class="mr-2 h-4 w-4" />
              <span v-if="isUploadingMedia">Sending...</span>
              <span v-else>Send</span>
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.sticky-date-enter-active,
.sticky-date-leave-active {
  transition: opacity 0.3s ease;
}

.sticky-date-enter-from,
.sticky-date-leave-to {
  opacity: 0;
}
</style>
