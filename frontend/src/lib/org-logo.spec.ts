import { describe, it, expect } from 'vitest'
import { dataUrlBytes, toLogoDataUrl } from './org-logo'

describe('org logo', () => {
  it('rejects SVG and non-image files', async () => {
    await expect(toLogoDataUrl(new File(['<svg/>'], 'a.svg', { type: 'image/svg+xml' }))).rejects.toThrow('unsupported')
    await expect(toLogoDataUrl(new File(['x'], 'a.txt', { type: 'text/plain' }))).rejects.toThrow('unsupported')
  })
  it('estimates decoded data URL size', () => {
    expect(dataUrlBytes('data:image/png;base64,' + 'A'.repeat(400))).toBe(300)
  })
})
