import { describe, it, expect } from 'vitest'
import { resolveScheme } from './useColorMode'

describe('resolveScheme', () => {
  it('follows the system when mode is system', () => {
    expect(resolveScheme('system', true)).toBe('dark')
    expect(resolveScheme('system', false)).toBe('light')
  })
  it('ignores the system for explicit modes', () => {
    expect(resolveScheme('light', true)).toBe('light')
    expect(resolveScheme('dark', false)).toBe('dark')
  })
})
