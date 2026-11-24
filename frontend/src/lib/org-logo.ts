// Organization logos are stored inline (a small data URL in the org settings),
// so uploads are downscaled in the browser first. SVG is not accepted: it can
// carry scripts.

export const LOGO_TYPES = ['image/png', 'image/jpeg', 'image/webp']
export const LOGO_SIZE = 128
// Must stay within the server's 64 KB limit (maxOrgLogoBytes).
export const MAX_LOGO_BYTES = 64 * 1024

// Shown when an organization has no logo.
export const DEFAULT_LOGO = `${import.meta.env.BASE_URL}favicon.svg`

/** Approximate decoded size of a base64 data URL, in bytes. */
export function dataUrlBytes(dataUrl: string): number {
  const payload = dataUrl.slice(dataUrl.indexOf(',') + 1)
  return Math.floor((payload.length * 3) / 4)
}

/**
 * Fits the image inside LOGO_SIZE x LOGO_SIZE (keeping transparency) and
 * returns a WebP data URL, or PNG where the browser cannot encode WebP.
 * Rejects non-image types and results over MAX_LOGO_BYTES.
 */
export async function toLogoDataUrl(file: File): Promise<string> {
  if (!LOGO_TYPES.includes(file.type)) throw new Error('unsupported')
  const bitmap = await createImageBitmap(file)
  const scale = Math.min(1, LOGO_SIZE / Math.max(bitmap.width, bitmap.height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(bitmap.width * scale))
  canvas.height = Math.max(1, Math.round(bitmap.height * scale))
  canvas.getContext('2d')!.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
  bitmap.close()
  const url = canvas.toDataURL('image/webp', 0.9)
  if (dataUrlBytes(url) > MAX_LOGO_BYTES) throw new Error('too-large')
  return url
}
