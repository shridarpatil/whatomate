import { describe, it, expect } from 'vitest'
import { campaignTone, templateTone, humanize, roleLabel } from './status'

describe('campaignTone', () => {
  it('maps campaign states to tones', () => {
    expect(campaignTone('scheduled')).toBe('neutral')
    expect(campaignTone('processing')).toBe('info')
    expect(campaignTone('paused')).toBe('warning')
    expect(campaignTone('completed')).toBe('success')
    expect(campaignTone('failed')).toBe('danger')
    expect(campaignTone('unknown')).toBe('neutral')
  })
  it('flags a completed campaign with failed messages', () => {
    expect(campaignTone('completed', 3)).toBe('warning')
  })
})

describe('templateTone', () => {
  it('maps Meta template states, case-insensitively', () => {
    expect(templateTone('APPROVED')).toBe('success')
    expect(templateTone('pending')).toBe('warning')
    expect(templateTone('REJECTED')).toBe('danger')
    expect(templateTone('')).toBe('neutral')
  })
})

describe('humanize', () => {
  it('turns codes into sentence case', () => {
    expect(humanize('UTILITY')).toBe('Utility')
    expect(humanize('in_review')).toBe('In review')
    expect(humanize('agent')).toBe('Agent')
    expect(humanize(null)).toBe('')
  })
})

describe('roleLabel', () => {
  it('capitalises built-in roles and keeps custom names as typed', () => {
    expect(roleLabel('admin')).toBe('Admin')
    expect(roleLabel('E2E-roles-test')).toBe('E2E-roles-test')
    expect(roleLabel('support_lead')).toBe('support_lead')
    expect(roleLabel(undefined)).toBe('')
  })
})
