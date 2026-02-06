<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick, computed, defineAsyncComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useContactsStore, type Contact, type Message } from '@/stores/contacts'
import { useAuthStore } from '@/stores/auth'
import { useUsersStore } from '@/stores/users'
import { useTransfersStore } from '@/stores/transfers'
import { wsService } from '@/services/websocket'
import { contactsService, chatbotService, messagesService, customActionsService, type CustomAction, type ActionResult } from '@/services/api'
import { useTagsStore } from '@/stores/tags'
import { TagBadge } from '@/components/ui/tag-badge'
import { getTagColorClass } from '@/lib/constants'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
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
// Lazy-load emoji picker to reduce initial bundle size
const EmojiPicker = defineAsyncComponent(() => {
  return import('vue3-emoji-picker').then(module => {
    // Import CSS when component loads
    import('vue3-emoji-picker/css')
    return module.default
  })
})
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
  RotateCw,
  Filter,
  StickyNote
} from 'lucide-vue-next'
import { getInitials, getAvatarGradient } from '@/lib/utils'
import { useColorMode } from '@/composables/useColorMode'
import { useInfiniteScroll } from '@/composables/useInfiniteScroll'
import CannedResponsePicker from '@/components/chat/CannedResponsePicker.vue'
import ContactInfoPanel from '@/components/chat/ContactInfoPanel.vue'
import ConversationNotes from '@/components/chat/ConversationNotes.vue'
import { useNotesStore } from '@/stores/notes'
import { CreateContactDialog } from '@/components/shared'
import { Info } from 'lucide-vue-next'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const contactsStore = useContactsStore()
const authStore = useAuthStore()
const usersStore = useUsersStore()
const transfersStore = useTransfersStore()
const tagsStore = useTagsStore()
const notesStore = useNotesStore()
const { isDark } = useColorMode()

const canWriteContacts = authStore.hasPermission('contacts', 'write')

const messageInput = ref('')
const messagesEndRef = ref<HTMLElement | null>(null)
const messageInputRef = ref<HTMLTextAreaElement | null>(null)
const isSending = ref(false)
const isAssignDialogOpen = ref(false)
const isTransferring = ref(false)
const isResuming = ref(false)
const isInfoPanelOpen = ref(false)
const isNotesPanelOpen = ref(false)
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

// Tags filter state
const isTagFilterOpen = ref(false)

// Add contact dialog state
const isAddContactOpen = ref(false)

function openAddContactDialog() {
  isAddContactOpen.value = true
}

async function onContactCreated(contact: any) {
  // Refresh contacts and select the new one
  await contactsStore.fetchContacts()
  if (contact?.id) {
    router.push({ name: 'chat-conversation', params: { contactId: contact.id } })
  }
}

// Infinite scroll for contacts (load more at bottom)
const contactsScroll = useInfiniteScroll({
  direction: 'bottom',
  onLoadMore: () => contactsStore.loadMoreContacts(),
  hasMore: computed(() => contactsStore.hasMoreContacts),
  isLoading: computed(() => contactsStore.isLoadingMoreContacts)
})

// Infinite scroll for messages (load older at top)
const messagesScroll = useInfiniteScroll({
  direction: 'top',
  onLoadMore: async () => {
    if (!contactsStore.currentContact) return
    await messagesScroll.preserveScrollPosition(async () => {
      await contactsStore.fetchOlderMessages(contactsStore.currentContact!.id)
      await nextTick()
      // Load media for any new messages
      try {
        loadMediaForMessages()
      } catch (e) {
        console.error('Error loading media:', e)
      }
    })
  },
  hasMore: computed(() => contactsStore.hasMoreMessages),
  isLoading: computed(() => contactsStore.isLoadingOlderMessages),
  onScroll: (event) => updateStickyDate(event.target as HTMLElement)
})

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
    const data = (response.data as any).data || response.data
    customActions.value = (data.custom_actions || []).filter((a: CustomAction) => a.is_active)
  } catch (error) {
    // Silently fail - custom actions are optional
    console.error('Failed to fetch custom actions:', error)
  }
}

function toggleTagFilter(tagName: string) {
  const index = contactsStore.selectedTags.indexOf(tagName)
  if (index === -1) {
    contactsStore.selectedTags.push(tagName)
  } else {
    contactsStore.selectedTags.splice(index, 1)
  }
  // Refetch contacts with new filter
  contactsStore.fetchContacts()
}

function clearTagFilter() {
  contactsStore.selectedTags = []
  contactsStore.fetchContacts()
}

async function executeCustomAction(action: CustomAction) {
  if (!contactsStore.currentContact || executingActionId.value) return

  executingActionId.value = action.id
  try {
    const response = await customActionsService.execute(action.id, contactsStore.currentContact.id)
    let result: ActionResult = (response.data as any).data || response.data

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
        toast.error(t('chat.jsError') + ': ' + jsError.message)
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
      toast.success(t('common.copiedToClipboard'))
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
      toast.success(result.message || t('chat.actionExecuted'))
    } else if (!result.success) {
      toast.error(result.message || t('chat.actionFailed'))
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

  // Setup infinite scroll for contacts list
  await nextTick()
  contactsScroll.setup()

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

  // Fetch available tags for filtering (if not already loaded)
  if (tagsStore.tags.length === 0) {
    tagsStore.fetchTags().catch(() => {})
  }

  if (contactId.value) {
    await selectContact(contactId.value)
  }
})

onUnmounted(() => {
  wsService.setCurrentContact(null)
  // Clear current contact when leaving chat view so notifications work on other pages
  contactsStore.setCurrentContact(null)
  notesStore.clearNotes()
  // Clean up blob URLs to prevent memory leaks
  Object.values(mediaBlobUrls.value).forEach(url => {
    URL.revokeObjectURL(url)
  })
  mediaBlobUrls.value = {}
  // Clear sticky date timeout
  if (stickyDateTimeout) clearTimeout(stickyDateTimeout)
})

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
    notesStore.clearNotes()
    await selectContact(newId)
  } else {
    wsService.setCurrentContact(null)
    contactsStore.setCurrentContact(null)
    contactsStore.clearMessages()
    notesStore.clearNotes()
  }
})

async function selectContact(id: string) {
  const contact = contactsStore.contacts.find(c => c.id === id)
  if (contact) {
    // Remove old scroll listener before switching contacts
    messagesScroll.cleanup()

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
      messagesScroll.setup()
    }, 50)

    // Fetch notes for badge count
    notesStore.fetchNotes(id)

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
    toast.error(t('chat.sendMessageFailed'))
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
    const messages = (contactsStore.messages as any).get?.(contactsStore.currentContact.id) as Message[] | undefined
    if (messages) {
      const index = messages.findIndex((m: Message) => m.id === message.id)
      if (index !== -1) {
        messages.splice(index, 1)
      }
    }

    toast.success(t('chat.messageSent'))
  } catch (error) {
    toast.error(t('chat.sendMessageFailed'))
  } finally {
    retryingMessageId.value = null
  }
}

function autoResizeTextarea() {
  const textarea = messageInputRef.value
  if (!textarea) return
  textarea.style.height = 'auto'
  textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px'
}

function resetTextareaHeight() {
  const textarea = messageInputRef.value
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
    const body = typeof reply.content === 'string' ? reply.content : ((reply as any).interactive_data?.body || reply.content?.body || '')
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
    toast.error(t('chat.reactionFailed'))
  }
  reactionPickerMessageId.value = null
}

function _toggleReactionPicker(messageId: string) {
  if (reactionPickerMessageId.value === messageId) {
    reactionPickerMessageId.value = null
  } else {
    reactionPickerMessageId.value = messageId
  }
}
void _toggleReactionPicker // Suppress unused warning

function replyToMessage(message: Message) {
  contactsStore.setReplyingTo(message)
  nextTick(() => {
    messageInputRef.value?.focus()
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
    toast.success(userId ? t('chat.contactAssigned') : t('chat.contactUnassigned'))
    // Update current contact with new assignment
    contactsStore.currentContact = {
      ...contactsStore.currentContact,
      assigned_user_id: userId || undefined
    }
    // Refresh contacts list
    await contactsStore.fetchContacts()
  } catch (error: any) {
    const message = error.response?.data?.message || t('chat.assignFailed')
    toast.error(message)
  }
}

async function transferToAgent() {
  if (!contactsStore.currentContact) return

  isTransferring.value = true
  try {
    await chatbotService.createTransfer({
      contact_id: contactsStore.currentContact.id,
      whatsapp_account: (contactsStore.currentContact as any).whatsapp_account,
      source: 'manual'
    })
    toast.success(t('chat.transferSuccess'), {
      description: t('chat.transferSuccessDesc')
    })
    // Refresh transfers store (WebSocket will also update, but this ensures immediate sync)
    await transfersStore.fetchTransfers({ status: 'active' })
  } catch (error: any) {
    const message = error.response?.data?.message || t('chat.transferFailed')
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
    toast.success(t('chat.resumeSuccess'), {
      description: t('chat.resumeSuccessDesc')
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
    const message = error.response?.data?.message || t('chat.resumeFailed')
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

interface CTAUrlData {
  type: 'cta_url'
  body: string
  button_text: string
  url: string
}

function getCTAUrlData(message: Message): CTAUrlData | null {
  if (message.message_type !== 'interactive' || !message.interactive_data) {
    return null
  }
  if (message.interactive_data.type !== 'cta_url') {
    return null
  }
  return {
    type: 'cta_url',
    body: message.interactive_data.body || '',
    button_text: (message.interactive_data as any).button_text || 'Open',
    url: (message.interactive_data as any).url || ''
  }
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
    toast.error(t('chat.unsupportedFileType'), {
      description: t('chat.unsupportedFileTypeDesc')
    })
    return
  }

  // Validate file size (16MB limit for WhatsApp)
  const maxSize = 16 * 1024 * 1024
  if (file.size > maxSize) {
    toast.error(t('chat.fileTooLarge'), {
      description: t('chat.fileTooLargeDesc')
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
      toast.error(t('errors.unauthorized'))
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

    toast.success(t('chat.mediaSent'))
    closeMediaDialog()
  } catch (error: any) {
    toast.error(t('chat.mediaFailed'), {
      description: error.message || t('chat.mediaFailedDesc')
    })
  } finally {
    isUploadingMedia.value = false
  }
}
</script>

<template>
  <div class="flex h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Contacts List -->
    <div class="w-80 border-r border-white/[0.08] light:border-gray-200 flex flex-col bg-[#0a0a0b] light:bg-white">
      <!-- Search Header -->
      <div class="p-2 border-b border-white/[0.08] light:border-gray-200">
        <div class="flex items-center gap-2">
          <div class="relative flex-1">
            <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-white/40 light:text-gray-400" />
            <Input
              v-model="contactsStore.searchQuery"
              :placeholder="$t('chat.searchContacts') + '...'"
              class="pl-8 h-8 text-sm bg-white/[0.04] border-white/[0.1] text-white placeholder:text-white/40 light:bg-gray-50 light:border-gray-200 light:text-gray-900 light:placeholder:text-gray-400"
            />
          </div>
          <!-- Add Contact -->
          <Tooltip v-if="canWriteContacts">
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon"
                class="h-8 w-8 shrink-0 text-white/40 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100"
                @click="openAddContactDialog"
              >
                <UserPlus class="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ $t('chat.addContact') }}</TooltipContent>
          </Tooltip>
          <!-- Tag Filter -->
          <Popover v-model:open="isTagFilterOpen">
            <PopoverTrigger as-child>
              <Button
                variant="ghost"
                size="icon"
                class="h-8 w-8 shrink-0 relative"
                :class="contactsStore.selectedTags.length > 0 ? 'text-emerald-400 bg-emerald-500/10' : 'text-white/40 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100'"
              >
                <Filter class="h-4 w-4" />
                <span v-if="contactsStore.selectedTags.length > 0" class="absolute -top-1 -right-1 h-4 w-4 rounded-full bg-emerald-500 text-[10px] text-white flex items-center justify-center">
                  {{ contactsStore.selectedTags.length }}
                </span>
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-56 p-2">
              <div class="space-y-2">
                <div class="flex items-center justify-between px-1">
                  <span class="text-sm font-medium">{{ $t('chat.filterByTags') }}</span>
                  <Button
                    v-if="contactsStore.selectedTags.length > 0"
                    variant="ghost"
                    size="sm"
                    class="h-6 px-2 text-xs"
                    @click="clearTagFilter"
                  >
                    Clear
                  </Button>
                </div>
                <Separator />
                <div v-if="tagsStore.tags.length === 0" class="py-2 text-center text-sm text-muted-foreground">
                  {{ $t('chat.noTagsAvailable') }}
                </div>
                <div v-else class="space-y-1 max-h-48 overflow-y-auto">
                  <button
                    v-for="tag in tagsStore.tags"
                    :key="tag.name"
                    class="w-full flex items-center gap-2 px-2 py-1.5 rounded-md text-sm hover:bg-white/[0.08] light:hover:bg-gray-100 transition-colors"
                    :class="contactsStore.selectedTags.includes(tag.name) && 'bg-white/[0.08] light:bg-gray-100'"
                    @click="toggleTagFilter(tag.name)"
                  >
                    <span :class="['w-2 h-2 rounded-full shrink-0', getTagColorClass(tag.color).split(' ')[0]]" />
                    <span class="flex-1 text-left truncate">{{ tag.name }}</span>
                    <Check
                      v-if="contactsStore.selectedTags.includes(tag.name)"
                      class="h-4 w-4 text-emerald-400 shrink-0"
                    />
                  </button>
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>
        <!-- Active tag filters -->
        <div v-if="contactsStore.selectedTags.length > 0" class="flex flex-wrap gap-1 mt-2">
          <TagBadge
            v-for="tagName in contactsStore.selectedTags"
            :key="tagName"
            :color="tagsStore.getTagByName(tagName)?.color"
            class="cursor-pointer hover:opacity-80"
            @click="toggleTagFilter(tagName)"
          >
            {{ tagName }}
            <X class="h-3 w-3 ml-1" />
          </TagBadge>
        </div>
      </div>

      <!-- Contacts -->
      <ScrollArea :ref="(el: any) => contactsScroll.scrollAreaRef.value = el" class="flex-1">
        <div class="py-1">
          <div
            v-for="contact in contactsStore.sortedContacts"
            :key="contact.id"
            :class="[
              'flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-white/[0.04] light:hover:bg-gray-50 transition-colors',
              contactsStore.currentContact?.id === contact.id && 'bg-white/[0.08] light:bg-gray-100'
            ]"
            @click="handleContactClick(contact)"
          >
            <Avatar class="h-9 w-9 ring-2 ring-white/[0.1] light:ring-gray-200">
              <AvatarImage :src="contact.avatar_url" />
              <AvatarFallback :class="'text-xs bg-gradient-to-br text-white ' + getAvatarGradient(contact.name || contact.phone_number)">
                {{ getInitials(contact.name || contact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between">
                <p class="text-sm font-medium truncate text-white light:text-gray-900">
                  {{ contact.name || contact.phone_number }}
                </p>
                <span class="text-[11px] text-white/40 light:text-gray-500">
                  {{ formatContactTime(contact.last_message_at) }}
                </span>
              </div>
              <div class="flex items-center justify-between">
                <p class="text-xs text-white/50 light:text-gray-500 truncate">
                  {{ contact.phone_number }}
                </p>
                <Badge v-if="contact.unread_count > 0" class="ml-2 h-5 text-[10px] bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700">
                  {{ contact.unread_count }}
                </Badge>
              </div>
            </div>
          </div>

          <!-- Loading indicator for infinite scroll -->
          <div v-if="contactsStore.isLoadingMoreContacts" class="p-3 text-center">
            <Loader2 class="h-5 w-5 mx-auto animate-spin text-white/40 light:text-gray-400" />
          </div>

          <div v-if="contactsStore.sortedContacts.length === 0" class="p-3 text-center text-white/40 light:text-gray-500">
            <User class="h-6 w-6 mx-auto mb-1.5 opacity-50" />
            <p class="text-sm">{{ $t('chat.noContacts') }}</p>
          </div>
        </div>
      </ScrollArea>
    </div>

    <!-- Chat Area -->
    <div class="flex-1 flex flex-col bg-[#0f0f10] light:bg-gray-50">
      <!-- No Contact Selected -->
      <div
        v-if="!contactsStore.currentContact"
        class="flex-1 flex items-center justify-center text-white/40 light:text-gray-500"
      >
        <div class="text-center">
          <div class="h-16 w-16 rounded-2xl bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center mx-auto mb-4 shadow-lg shadow-emerald-500/20">
            <Send class="h-8 w-8 text-white" />
          </div>
          <h3 class="font-medium text-lg mb-1 text-white light:text-gray-900">{{ $t('chat.selectConversation') }}</h3>
          <p class="text-sm text-white/50 light:text-gray-500">{{ $t('chat.chooseContact') }}</p>
        </div>
      </div>

      <!-- Chat Interface -->
      <template v-else>
        <!-- Chat Header -->
        <div class="h-14 px-4 border-b border-white/[0.08] light:border-gray-200 flex items-center justify-between bg-[#0f0f10] light:bg-white">
          <div class="flex items-center gap-2">
            <Avatar class="h-8 w-8 ring-2 ring-white/[0.1] light:ring-gray-200">
              <AvatarImage :src="contactsStore.currentContact.avatar_url" />
              <AvatarFallback :class="'text-xs bg-gradient-to-br text-white ' + getAvatarGradient(contactsStore.currentContact.name || contactsStore.currentContact.phone_number)">
                {{ getInitials(contactsStore.currentContact.name || contactsStore.currentContact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div>
              <div class="flex items-center gap-1.5">
                <p class="text-sm font-medium text-white light:text-gray-900">
                  {{ contactsStore.currentContact.name || contactsStore.currentContact.phone_number }}
                </p>
                <Badge v-if="activeTransferId" class="text-[10px] h-5 bg-orange-500/20 text-orange-400 light:bg-orange-100 light:text-orange-700">
                  Paused
                </Badge>
              </div>
              <p class="text-[11px] text-white/50 light:text-gray-500">
                {{ contactsStore.currentContact.phone_number }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-1">
            <Tooltip v-if="canAssignContacts">
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon" class="h-8 w-8 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100" @click="isAssignDialogOpen = true">
                  <UserPlus class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('chat.assignToAgent') }}</TooltipContent>
            </Tooltip>
            <Tooltip v-if="activeTransferId">
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon" class="h-8 w-8 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100" :disabled="isResuming" @click="resumeChatbot">
                  <Play class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('chat.resumeChatbot') }}</TooltipContent>
            </Tooltip>
            <!-- Custom Action Buttons -->
            <Tooltip v-for="action in customActions" :key="action.id">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100"
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
                  id="notes-button"
                  class="h-8 w-8 relative text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100"
                  :class="isNotesPanelOpen && 'bg-amber-500/10 text-amber-400 light:bg-amber-50 light:text-amber-600'"
                  @click="isNotesPanelOpen = !isNotesPanelOpen"
                >
                  <StickyNote class="h-4 w-4" />
                  <span
                    v-if="notesStore.notes.length > 0 && !isNotesPanelOpen"
                    id="notes-badge"
                    class="absolute -top-0.5 -right-0.5 h-4 min-w-[16px] rounded-full bg-amber-500 text-[10px] text-white flex items-center justify-center px-1"
                  >
                    {{ notesStore.notes.length }}
                  </span>
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('chat.internalNotes') }}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100"
                  :class="isInfoPanelOpen && 'bg-white/[0.08] text-white light:bg-gray-100 light:text-gray-900'"
                  @click="isInfoPanelOpen = !isInfoPanelOpen"
                >
                  <Info class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('chat.contactInfo') }}</TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon" class="h-8 w-8 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100">
                  <MoreVertical class="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{{ $t('chat.contactOptions') }}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem v-if="canAssignContacts" @click="isAssignDialogOpen = true">
                  <UserPlus class="mr-2 h-4 w-4" />
                  <span>{{ $t('chat.assignToAgent') }}</span>
                </DropdownMenuItem>
                <DropdownMenuItem v-if="!activeTransferId" @click="transferToAgent" :disabled="isTransferring">
                  <UserX class="mr-2 h-4 w-4" />
                  <span>{{ $t('chat.transferToAgent') }}</span>
                </DropdownMenuItem>
                <DropdownMenuItem v-if="activeTransferId" @click="resumeChatbot" :disabled="isResuming">
                  <Play class="mr-2 h-4 w-4" />
                  <span>{{ $t('chat.resumeChatbot') }}</span>
                </DropdownMenuItem>
                <DropdownMenuItem @click="isInfoPanelOpen = !isInfoPanelOpen">
                  <Info class="mr-2 h-4 w-4" />
                  <span>{{ isInfoPanelOpen ? $t('chat.hideContactDetails') : $t('chat.viewContactDetails') }}</span>
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
              class="absolute top-2 left-1/2 -translate-x-1/2 z-10 px-3 py-1 bg-white/[0.08] light:bg-gray-200 backdrop-blur-sm rounded-full text-[11px] text-white/50 light:text-gray-600 font-medium shadow-sm"
            >
              {{ stickyDate }}
            </div>
          </Transition>

          <ScrollArea :ref="(el: any) => messagesScroll.scrollAreaRef.value = el" class="h-full p-3 chat-background">
            <div class="space-y-2">
              <!-- Loading indicator for older messages -->
              <div v-if="contactsStore.isLoadingOlderMessages" class="flex justify-center py-2">
                <div class="flex items-center gap-2 text-white/40 light:text-gray-500 text-sm">
                  <div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                  <span>{{ $t('chat.loadingOlderMessages') }}...</span>
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
                  <div class="px-3 py-1 bg-white/[0.06] light:bg-gray-200 rounded-full text-[11px] text-white/40 light:text-gray-600 font-medium">
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
                  class="reply-preview cursor-pointer text-xs"
                  @click="scrollToMessage(message.reply_to_message_id)"
                >
                  <p class="font-medium">
                    {{ message.reply_to_message.direction === 'incoming' ? (contactsStore.currentContact?.profile_name || contactsStore.currentContact?.name || 'Customer') : 'You' }}
                  </p>
                  <p class="truncate">
                    {{ getReplyPreviewContent(message) }}
                  </p>
                </div>
                <!-- Image message -->
                <div v-if="message.message_type === 'image' && message.media_url" class="mb-2">
                  <div v-if="isMediaLoading(message)" class="w-[200px] h-[150px] bg-muted rounded-lg animate-pulse flex items-center justify-center">
                    <span class="text-muted-foreground text-sm">{{ $t('common.loading') }}...</span>
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
                    <span class="text-muted-foreground text-sm">{{ $t('common.loading') }}...</span>
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
                    <span class="text-muted-foreground text-sm">{{ $t('common.loading') }}...</span>
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
                    <span class="text-sm text-muted-foreground">{{ $t('common.loading') }}...</span>
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
                    <div class="h-10 w-10 rounded-full bg-red-900/30 light:bg-red-100 flex items-center justify-center shrink-0">
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
                <span v-else-if="getMessageContent(message)" class="whitespace-pre-wrap break-words">{{ getMessageContent(message) }}<span class="chat-bubble-time"><span>{{ formatMessageTime(message.created_at) }}</span><component v-if="message.direction === 'outgoing'" :is="getMessageStatusIcon(message.status)" :class="['h-4 w-4 status-icon', getMessageStatusClass(message.status)]" /></span></span>
                <!-- Fallback for media without URL -->
                <span v-else-if="isMediaMessage(message) && !message.media_url" class="text-muted-foreground italic">[{{ message.message_type.charAt(0).toUpperCase() + message.message_type.slice(1) }}]<span class="chat-bubble-time"><span>{{ formatMessageTime(message.created_at) }}</span><component v-if="message.direction === 'outgoing'" :is="getMessageStatusIcon(message.status)" :class="['h-4 w-4 status-icon', getMessageStatusClass(message.status)]" /></span></span>
                <!-- Interactive buttons - WhatsApp style -->
                <div
                  v-if="getInteractiveButtons(message).length > 0"
                  class="interactive-buttons mt-2 -mx-2 -mb-1.5 border-t"
                >
                  <div
                    v-for="(btn, index) in getInteractiveButtons(message)"
                    :key="btn.id"
                    :class="[
                      'py-2 text-sm text-center font-medium cursor-pointer',
                      index > 0 && 'border-t'
                    ]"
                  >
                    {{ btn.title }}
                  </div>
                </div>
                <!-- CTA URL button - WhatsApp style -->
                <a
                  v-if="getCTAUrlData(message)"
                  :href="getCTAUrlData(message)?.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="interactive-buttons mt-2 -mx-2 -mb-1.5 border-t block"
                >
                  <div class="py-2 text-sm text-center font-medium cursor-pointer flex items-center justify-center gap-1.5">
                    <ExternalLink class="h-3.5 w-3.5" />
                    {{ getCTAUrlData(message)?.button_text }}
                  </div>
                </a>
                <!-- Time for messages without text content -->
                <span v-if="!getMessageContent(message) && !(isMediaMessage(message) && !message.media_url)" class="chat-bubble-time block clear-both">
                  <span>{{ formatMessageTime(message.created_at) }}</span>
                  <component
                    v-if="message.direction === 'outgoing'"
                    :is="getMessageStatusIcon(message.status)"
                    :class="['h-4 w-4 status-icon', getMessageStatusClass(message.status)]"
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
          class="px-4 py-2 border-t border-white/[0.08] light:border-gray-200 bg-white/[0.04] light:bg-gray-50 flex items-center justify-between"
        >
          <div class="flex-1 min-w-0">
            <p class="text-xs font-medium text-white/50 light:text-gray-500">
              Replying to {{ contactsStore.replyingTo.direction === 'incoming' ? (contactsStore.currentContact?.profile_name || contactsStore.currentContact?.name || 'Customer') : 'Yourself' }}
            </p>
            <p class="text-sm truncate text-white/70 light:text-gray-700">
              {{ getMessageContent(contactsStore.replyingTo) || '[Media]' }}
            </p>
          </div>
          <button class="w-6 h-6 rounded hover:bg-white/[0.08] light:hover:bg-gray-200 flex items-center justify-center shrink-0 transition-colors" @click="contactsStore.clearReplyingTo">
            <X class="h-4 w-4 text-white/50 light:text-gray-500" />
          </button>
        </div>

        <!-- Message Input -->
        <div class="p-4 border-t border-white/[0.08] light:border-gray-200 bg-[#0f0f10] light:bg-white">
          <form @submit.prevent="sendMessage" class="flex items-center gap-2 p-2 rounded-xl bg-white/[0.06] light:bg-gray-100 border border-white/[0.08] light:border-gray-200">
            <Tooltip>
              <TooltipTrigger as-child>
                <span>
                  <Popover v-model:open="emojiPickerOpen">
                    <PopoverTrigger as-child>
                      <button type="button" class="w-9 h-9 rounded-lg hover:bg-white/[0.08] light:hover:bg-gray-200 flex items-center justify-center transition-colors">
                        <Smile class="w-[18px] h-[18px] text-white/40 light:text-gray-500" />
                      </button>
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
              <TooltipContent>{{ $t('chat.emoji') }}</TooltipContent>
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
              <TooltipContent>{{ $t('chat.cannedResponses') }}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <button type="button" class="w-9 h-9 rounded-lg hover:bg-white/[0.08] light:hover:bg-gray-200 flex items-center justify-center transition-colors" @click="openFilePicker">
                  <Paperclip class="w-[18px] h-[18px] text-white/40 light:text-gray-500" />
                </button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('chat.attachFile') }}</TooltipContent>
            </Tooltip>
            <input
              ref="fileInputRef"
              type="file"
              accept="image/*,video/*,audio/*,.pdf,.doc,.docx"
              class="hidden"
              @change="handleFileSelect"
            />
            <textarea
              ref="messageInputRef"
              v-model="messageInput"
              :placeholder="$t('chat.typeMessage') + '...'"
              rows="1"
              class="flex-1 bg-transparent text-[14px] text-white light:text-gray-900 placeholder:text-white/30 light:placeholder:text-gray-400 focus:outline-none resize-none min-h-[36px] max-h-[120px] py-2 overflow-y-auto"
              @keydown.enter.exact.prevent="sendMessage"
              @input="autoResizeTextarea"
            />
            <button type="submit" class="w-9 h-9 rounded-lg bg-emerald-600 hover:bg-emerald-500 light:bg-emerald-500 light:hover:bg-emerald-600 flex items-center justify-center transition-colors disabled:opacity-50" :disabled="!messageInput.trim() || isSending">
              <Send class="w-4 h-4 text-white" />
            </button>
          </form>
        </div>
      </template>
    </div>

    <!-- Notes Side Panel -->
    <ConversationNotes
      v-if="contactsStore.currentContact && isNotesPanelOpen"
      :contact-id="contactsStore.currentContact.id"
      @close="isNotesPanelOpen = false"
    />

    <!-- Contact Info Panel -->
    <ContactInfoPanel
      v-if="contactsStore.currentContact && isInfoPanelOpen"
      :contact="contactsStore.currentContact"
      :session-data="contactSessionData"
      @close="isInfoPanelOpen = false"
      @tags-updated="(tags) => contactsStore.updateContactTags(contactsStore.currentContact!.id, tags)"
    />

    <!-- Assign Contact Dialog -->
    <Dialog v-model:open="isAssignDialogOpen" @update:open="(open) => !open && (assignSearchQuery = '')">
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ $t('chat.assignContact') }}</DialogTitle>
          <DialogDescription>
            {{ $t('chat.assignContactDesc') }}
          </DialogDescription>
        </DialogHeader>
        <div class="py-4 space-y-3">
          <!-- Search input -->
          <div class="relative">
            <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              v-model="assignSearchQuery"
              :placeholder="$t('chat.searchUsers') + '...'"
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
            {{ $t('chat.unassignContact') }}
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
                {{ $t('chat.noUsersFound') }}
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
          <DialogTitle>{{ $t('chat.sendMedia') }}</DialogTitle>
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
                <p class="text-xs text-muted-foreground">{{ $t('chat.audioFile') }}</p>
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
              :placeholder="$t('chat.mediaCaption') + '...'"
              class="min-h-[60px] max-h-[100px] resize-none"
              :rows="2"
            />
          </div>

          <!-- Actions -->
          <div class="flex justify-end gap-2">
            <Button variant="outline" @click="closeMediaDialog" :disabled="isUploadingMedia">
              {{ $t('common.cancel') }}
            </Button>
            <Button @click="sendMediaMessage" :disabled="isUploadingMedia">
              <Send v-if="!isUploadingMedia" class="mr-2 h-4 w-4" />
              <span v-if="isUploadingMedia">{{ $t('chat.sending') }}...</span>
              <span v-else>{{ $t('chat.send') }}</span>
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Add Contact Dialog -->
    <CreateContactDialog v-model:open="isAddContactOpen" @created="onContactCreated" />
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
