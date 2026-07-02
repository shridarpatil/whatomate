// Pure notification-policy helpers, split out from websocket.ts so they can be
// unit tested without pulling in the router/store/DOM graph.

export interface IncomingMessagePayload {
  direction?: string
  assigned_user_id?: string | null
}

export interface AlertSettings {
  new_message_alerts?: boolean
}

// Decide whether an incoming message should alert the current user.
// Alert when the chat is assigned to this user or unassigned, and the user has
// new-message alerts enabled — stay quiet on chats another agent owns, and when
// the user is already viewing the chat.
export function shouldNotifyIncoming(
  payload: IncomingMessagePayload,
  currentUserId: string | undefined,
  settings: AlertSettings,
  isViewingThisContact: boolean | null | undefined
): boolean {
  if (payload.direction !== 'incoming' || isViewingThisContact) return false

  const isAssignedToUser = payload.assigned_user_id === currentUserId
  const isUnassigned = !payload.assigned_user_id
  const shouldAlert = isAssignedToUser || isUnassigned

  // Default to true when the setting is unset
  const alertsEnabled = settings.new_message_alerts !== false

  return shouldAlert && alertsEnabled
}
