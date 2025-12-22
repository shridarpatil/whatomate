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
  custom_fields: Record<string, any>
  last_message_at?: string
  unread_count: number
  created_at: string
  updated_at: string
}

export interface Message {
  id: string
  contact_id: string
  direction: 'incoming' | 'outgoing'
  message_type: string
  content: any
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
  created_at: string
  updated_at: string
}

export const useContactsStore = defineStore('contacts', () => {
  const contacts = ref<Contact[]>([])
  const currentContact = ref<Contact | null>(null)
  const messages = ref<Message[]>([])
  const isLoading = ref(false)
  const isLoadingMessages = ref(false)
  const searchQuery = ref('')

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

  async function fetchContacts(params?: { search?: string; page?: number; limit?: number }) {
    isLoading.value = true
    try {
      const response = await contactsService.list(params)
      // API returns { status: "success", data: { contacts: [...] } }
      const data = response.data.data || response.data
      contacts.value = data.contacts || []
    } catch (error) {
      console.error('Failed to fetch contacts:', error)
    } finally {
      isLoading.value = false
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
      // API returns { status: "success", data: { messages: [...] } }
      const data = response.data.data || response.data
      messages.value = data.messages || []
    } catch (error) {
      console.error('Failed to fetch messages:', error)
    } finally {
      isLoadingMessages.value = false
    }
  }

  async function sendMessage(contactId: string, type: string, content: any) {
    try {
      const response = await messagesService.send(contactId, { type, content })
      // API returns { status: "success", data: { ... } }
      const newMessage = response.data.data || response.data
      messages.value.push(newMessage)

      // Update contact's last message
      const contact = contacts.value.find(c => c.id === contactId)
      if (contact) {
        contact.last_message_at = newMessage.created_at
      }

      return newMessage
    } catch (error) {
      console.error('Failed to send message:', error)
      throw error
    }
  }

  async function sendTemplate(contactId: string, templateName: string, components?: any[]) {
    try {
      const response = await messagesService.sendTemplate(contactId, {
        template_name: templateName,
        components
      })
      const newMessage = response.data
      messages.value.push(newMessage)
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
    if (contact) {
      contact.unread_count = 0
    }
  }

  function clearMessages() {
    messages.value = []
  }

  return {
    contacts,
    currentContact,
    messages,
    isLoading,
    isLoadingMessages,
    searchQuery,
    filteredContacts,
    sortedContacts,
    fetchContacts,
    fetchContact,
    fetchMessages,
    sendMessage,
    sendTemplate,
    addMessage,
    updateMessageStatus,
    setCurrentContact,
    clearMessages
  }
})
