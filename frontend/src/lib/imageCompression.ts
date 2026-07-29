// Client-side image downscale/recompress so photos fit WhatsApp Cloud API's
// 5 MB image limit (Meta accepts only image/jpeg & image/png for `image`
// messages). Canvas-only, no dependencies. Non-images, animated GIFs and
// decode failures are returned untouched so the caller's per-type size gate
// stays authoritative.

export interface CompressOptions {
  maxDimension?: number   // longest edge cap, px (default 2048)
  targetBytes?: number    // stay under this (default 4.5 MB, margin below Meta's 5 MB)
  minQuality?: number     // JPEG quality floor before shrinking dimensions (default 0.5)
  skipBelowBytes?: number // leave already-small jpeg/png untouched (default 4.5 MB)
}

const MB = 1024 * 1024
const DEFAULTS: Required<CompressOptions> = {
  maxDimension: 2048,
  targetBytes: 4.5 * MB,
  minQuality: 0.5,
  skipBelowBytes: 4.5 * MB,
}

/**
 * Returns a compressed JPEG File when the input is an image that needs it,
 * otherwise returns the original File unchanged.
 */
export async function compressImage(file: File, opts: CompressOptions = {}): Promise<File> {
  const o = { ...DEFAULTS, ...opts }

  if (!file.type.startsWith('image/')) return file           // only images
  if (file.type === 'image/gif') return file                 // canvas flattens animation -> skip

  const isJpegOrPng = file.type === 'image/jpeg' || file.type === 'image/png'
  if (isJpegOrPng && file.size <= o.skipBelowBytes) return file // already fine, keep pristine

  let bitmap: ImageBitmap
  try {
    // imageOrientation:'from-image' bakes EXIF rotation into pixels (iPhone photos)
    bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
  } catch {
    return file                                              // HEIC / undecodable -> original, size gate rejects
  }

  try {
    let width = bitmap.width
    let height = bitmap.height
    const scale = Math.min(1, o.maxDimension / Math.max(width, height))
    width = Math.round(width * scale)
    height = Math.round(height * scale)

    let quality = 0.9
    let blob = await encode(bitmap, width, height, quality)
    if (!blob) return file                                   // toBlob null -> keep original

    while (blob.size > o.targetBytes) {
      if (quality > o.minQuality) {
        quality = Math.max(o.minQuality, quality - 0.1)
      } else if (Math.max(width, height) > 640) {
        width = Math.round(width * 0.85)
        height = Math.round(height * 0.85)
        quality = 0.8                                        // reset quality after a dimension cut
      } else {
        break                                                // best effort; size gate is the final arbiter
      }
      const next = await encode(bitmap, width, height, quality)
      if (!next) break
      blob = next
    }

    if (blob.size >= file.size) return file                  // never adopt a bigger result
    const name = file.name.replace(/\.[^.]+$/, '') + '.jpg'
    return new File([blob], name, { type: 'image/jpeg', lastModified: Date.now() })
  } finally {
    bitmap.close()
  }
}

function encode(bitmap: ImageBitmap, w: number, h: number, q: number): Promise<Blob | null> {
  const canvas = document.createElement('canvas')
  canvas.width = w
  canvas.height = h
  const ctx = canvas.getContext('2d')
  if (!ctx) return Promise.resolve(null)
  ctx.fillStyle = '#ffffff'                                  // flatten alpha -> white (transparent PNG -> JPEG)
  ctx.fillRect(0, 0, w, h)
  ctx.drawImage(bitmap, 0, 0, w, h)
  return new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', q))
}
