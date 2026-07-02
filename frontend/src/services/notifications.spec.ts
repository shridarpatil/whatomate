import { describe, it, expect } from 'vitest'
import { shouldNotifyIncoming } from '@/services/notifications'

const ME = 'user-1'
const OTHER = 'user-2'

describe('shouldNotifyIncoming', () => {
  it('alerts on an incoming message assigned to the current user', () => {
    expect(
      shouldNotifyIncoming({ direction: 'incoming', assigned_user_id: ME }, ME, {}, false)
    ).toBe(true)
  })

  it('alerts on an incoming message that is unassigned', () => {
    expect(
      shouldNotifyIncoming({ direction: 'incoming', assigned_user_id: null }, ME, {}, false)
    ).toBe(true)
    expect(
      shouldNotifyIncoming({ direction: 'incoming' }, ME, {}, false)
    ).toBe(true)
  })

  it('stays quiet on chats assigned to another agent', () => {
    expect(
      shouldNotifyIncoming({ direction: 'incoming', assigned_user_id: OTHER }, ME, {}, false)
    ).toBe(false)
  })

  it('does not alert for outgoing messages', () => {
    expect(
      shouldNotifyIncoming({ direction: 'outgoing', assigned_user_id: ME }, ME, {}, false)
    ).toBe(false)
  })

  it('does not alert while the user is viewing the chat', () => {
    expect(
      shouldNotifyIncoming({ direction: 'incoming', assigned_user_id: ME }, ME, {}, true)
    ).toBe(false)
  })

  it('respects the new_message_alerts opt-out', () => {
    expect(
      shouldNotifyIncoming(
        { direction: 'incoming', assigned_user_id: ME },
        ME,
        { new_message_alerts: false },
        false
      )
    ).toBe(false)
  })

  it('defaults to alerting when new_message_alerts is unset', () => {
    expect(
      shouldNotifyIncoming(
        { direction: 'incoming', assigned_user_id: ME },
        ME,
        { new_message_alerts: undefined },
        false
      )
    ).toBe(true)
  })
})
