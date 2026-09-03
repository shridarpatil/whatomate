import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { contactsService, messagesService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'

// Phones are stored without leading + or whitespace (see CreateContact in
// internal/handlers/contacts.go). Strip them from a digit-only query so a user
// typing "+91 98765 43210" still matches a stored "919876543210" via the
// server's substring LIKE.
function normalizeContactSearch(raw: string): string {
  const trimmed = raw.trim().replace(/^\+/, '')
  if (trimmed && /^[\d\s+()-]+$/.test(trimmed)) {
    return trimmed.replace(/[\s+()-]/g, '')
  }
  return trimmed
}

export type ContactStatus = 'new' | 'in_progress' | 'resolved'
export type ContactStatusFilter = 'all' | ContactStatus

export interface Contact {
  id: string
  phone_number: string
  name: string
  profile_name?: string
  avatar_url?: string
  // `status` is legacy and always "active"; contact_status is the real
  // service state of the conversation.
  status: string
  contact_status: ContactStatus
  last_message_preview?: string
  tags: string[]
  metadata: Record<string, any>
  last_message_at?: string
  last_inbound_at?: string
  service_window_open?: boolean
  unread_count: number
  assigned_user_id?: string
  assigned_user_name?: string
  whatsapp_account?: string
  marketing_opt_out?: boolean
  created_at: string
  updated_at: string
}

export interface ReplyPreview {
  id: string
  content: any
  message_type: string
  direction: 'incoming' | 'outgoing'
}

export interface Reaction {
  emoji: string
  from_phone?: string
  from_user?: string
}

export interface Message {
  id: string
  contact_id: string
  direction: 'incoming' | 'outgoing'
  message_type: string
  content: any
  media_url?: string
  media_mime_type?: string
  media_filename?: string
  interactive_data?: {
    type?: string
    body?: string
    buttons?: Array<{
      type?: string
      reply?: { id: string; title: string }
      id?: string
      title?: string
    }>
    rows?: Array<{
      id?: string
      title?: string
    }>
  }
  status: string
  wamid?: string
  error_message?: string
  is_reply?: boolean
  reply_to_message_id?: string
  reply_to_message?: ReplyPreview
  reactions?: Reaction[]
  whatsapp_account?: string
  sent_by_user_id?: string
  sent_by_user_name?: string
  created_at: string
  updated_at: string
}

export interface TypingAgent {
  user_id: string
  user_name: string
  at: number
}

export const useContactsStore = defineStore('contacts', () => {
  const authStore = useAuthStore()
  const contacts = ref<Contact[]>([])
  const currentContact = ref<Contact | null>(null)
  const messages = ref<Message[]>([])
  const isLoading = ref(false)
  const isLoadingMessages = ref(false)
  const isLoadingOlderMessages = ref(false)
  const hasMoreMessages = ref(false)
  const searchQuery = ref('')
  const selectedTags = ref<string[]>([])
  const statusFilter = ref<ContactStatusFilter>('all')
  const newCount = ref(0)
  const replyingTo = ref<Message | null>(null)
  const accountFilter = ref<string | null>(null)

  // Record, not Map: Map is not reactive in Vue 3.
  const typingByContact = ref<Record<string, TypingAgent>>({})

  // Timer handles live outside the ref on purpose — they are scheduling
  // detail, not UI state, and must not trigger re-renders.
  const typingTimers: Record<string, ReturnType<typeof setTimeout>> = {}

  const TYPING_TTL_MS = 3000

  // Contacts pagination
  const contactsPage = ref(1)
  const contactsLimit = ref(50)
  const contactsTotal = ref(0)
  const isLoadingMoreContacts = ref(false)
  const hasMoreContacts = computed(() => contacts.value.length < contactsTotal.value)

  // Search is now driven server-side via fetchContacts({ search }), so the
  // visible list is whatever the server returned — no extra local filtering.
  const filteredContacts = computed(() => contacts.value)

  const sortedContacts = computed(() => {
    return [...filteredContacts.value].sort((a, b) => {
      const dateA = a.last_message_at ? new Date(a.last_message_at).getTime() : 0
      const dateB = b.last_message_at ? new Date(b.last_message_at).getTime() : 0
      return dateB - dateA
    })
  })

  async function fetchContacts(params?: { search?: string; page?: number; limit?: number; tags?: string }) {
    isLoading.value = true
    try {
      const tagsParam = selectedTags.value.length > 0 ? selectedTags.value.join(',') : undefined
      const statusParam = statusFilter.value === 'all' ? undefined : statusFilter.value
      const response = await contactsService.list({
        page: 1,
        limit: contactsLimit.value,
        tags: tagsParam,
        status: statusParam,
        ...params
      })
      // API returns { status: "success", data: { contacts: [...], total: number } }
      const data = response.data.data || response.data
      contacts.value = data.contacts || []
      contactsTotal.value = data.total ?? contacts.value.length
      contactsPage.value = 1
    } catch (error) {
      console.error('Failed to fetch contacts:', error)
    } finally {
      isLoading.value = false
    }
  }

  async function loadMoreContacts() {
    if (isLoadingMoreContacts.value || !hasMoreContacts.value) return

    isLoadingMoreContacts.value = true
    try {
      const nextPage = contactsPage.value + 1
      const tagsParam = selectedTags.value.length > 0 ? selectedTags.value.join(',') : undefined
      const search = normalizeContactSearch(searchQuery.value) || undefined
      // The status filter has to travel with pagination too, or scrolling
      // silently mixes other statuses back into a filtered list.
      const statusParam = statusFilter.value === 'all' ? undefined : statusFilter.value
      const response = await contactsService.list({
        page: nextPage,
        limit: contactsLimit.value,
        tags: tagsParam,
        status: statusParam,
        search
      })
      const data = response.data.data || response.data
      const newContacts = data.contacts || []

      if (newContacts.length > 0) {
        // Append new contacts, avoiding duplicates
        const existingIds = new Set(contacts.value.map(c => c.id))
        const uniqueNew = newContacts.filter((c: Contact) => !existingIds.has(c.id))
        contacts.value = [...contacts.value, ...uniqueNew]
        contactsPage.value = nextPage
      }
      contactsTotal.value = data.total ?? contactsTotal.value
    } catch (error) {
      console.error('Failed to load more contacts:', error)
    } finally {
      isLoadingMoreContacts.value = false
    }
  }

  async function fetchContact(id: string) {
    try {
      const response = await contactsService.get(id)
      // API returns { status: "success", data: { ... } }
      const data = response.data.data || response.data
      currentContact.value = data
      return data
    } catch (error) {
      console.error('Failed to fetch contact:', error)
      return null
    }
  }

  async function fetchMessages(contactId: string, params?: { page?: number; limit?: number; account?: string }) {
    isLoadingMessages.value = true
    // Drop the previous contact's messages immediately so the list doesn't show
    // stale content under the new contact's header while the fetch is in flight.
    messages.value = []
    try {
      const response = await messagesService.list(contactId, params)
      // API returns { status: "success", data: { messages: [...], has_more: boolean } }
      const data = response.data.data || response.data
      messages.value = data.messages || []
      hasMoreMessages.value = data.has_more === true
    } catch (error) {
      console.error('Failed to fetch messages:', error)
    } finally {
      isLoadingMessages.value = false
    }
  }

  async function fetchOlderMessages(contactId: string, account?: string) {
    if (isLoadingOlderMessages.value || !hasMoreMessages.value || messages.value.length === 0) {
      return
    }

    isLoadingOlderMessages.value = true
    try {
      // Get the oldest message ID for cursor-based pagination
      const oldestMessageId = messages.value[0].id
      const response = await messagesService.list(contactId, { before_id: oldestMessageId, account })
      const data = response.data.data || response.data
      const olderMessages = data.messages || []

      if (olderMessages.length > 0) {
        // Prepend older messages (they come in chronological order, oldest first)
        messages.value = [...olderMessages, ...messages.value]
      }
      hasMoreMessages.value = data.has_more === true
    } catch (error) {
      console.error('Failed to fetch older messages:', error)
    } finally {
      isLoadingOlderMessages.value = false
    }
  }

  async function sendMessage(
    contactId: string,
    type: string,
    content: any,
    replyToMessageId?: string,
    whatsappAccount?: string,
    extra?: { interactive?: Parameters<typeof messagesService.send>[1]['interactive'] },
  ) {
    try {
      const response = await messagesService.send(contactId, {
        type,
        content,
        reply_to_message_id: replyToMessageId,
        whatsapp_account: whatsappAccount,
        ...(extra?.interactive ? { interactive: extra.interactive } : {}),
      })
      // API returns { status: "success", data: { ... } }
      const newMessage = response.data.data || response.data
      // Use addMessage which has duplicate checking (WebSocket may also broadcast this)
      addMessage(newMessage)

      return newMessage
    } catch (error) {
      console.error('Failed to send message:', error)
      throw error
    }
  }

  async function sendTemplate(
    contactId: string,
    templateName: string,
    templateParams?: Record<string, string>,
    accountName?: string,
    headerFile?: File,
    buttonParams?: Record<string, string>,
    headerParams?: Record<string, string>
  ) {
    try {
      const response = await messagesService.sendTemplate(contactId, {
        template_name: templateName,
        template_params: templateParams,
        header_params: headerParams,
        button_params: buttonParams,
        account_name: accountName
      }, headerFile)
      const data = response.data.data || response.data
      // Use addMessage which has duplicate checking (WebSocket may also broadcast this)
      addMessage(data)
      return data
    } catch (error) {
      console.error('Failed to send template:', error)
      throw error
    }
  }

  function setReplyingTo(message: Message | null) {
    replyingTo.value = message
  }

  function clearReplyingTo() {
    replyingTo.value = null
  }

  function addMessage(message: Message) {
    // Update contact metadata regardless of account filter
    const contact = contacts.value.find(c => c.id === message.contact_id)
    if (contact) {
      contact.last_message_at = message.created_at
      if (message.direction === 'incoming') {
        contact.unread_count++
        contact.last_inbound_at = message.created_at
        contact.service_window_open = true
      }
    }
    // Also update currentContact if it matches
    if (currentContact.value && currentContact.value.id === message.contact_id && message.direction === 'incoming') {
      currentContact.value.last_inbound_at = message.created_at
      currentContact.value.service_window_open = true
    }

    // Skip adding to messages array if account filter is active and doesn't match
    if (accountFilter.value && message.whatsapp_account && message.whatsapp_account !== accountFilter.value) {
      return
    }

    // Check if message already exists
    const index = messages.value.findIndex(m => m.id === message.id)
    if (index === -1) {
      messages.value.push(message)
      return
    }

    // The same message reaches us twice on the sender's own client: once from
    // the POST /messages response (whose MessageResponse omits the sender
    // identity) and once from the org-wide websocket broadcast (which carries
    // it). Whichever lands second must be able to fill the gap, or the
    // sender's own bubble stays unlabelled until a reload.
    if (message.sent_by_user_name && !messages.value[index].sent_by_user_name) {
      messages.value[index] = {
        ...messages.value[index],
        sent_by_user_id: message.sent_by_user_id ?? messages.value[index].sent_by_user_id,
        sent_by_user_name: message.sent_by_user_name,
      }
    }
  }

  function updateMessageStatus(messageId: string, status: string, errorMessage?: string) {
    const index = messages.value.findIndex(m => m.id === messageId)
    if (index !== -1) {
      messages.value[index] = {
        ...messages.value[index],
        status,
        ...(errorMessage ? { error_message: errorMessage } : {})
      }
    }
  }

  function setCurrentContact(contact: Contact | null) {
    currentContact.value = contact
    replyingTo.value = null // Clear reply state when switching contacts
    if (contact) {
      contact.unread_count = 0
    }
  }

  function setAccountFilter(account: string | null) {
    accountFilter.value = account
  }

  function clearMessages() {
    messages.value = []
    hasMoreMessages.value = false
    accountFilter.value = null
  }

  function updateMessageReactions(messageId: string, reactions: Reaction[]) {
    const message = messages.value.find(m => m.id === messageId)
    if (message) {
      message.reactions = reactions
    }
  }

  function updateContactTags(contactId: string, tags: string[]) {
    // Update in contacts list
    const contact = contacts.value.find(c => c.id === contactId)
    if (contact) {
      contact.tags = tags
    }
    // Update current contact if it matches
    if (currentContact.value?.id === contactId) {
      currentContact.value = { ...currentContact.value, tags }
    }
  }

  function updateContactName(contactId: string, name: string) {
    // The API returns the same value in both `name` and `profile_name`, and
    // different parts of the UI read one or the other — sync both or the
    // name changes in the panel but not in the conversation list.
    const contact = contacts.value.find(c => c.id === contactId)
    if (contact) {
      contact.name = name
      contact.profile_name = name
    }
    if (currentContact.value?.id === contactId) {
      currentContact.value = { ...currentContact.value, name, profile_name: name }
    }
  }

  // Debounce server-side search so each keystroke doesn't fire a request.
  let searchDebounceHandle: ReturnType<typeof setTimeout> | null = null
  watch(searchQuery, (query) => {
    if (searchDebounceHandle) clearTimeout(searchDebounceHandle)
    searchDebounceHandle = setTimeout(() => {
      const search = normalizeContactSearch(query) || undefined
      fetchContacts({ search })
    }, 300)
  })

  async function setStatusFilter(status: ContactStatusFilter) {
    if (statusFilter.value === status) return
    statusFilter.value = status
    const search = normalizeContactSearch(searchQuery.value) || undefined
    await fetchContacts({ search })
  }

  async function fetchStatusCounts() {
    try {
      const response = await contactsService.statusCounts()
      const data = response.data.data || response.data
      newCount.value = data.new ?? 0
    } catch (error) {
      console.error('Failed to fetch status counts:', error)
    }
  }

  async function updateContactStatus(id: string, status: ContactStatus) {
    await contactsService.updateStatus(id, status)
    // The WebSocket event does the list bookkeeping. Update the open chat
    // straight away so the header button does not lag behind the click.
    if (currentContact.value?.id === id) {
      currentContact.value.contact_status = status
    }
  }

  // applyStatusChange reconciles a contact_status_changed event with the list.
  function applyStatusChange(payload: {
    contact_id: string
    old_status: string
    new_status: string
  }) {
    const status = payload.new_status as ContactStatus

    if (payload.old_status === 'new' && status !== 'new') {
      newCount.value = Math.max(0, newCount.value - 1)
    } else if (payload.old_status !== 'new' && status === 'new') {
      newCount.value += 1
    }

    const contact = contacts.value.find(c => c.id === payload.contact_id)
    if (contact) {
      contact.contact_status = status
      // Drop it from the list once it no longer matches the active filter.
      if (statusFilter.value !== 'all' && statusFilter.value !== status) {
        contacts.value = contacts.value.filter(c => c.id !== payload.contact_id)
        contactsTotal.value = Math.max(0, contactsTotal.value - 1)
      }
    }

    if (currentContact.value?.id === payload.contact_id) {
      currentContact.value.contact_status = status
    }
  }

  function clearTyping(contactId: string) {
    if (typingTimers[contactId]) {
      clearTimeout(typingTimers[contactId])
      delete typingTimers[contactId]
    }
    if (typingByContact.value[contactId]) {
      delete typingByContact.value[contactId]
    }
  }

  // applyAgentTyping records that an agent is typing on a contact and schedules
  // its expiry. Each new event restarts the countdown, so no explicit
  // "stopped typing" signal is needed — and a closed tab or dropped connection
  // cannot leave the indicator stuck.
  function applyAgentTyping(payload: {
    contact_id: string
    user_id: string
    user_name: string
  }) {
    // Never show the current user their own typing
    if (payload.user_id === authStore.user?.id) return

    typingByContact.value[payload.contact_id] = {
      user_id: payload.user_id,
      user_name: payload.user_name,
      at: Date.now()
    }

    if (typingTimers[payload.contact_id]) {
      clearTimeout(typingTimers[payload.contact_id])
    }
    typingTimers[payload.contact_id] = setTimeout(() => {
      delete typingByContact.value[payload.contact_id]
      delete typingTimers[payload.contact_id]
    }, TYPING_TTL_MS)
  }

  return {
    contacts,
    currentContact,
    messages,
    isLoading,
    isLoadingMessages,
    isLoadingOlderMessages,
    hasMoreMessages,
    searchQuery,
    selectedTags,
    statusFilter,
    newCount,
    setStatusFilter,
    fetchStatusCounts,
    updateContactStatus,
    applyStatusChange,
    replyingTo,
    filteredContacts,
    sortedContacts,
    // Contacts pagination
    contactsTotal,
    hasMoreContacts,
    isLoadingMoreContacts,
    fetchContacts,
    loadMoreContacts,
    // Other
    fetchContact,
    fetchMessages,
    fetchOlderMessages,
    sendMessage,
    sendTemplate,
    addMessage,
    updateMessageStatus,
    setCurrentContact,
    clearMessages,
    setAccountFilter,
    setReplyingTo,
    clearReplyingTo,
    updateMessageReactions,
    updateContactTags,
    updateContactName,
    typingByContact,
    applyAgentTyping,
    clearTyping
  }
})
