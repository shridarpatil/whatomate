import { describe, it, expect } from 'vitest'
import { getErrorMessage } from './api-utils'
import { HttpError } from './http'

describe('getErrorMessage', () => {
  it('reads response message', () => {
    const e = new HttpError('fail', { method: 'get', url: '/x' }, { status: 400, data: { message: 'x' } })
    expect(getErrorMessage(e, 'D')).toBe('x')
  })
  it('falls back', () => {
    const e = new HttpError('', { method: 'get', url: '/x' })
    expect(getErrorMessage(e, 'D')).toBe('D')
  })
})
