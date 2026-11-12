<script setup lang="ts">
// In-app media viewer (lightbox) for chat attachments: images, stickers, video
// and PDF/documents. Replaces the old "open in a new browser tab" behaviour.
//
// Built on the shared Oat Modal (native <dialog>), so the top layer, backdrop,
// ESC-to-close and focus containment all come for free. Images and video
// load straight from the auth-cookie-gated /api/media/{id} endpoint. PDFs are
// the tricky case: the backend sets a global `X-Frame-Options: DENY`, so a
// direct <iframe src="/api/media/{id}"> is blocked even same-origin. We work
// around it WITHOUT touching the backend by fetch()ing the bytes (the cookie
// rides along) and framing an object-URL blob, which carries no such header.
import { ref, computed, watch, onBeforeUnmount, onMounted, onUnmounted } from 'vue'
import Modal from '@/components/oat/Modal.vue'
import IconButton from '@/components/shared/IconButton.vue'
import {
  Download,
  ExternalLink,
  ZoomIn,
  ZoomOut,
  ChevronLeft,
  ChevronRight,
  FileText,
} from '@/lib/icons'
import type { Message } from '@/stores/contacts'

const props = defineProps<{ items: Message[] }>()
const open = defineModel<boolean>('open', { default: false })
const index = defineModel<number>('index', { default: 0 })

const current = computed<Message | undefined>(() => props.items[index.value])
const total = computed(() => props.items.length)
const hasPrev = computed(() => index.value > 0)
const hasNext = computed(() => index.value < total.value - 1)

// Same URL the chat bubbles use: relative, same-origin, auth-cookie gated.
function mediaUrl(m: Message): string {
  if (!m.media_url) return ''
  const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
  return `${basePath}/api/media/${m.id}`
}

type Kind = 'image' | 'video' | 'pdf' | 'file'
function mediaKind(m: Message): Kind {
  const mime = m.media_mime_type || ''
  const name = (m.media_filename || '').toLowerCase()
  if (m.message_type === 'image' || m.message_type === 'sticker' || mime.startsWith('image/')) return 'image'
  if (m.message_type === 'video' || mime.startsWith('video/')) return 'video'
  if (mime.includes('pdf') || name.endsWith('.pdf')) return 'pdf'
  return 'file'
}
const currentKind = computed<Kind>(() => (current.value ? mediaKind(current.value) : 'file'))
const currentUrl = computed(() => (current.value ? mediaUrl(current.value) : ''))
const filename = computed(() => current.value?.media_filename || '')

// --- Navigation ---------------------------------------------------------
function prev() { if (hasPrev.value) index.value -= 1 }
function next() { if (hasNext.value) index.value += 1 }
function onKeydown(e: KeyboardEvent) {
  if (!open.value) return
  if (e.key === 'ArrowLeft') prev()
  else if (e.key === 'ArrowRight') next()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

function openInTab() { if (currentUrl.value) window.open(currentUrl.value, '_blank') }

// --- Image zoom / pan ---------------------------------------------------
const scale = ref(1)
const tx = ref(0)
const ty = ref(0)
const dragging = ref(false)
let startX = 0, startY = 0, startTx = 0, startTy = 0
const MIN = 1, MAX = 5, STEP = 0.25

function resetZoom() { scale.value = 1; tx.value = 0; ty.value = 0 }
function clamp(v: number) { return Math.min(MAX, Math.max(MIN, +v.toFixed(2))) }
function zoomIn() { scale.value = clamp(scale.value + STEP) }
function zoomOut() { scale.value = clamp(scale.value - STEP); if (scale.value === 1) { tx.value = 0; ty.value = 0 } }
function onWheel(e: WheelEvent) {
  e.preventDefault()
  scale.value = clamp(scale.value + (e.deltaY < 0 ? STEP : -STEP))
  if (scale.value === 1) { tx.value = 0; ty.value = 0 }
}
function onPointerDown(e: PointerEvent) {
  if (scale.value <= 1) return
  dragging.value = true
  startX = e.clientX; startY = e.clientY; startTx = tx.value; startTy = ty.value
  ;(e.target as HTMLElement).setPointerCapture?.(e.pointerId)
}
function onPointerMove(e: PointerEvent) {
  if (!dragging.value) return
  tx.value = startTx + (e.clientX - startX)
  ty.value = startTy + (e.clientY - startY)
}
function onPointerUp() { dragging.value = false }
function toggleZoom() { scale.value > 1 ? resetZoom() : (scale.value = 2) }

const imageStyle = computed(() => ({
  transform: `translate(${tx.value}px, ${ty.value}px) scale(${scale.value})`,
  cursor: scale.value > 1 ? (dragging.value ? 'grabbing' : 'grab') : 'zoom-in',
  transition: dragging.value ? 'none' : 'transform 0.15s ease',
}))

// --- PDF blob (X-Frame-Options: DENY workaround) ------------------------
const pdfBlobUrl = ref<string | null>(null)
const pdfLoading = ref(false)
const pdfError = ref(false)
let pdfReqId = 0

function clearPdf() {
  if (pdfBlobUrl.value) { URL.revokeObjectURL(pdfBlobUrl.value); pdfBlobUrl.value = null }
  pdfLoading.value = false
  pdfError.value = false
}
// SECURITY (load-bearing, do not relax): a blob: URL inherits the origin of the
// document that created it, and this app ships no CSP, so framing a blob whose
// bytes are HTML would be a same-origin XSS sink. What makes this safe is the
// backend: ServeMedia maps the on-disk extension to a whitelisted Content-Type
// and falls back to application/octet-stream — it never echoes the DB's
// media_mime_type. The blob.type check below keeps us on that whitelist: only
// something the server itself vouched for as a PDF is ever framed.
async function loadPdf(m: Message) {
  const my = ++pdfReqId
  clearPdf()
  pdfLoading.value = true
  try {
    const res = await fetch(mediaUrl(m), { credentials: 'same-origin' })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    if (my !== pdfReqId) return // superseded by a newer navigation
    // A file named *.pdf whose MIME the server didn't recognise comes back as
    // octet-stream: framing it would just show a blank page, so fall through to
    // the download card instead.
    if (!blob.type.includes('pdf')) return
    pdfBlobUrl.value = URL.createObjectURL(blob)
  } catch {
    if (my === pdfReqId) pdfError.value = true
  } finally {
    if (my === pdfReqId) pdfLoading.value = false
  }
}

// React to open / navigation: reset zoom and (re)load PDFs on demand.
//
// Watch the current item's IDENTITY, not the items array: `items` is a computed
// .filter() that allocates a new array on every recompute, and the messages it
// filters are mutated by inbound WS messages and delivery-status updates. A
// sent → delivered → read receipt would otherwise reset zoom/pan and re-fetch
// the whole PDF while the viewer is open. Keying on the id also absorbs the
// index shift when loadOlderMessages() prepends history.
watch(
  [open, () => current.value?.id],
  () => {
    resetZoom()
    const m = current.value
    if (open.value && m && mediaKind(m) === 'pdf') loadPdf(m)
    else clearPdf()
  },
  { immediate: true },
)
onBeforeUnmount(clearPdf)

function closeOnBackdrop() { open.value = false }
</script>

<template>
  <Modal v-model:open="open" class="modal-wide" :title="filename || $t('chat.mediaViewer.title')">
    <div class="vstack gap-2">
      <!-- Toolbar -->
      <div class="hstack justify-between">
        <small class="text-light">
          <template v-if="total > 1">{{ index + 1 }} / {{ total }}</template>
        </small>
        <div class="hstack gap-1">
          <template v-if="currentKind === 'image'">
            <IconButton :icon="ZoomOut" :label="$t('chat.mediaViewer.zoomOut')" :disabled="scale <= 1" @click="zoomOut" />
            <IconButton :icon="ZoomIn" :label="$t('chat.mediaViewer.zoomIn')" @click="zoomIn" />
          </template>
          <a
            v-if="currentUrl"
            :href="currentUrl"
            :download="filename || 'file'"
            class="button ghost icon"
            :title="$t('chat.mediaViewer.download')"
            :aria-label="$t('chat.mediaViewer.download')"
          >
            <Download />
          </a>
          <IconButton :icon="ExternalLink" :label="$t('chat.mediaViewer.openInTab')" @click="openInTab" />
        </div>
      </div>

      <div class="flex items-center gap-2">
        <!-- Prev -->
        <button
          v-if="total > 1"
          type="button"
          class="outline icon shrink-0"
          :disabled="!hasPrev"
          :aria-label="$t('chat.mediaViewer.previous')"
          :title="$t('chat.mediaViewer.previous')"
          @click="prev"
        >
          <ChevronLeft :size="24" />
        </button>

        <!-- Stage: click empty space to close -->
        <div
          class="flex-1 min-w-0 flex items-center justify-center overflow-hidden h-builder"
          :aria-busy="currentKind === 'pdf' && pdfLoading ? 'true' : undefined"
          data-spinner="large"
          @click.self="closeOnBackdrop"
        >
          <!-- Image / sticker -->
          <img
            v-if="current && currentKind === 'image'"
            :src="currentUrl"
            :alt="filename || 'Image'"
            class="media-fit"
            :style="imageStyle"
            draggable="false"
            @wheel="onWheel"
            @pointerdown="onPointerDown"
            @pointermove="onPointerMove"
            @pointerup="onPointerUp"
            @pointercancel="onPointerUp"
            @dblclick="toggleZoom"
          />

          <!-- Video -->
          <video
            v-else-if="current && currentKind === 'video'"
            :src="currentUrl"
            controls
            class="media-fit"
          />

          <!-- PDF -->
          <template v-else-if="current && currentKind === 'pdf'">
            <iframe
              v-if="pdfBlobUrl"
              :src="pdfBlobUrl"
              class="w-100 h-full"
              :title="filename || 'PDF'"
            />
            <div v-else-if="!pdfLoading" class="vstack items-center align-center">
              <FileText :size="48" />
              <p>{{ pdfError ? $t('chat.mediaViewer.pdfError') : filename }}</p>
              <div class="hstack gap-2 justify-center">
                <a :href="currentUrl" :download="filename || 'file'" class="button" data-variant="secondary">
                  <Download />{{ $t('chat.mediaViewer.download') }}
                </a>
                <button type="button" class="outline" @click="openInTab">
                  <ExternalLink />{{ $t('chat.mediaViewer.openInTab') }}
                </button>
              </div>
            </div>
          </template>

          <!-- Other documents (docx, xlsx, …): no inline viewer -->
          <div v-else-if="current" class="vstack items-center align-center min-w-0">
            <FileText :size="48" />
            <p class="truncate w-100">{{ filename }}</p>
            <div class="hstack gap-2 justify-center">
              <a :href="currentUrl" :download="filename || 'file'" class="button" data-variant="secondary">
                <Download />{{ $t('chat.mediaViewer.download') }}
              </a>
              <button type="button" class="outline" @click="openInTab">
                <ExternalLink />{{ $t('chat.mediaViewer.openInTab') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Next -->
        <button
          v-if="total > 1"
          type="button"
          class="outline icon shrink-0"
          :disabled="!hasNext"
          :aria-label="$t('chat.mediaViewer.next')"
          :title="$t('chat.mediaViewer.next')"
          @click="next"
        >
          <ChevronRight :size="24" />
        </button>
      </div>
    </div>
  </Modal>
</template>
