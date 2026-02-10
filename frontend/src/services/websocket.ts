import { useContactsStore } from '@/stores/contacts'
import { useTransfersStore } from '@/stores/transfers'
import { useAuthStore } from '@/stores/auth'
import { useNotesStore } from '@/stores/notes'
import { toast } from 'vue-sonner'
import router from '@/router'

// Notification sound
let notificationSound: HTMLAudioElement | null = null

function playNotificationSound() {
  if (!notificationSound) {
    notificationSound = new Audio('/notification.mp3')
    notificationSound.volume = 0.5
  }
  notificationSound.currentTime = 0
  notificationSound.play().catch(() => {
    // Ignore autoplay errors (browser may block until user interaction)
  })
}

// Show toast notification with click handler
function showNotification(title: string, body: string, contactId: string) {
  toast.info(title, {
    description: body,
    duration: 5000,
    action: {
      label: 'View',
      onClick: () => {
        router.push(`/chat/${contactId}`)
      },
      actionButtonStyle: {
        background: 'transparent',
        border: '1px solid #e5e7eb',
        color: '#3b82f6',
        fontWeight: '500'
      }
    }
  })
}

// WebSocket message types
const WS_TYPE_NEW_MESSAGE = 'new_message'
const WS_TYPE_STATUS_UPDATE = 'status_update'
const WS_TYPE_SET_CONTACT = 'set_contact'
const WS_TYPE_PING = 'ping'
const WS_TYPE_PONG = 'pong'

// Reaction types
const WS_TYPE_REACTION_UPDATE = 'reaction_update'

// Agent transfer types
const WS_TYPE_AGENT_TRANSFER = 'agent_transfer'
const WS_TYPE_AGENT_TRANSFER_RESUME = 'agent_transfer_resume'
const WS_TYPE_AGENT_TRANSFER_ASSIGN = 'agent_transfer_assign'
const WS_TYPE_TRANSFER_ESCALATION = 'transfer_escalation'

// Campaign types
const WS_TYPE_CAMPAIGN_STATS_UPDATE = 'campaign_stats_update'

// Permission types
const WS_TYPE_PERMISSIONS_UPDATED = 'permissions_updated'

// Conversation note types
const WS_TYPE_CONVERSATION_NOTE_CREATED = 'conversation_note_created'
const WS_TYPE_CONVERSATION_NOTE_UPDATED = 'conversation_note_updated'
const WS_TYPE_CONVERSATION_NOTE_DELETED = 'conversation_note_deleted'

interface WSMessage {
  type: string
  payload: any
}

class WebSocketService {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private pingInterval: number | null = null
  private isConnected = false
  private hasConnectedBefore = false
  private campaignStatsCallbacks: ((payload: any) => void)[] = []
  private getTokenFn: (() => Promise<string | null>) | null = null

  async connect(getToken?: () => Promise<string | null>) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return
    }

    // Store the token function for reconnects
    if (getToken) {
      this.getTokenFn = getToken
    }

    // Get a fresh short-lived WS token
    const token = this.getTokenFn ? await this.getTokenFn() : null
    if (!token) {
      return
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
    const url = `${protocol}//${host}${basePath}/ws?token=${token}`

    try {
      this.ws = new WebSocket(url)

      this.ws.onopen = () => {
        const isReconnection = this.hasConnectedBefore
        this.isConnected = true
        this.hasConnectedBefore = true
        this.reconnectAttempts = 0
        this.startPing()

        // Force refresh data after reconnection to sync any missed updates
        if (isReconnection) {
          this.refreshStaleData()
        }
      }

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }

      this.ws.onclose = () => {
        this.isConnected = false
        this.stopPing()
        this.handleReconnect()
      }

      this.ws.onerror = () => {
        // Error handled by onclose
      }
    } catch {
      this.handleReconnect()
    }
  }

  disconnect() {
    this.stopPing()
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.isConnected = false
    this.reconnectAttempts = this.maxReconnectAttempts // Prevent reconnect
  }

  private handleMessage(data: string) {
    try {
      const message: WSMessage = JSON.parse(data)
      const store = useContactsStore()

      switch (message.type) {
        case WS_TYPE_NEW_MESSAGE:
          this.handleNewMessage(store, message.payload)
          break
        case WS_TYPE_STATUS_UPDATE:
          this.handleStatusUpdate(store, message.payload)
          break
        case WS_TYPE_AGENT_TRANSFER:
          this.handleAgentTransfer(message.payload)
          break
        case WS_TYPE_AGENT_TRANSFER_RESUME:
          this.handleAgentTransferResume(message.payload)
          break
        case WS_TYPE_AGENT_TRANSFER_ASSIGN:
          this.handleAgentTransferAssign(message.payload)
          break
        case WS_TYPE_TRANSFER_ESCALATION:
          this.handleTransferEscalation(message.payload)
          break
        case WS_TYPE_REACTION_UPDATE:
          this.handleReactionUpdate(store, message.payload)
          break
        case WS_TYPE_PONG:
          // Pong received, connection is alive
          break
        case WS_TYPE_CAMPAIGN_STATS_UPDATE:
          this.handleCampaignStatsUpdate(message.payload)
          break
        case WS_TYPE_PERMISSIONS_UPDATED:
          this.handlePermissionsUpdated()
          break
        case WS_TYPE_CONVERSATION_NOTE_CREATED:
          useNotesStore().addNote(message.payload)
          break
        case WS_TYPE_CONVERSATION_NOTE_UPDATED:
          useNotesStore().onNoteUpdated(message.payload)
          break
        case WS_TYPE_CONVERSATION_NOTE_DELETED:
          useNotesStore().onNoteDeleted(message.payload.id)
          break
        default:
          // Unknown message type, ignore
          break
      }
    } catch {
      // Failed to parse message, ignore
    }
  }

  private handleNewMessage(store: ReturnType<typeof useContactsStore>, payload: any) {
    // Check if this message is for the current contact
    const currentContact = store.currentContact
    const isViewingThisContact = currentContact && payload.contact_id === currentContact.id

    if (isViewingThisContact) {
      // Add message to the store
      store.addMessage({
        id: payload.id,
        contact_id: payload.contact_id,
        direction: payload.direction,
        message_type: payload.message_type,
        content: payload.content,
        media_url: payload.media_url,
        media_mime_type: payload.media_mime_type,
        media_filename: payload.media_filename,
        interactive_data: payload.interactive_data,
        status: payload.status,
        wamid: payload.wamid,
        error_message: payload.error_message,
        is_reply: payload.is_reply,
        reply_to_message_id: payload.reply_to_message_id,
        reply_to_message: payload.reply_to_message,
        reactions: payload.reactions,
        created_at: payload.created_at,
        updated_at: payload.updated_at
      })
    }

    // Show toast notification for incoming messages if:
    // 1. Message is incoming (from customer, not chatbot/agent)
    // 2. Current user is assigned to this contact
    // 3. User has new_message_alerts enabled
    // 4. User is not currently viewing this contact
    if (payload.direction === 'incoming' && !isViewingThisContact) {
      const authStore = useAuthStore()
      const currentUserId = authStore.user?.id
      const settings = authStore.userSettings

      // Check if user is assigned to this contact
      const isAssignedToUser = payload.assigned_user_id === currentUserId

      // Check if new message alerts are enabled (default to true if not set)
      const alertsEnabled = settings.new_message_alerts !== false

      if (isAssignedToUser && alertsEnabled) {
        const senderName = payload.profile_name || 'Unknown'
        const messagePreview = payload.content?.body || 'New message'
        const preview = messagePreview.length > 50
          ? messagePreview.substring(0, 50) + '...'
          : messagePreview
        const contactId = payload.contact_id

        // Play notification sound and show browser notification
        playNotificationSound()
        showNotification(senderName, preview, contactId)
      }
    }

    // Update contacts list (for unread count, last message preview)
    store.fetchContacts()
  }

  private handleStatusUpdate(store: ReturnType<typeof useContactsStore>, payload: any) {
    store.updateMessageStatus(payload.message_id, payload.status)
  }

  private handleReactionUpdate(store: ReturnType<typeof useContactsStore>, payload: any) {
    // Update the message reactions if we're viewing the contact
    const currentContact = store.currentContact
    if (currentContact && payload.contact_id === currentContact.id) {
      store.updateMessageReactions(payload.message_id, payload.reactions)
    }
  }

  private handleAgentTransfer(payload: any) {
    const transfersStore = useTransfersStore()
    const authStore = useAuthStore()

    // Add transfer to store with default SLA values
    transfersStore.addTransfer({
      id: payload.id,
      contact_id: payload.contact_id,
      contact_name: payload.contact_name || payload.phone_number,
      phone_number: payload.phone_number,
      whatsapp_account: payload.whatsapp_account,
      status: payload.status,
      source: payload.source || 'manual',
      agent_id: payload.agent_id,
      team_id: payload.team_id,
      notes: payload.notes,
      transferred_at: payload.transferred_at,
      // Default SLA values - will be updated on next fetch
      sla_breached: false,
      escalation_level: 0
    })

    // Refresh to get complete data including SLA fields
    transfersStore.fetchTransfers()

    // Show toast notification for admin/manager or assigned agent
    const userRole = authStore.user?.role?.name
    const currentUserId = authStore.user?.id
    const isAssignedToMe = payload.agent_id === currentUserId

    if (userRole === 'admin' || userRole === 'manager' || isAssignedToMe) {
      const contactName = payload.contact_name || payload.phone_number
      toast.info('New Transfer', {
        description: `${contactName} has been transferred to ${isAssignedToMe ? 'you' : 'agent queue'}`,
        duration: 5000,
        action: {
          label: 'View',
          onClick: () => router.push('/chatbot/transfers')
        }
      })
    }
  }

  private handleAgentTransferResume(payload: any) {
    const transfersStore = useTransfersStore()

    const updated = transfersStore.updateTransfer(payload.id, {
      status: payload.status,
      resumed_at: payload.resumed_at,
      resumed_by: payload.resumed_by
    })

    // If transfer wasn't found in store, refresh to get latest data
    if (!updated) {
      transfersStore.fetchTransfers()
    }
  }

  private handleAgentTransferAssign(payload: any) {
    const transfersStore = useTransfersStore()
    const authStore = useAuthStore()

    // Try to update existing transfer
    transfersStore.updateTransfer(payload.id, {
      agent_id: payload.agent_id,
      team_id: payload.team_id
    })

    // Always refresh to ensure UI is in sync (queue counts, etc.)
    transfersStore.fetchTransfers()

    // Notify if assigned to current user
    const currentUserId = authStore.user?.id
    if (payload.agent_id === currentUserId) {
      toast.info('Transfer Assigned', {
        description: 'A transfer has been assigned to you',
        duration: 5000,
        action: {
          label: 'View',
          onClick: () => router.push('/chatbot/transfers')
        }
      })
    }
  }

  private handleTransferEscalation(payload: any) {
    const authStore = useAuthStore()
    const currentUserId = authStore.user?.id

    // Check if current user should be notified
    const notifyIds: string[] = payload.escalation_notify_ids || []
    const shouldNotify = notifyIds.includes(currentUserId || '')

    // Also notify admins/managers
    const userRole = authStore.user?.role?.name
    const isAdminOrManager = userRole === 'admin' || userRole === 'manager'

    if (shouldNotify || isAdminOrManager) {
      const levelName = payload.level_name === 'critical' ? 'Critical' : 'Warning'
      const contactName = payload.contact_name || payload.phone_number

      // Play notification sound
      playNotificationSound()

      // Show urgent toast
      toast.warning(`SLA Escalation: ${levelName}`, {
        description: `${contactName} has been waiting since ${new Date(payload.waiting_since).toLocaleTimeString()}`,
        duration: 10000,
        action: {
          label: 'View',
          onClick: () => router.push('/chatbot/transfers')
        }
      })
    }
  }

  private handleCampaignStatsUpdate(payload: any) {
    // Notify all registered callbacks
    this.campaignStatsCallbacks.forEach(callback => callback(payload))
  }

  private async handlePermissionsUpdated() {
    const authStore = useAuthStore()

    // Refresh user data from server
    const success = await authStore.refreshUserData()

    if (success) {
      toast.info('Permissions Updated', {
        description: 'Your permissions have been updated. The page will refresh.',
        duration: 3000
      })

      // Reload the page after a short delay to apply new permissions
      setTimeout(() => {
        window.location.reload()
      }, 1500)
    }
  }

  onCampaignStatsUpdate(callback: (payload: any) => void) {
    this.campaignStatsCallbacks.push(callback)
    // Return unsubscribe function
    return () => {
      const index = this.campaignStatsCallbacks.indexOf(callback)
      if (index > -1) {
        this.campaignStatsCallbacks.splice(index, 1)
      }
    }
  }

  private handleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      return
    }

    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)

    setTimeout(() => {
      this.connect()
    }, delay)
  }

  setCurrentContact(contactId: string | null) {
    this.send({
      type: WS_TYPE_SET_CONTACT,
      payload: { contact_id: contactId || '' }
    })
  }

  private send(message: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message))
    }
  }

  private startPing() {
    this.stopPing()
    this.pingInterval = window.setInterval(() => {
      this.send({ type: WS_TYPE_PING, payload: {} })
    }, 30000) // Ping every 30 seconds
  }

  private stopPing() {
    if (this.pingInterval) {
      clearInterval(this.pingInterval)
      this.pingInterval = null
    }
  }

  private refreshStaleData() {
    // Refresh contacts list
    const contactsStore = useContactsStore()
    contactsStore.fetchContacts()

    // Refresh transfers
    const transfersStore = useTransfersStore()
    transfersStore.fetchTransfers()

    // Show subtle notification
    toast.info('Connection restored', {
      description: 'Data has been refreshed',
      duration: 3000
    })
  }

  getIsConnected() {
    return this.isConnected
  }
}

// Export singleton instance
export const wsService = new WebSocketService()
