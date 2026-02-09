import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { contactsService, messagesService } from '@/services/api'

export interface Contact {
  id: string
  phone_number: string
  name: string
  profile_name?: string
  avatar_url?: string
  status: string
  tags: string[]
  metadata: Record<string, any>
  last_message_at?: string
  unread_count: number
  assigned_user_id?: string
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
  created_at: string
  updated_at: string
}

export const useContactsStore = defineStore('contacts', () => {
  const contacts = ref<Contact[]>([])
  const currentContact = ref<Contact | null>(null)
  const messages = ref<Message[]>([])
  const isLoading = ref(false)
  const isLoadingMessages = ref(false)
  const isLoadingOlderMessages = ref(false)
  const hasMoreMessages = ref(false)
  const searchQuery = ref('')
  const selectedTags = ref<string[]>([])
  const replyingTo = ref<Message | null>(null)

  // Contacts pagination
  const contactsPage = ref(1)
  const contactsLimit = ref(25)
  const contactsTotal = ref(0)
  const isLoadingMoreContacts = ref(false)
  const hasMoreContacts = computed(() => contacts.value.length < contactsTotal.value)

  const filteredContacts = computed(() => {
    if (!searchQuery.value) return contacts.value
    const query = searchQuery.value.toLowerCase()
    return contacts.value.filter(c =>
      c.name.toLowerCase().includes(query) ||
      c.phone_number.includes(query) ||
      (c.profile_name?.toLowerCase().includes(query))
    )
  })

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
      const response = await contactsService.list({
        page: 1,
        limit: contactsLimit.value,
        tags: tagsParam,
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

  function setContactsLimit(limit: number) {
    contactsLimit.value = limit
  }

  async function loadMoreContacts() {
    if (isLoadingMoreContacts.value || !hasMoreContacts.value) return

    isLoadingMoreContacts.value = true
    try {
      const nextPage = contactsPage.value + 1
      const tagsParam = selectedTags.value.length > 0 ? selectedTags.value.join(',') : undefined
      const response = await contactsService.list({
        page: nextPage,
        limit: contactsLimit.value,
        tags: tagsParam
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

  async function fetchMessages(contactId: string, params?: { page?: number; limit?: number }) {
    isLoadingMessages.value = true
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

  async function fetchOlderMessages(contactId: string) {
    if (isLoadingOlderMessages.value || !hasMoreMessages.value || messages.value.length === 0) {
      return
    }

    isLoadingOlderMessages.value = true
    try {
      // Get the oldest message ID for cursor-based pagination
      const oldestMessageId = messages.value[0].id
      const response = await messagesService.list(contactId, { before_id: oldestMessageId })
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

  async function sendMessage(contactId: string, type: string, content: any, replyToMessageId?: string) {
    try {
      const response = await messagesService.send(contactId, { type, content, reply_to_message_id: replyToMessageId })
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

  function setReplyingTo(message: Message | null) {
    replyingTo.value = message
  }

  function clearReplyingTo() {
    replyingTo.value = null
  }

  async function sendTemplate(contactId: string, templateName: string, components?: any[]) {
    try {
      const response = await messagesService.sendTemplate(contactId, {
        template_name: templateName,
        components
      })
      const newMessage = response.data
      // Use addMessage which has duplicate checking (WebSocket may also broadcast this)
      addMessage(newMessage)
      return newMessage
    } catch (error) {
      console.error('Failed to send template:', error)
      throw error
    }
  }

  function addMessage(message: Message) {
    // Check if message already exists
    const exists = messages.value.some(m => m.id === message.id)
    if (!exists) {
      messages.value.push(message)

      // Update contact
      const contact = contacts.value.find(c => c.id === message.contact_id)
      if (contact) {
        contact.last_message_at = message.created_at
        if (message.direction === 'incoming') {
          contact.unread_count++
        }
      }
    }
  }

  function updateMessageStatus(messageId: string, status: string) {
    const message = messages.value.find(m => m.id === messageId)
    if (message) {
      message.status = status
    }
  }

  function setCurrentContact(contact: Contact | null) {
    currentContact.value = contact
    replyingTo.value = null // Clear reply state when switching contacts
    if (contact) {
      contact.unread_count = 0
    }
  }

  function clearMessages() {
    messages.value = []
    hasMoreMessages.value = false
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
    replyingTo,
    filteredContacts,
    sortedContacts,
    // Contacts pagination
    contactsLimit,
    contactsTotal,
    hasMoreContacts,
    isLoadingMoreContacts,
    setContactsLimit,
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
    setReplyingTo,
    clearReplyingTo,
    updateMessageReactions,
    updateContactTags
  }
})
