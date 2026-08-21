// Client-side video downscale/recompress helper for browsers that support
// MediaRecorder with MP4 container encoding (WhatsApp Cloud API requires H.264/AAC MP4 or 3GP).
// If unsupported (e.g. Firefox) or on failure/cancel, returns the original File untouched,
// allowing seamless document sending (up to 100 MB).

export interface VideoCompressOptions {
  maxDimension?: number   // longest edge cap, px (default 1280 for 720p)
  targetBytes?: number    // target size floor (default 15 MB to fit WhatsApp's 16 MB video cap)
  maxBitrate?: number     // bitrate ceiling in bits/sec (default 1.2 Mbps)
}

const MB = 1024 * 1024

const DEFAULTS: Required<VideoCompressOptions> = {
  maxDimension: 1280,
  targetBytes: 15 * MB,
  maxBitrate: 1_200_000,
}

/**
 * Returns true if the current browser environment supports encoding MP4 video
 * compatible with WhatsApp Cloud API.
 */
export function canOptimizeVideo(file: File): boolean {
  if (!file.type.startsWith('video/')) return false
  if (typeof MediaRecorder === 'undefined') return false
  // Check if MediaRecorder supports MP4 encoding
  return (
    MediaRecorder.isTypeSupported('video/mp4; codecs="avc1.42E01E,mp4a.40.2"') ||
    MediaRecorder.isTypeSupported('video/mp4; codecs=avc1') ||
    MediaRecorder.isTypeSupported('video/mp4')
  )
}

/**
 * Attempts to compress a video file client-side to fit within WhatsApp's 16 MB inline video limit.
 * Returns a new File if compression succeeds and produces a smaller file; otherwise returns the original File.
 */
export async function compressVideo(
  file: File,
  onProgress?: (ratio: number) => void,
  opts: VideoCompressOptions = {}
): Promise<File> {
  const o = { ...DEFAULTS, ...opts }

  if (!canOptimizeVideo(file)) return file
  if (file.size <= o.targetBytes) return file

  const url = URL.createObjectURL(file)
  const video = document.createElement('video')
  video.muted = true
  video.playsInline = true
  video.preload = 'auto'
  video.src = url

  try {
    await new Promise<void>((resolve, reject) => {
      video.onloadedmetadata = () => resolve()
      video.onerror = () => reject(new Error('Failed to load video metadata'))
    })

    const duration = video.duration || 1
    // Calculate target bitrate based on duration so the output fits in targetBytes
    const targetBitrate = Math.min(
      o.maxBitrate,
      Math.max(300_000, Math.floor((o.targetBytes * 8 * 0.85) / duration))
    )

    let width = video.videoWidth || 1280
    let height = video.videoHeight || 720
    const scale = Math.min(1, o.maxDimension / Math.max(width, height))
    width = Math.round(width * scale)
    height = Math.round(height * scale)
    // Make even for encoder compatibility
    width = width - (width % 2)
    height = height - (height % 2)

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) return file

    const stream = canvas.captureStream(30)

    // Capture audio from video if possible
    try {
      const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext
      if (AudioContextClass) {
        const audioCtx = new AudioContextClass()
        const source = audioCtx.createMediaElementSource(video)
        const dest = audioCtx.createMediaStreamDestination()
        source.connect(dest)
        source.connect(audioCtx.destination)
        const audioTracks = dest.stream.getAudioTracks()
        if (audioTracks.length > 0) {
          stream.addTrack(audioTracks[0])
        }
      }
    } catch {
      // Audio capture fallback: continue with video stream
    }

    const mimeType = MediaRecorder.isTypeSupported('video/mp4; codecs=avc1')
      ? 'video/mp4; codecs=avc1'
      : 'video/mp4'

    const recorder = new MediaRecorder(stream, {
      mimeType,
      videoBitsPerSecond: targetBitrate,
    })

    const chunks: Blob[] = []
    recorder.ondataavailable = (e) => {
      if (e.data && e.data.size > 0) chunks.push(e.data)
    }

    const recordPromise = new Promise<Blob>((resolve, reject) => {
      recorder.onstop = () => resolve(new Blob(chunks, { type: 'video/mp4' }))
      recorder.onerror = (e) => reject(e)
    })

    recorder.start(100)

    let animId = 0
    const drawFrame = () => {
      if (video.paused || video.ended) return
      ctx.drawImage(video, 0, 0, width, height)
      if (onProgress && duration > 0) {
        onProgress(Math.min(1, video.currentTime / duration))
      }
      animId = requestAnimationFrame(drawFrame)
    }

    video.play()
    drawFrame()

    await new Promise<void>((resolve) => {
      video.onended = () => resolve()
    })

    if (animId) {
      cancelAnimationFrame(animId)
    }
    recorder.stop()

    const resultBlob = await recordPromise
    if (resultBlob.size >= file.size || resultBlob.size === 0) {
      return file
    }

    const newName = file.name.replace(/\.[^.]+$/, '') + '.mp4'
    return new File([resultBlob], newName, { type: 'video/mp4', lastModified: Date.now() })
  } catch (err) {
    console.warn('Video optimization failed, keeping original:', err)
    return file
  } finally {
    URL.revokeObjectURL(url)
    video.remove()
  }
}
