<script setup lang="ts">
import { ref, reactive, watch, onMounted, onUnmounted, nextTick, computed, useId } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useContactsStore, type Message } from '@/stores/contacts'
import { useAuthStore } from '@/stores/auth'
import { useUsersStore } from '@/stores/users'
import { useTransfersStore } from '@/stores/transfers'
import { wsService } from '@/services/websocket'
import { contactsService, chatbotService, messagesService, customActionsService, accountsService, cannedResponsesService, getRequestHeaders, type CustomAction, type ActionResult, type CannedResponse } from '@/services/api'
import { useTagsStore } from '@/stores/tags'
import { getErrorMessage } from '@/lib/api-utils'
import { compressImage } from '@/lib/imageCompression'
import Modal from '@/components/oat/Modal.vue'
import { COMMON_EMOJIS } from '@/lib/emojis'
import { toast } from '@/lib/toast'
import {
  Send,
  Paperclip,
  FileText,
  Smile,
  MoreVertical,
  Phone,
  PhoneCall,
  PhoneMissed,
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
  Zap,
  Ticket,
  BarChart,
  Link,
  Mail,
  Globe,
  Code,
  RotateCw,
  Filter,
  StickyNote,
  ArrowLeft,
  Info
} from '@/lib/icons'
import { getInitials } from '@/lib/utils'
import { useInfiniteScroll } from '@/composables/useInfiniteScroll'
import CannedResponsePicker from '@/components/chat/CannedResponsePicker.vue'
import PreviewButtonGroup from '@/components/chatbot/flow-preview/PreviewButtonGroup.vue'
import TemplatePicker from '@/components/chat/TemplatePicker.vue'
import MediaViewerDialog from '@/components/chat/MediaViewerDialog.vue'
import ContactInfoPanel from '@/components/chat/ContactInfoPanel.vue'
import ConversationNotes from '@/components/chat/ConversationNotes.vue'
import CallButton from '@/components/calling/CallButton.vue'
import { useNotesStore } from '@/stores/notes'
import { useHeaderMedia } from '@/composables/useHeaderMedia'
import { CreateContactDialog } from '@/components/shared'
import HeaderMediaUpload from '@/components/shared/HeaderMediaUpload.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const contactsStore = useContactsStore()
const authStore = useAuthStore()
const usersStore = useUsersStore()
const transfersStore = useTransfersStore()
const tagsStore = useTagsStore()
const notesStore = useNotesStore()
const uid = useId()
const emojiMenuId = `${uid}-emoji`
const moreMenuId = `${uid}-more`
const tagFilterId = `${uid}-tags`
const emojiMenu = ref<HTMLElement | null>(null)

// Phones show one column at a time: the conversation list, the thread or a side panel.
const narrowQuery = window.matchMedia('(max-width: 47.99rem)')
const isNarrow = ref(narrowQuery.matches)
// Avatar URLs that failed to load; those contacts show initials instead.
const failedAvatars = reactive(new Set<string>())

function onNarrowChange(e: MediaQueryListEvent) {
  isNarrow.value = e.matches
}

const canWriteContacts = authStore.hasPermission('contacts', 'write')

const messageInput = ref('')
const messagesEndRef = ref<HTMLElement | null>(null)
const messageInputRef = ref<HTMLTextAreaElement | null>(null)
const isSending = ref(false)
const isAssignDialogOpen = ref(false)
const isTransferring = ref(false)
const isResuming = ref(false)
// Tracks incoming messages that arrived while the chat is open.
// Surfaced as a "N unread messages" pill at the top of the chat panel
// (WhatsApp-style). Click the pill to jump up to the first message of
// the unread batch; cleared on click or contact switch. See issue #280.
const newMessagesCount = ref(0)
const firstUnreadId = ref<string | null>(null)
const isAtBottom = ref(true)
const SCROLL_BOTTOM_THRESHOLD = 80
const isInfoPanelOpen = ref(false)
const isNotesPanelOpen = ref(false)
const contactSessionData = ref<any>(null)

// Multi-account state
const selectedAccount = ref<string | null>(null)
const contactAccounts = ref<string[]>([])
const orgAccounts = ref<any[]>([])
const isBusinessCallingEnabled = computed(() =>
  orgAccounts.value.find(account => account.name === selectedAccount.value)?.business_calling_enabled === true
)

// File upload state
const fileInputRef = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const filePreviewUrl = ref<string | null>(null)
const isMediaDialogOpen = ref(false)
const mediaCaption = ref('')
const isUploadingMedia = ref(false)

// In-app media viewer (lightbox) state — see MediaViewerDialog.vue
const mediaViewerOpen = ref(false)
const mediaViewerIndex = ref(0)

// Cache for media blob URLs (message_id -> blob URL)


// Canned responses slash command state
const cannedPickerOpen = ref(false)
const cannedSearchQuery = ref('')

// Canned response preview dialog state
const cannedDialogOpen = ref(false)
const selectedCannedResponse = ref<CannedResponse | null>(null)
const cannedParamNames = ref<string[]>([])
const cannedParamValues = ref<Record<string, string>>({})
const isSendingCanned = ref(false)
// Tokens that the chat already knows how to fill from the current contact /
// signed-in agent. Shared by canned responses (resolved client-side into the
// outgoing message) and templates (pre-filled into the param payload so the
// backend forwards the resolved value to Meta).
const AUTO_RESOLVED_CONTEXT_TOKENS = new Set(['contact_name', 'phone_number', 'user_name', 'agent_name'])

// Sticky date header state
const stickyDate = ref('')
const showStickyDate = ref(false)
let stickyDateTimeout: ReturnType<typeof setTimeout> | null = null

// Template picker state
const templatePickerRef = ref<InstanceType<typeof TemplatePicker> | null>(null)
const templateDialogOpen = ref(false)
const selectedTemplate = ref<any>(null)
const templateParamNames = ref<string[]>([])
const templateParamValues = ref<Record<string, string>>({})
// Name of the TEXT-header variable (max 1 per Meta) and its value. Kept in
// its own ref so a positional {{1}} in the header doesn't collide with a
// {{1}} body parameter — both can be filled independently.
const templateHeaderParamName = ref<string | null>(null)
const templateHeaderParamValue = ref('')
const templateButtonUrlParams = ref<{ index: number; text: string; value: string; type: string }[]>([])
const isSendingTemplate = ref(false)
const templateHeaderType = computed(() => selectedTemplate.value?.header_type)
const {
  file: templateHeaderFile,
  previewUrl: templateHeaderPreview,
  needsMedia: templateNeedsHeaderMedia,
  acceptTypes: templateHeaderAccept,
  handleFileChange: handleTemplateHeaderFile,
  clear: clearTemplateHeaderMedia,
} = useHeaderMedia(templateHeaderType)

// Custom actions state
const customActions = ref<CustomAction[]>([])
const executingActionId = ref<string | null>(null)

// Service window state
const isServiceWindowExpired = computed(() => {
  const contact = contactsStore.currentContact
  if (!contact) return false
  return contact.service_window_open === false
})

function openTemplatePicker() {
  templatePickerRef.value?.open()
}

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
      await contactsStore.fetchOlderMessages(contactsStore.currentContact!.id, selectedAccount.value || undefined)
      await nextTick()
    })
  },
  hasMore: computed(() => contactsStore.hasMoreMessages),
  isLoading: computed(() => contactsStore.isLoadingOlderMessages),
  onScroll: (event) => {
    const el = event.target as HTMLElement
    updateStickyDate(el)
    updateAtBottom(el)
  }
})

function updateAtBottom(el: HTMLElement) {
  const distanceFromBottom = el.scrollHeight - el.clientHeight - el.scrollTop
  isAtBottom.value = distanceFromBottom < SCROLL_BOTTOM_THRESHOLD
}

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

    // JavaScript actions are now executed server-side via goja.
    // The response already contains structured result fields (toast, clipboard, redirect_url, message).

    // Handle different result types
    if (result.redirect_url) {
      // Open URL action result - prepend base path for relative URLs
      let redirectUrl = result.redirect_url
      if (redirectUrl.startsWith('/api/')) {
        const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
        redirectUrl = basePath + redirectUrl
      }
      try {
        const parsed = new URL(redirectUrl, window.location.origin)
        if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
          window.open(parsed.href, '_blank')
        }
      } catch {
        // Invalid URL, ignore
      }
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
watch(isAssignDialogOpen, (open) => {
  if (!open) assignSearchQuery.value = ''
})

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

  // Fetch org-level WhatsApp accounts for account tabs
  try {
    const res = await accountsService.list()
    orgAccounts.value = res.data.data?.accounts || []
  } catch {
    orgAccounts.value = []
  }

  // Fetch available tags for filtering (if not already loaded)
  if (tagsStore.tags.length === 0) {
    tagsStore.fetchTags().catch(() => {})
  }

  if (contactId.value) {
    await selectContact(contactId.value)
  }

  // Auto-scroll to the unread divider and mark messages read when the agent
  // returns — covers both tab-switch (visibilitychange) and OS window focus
  // (focus event), since "tab visible but window unfocused" is a real state
  // and we don't want to send blue-tick receipts when no one is looking.
  // See issue #280.
  document.addEventListener('visibilitychange', onUserActive)
  window.addEventListener('focus', onUserActive)
  narrowQuery.addEventListener('change', onNarrowChange)
})

function onUserActive() {
  if (document.visibilityState !== 'visible' || !document.hasFocus()) return
  if (!firstUnreadId.value) return
  if (contactsStore.currentContact) {
    const contactId = contactsStore.currentContact.id
    contactsService.markRead(contactId)
      .then(() => contactsStore.markContactRead(contactId))
      .catch(() => { /* non-critical */ })
  }
  nextTick(() => {
    const el = document.getElementById(`message-${firstUnreadId.value}`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  })
}

onUnmounted(() => {
  wsService.setCurrentContact(null)
  // Clear current contact when leaving chat view so notifications work on other pages
  contactsStore.setCurrentContact(null)
  notesStore.clearNotes()
  // Clear sticky date timeout
  if (stickyDateTimeout) clearTimeout(stickyDateTimeout)
  document.removeEventListener('visibilitychange', onUserActive)
  window.removeEventListener('focus', onUserActive)
  narrowQuery.removeEventListener('change', onNarrowChange)
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
  // On phones a side panel replaces the thread; start the next contact on the thread.
  if (isNarrow.value) {
    isInfoPanelOpen.value = false
    isNotesPanelOpen.value = false
  }
  if (newId) {
    notesStore.notes = []
    notesStore.hasMore = false
    await selectContact(newId)
  } else {
    wsService.setCurrentContact(null)
    contactsStore.setCurrentContact(null)
    contactsStore.clearMessages()
    notesStore.clearNotes()
  }
})

async function selectContact(id: string) {
  // Direct deep links to /chat/:id may target a contact that isn't in the
  // currently-loaded (paginated) list — fall back to fetching it directly.
  let contact = contactsStore.contacts.find(c => c.id === id)
  if (!contact) {
    contact = await contactsStore.fetchContact(id)
  }
  if (contact) {
    // Reset unread pill — fetchMessages will mark everything read on the server
    newMessagesCount.value = 0
    firstUnreadId.value = null
    isAtBottom.value = true

    // Remove old scroll listener before switching contacts
    messagesScroll.cleanup()

    // Reset account selection when switching contacts
    selectedAccount.value = null
    contactAccounts.value = []
    contactsStore.setAccountFilter(null)

    contactsStore.setCurrentContact(contact)
    await contactsStore.fetchMessages(id)

    // Discover distinct accounts from the unfiltered message set
    const accounts = new Set<string>()
    for (const msg of contactsStore.messages) {
      if (msg.whatsapp_account) accounts.add(msg.whatsapp_account)
    }
    contactAccounts.value = Array.from(accounts).sort()

    // Auto-select account and filter client-side (avoids a second fetch)
    if (orgAccounts.value.length > 1) {
      // Find account of the most recent incoming message
      for (let i = contactsStore.messages.length - 1; i >= 0; i--) {
        const msg = contactsStore.messages[i]
        if (msg.direction === 'incoming' && msg.whatsapp_account) {
          selectedAccount.value = msg.whatsapp_account
          break
        }
      }
      // Fallback to contact's default account, then first org account
      if (!selectedAccount.value) {
        selectedAccount.value = contact.whatsapp_account || contactAccounts.value[0] || orgAccounts.value[0]?.name
      }
      if (selectedAccount.value) {
        contactsStore.setAccountFilter(selectedAccount.value)
        // Filter messages client-side instead of re-fetching
        contactsStore.messages = contactsStore.messages.filter(
          (m: any) => m.whatsapp_account === selectedAccount.value
        )
      }
    } else if (contactAccounts.value.length === 1) {
      selectedAccount.value = contactAccounts.value[0]
    } else if (contact.whatsapp_account) {
      selectedAccount.value = contact.whatsapp_account
    }

    // Tell WebSocket server which contact we're viewing
    wsService.setCurrentContact(id)
    // Wait for DOM to render messages before scrolling
    await nextTick()
    // Scroll after a brief delay to ensure content is rendered (instant on initial load)
    setTimeout(() => {
      scrollToBottom(true)
      // Setup scroll listener for infinite scroll after initial scroll
      messagesScroll.setup()
    }, 50)

    // Fetch notes and session data in parallel (independent requests)
    const [, sessionResult] = await Promise.all([
      notesStore.fetchNotes(id),
      contactsService.getSessionData(id).catch(() => null)
    ])
    if (sessionResult) {
      contactSessionData.value = sessionResult.data.data || sessionResult.data
      // On phones the panel would cover the thread, so it stays closed there.
      if (contactSessionData.value?.panel_config?.sections?.length > 0 && !isNarrow.value) {
        isInfoPanelOpen.value = true
      }
    } else {
      contactSessionData.value = null
    }
  }
}

// Watch for new messages. WhatsApp Web style: while the browser tab is
// focused on this chat the user is "watching", so auto-scroll if they're
// at the bottom. When they're on another tab, pile up unread and surface
// a divider above the first message that arrived while away (issue #280).
// The two branches are mutually exclusive — auto-scrolling while the tab
// is hidden races with the divider state.
watch(() => contactsStore.messages.length, (newLen, oldLen) => {
  if (newLen <= oldLen) return
  const latest = contactsStore.messages[newLen - 1]
  const isIncoming = latest?.direction === 'incoming'
  // "Not actively looking" covers both other-tab (hidden) and other-window
  // (visible but unfocused). The divider should pile in either case.
  const userAway = typeof document !== 'undefined'
    && (document.visibilityState === 'hidden' || !document.hasFocus())
  if (isIncoming && userAway) {
    if (newMessagesCount.value === 0) {
      firstUnreadId.value = latest.id
    }
    newMessagesCount.value += 1
    return
  }
  // Outgoing (the agent replied) — they've seen the unread, drop the divider.
  if (!isIncoming && newMessagesCount.value > 0) {
    newMessagesCount.value = 0
    firstUnreadId.value = null
  }
  if (isAtBottom.value || !isIncoming) {
    scrollToBottom()
  }
})

async function switchAccount(accountName: string) {
  if (!contactsStore.currentContact || accountName === selectedAccount.value) return
  selectedAccount.value = accountName
  contactsStore.setAccountFilter(accountName)
  await contactsStore.fetchMessages(contactsStore.currentContact.id, { account: accountName })
  await nextTick()
  scrollToBottom(true)
}

async function sendMessage() {
  if (!messageInput.value.trim() || !contactsStore.currentContact) return

  isSending.value = true
  try {
    await contactsStore.sendMessage(
      contactsStore.currentContact.id,
      'text',
      { body: messageInput.value },
      contactsStore.replyingTo?.id,
      selectedAccount.value || undefined
    )
    messageInput.value = ''
    contactsStore.clearReplyingTo()
    resetTextareaHeight()
    await nextTick()
    scrollToBottom()
  } catch (error) {
    toast.error(getErrorMessage(error, t('chat.sendMessageFailed')))
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
      content,
      undefined,
      message.whatsapp_account || selectedAccount.value || undefined
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
    toast.error(getErrorMessage(error, t('chat.sendMessageFailed')))
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
  if (reply.message_type === 'call') return '[Missed call]'
  if (reply.message_type === 'sticker') return '[Sticker]'
  return '[Message]'
}

function scrollToMessage(messageId: string | undefined) {
  if (!messageId) return
  const messageEl = document.getElementById(`message-${messageId}`)
  if (messageEl) {
    messageEl.scrollIntoView({ behavior: 'smooth', block: 'center' })
    // Briefly ring the bubble so the agent can spot it.
    messageEl.querySelector('[data-bubble]')?.animate(
      [{ outline: '2px solid var(--ring)' }, { outline: '2px solid transparent' }],
      { duration: 2000, easing: 'ease-out' },
    )
  }
}

function extractCannedTokens(content: string): string[] {
  const seen = new Set<string>()
  const matches = content.matchAll(/\{\{\s*([\w.-]+)\s*\}\}/g)
  for (const m of matches) seen.add(m[1])
  return Array.from(seen)
}

// Collect tokens from the message body AND every button field, so the param
// dialog prompts for custom tokens used anywhere on the response.
function extractCannedTokensFromResponse(r: CannedResponse): string[] {
  const seen = new Set<string>(extractCannedTokens(r.content))
  for (const btn of r.buttons || []) {
    for (const t of extractCannedTokens(btn.title || '')) seen.add(t)
    for (const t of extractCannedTokens(btn.url || '')) seen.add(t)
    for (const t of extractCannedTokens(btn.phone_number || '')) seen.add(t)
  }
  return Array.from(seen)
}

// Resolve a single context token (contact_name / phone_number / user_name /
// agent_name) against the current chat. Returns null for any key that isn't
// in AUTO_RESOLVED_CONTEXT_TOKENS so callers can fall back to their own param
// dict.
function resolveContextToken(key: string): string | null {
  const contact = contactsStore.currentContact
  if (key === 'contact_name') return contact?.profile_name || contact?.name || 'there'
  if (key === 'phone_number') return contact?.phone_number || ''
  if (key === 'user_name' || key === 'agent_name') return authStore.user?.full_name || ''
  return null
}

// Shared {{...}} resolver used by the body preview and the button fields, so
// `{{phone_number}}` works inside a button URL the same way it does in content.
function resolveCannedTokens(text: string): string {
  if (!text) return text
  return text.replace(/\{\{\s*([\w.-]+)\s*\}\}/g, (_match, key: string) => {
    const ctx = resolveContextToken(key)
    if (ctx !== null) return ctx
    const value = cannedParamValues.value[key]
    return value ? value : `{{${key}}}`
  })
}

const cannedPreview = computed(() =>
  selectedCannedResponse.value ? resolveCannedTokens(selectedCannedResponse.value.content) : '',
)

// Resolved buttons (with {{...}} substitution applied) for the dialog preview.
// Empty array when no response is selected or it has no buttons.
const cannedPreviewButtons = computed(() => {
  const raw = selectedCannedResponse.value?.buttons || []
  return raw.map(b => ({
    ...b,
    title: resolveCannedTokens(b.title),
    ...(b.url !== undefined ? { url: resolveCannedTokens(b.url) } : {}),
    ...(b.phone_number !== undefined ? { phone_number: resolveCannedTokens(b.phone_number) } : {}),
  }))
})

function handleCannedSelect(response: CannedResponse) {
  selectedCannedResponse.value = response
  const tokens = extractCannedTokensFromResponse(response).filter(
    t => !AUTO_RESOLVED_CONTEXT_TOKENS.has(t)
  )
  cannedParamNames.value = tokens
  cannedParamValues.value = Object.fromEntries(tokens.map(t => [t, '']))
  // Drop the slash command (or any stray text) so the textarea starts clean.
  messageInput.value = ''
  resetTextareaHeight()
  cannedPickerOpen.value = false
  cannedSearchQuery.value = ''
  cannedDialogOpen.value = true
}

async function sendCannedResponse() {
  if (!contactsStore.currentContact || !selectedCannedResponse.value) return

  const missing = cannedParamNames.value.some(n => !cannedParamValues.value[n]?.trim())
  if (missing) {
    toast.error(t('chat.parameterRequired'))
    return
  }

  const body = cannedPreview.value
  const responseId = selectedCannedResponse.value.id
  // Substitute {{...}} tokens in every button field — same rules as the body —
  // so URLs like https://x.com/u/{{phone_number}} resolve at send time.
  const buttons = (selectedCannedResponse.value.buttons || []).map(b => ({
    ...b,
    title: resolveCannedTokens(b.title),
    ...(b.url !== undefined ? { url: resolveCannedTokens(b.url) } : {}),
    ...(b.phone_number !== undefined ? { phone_number: resolveCannedTokens(b.phone_number) } : {}),
  }))
  const replyButtons = buttons.filter(b => !b.type || b.type === 'reply')
  const urlButtons = buttons.filter(b => b.type === 'url')

  // WhatsApp Cloud API supports: 1-3 reply buttons (interactive.button),
  // 4-10 reply rows (interactive.list — backend's SendInteractiveButtons
  // auto-picks the right shape), a single cta_url, or a single voice_call
  // (Business Calling click-to-call). Phone buttons and multi-URL / mixed
  // combos aren't representable; the detail-page validator blocks save for
  // those, so the text fallback here is just a safety net.
  const voiceCallButtons = buttons.filter(b => b.type === 'voice_call')
  const flowButtons = buttons.filter(b => b.type === 'flow')
  let sendType: 'text' | 'interactive' = 'text'
  let interactive: {
    type: 'button' | 'list' | 'cta_url' | 'voice_call' | 'flow'
    body: string
    buttons?: Array<{ id: string; title: string }>
    button_text?: string
    url?: string
    display_text?: string
    ttl_minutes?: number
    flow_id?: string
    first_screen?: string
  } | undefined

  if (buttons.length === 1 && flowButtons.length === 1) {
    const f = flowButtons[0]
    sendType = 'interactive'
    interactive = {
      type: 'flow',
      body,
      // The button title is the CTA label shown to the customer.
      button_text: resolveCannedTokens(f.title),
      flow_id: f.flow_id,
      first_screen: f.screen,
    }
  } else if (buttons.length === 1 && voiceCallButtons.length === 1) {
    const vc = voiceCallButtons[0]
    sendType = 'interactive'
    interactive = {
      type: 'voice_call',
      body,
      // {{...}} tokens already resolved in the canned-preview path; the
      // button title is what becomes Meta's display_text. Backend
      // truncates to 20 chars and stamps the agent-id payload.
      display_text: resolveCannedTokens(vc.title),
      ttl_minutes: vc.ttl_minutes ?? 15,
    }
  } else if (buttons.length > 0 && replyButtons.length === buttons.length && replyButtons.length <= 10) {
    sendType = 'interactive'
    interactive = {
      type: replyButtons.length <= 3 ? 'button' : 'list',
      body,
      buttons: replyButtons.map(b => ({ id: b.id, title: b.title })),
    }
  } else if (buttons.length === 1 && urlButtons.length === 1) {
    sendType = 'interactive'
    interactive = {
      type: 'cta_url',
      body,
      button_text: urlButtons[0].title,
      url: urlButtons[0].url || '',
    }
  }

  isSendingCanned.value = true
  try {
    await contactsStore.sendMessage(
      contactsStore.currentContact.id,
      sendType,
      sendType === 'interactive' ? { body } : { body },
      contactsStore.replyingTo?.id,
      selectedAccount.value || undefined,
      interactive ? { interactive } : undefined,
    )
    cannedResponsesService.use(responseId).catch(() => {})
    contactsStore.clearReplyingTo()
    cannedDialogOpen.value = false
    selectedCannedResponse.value = null
    cannedParamNames.value = []
    cannedParamValues.value = {}
    await nextTick()
    scrollToBottom()
  } catch (error) {
    toast.error(getErrorMessage(error, t('chat.sendMessageFailed')))
  } finally {
    isSendingCanned.value = false
  }
}

function closeCannedPicker() {
  cannedPickerOpen.value = false
  cannedSearchQuery.value = ''
}

function insertEmoji(emoji: string) {
  messageInput.value += emoji
  emojiMenu.value?.hidePopover()
  messageInputRef.value?.focus()
}

// Template message handling
function getTemplateBodyContent(tpl: any): string {
  return tpl.body_content || ''
}

const templatePreview = computed(() => {
  if (!selectedTemplate.value) return ''
  const body = getTemplateBodyContent(selectedTemplate.value)
  return body.replace(/\{\{\s*([\w.-]+)\s*\}\}/g, (_match, key: string) => {
    const supplied = templateParamValues.value[key]
    if (supplied) return supplied
    const ctx = resolveContextToken(key)
    if (ctx !== null) return ctx
    return `{{${key}}}`
  })
})

// Show the header input only when the user has to fill it. Context-token
// names (contact_name, phone_number, …) auto-resolve and stay hidden — same
// rule body params follow via templateParamNames filtering.
const showHeaderParamInput = computed(() =>
  !!templateHeaderParamName.value &&
  !AUTO_RESOLVED_CONTEXT_TOKENS.has(templateHeaderParamName.value)
)

function extractButtonUrlParams(buttons: any[]): { index: number; text: string; value: string; type: string }[] {
  if (!buttons?.length) return []
  return buttons
    .map((btn: any, index: number) => {
      if (btn.type === 'COPY_CODE') {
        return { index, text: btn.text || 'Copy Code', value: btn.example?.[0] || '', type: 'COPY_CODE' }
      }
      if (btn.type !== 'URL' || !btn.url) return null
      const hasParams = /\{\{[^}]+\}\}/.test(btn.url)
      if (!hasParams) return null
      return { index, text: btn.text || 'URL Button', value: '', type: 'URL' }
    })
    .filter((b): b is { index: number; text: string; value: string; type: string } => b !== null)
}

function handleTemplateWithParams(template: any, paramNames: string[]) {
  selectedTemplate.value = template
  // Pre-fill body context tokens from the conversation; keep them in the
  // payload dict so the backend forwards them to Meta, but hide them from
  // the dialog so the agent doesn't have to type values we already know —
  // same pattern as canned responses (see handleCannedSelect).
  const initial: Record<string, string> = {}
  for (const name of paramNames) {
    const resolved = resolveContextToken(name)
    initial[name] = resolved ?? ''
  }
  templateParamValues.value = initial
  templateParamNames.value = paramNames.filter(n => !AUTO_RESOLVED_CONTEXT_TOKENS.has(n))

  // Identify the TEXT-header variable (max 1) and pre-fill from context.
  // Context-token names (contact_name / phone_number / agent_name / user_name)
  // resolve automatically and stay hidden from the dialog — same convention
  // as body params.
  templateHeaderParamName.value = null
  templateHeaderParamValue.value = ''
  if (template.header_type === 'TEXT' && template.header_content) {
    const m = template.header_content.match(/\{\{([^}]+)\}\}/)
    if (m) {
      const name = m[1].trim()
      templateHeaderParamName.value = name
      templateHeaderParamValue.value = resolveContextToken(name) ?? ''
    }
  }

  clearTemplateHeaderMedia()
  templateButtonUrlParams.value = extractButtonUrlParams(template.buttons)
  templateDialogOpen.value = true
}

async function sendTemplateMessage() {
  if (!contactsStore.currentContact || !selectedTemplate.value) return

  // Validate header param (separate ref so it can hold its own value even
  // when the body has a {{1}} that would otherwise collide). Auto-resolved
  // context tokens are exempt — their value comes from the conversation.
  if (showHeaderParamInput.value && !templateHeaderParamValue.value.trim()) {
    toast.error(t('chat.parameterRequired'))
    return
  }

  // Validate all body params are filled
  const missingBody = templateParamNames.value.some(n => !templateParamValues.value[n]?.trim())
  if (missingBody) {
    toast.error(t('chat.parameterRequired'))
    return
  }

  // Validate header media if required
  if (templateNeedsHeaderMedia.value && !templateHeaderFile.value) {
    toast.error(t('chat.headerMediaRequired'))
    return
  }

  // Validate all button URL params are filled
  const missingButton = templateButtonUrlParams.value.some(b => !b.value?.trim())
  if (missingButton) {
    toast.error(t('chat.parameterRequired'))
    return
  }

  // Build button params map: button index -> value
  const buttonParams: Record<string, string> | undefined =
    templateButtonUrlParams.value.length > 0
      ? Object.fromEntries(templateButtonUrlParams.value.map(b => [String(b.index), b.value]))
      : undefined

  // Header value goes in its own payload field so a positional {{1}} header
  // doesn't overwrite a positional {{1}} body parameter in the flat map.
  const headerParams: Record<string, string> | undefined =
    templateHeaderParamName.value && templateHeaderParamValue.value
      ? { [templateHeaderParamName.value]: templateHeaderParamValue.value }
      : undefined

  isSendingTemplate.value = true
  try {
    await contactsStore.sendTemplate(
      contactsStore.currentContact.id,
      selectedTemplate.value.name,
      templateParamValues.value,
      selectedAccount.value || undefined,
      templateHeaderFile.value || undefined,
      buttonParams,
      headerParams
    )
    toast.success(t('chat.templateSent'))
    templateDialogOpen.value = false
    selectedTemplate.value = null
    templateParamNames.value = []
    templateParamValues.value = {}
    templateHeaderParamName.value = null
    templateHeaderParamValue.value = ''
    clearTemplateHeaderMedia()
    templateButtonUrlParams.value = []
  } catch (error: any) {
    const message = error.response?.data?.message || t('chat.templateSendFailed')
    toast.error(message)
  } finally {
    isSendingTemplate.value = false
  }
}

// Reaction handling
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
  document.getElementById(`react-${messageId}`)?.hidePopover()
}

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

// Read ticks use the primary colour, failures the danger colour; sent/delivered stay muted.
function getMessageStatusStroke(status: string): string {
  if (status === 'read') return 'var(--primary)'
  if (status === 'failed') return 'var(--danger)'
  return 'currentColor'
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
    return t('chat.yesterday')
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
    return t('chat.today')
  } else if (diffDays === 1) {
    return t('chat.yesterday')
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
  if (message.message_type === 'button_reply' || message.message_type === 'nfm_reply') {
    // Button/flow reply stores the response text in content
    if (typeof message.content === 'string') {
      return message.content
    }
    return message.content?.body || ''
  }
  if (message.message_type === 'interactive' || message.message_type === 'flow') {
    // Interactive/flow messages store body text in content (string) or content.body or interactive_data.body
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
  if (message.message_type === 'call') {
    return '' // Missed calls render as their own card
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

function getInteractiveButtons(message: Message): Array<{ id: string; title: string; type: string; url: string }> {
  if (!message.interactive_data) {
    return []
  }
  // Support both interactive and template messages with buttons
  if (message.message_type !== 'interactive' && message.message_type !== 'template') {
    return []
  }
  // Handle both "buttons" (<=3) and "rows" (>3 list format)
  const items = message.interactive_data.buttons || message.interactive_data.rows
  if (!items || !Array.isArray(items)) {
    return []
  }
  return items.map((btn: any) => ({
    id: btn.reply?.id || btn.id || '',
    title: btn.reply?.title || btn.title || btn.text || '',
    type: btn.type || 'QUICK_REPLY',
    url: btn.url || ''
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

interface VoiceCallData {
  display_text: string
  ttl_minutes?: number
}

function getVoiceCallData(message: Message): VoiceCallData | null {
  if (message.message_type !== 'interactive' || !message.interactive_data) {
    return null
  }
  if (message.interactive_data.type !== 'voice_call') {
    return null
  }
  return {
    display_text: (message.interactive_data as any).display_text || 'Call',
    ttl_minutes: (message.interactive_data as any).ttl_minutes,
  }
}

function getFlowButtonText(message: Message): string | null {
  if (message.message_type !== 'flow') {
    return null
  }
  if (!message.interactive_data) {
    return null
  }
  return (message.interactive_data as any).button_text || 'Open'
}

function isOutgoing(message: Message): boolean {
  return message.direction === 'outgoing'
}

function isMediaMessage(message: Message): boolean {
  return ['image', 'video', 'audio', 'document'].includes(message.message_type)
}

function getMediaUrl(message: Message): string {
  if (!message.media_url) return ''
  const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
  return `${basePath}/api/media/${message.id}`
}

// Every media attachment in the open conversation, in chronological order, that
// the in-app viewer can show (images, stickers, video, documents/PDF, plus
// template header media). This is the gallery the lightbox pages through.
const viewableMedia = computed(() =>
  contactsStore.messages.filter(
    m =>
      !!m.media_url &&
      (['image', 'sticker', 'video', 'document'].includes(m.message_type) ||
        m.message_type === 'template'),
  ),
)

// Only PDFs gain anything from the lightbox — docx/xlsx/zip have no inline
// viewer, so routing them through the modal would just add a click before the
// same download. Those bubbles keep their one-click download link; the viewer
// still shows them (with a download card) when paging through the gallery.
function isPreviewableDocument(message: Message): boolean {
  const mime = message.media_mime_type || ''
  const name = (message.media_filename || '').toLowerCase()
  return mime.includes('pdf') || name.endsWith('.pdf')
}

// Open the in-app viewer at the clicked attachment instead of a new browser tab.
function openMediaPreview(message: Message) {
  const idx = viewableMedia.value.findIndex(m => m.id === message.id)
  if (idx === -1) return
  mediaViewerIndex.value = idx
  mediaViewerOpen.value = true
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

async function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // reset so the same file can be selected again
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

  // Compress images client-side so they fit the Cloud API's 5 MB image limit
  // (Meta accepts only jpeg/png for `image` messages). No-op for non-images.
  let outFile = file
  if (file.type.startsWith('image/')) {
    try {
      outFile = await compressImage(file)
    } catch {
      outFile = file
    }
  }

  // Per-type size validation: images 5 MB (Meta's limit), other media 14.5 MB
  // (under the 15 MB fasthttp request body cap).
  const type = getMediaType(outFile.type)
  const maxSize = type === 'image' ? 5 * 1024 * 1024 : 14.5 * 1024 * 1024
  if (outFile.size > maxSize) {
    toast.error(t('chat.fileTooLarge'), {
      description: type === 'image' ? t('chat.fileTooLargeImage') : t('chat.fileTooLargeMedia')
    })
    return
  }

  selectedFile.value = outFile
  mediaCaption.value = ''

  // Create preview URL for images and videos
  if (outFile.type.startsWith('image/') || outFile.type.startsWith('video/')) {
    filePreviewUrl.value = URL.createObjectURL(outFile)
  } else {
    filePreviewUrl.value = null
  }

  isMediaDialogOpen.value = true
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
    if (selectedAccount.value) {
      formData.append('whatsapp_account', selectedAccount.value)
    }

    const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
    const response = await fetch(`${basePath}/api/messages/media`, {
      method: 'POST',
      credentials: 'include',
      headers: getRequestHeaders({ csrf: true }),
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
  <div class="flex h-full min-h-0 gap-4 p-4">
    <!-- Contacts List -->
    <article v-show="!isNarrow || !contactsStore.currentContact" data-testid="contacts" class="card p-4 flex flex-col min-h-0 gap-2 chat-col">
      <!-- Search Header -->
      <div class="vstack gap-2 shrink-0">
        <div class="flex items-center gap-1">
          <input
            v-model="contactsStore.searchQuery"
            type="search"
            class="flex-1 min-w-0"
            :placeholder="$t('chat.searchContacts') + '...'"
            :aria-label="$t('chat.searchContacts')"
          />
          <!-- Add Contact -->
          <button
            v-if="canWriteContacts"
            type="button"
            class="ghost small icon shrink-0"
            :title="$t('chat.addContact')"
            :aria-label="$t('chat.addContact')"
            @click="openAddContactDialog"
          >
            <UserPlus />
          </button>
          <!-- Tag Filter -->
          <ot-dropdown>
            <button
              type="button"
              class="shrink-0 small"
              :class="contactsStore.selectedTags.length > 0 ? 'outline' : 'ghost'"
              :popovertarget="tagFilterId"
              :title="$t('chat.filterByTags')"
              :aria-label="$t('chat.filterByTags')"
            >
              <Filter />
              <span v-if="contactsStore.selectedTags.length > 0" class="badge">{{ contactsStore.selectedTags.length }}</span>
            </button>
            <div :id="tagFilterId" popover class="picker">
              <div class="vstack gap-2 p-4">
                <div class="hstack justify-between">
                  <strong>{{ $t('chat.filterByTags') }}</strong>
                  <button
                    v-if="contactsStore.selectedTags.length > 0"
                    type="button"
                    class="ghost small"
                    @click="clearTagFilter"
                  >
                    {{ $t('common.clear') }}
                  </button>
                </div>
                <hr />
                <p v-if="tagsStore.tags.length === 0" class="align-center text-light">
                  <small>{{ $t('chat.noTagsAvailable') }}</small>
                </p>
                <div v-else class="scroll-y picker-list vstack gap-2">
                  <label v-for="tag in tagsStore.tags" :key="tag.name">
                    <input
                      type="checkbox"
                      :checked="contactsStore.selectedTags.includes(tag.name)"
                      @change="toggleTagFilter(tag.name)"
                    />
                    <span class="truncate">{{ tag.name }}</span>
                  </label>
                </div>
              </div>
            </div>
          </ot-dropdown>
        </div>
        <!-- Active tag filters -->
        <div v-if="contactsStore.selectedTags.length > 0" class="hstack gap-1">
          <span v-for="tagName in contactsStore.selectedTags" :key="tagName" class="badge" data-variant="secondary">
            {{ tagName }}
            <button type="button" :aria-label="`${$t('contacts.removeTag')}: ${tagName}`" @click="toggleTagFilter(tagName)">
              <X :size="12" />
            </button>
          </span>
        </div>
      </div>

      <!-- Contacts -->
      <nav
        :ref="(el: any) => contactsScroll.scrollAreaRef.value = el"
        class="flex-1 scroll-y"
        :aria-label="$t('chat.conversations')"
      >
        <ul class="unstyled vstack gap-1">
          <li v-for="contact in contactsStore.sortedContacts" :key="contact.id">
            <RouterLink
              :to="`/chat/${contact.id}`"
              data-testid="contact"
              class="button w-100"
              :class="contactsStore.currentContact?.id === contact.id ? undefined : 'ghost'"
              :data-variant="contactsStore.currentContact?.id === contact.id ? 'secondary' : undefined"
              :aria-current="contactsStore.currentContact?.id === contact.id ? 'true' : undefined"
            >
              <figure data-variant="avatar" class="small shrink-0">
                <img v-if="contact.avatar_url && !failedAvatars.has(contact.avatar_url)" :src="contact.avatar_url" alt="" @error="failedAvatars.add(contact.avatar_url)" />
                <abbr v-else>{{ getInitials(contact.name || contact.phone_number) }}</abbr>
              </figure>
              <span class="flex flex-col flex-1 min-w-0 align-left">
                <span class="flex items-center justify-between gap-2">
                  <strong class="truncate" :title="contact.name || contact.phone_number">
                    {{ contact.name || contact.phone_number }}
                  </strong>
                  <small class="text-light shrink-0">{{ formatContactTime(contact.last_message_at) }}</small>
                </span>
                <span class="flex items-center justify-between gap-2">
                  <small class="text-light truncate">{{ contact.phone_number }}</small>
                  <span v-if="contact.unread_count > 0" class="badge" data-variant="success">{{ contact.unread_count }}</span>
                </span>
              </span>
            </RouterLink>
          </li>
        </ul>

        <!-- Loading indicator for infinite scroll -->
        <div v-if="contactsStore.isLoadingMoreContacts" class="flex justify-center p-4">
          <span role="status" aria-busy="true" :aria-label="$t('common.loading')" />
        </div>

        <div v-if="contactsStore.sortedContacts.length === 0" class="vstack gap-1 items-center align-center text-light p-4">
          <User :size="24" />
          <small>{{ $t('chat.noContacts') }}</small>
        </div>
      </nav>
    </article>

    <!-- Chat Area -->
    <section
      v-show="!isNarrow || (contactsStore.currentContact && !isNotesPanelOpen && !isInfoPanelOpen)"
      class="flex-1 min-w-0 flex flex-col min-h-0 gap-2"
    >
      <!-- No Contact Selected -->
      <div v-if="!contactsStore.currentContact" class="flex-1 flex items-center justify-center">
        <div class="vstack items-center align-center text-light">
          <Send :size="32" />
          <h3>{{ $t('chat.selectConversation') }}</h3>
          <p>{{ $t('chat.chooseContact') }}</p>
        </div>
      </div>

      <!-- Chat Interface -->
      <template v-else>
        <!-- Chat Header -->
        <header data-chat-header class="flex flex-wrap items-center justify-between gap-2 shrink-0">
          <div class="flex items-center gap-2 min-w-0">
            <RouterLink
              v-if="isNarrow"
              to="/chat"
              class="button ghost small icon shrink-0"
              :title="$t('common.back')"
              :aria-label="$t('common.back')"
            >
              <ArrowLeft />
            </RouterLink>
            <figure data-variant="avatar" class="small shrink-0">
              <img
                v-if="contactsStore.currentContact.avatar_url && !failedAvatars.has(contactsStore.currentContact.avatar_url)"
                :src="contactsStore.currentContact.avatar_url"
                alt=""
                @error="failedAvatars.add(contactsStore.currentContact.avatar_url)"
              />
              <abbr v-else>{{ getInitials(contactsStore.currentContact.name || contactsStore.currentContact.phone_number) }}</abbr>
            </figure>
            <div class="min-w-0 flex flex-col">
              <div class="hstack gap-2">
                <strong class="truncate">{{ contactsStore.currentContact.name || contactsStore.currentContact.phone_number }}</strong>
                <span v-if="activeTransferId" class="badge" data-variant="warning">{{ $t('chat.paused') }}</span>
                <span v-if="contactsStore.currentContact?.marketing_opt_out" class="badge" data-variant="danger" :title="$t('chat.marketingOptOut')">
                  {{ $t('chat.marketingOptOut', 'Marketing Opt-out') }}
                </span>
              </div>
              <small class="text-light truncate">
                {{ contactsStore.currentContact.phone_number }}<template v-if="contactsStore.currentContact.last_inbound_at">
                  · {{ $t('chat.lastActive', { time: formatContactTime(contactsStore.currentContact.last_inbound_at) }) }}</template>
              </small>
            </div>
          </div>
          <div class="hstack gap-1 justify-end">
            <CallButton
              v-if="contactsStore.currentContact?.phone_number && selectedAccount && isBusinessCallingEnabled"
              :contact-id="contactsStore.currentContact.id"
              :contact-phone="contactsStore.currentContact.phone_number"
              :contact-name="contactsStore.currentContact.name || contactsStore.currentContact.phone_number"
              :whatsapp-account="selectedAccount"
            />
            <button
              v-if="canAssignContacts"
              type="button"
              class="ghost small icon"
              :title="$t('chat.assignToAgent')"
              :aria-label="$t('chat.assignToAgent')"
              @click="isAssignDialogOpen = true"
            >
              <UserPlus />
            </button>
            <button
              v-if="activeTransferId"
              type="button"
              class="ghost small icon"
              :title="$t('chat.resumeChatbot')"
              :aria-label="$t('chat.resumeChatbot')"
              :disabled="isResuming"
              @click="resumeChatbot"
            >
              <Play />
            </button>
            <!-- Custom Action Buttons -->
            <button
              v-for="action in customActions"
              :key="action.id"
              type="button"
              class="ghost small icon"
              :title="action.name"
              :aria-label="action.name"
              :disabled="executingActionId === action.id"
              :aria-busy="executingActionId === action.id ? 'true' : undefined"
              data-spinner="small"
              @click="executeCustomAction(action)"
            >
              <component :is="getActionIcon(action.icon)" v-if="executingActionId !== action.id" />
            </button>
            <button
              id="notes-button"
              type="button"
              class="small"
              :class="isNotesPanelOpen ? 'outline' : 'ghost'"
              :aria-pressed="isNotesPanelOpen"
              :title="$t('chat.internalNotes')"
              :aria-label="$t('chat.internalNotes')"
              @click="isNotesPanelOpen = !isNotesPanelOpen"
            >
              <StickyNote />
              <span
                v-if="notesStore.notes.length > 0 && !isNotesPanelOpen"
                id="notes-badge"
                class="badge"
                data-variant="warning"
              >
                {{ notesStore.notes.length }}
              </span>
            </button>
            <button
              id="info-button"
              type="button"
              class="small icon"
              :class="isInfoPanelOpen ? 'outline' : 'ghost'"
              :aria-pressed="isInfoPanelOpen"
              :title="$t('chat.contactInfo')"
              :aria-label="$t('chat.contactInfo')"
              @click="isInfoPanelOpen = !isInfoPanelOpen"
            >
              <Info />
            </button>
            <ot-dropdown>
              <button
                type="button"
                class="ghost small icon"
                :popovertarget="moreMenuId"
                :title="$t('chat.contactOptions')"
                :aria-label="$t('chat.contactOptions')"
              >
                <MoreVertical />
              </button>
              <div :id="moreMenuId" popover role="menu" :aria-label="$t('chat.contactOptions')">
                <div role="presentation" class="p-4"><small class="text-light">{{ $t('chat.contactOptions') }}</small></div>
                <hr />
                <button
                  v-if="canAssignContacts"
                  type="button"
                  role="menuitem"
                  class="ghost"
                  :popovertarget="moreMenuId"
                  popovertargetaction="hide"
                  @click="isAssignDialogOpen = true"
                >
                  <UserPlus />{{ $t('chat.assignToAgent') }}
                </button>
                <button
                  v-if="!activeTransferId"
                  type="button"
                  role="menuitem"
                  class="ghost"
                  :disabled="isTransferring"
                  :popovertarget="moreMenuId"
                  popovertargetaction="hide"
                  @click="transferToAgent"
                >
                  <UserX />{{ $t('chat.transferToAgent') }}
                </button>
                <button
                  v-if="activeTransferId"
                  type="button"
                  role="menuitem"
                  class="ghost"
                  :disabled="isResuming"
                  :popovertarget="moreMenuId"
                  popovertargetaction="hide"
                  @click="resumeChatbot"
                >
                  <Play />{{ $t('chat.resumeChatbot') }}
                </button>
                <button
                  type="button"
                  role="menuitem"
                  class="ghost"
                  :popovertarget="moreMenuId"
                  popovertargetaction="hide"
                  @click="isInfoPanelOpen = !isInfoPanelOpen"
                >
                  <Info />{{ isInfoPanelOpen ? $t('chat.hideContactDetails') : $t('chat.viewContactDetails') }}
                </button>
              </div>
            </ot-dropdown>
          </div>
        </header>

        <!-- Account Tabs (shown when the org has several WhatsApp accounts) -->
        <div v-if="orgAccounts.length > 1 && selectedAccount" class="shrink-0 scroll-x">
          <div role="tablist" :aria-label="$t('accounts.title')">
            <button
              v-for="acct in orgAccounts"
              :key="acct.name"
              type="button"
              role="tab"
              aria-controls="chat-messages"
              :aria-selected="acct.name === selectedAccount"
              @click="switchAccount(acct.name)"
            >
              {{ acct.name }}
            </button>
          </div>
        </div>

        <!-- Messages -->
        <div
          class="relative flex-1 min-h-0 flex flex-col"
          :aria-busy="contactsStore.isLoadingMessages ? 'true' : undefined"
          data-spinner="large overlay"
        >
          <!-- Sticky date header -->
          <div v-if="showStickyDate" class="overlay-top">
            <span class="badge" data-variant="secondary" data-date-pill>{{ stickyDate }}</span>
          </div>

          <div
            id="chat-messages"
            :ref="(el: any) => messagesScroll.scrollAreaRef.value = el"
            data-testid="messages"
            data-chat-canvas
            class="flex-1 scroll-y"
          >
            <div class="vstack gap-2">
              <!-- Loading indicator for older messages -->
              <div v-if="contactsStore.isLoadingOlderMessages" class="flex justify-center">
                <small class="hstack gap-2 text-light">
                  <span aria-busy="true" data-spinner="small" />
                  {{ $t('chat.loadingOlderMessages') }}...
                </small>
              </div>
              <template
                v-for="(message, index) in contactsStore.messages"
                :key="message.id"
              >
                <!-- Date separator -->
                <div
                  v-if="shouldShowDateSeparator(index)"
                  class="flex justify-center"
                  :data-date-separator="getDateLabel(message.created_at)"
                >
                  <span class="badge" data-variant="secondary" data-date-pill>{{ getDateLabel(message.created_at) }}</span>
                </div>

                <!-- Unread divider (WhatsApp-style; appears above the first
                     message that arrived while the tab was hidden) -->
                <div v-if="newMessagesCount > 0 && message.id === firstUnreadId" class="flex justify-center">
                  <span class="badge">
                    {{ newMessagesCount }} {{ newMessagesCount === 1 ? $t('chat.unreadMessage', 'unread message') : $t('chat.unreadMessages', 'unread messages') }}
                  </span>
                </div>

                <!-- Message row: bubble + hover actions -->
                <div
                  :id="`message-${message.id}`"
                  data-testid="message"
                  data-hover-group
                  class="flex items-center gap-1"
                  :class="{ 'justify-end': isOutgoing(message) }"
                >
                  <article class="card min-w-0" :data-bubble="message.direction">
                    <!-- Reply preview (if this message is replying to another) -->
                    <button
                      v-if="message.is_reply && message.reply_to_message"
                      type="button"
                      class="outline small w-100 mb-2"
                      data-reply-preview
                      @click="scrollToMessage(message.reply_to_message_id)"
                    >
                      <span class="flex flex-col w-100 min-w-0 align-left">
                        <strong>
                          {{ message.reply_to_message.direction === 'incoming' ? (contactsStore.currentContact?.profile_name || contactsStore.currentContact?.name || $t('chat.customer')) : $t('chat.you') }}
                        </strong>
                        <span class="truncate">{{ getReplyPreviewContent(message) }}</span>
                      </span>
                    </button>

                    <!-- Template header media (image/video/document shown above template text) -->
                    <div v-if="message.message_type === 'template' && message.media_url" class="mb-2">
                      <img
                        v-if="message.media_mime_type?.startsWith('image/')"
                        :src="getMediaUrl(message)"
                        :alt="$t('chat.image')"
                        class="media-preview"
                        @click="openMediaPreview(message)"
                        @error="handleImageError($event)"
                      />
                      <video
                        v-else-if="message.media_mime_type?.startsWith('video/')"
                        :src="getMediaUrl(message)"
                        controls
                        class="media-preview"
                      />
                      <button
                        v-else-if="isPreviewableDocument(message)"
                        type="button"
                        class="outline w-100"
                        @click="openMediaPreview(message)"
                      >
                        <FileText /><span class="truncate">{{ message.media_filename || $t('chat.document') }}</span>
                      </button>
                      <a
                        v-else
                        :href="getMediaUrl(message)"
                        :download="message.media_filename || 'document'"
                        class="button outline w-100"
                      >
                        <FileText /><span class="truncate">{{ message.media_filename || $t('chat.document') }}</span>
                      </a>
                    </div>
                    <!-- Image message -->
                    <div v-else-if="message.message_type === 'image' && message.media_url" class="mb-2">
                      <img
                        :src="getMediaUrl(message)"
                        :alt="message.content?.body || $t('chat.image')"
                        class="media-preview"
                        @click="openMediaPreview(message)"
                        @error="handleImageError($event)"
                      />
                    </div>
                    <!-- Sticker message -->
                    <div v-else-if="message.message_type === 'sticker' && message.media_url" class="mb-2">
                      <img
                        :src="getMediaUrl(message)"
                        :alt="$t('chat.sticker')"
                        class="media-preview"
                        @click="openMediaPreview(message)"
                        @error="handleImageError($event)"
                      />
                    </div>
                    <!-- Video message -->
                    <div v-else-if="message.message_type === 'video' && message.media_url" class="mb-2">
                      <video
                        :src="getMediaUrl(message)"
                        controls
                        class="media-preview"
                        @error="handleMediaError($event, 'video')"
                      />
                    </div>
                    <!-- Audio message -->
                    <div v-else-if="message.message_type === 'audio' && message.media_url" class="mb-2">
                      <audio
                        :src="getMediaUrl(message)"
                        controls
                        class="w-100"
                        @error="handleMediaError($event, 'audio')"
                      />
                    </div>
                    <!-- Document message -->
                    <div v-else-if="message.message_type === 'document' && message.media_url" class="mb-2">
                      <button
                        v-if="isPreviewableDocument(message)"
                        type="button"
                        class="outline w-100"
                        @click="openMediaPreview(message)"
                      >
                        <FileText /><span class="truncate">{{ message.media_filename || $t('chat.document') }}</span>
                      </button>
                      <a
                        v-else
                        :href="getMediaUrl(message)"
                        :download="message.media_filename || 'document'"
                        class="button outline w-100"
                      >
                        <FileText /><span class="truncate">{{ message.media_filename || $t('chat.document') }}</span>
                      </a>
                    </div>
                    <!-- Location message -->
                    <div v-else-if="message.message_type === 'location' && getLocationData(message)" class="mb-2">
                      <a
                        :href="getGoogleMapsUrl(getLocationData(message)!)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="button outline w-100"
                      >
                        <MapPin :size="20" stroke="var(--danger)" />
                        <span class="flex flex-col flex-1 min-w-0 align-left">
                          <strong class="truncate">{{ getLocationData(message)?.name || $t('chat.location') }}</strong>
                          <small v-if="getLocationData(message)?.address" class="text-light truncate">
                            {{ getLocationData(message)?.address }}
                          </small>
                          <small class="text-light">
                            {{ getLocationData(message)?.latitude.toFixed(6) }}, {{ getLocationData(message)?.longitude.toFixed(6) }}
                          </small>
                        </span>
                        <ExternalLink />
                      </a>
                    </div>
                    <!-- Contacts message -->
                    <div v-else-if="message.message_type === 'contacts' && getContactsData(message).length > 0" class="vstack gap-2 mb-2">
                      <div
                        v-for="(contact, idx) in getContactsData(message)"
                        :key="idx"
                        class="flex items-center gap-2"
                      >
                        <figure data-variant="avatar" class="small shrink-0"><User /></figure>
                        <div class="min-w-0">
                          <strong class="truncate">{{ contact.name }}</strong>
                          <small v-if="contact.phones?.length" class="flex items-center gap-1 text-light">
                            <Phone :size="12" />
                            <span class="truncate">{{ contact.phones.join(', ') }}</span>
                          </small>
                        </div>
                      </div>
                    </div>
                    <!-- Missed call (click-to-call the originating agent never picked up) -->
                    <p v-else-if="message.message_type === 'call'" class="flex items-center gap-2 mb-2">
                      <PhoneMissed stroke="var(--danger)" />
                      <span>{{ $t('chat.missedCall', 'Missed call') }}</span>
                    </p>
                    <!-- Unsupported message -->
                    <p v-else-if="message.message_type === 'unsupported'" class="flex items-center gap-2 text-light mb-2">
                      <AlertCircle />
                      <em>{{ $t('chat.unsupportedMessage') }}</em>
                    </p>

                    <!-- Text content (text, button replies and captions) -->
                    <p v-if="getMessageContent(message)" class="pre-wrap">{{ getMessageContent(message) }}</p>
                    <!-- Fallback for media without URL -->
                    <p v-else-if="isMediaMessage(message) && !message.media_url" class="text-light">
                      <em>[{{ message.message_type.charAt(0).toUpperCase() + message.message_type.slice(1) }}]</em>
                    </p>

                    <!-- Interactive buttons - WhatsApp style -->
                    <div v-if="getInteractiveButtons(message).length > 0" class="vstack gap-1 mt-2" data-interactive-buttons>
                      <template v-for="btn in getInteractiveButtons(message)" :key="btn.id">
                        <a
                          v-if="btn.type === 'URL' && btn.url"
                          :href="btn.url"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="button outline small w-100"
                        >
                          <ExternalLink :size="14" />
                          {{ btn.title }}
                        </a>
                        <span v-else class="badge outline w-100">{{ btn.title }}</span>
                      </template>
                    </div>
                    <!-- CTA URL button - WhatsApp style -->
                    <a
                      v-if="getCTAUrlData(message)"
                      :href="getCTAUrlData(message)?.url"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="button outline small w-100 mt-2"
                    >
                      <ExternalLink :size="14" />
                      {{ getCTAUrlData(message)?.button_text }}
                    </a>
                    <!-- Voice call button - WhatsApp style, non-clickable in our chat -->
                    <div v-if="getVoiceCallData(message)" class="mt-2">
                      <span class="badge outline w-100">
                        <PhoneCall :size="14" />
                        {{ getVoiceCallData(message)?.display_text }}
                      </span>
                    </div>
                    <!-- Flow button - WhatsApp style -->
                    <div v-if="getFlowButtonText(message)" class="mt-2">
                      <span class="badge outline w-100">{{ getFlowButtonText(message) }}</span>
                    </div>

                    <!-- Time and delivery status -->
                    <div class="flex items-center justify-end gap-1" data-bubble-time>
                      <small class="text-light">{{ formatMessageTime(message.created_at) }}</small>
                      <component
                        :is="getMessageStatusIcon(message.status)"
                        v-if="message.direction === 'outgoing'"
                        data-status-icon
                        class="text-light"
                        :stroke="getMessageStatusStroke(message.status)"
                      />
                    </div>

                    <!-- Reactions display -->
                    <div v-if="message.reactions && message.reactions.length > 0" class="hstack gap-1 mt-2">
                      <span
                        v-for="(reaction, idx) in message.reactions"
                        :key="idx"
                        class="badge outline"
                        :title="reaction.from_phone || reaction.from_user || ''"
                      >
                        {{ reaction.emoji }}
                      </span>
                    </div>

                    <!-- Failed message error -->
                    <small
                      v-if="message.status === 'failed' && message.direction === 'outgoing'"
                      class="flex items-center gap-1 mt-2"
                    >
                      <AlertCircle :size="12" stroke="var(--danger)" class="shrink-0" />
                      <span>{{ message.error_message || $t('chat.failedToSend') }}</span>
                    </small>
                  </article>

                  <!-- Message actions (revealed on hover / focus) -->
                  <div class="vstack gap-1 shrink-0" data-hover-reveal>
                    <ot-dropdown>
                      <button
                        type="button"
                        class="ghost small icon"
                        :popovertarget="`react-${message.id}`"
                        :title="$t('chat.addReaction')"
                        :aria-label="$t('chat.addReaction')"
                      >
                        <SmilePlus :size="14" />
                      </button>
                      <div :id="`react-${message.id}`" popover>
                        <div class="hstack gap-1 p-4">
                          <button
                            v-for="emoji in quickReactionEmojis"
                            :key="emoji"
                            type="button"
                            class="ghost small"
                            @click="sendReaction(message.id, emoji)"
                          >
                            {{ emoji }}
                          </button>
                        </div>
                      </div>
                    </ot-dropdown>
                    <button
                      type="button"
                      class="ghost small icon"
                      :title="$t('chat.reply')"
                      :aria-label="$t('chat.reply')"
                      @click="replyToMessage(message)"
                    >
                      <Reply :size="14" />
                    </button>
                    <button
                      v-if="message.direction === 'outgoing' && message.status === 'failed' && message.message_type !== 'template'"
                      type="button"
                      class="ghost small icon"
                      data-variant="danger"
                      :title="$t('chat.retry')"
                      :aria-label="$t('chat.retry')"
                      :disabled="retryingMessageId === message.id"
                      :aria-busy="retryingMessageId === message.id ? 'true' : undefined"
                      data-spinner="small"
                      @click="retryMessage(message)"
                    >
                      <RotateCw v-if="retryingMessageId !== message.id" :size="14" />
                    </button>
                  </div>
                </div>
              </template>
              <div ref="messagesEndRef" />
            </div>
          </div>
        </div>

        <!-- Composer -->
        <footer class="vstack gap-2 shrink-0">
          <!-- Service window expired banner -->
          <div v-if="isServiceWindowExpired" role="alert" data-variant="danger" class="items-center">
            <Clock class="shrink-0" />
            <span class="flex-1">{{ $t('chat.serviceWindowExpired') }}</span>
            <button type="button" class="outline small shrink-0" @click="openTemplatePicker">
              {{ $t('chat.sendTemplateAction') }}
            </button>
          </div>

          <!-- Reply indicator -->
          <div v-if="contactsStore.replyingTo" class="flex items-center gap-2" data-reply-indicator>
            <div class="flex-1 min-w-0">
              <small class="text-light">
                {{ $t('chat.replyingTo') }} {{ contactsStore.replyingTo.direction === 'incoming' ? (contactsStore.currentContact?.profile_name || contactsStore.currentContact?.name || $t('chat.customer')) : $t('chat.yourself') }}
              </small>
              <p class="truncate">{{ getMessageContent(contactsStore.replyingTo) || `[${$t('chat.media')}]` }}</p>
            </div>
            <button
              type="button"
              class="ghost small icon shrink-0"
              :title="$t('chat.cancelReply')"
              :aria-label="$t('chat.cancelReply')"
              @click="contactsStore.clearReplyingTo"
            >
              <X />
            </button>
          </div>

          <!-- Message Input -->
          <!-- Only the textarea and Send live in the <form>: picker search boxes
               must not trigger implicit submission (Enter would send the draft). -->
          <div class="flex items-center gap-1">
            <ot-dropdown>
              <button
                type="button"
                class="ghost icon"
                :popovertarget="emojiMenuId"
                :title="$t('chat.emoji')"
                :aria-label="$t('chat.emoji')"
              >
                <Smile :size="20" />
              </button>
              <div :id="emojiMenuId" ref="emojiMenu" popover class="picker">
                <div class="hstack gap-1 p-4">
                  <button
                    v-for="emoji in COMMON_EMOJIS"
                    :key="emoji"
                    type="button"
                    class="ghost small"
                    @click="insertEmoji(emoji)"
                  >
                    {{ emoji }}
                  </button>
                </div>
              </div>
            </ot-dropdown>
            <CannedResponsePicker
              :external-open="cannedPickerOpen"
              :external-search="cannedSearchQuery"
              @select="handleCannedSelect"
              @close="closeCannedPicker"
            />
            <TemplatePicker
              ref="templatePickerRef"
              :selected-account="selectedAccount"
              @select-with-params="handleTemplateWithParams"
            />
            <button
              type="button"
              class="ghost icon"
              :title="$t('chat.attachFile')"
              :aria-label="$t('chat.attachFile')"
              @click="openFilePicker"
            >
              <Paperclip :size="20" />
            </button>
            <input
              ref="fileInputRef"
              type="file"
              accept="image/*,video/*,audio/*,.pdf,.doc,.docx"
              hidden
              @change="handleFileSelect"
            />
            <form class="flex-1 min-w-0 flex items-center gap-1" @submit.prevent="sendMessage">
              <textarea
                ref="messageInputRef"
                v-model="messageInput"
                class="flex-1 min-w-0 min-h-0"
                :placeholder="$t('chat.typeMessage') + '...'"
                :aria-label="$t('chat.typeMessage')"
                rows="1"
                @keydown.enter.exact.prevent="sendMessage"
                @input="autoResizeTextarea"
              />
              <button
                type="submit"
                class="icon shrink-0"
                :title="$t('chat.send')"
                :aria-label="$t('chat.send')"
                :disabled="!messageInput.trim() || isSending"
              >
                <Send />
              </button>
            </form>
          </div>
        </footer>
      </template>
    </section>

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
      :narrow="isNarrow"
      @close="isInfoPanelOpen = false"
      @tags-updated="(tags) => contactsStore.updateContactTags(contactsStore.currentContact!.id, tags)"
    />

    <!-- Template Params Dialog -->
    <Modal
      v-model:open="templateDialogOpen"
      size="small"
      :title="templateParamNames.length > 0 ? $t('chat.fillParameters') : $t('chat.preview')"
      :description="selectedTemplate?.display_name || selectedTemplate?.name"
    >
      <div class="vstack gap-4">
        <!-- Header media upload -->
        <HeaderMediaUpload
          v-if="templateNeedsHeaderMedia"
          :file="templateHeaderFile"
          :preview-url="templateHeaderPreview"
          :accept-types="templateHeaderAccept"
          :label="selectedTemplate?.header_type === 'IMAGE' ? $t('chat.headerImage') : selectedTemplate?.header_type === 'VIDEO' ? $t('chat.headerVideo') : $t('chat.headerDocument')"
          @change="handleTemplateHeaderFile"
          @clear="clearTemplateHeaderMedia"
        />

        <label v-if="showHeaderParamInput" data-field data-param-header>
          <span class="hstack gap-2">
            {{ templateHeaderParamName }}
            <span class="badge" data-variant="secondary">{{ $t('chat.headerParamBadge', 'Header') }}</span>
          </span>
          <input v-model="templateHeaderParamValue" :placeholder="templateHeaderParamName ?? ''" />
        </label>
        <label v-for="param in templateParamNames" :key="param" data-field :data-param="param">
          {{ param }}
          <input v-model="templateParamValues[param]" :placeholder="param" />
        </label>
        <label v-for="(btnParam, idx) in templateButtonUrlParams" :key="`btn-${btnParam.index}`" data-field>
          {{ btnParam.type === 'COPY_CODE' ? `Coupon Code (${btnParam.text})` : $t('chat.urlButtonParam', { button: btnParam.text }) }}
          <input
            v-model="templateButtonUrlParams[idx].value"
            :placeholder="btnParam.type === 'COPY_CODE' ? 'WELCOME10' : $t('chat.urlButtonParamPlaceholder')"
          />
        </label>
        <div v-if="templatePreview" class="vstack gap-1">
          <small class="text-light">{{ $t('chat.preview') }}</small>
          <article class="card" data-bubble="outgoing">
            <img v-if="templateHeaderPreview" :src="templateHeaderPreview" alt="" class="media-preview mb-2" />
            <p class="pre-wrap">{{ templatePreview }}</p>
            <div v-if="selectedTemplate?.buttons?.length" class="vstack gap-1 mt-2" data-interactive-buttons>
              <span v-for="(btn, index) in selectedTemplate.buttons" :key="index" class="badge outline w-100">
                {{ btn.text }}
              </span>
            </div>
          </article>
        </div>
      </div>
      <template #footer>
        <button type="button" class="outline" @click="templateDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button
          type="button"
          :disabled="isSendingTemplate"
          :aria-busy="isSendingTemplate ? 'true' : undefined"
          data-spinner="small"
          @click="sendTemplateMessage"
        >
          {{ $t('chat.send') }}
        </button>
      </template>
    </Modal>

    <!-- Canned Response Preview Dialog -->
    <Modal
      id="canned-response-dialog"
      v-model:open="cannedDialogOpen"
      size="small"
      :title="cannedParamNames.length > 0 ? $t('chat.fillParameters') : $t('chat.preview')"
      :description="selectedCannedResponse?.name"
    >
      <div class="vstack gap-4">
        <label v-for="param in cannedParamNames" :key="param" data-field>
          {{ param }}
          <input
            :id="`canned-response-param-${param}`"
            v-model="cannedParamValues[param]"
            data-canned-param
            :placeholder="param"
          />
        </label>
        <div v-if="cannedPreview || cannedPreviewButtons.length" class="vstack gap-1">
          <small class="text-light">{{ $t('chat.preview') }}</small>
          <article id="canned-response-preview" class="card" data-bubble="outgoing">
            <p v-if="cannedPreview" class="pre-wrap">{{ cannedPreview }}</p>
            <PreviewButtonGroup
              v-if="cannedPreviewButtons.length"
              :buttons="cannedPreviewButtons"
              disabled
            />
          </article>
        </div>
      </div>
      <template #footer>
        <button id="canned-response-cancel" type="button" class="outline" @click="cannedDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button
          id="canned-response-send"
          type="button"
          :disabled="isSendingCanned"
          :aria-busy="isSendingCanned ? 'true' : undefined"
          data-spinner="small"
          @click="sendCannedResponse"
        >
          {{ $t('chat.send') }}
        </button>
      </template>
    </Modal>

    <!-- Assign Contact Dialog -->
    <Modal
      v-model:open="isAssignDialogOpen"
      size="small"
      :title="$t('chat.assignContact')"
      :description="$t('chat.assignContactDesc')"
    >
      <div class="vstack gap-2">
        <input
          v-model="assignSearchQuery"
          type="search"
          :placeholder="$t('chat.searchUsers') + '...'"
          :aria-label="$t('chat.searchUsers')"
        />
        <button
          v-if="contactsStore.currentContact?.assigned_user_id"
          type="button"
          class="outline w-100"
          @click="assignContactToUser(null); isAssignDialogOpen = false"
        >
          <UserMinus />
          {{ $t('chat.unassignContact') }}
        </button>
        <hr />
        <div class="scroll-y picker-list">
          <div class="table">
            <table>
              <thead>
                <tr>
                  <th>{{ $t('teams.agent') }}</th>
                  <th>{{ $t('users.role') }}</th>
                  <th :aria-label="$t('common.actions')"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="user in filteredAssignableUsers" :key="user.id">
                  <td>
                    <div class="hstack gap-2 items-center min-w-0">
                      <figure data-variant="avatar" class="small" :aria-label="user.full_name">
                        <abbr :title="user.full_name">{{ getInitials(user.full_name) }}</abbr>
                      </figure>
                      <div class="flex flex-col min-w-0">
                        <strong class="truncate">{{ user.full_name }}</strong>
                        <small class="text-light truncate">{{ user.email }}</small>
                      </div>
                    </div>
                  </td>
                  <td><span class="badge outline">{{ user.role?.name }}</span></td>
                  <td class="align-right">
                    <span
                      v-if="contactsStore.currentContact?.assigned_user_id === user.id"
                      class="badge"
                      data-variant="success"
                    >
                      <Check :size="12" />
                      {{ $t('chat.assigned') }}
                    </span>
                    <button
                      v-else
                      type="button"
                      class="small"
                      :aria-label="`${$t('chat.assign')} ${user.full_name}`"
                      @click="assignContactToUser(user.id); isAssignDialogOpen = false"
                    >
                      {{ $t('chat.assign') }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="filteredAssignableUsers.length === 0" class="align-center text-light p-4">
            <small>{{ $t('chat.noUsersFound') }}</small>
          </p>
        </div>
      </div>
    </Modal>

    <!-- Media Preview Dialog -->
    <Modal v-model:open="isMediaDialogOpen" :title="$t('chat.sendMedia')" :description="selectedFile?.name">
      <div class="vstack gap-4">
        <!-- Image preview -->
        <div v-if="selectedFile?.type.startsWith('image/') && filePreviewUrl" class="flex justify-center">
          <img :src="filePreviewUrl" :alt="selectedFile.name" class="media-preview" />
        </div>
        <!-- Video preview -->
        <div v-else-if="selectedFile?.type.startsWith('video/') && filePreviewUrl" class="flex justify-center">
          <video :src="filePreviewUrl" controls class="media-preview" />
        </div>
        <!-- Audio preview -->
        <div v-else-if="selectedFile?.type.startsWith('audio/')" class="flex items-center gap-2">
          <figure data-variant="avatar" class="shrink-0"><Paperclip :size="20" /></figure>
          <div class="min-w-0">
            <strong class="truncate">{{ selectedFile.name }}</strong>
            <small class="text-light">{{ $t('chat.audioFile') }}</small>
          </div>
        </div>
        <!-- Document preview -->
        <div v-else-if="selectedFile" class="flex items-center gap-2">
          <figure data-variant="avatar" class="shrink-0"><FileText :size="20" /></figure>
          <div class="flex flex-col min-w-0">
            <strong class="truncate">{{ selectedFile.name }}</strong>
            <small class="text-light">{{ (selectedFile.size / 1024).toFixed(1) }} KB</small>
          </div>
        </div>

        <!-- Caption input (not for audio) -->
        <textarea
          v-if="selectedFile && !selectedFile.type.startsWith('audio/')"
          v-model="mediaCaption"
          :placeholder="$t('chat.mediaCaption') + '...'"
          :aria-label="$t('chat.mediaCaption')"
          rows="2"
        />
      </div>
      <template #footer>
        <button type="button" class="outline" :disabled="isUploadingMedia" @click="closeMediaDialog">
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          :disabled="isUploadingMedia"
          :aria-busy="isUploadingMedia ? 'true' : undefined"
          data-spinner="small"
          @click="sendMediaMessage"
        >
          <Send v-if="!isUploadingMedia" />
          {{ isUploadingMedia ? `${$t('chat.sending')}...` : $t('chat.send') }}
        </button>
      </template>
    </Modal>

    <!-- Add Contact Dialog -->
    <CreateContactDialog v-model:open="isAddContactOpen" @created="onContactCreated" />

    <!-- In-app media viewer (lightbox) -->
    <MediaViewerDialog
      v-model:open="mediaViewerOpen"
      v-model:index="mediaViewerIndex"
      :items="viewableMedia"
    />
  </div>
</template>
