<script setup lang="ts">
/**
 * AppTour — Quick Start Guide (Guia de Início Rápido)
 *
 * Self-contained tour component. Highlights real DOM elements using a
 * spotlight overlay. No external dependencies.
 *
 * Persistence: localStorage key "whatomate_tour_v1"
 * Trigger:     automatically on first login; re-openable via "?" button
 * Scope:       agent role — focuses on chat, transfers, canned responses
 */
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const STORAGE_KEY = 'whatomate_tour_v1'
const router = useRouter()
const authStore = useAuthStore()

// ── State ─────────────────────────────────────────────────────
const visible = ref(false)
const step = ref(0)
const spotlightRect = ref<DOMRect | null>(null)
const tooltipPos = ref({ top: 0, left: 0, placement: 'bottom' as 'top' | 'bottom' | 'left' | 'right' | 'center' })
let resizeObs: ResizeObserver | null = null
let positionTimer: ReturnType<typeof setInterval> | null = null

// ── Tour steps definition ──────────────────────────────────────
// selector: CSS selector for the element to highlight (null = center modal)
// route:    navigate to this route before showing the step (optional)
const STEPS = [
  {
    id: 'welcome',
    selector: null,
    route: null,
    icon: '👋',
    title: 'Bem-vindo ao Whatomate!',
    body: 'Este guia rápido vai mostrar os 6 pontos essenciais para você começar a atender em menos de 2 minutos. Use as setas ou clique nos pontos para navegar.',
    cta: 'Começar →',
  },
  {
    id: 'nav-chat',
    selector: 'a[href="/chat"], [data-nav="chat"], nav a:has([class*="chat" i])',
    fallbackSelector: 'nav, aside',
    route: '/chat',
    icon: '💬',
    title: '1. Chat — sua área de trabalho',
    body: 'Todo atendimento acontece aqui. Clique em <strong>Chat</strong> no menu lateral para ver as conversas do seu departamento.',
    cta: 'Entendi →',
  },
  {
    id: 'contact-tabs',
    selector: '[role="tablist"], .tabs, [class*="tab"]',
    fallbackSelector: 'main',
    route: '/chat',
    icon: '📋',
    title: '2. Abas de Status',
    body: '<strong>Novo</strong> = clientes sem atendimento — sua prioridade. <strong>Andamento</strong> = você está atendendo. <strong>Concluído</strong> = finalizados.',
    cta: 'Próximo →',
  },
  {
    id: 'message-input',
    selector: 'textarea[placeholder*="mensagem" i], textarea[placeholder*="message" i], form textarea',
    fallbackSelector: 'form',
    route: '/chat',
    icon: '⌨️',
    title: '3. Campo de Mensagem',
    body: 'Digite aqui e pressione <kbd>Enter</kbd> para enviar. <br><br>💡 Digite <kbd>/</kbd> para abrir o menu de <strong>respostas rápidas</strong> — selecione com ↑↓ e insira com Enter.',
    cta: 'Próximo →',
  },
  {
    id: 'transfers',
    selector: 'a[href*="transfers"], [data-nav="transfers"]',
    fallbackSelector: 'nav, aside',
    route: '/chatbot/transfers',
    icon: '🔄',
    title: '4. Fila de Transferências',
    body: 'Clientes vindos do chatbot chegam aqui. Clique em <strong>Pegar próximo</strong> para aceitar o primeiro da fila, ou escolha um cliente específico.',
    cta: 'Próximo →',
  },
  {
    id: 'resolve',
    selector: null,
    route: '/chat',
    icon: '✅',
    title: '5. Sempre Resolva ao Final',
    body: 'Quando o atendimento terminar, clique no botão <strong>Resolver ↺</strong> no topo da conversa. Isso libera o chatbot para o cliente e organiza a fila da equipe.',
    cta: 'Próximo →',
  },
  {
    id: 'done',
    selector: null,
    route: null,
    icon: '🚀',
    title: 'Pronto! Você está preparado.',
    body: 'Resumo rápido:<br><br>✓ Chat → atender conversas<br>✓ <kbd>/</kbd> → respostas rápidas<br>✓ Transferências → pegar fila<br>✓ Resolver → sempre ao finalizar<br><br>Clique no <strong>?</strong> no canto da tela para rever este guia.',
    cta: 'Começar a atender ✓',
  },
]

const total = STEPS.length
const current = computed(() => STEPS[step.value])
const isFirst = computed(() => step.value === 0)
const isLast = computed(() => step.value === total - 1)
const progress = computed(() => Math.round((step.value / (total - 1)) * 100))
const cardPlacementClass = computed(() => 'tour-card--' + tooltipPos.value.placement)

// ── Spotlight ─────────────────────────────────────────────────
function getElement(s: typeof STEPS[0]): Element | null {
  if (!s.selector) return null
  const selectors = s.selector.split(', ')
  for (const sel of selectors) {
    try {
      const el = document.querySelector(sel.trim())
      if (el) return el
    } catch {}
  }
  if (s.fallbackSelector) {
    try { return document.querySelector(s.fallbackSelector) } catch {}
  }
  return null
}

async function positionTooltip() {
  await nextTick()
  const s = current.value
  const el = getElement(s)

  if (!el || s.selector === null) {
    spotlightRect.value = null
    tooltipPos.value = { top: 0, left: 0, placement: 'center' }
    return
  }

  const rect = el.getBoundingClientRect()
  // Add 8px padding to spotlight
  spotlightRect.value = new DOMRect(
    rect.left - 8, rect.top - 8,
    rect.width + 16, rect.height + 16
  )

  // Decide tooltip placement
  const vw = window.innerWidth
  const vh = window.innerHeight
  const TOOLTIP_W = 340
  const TOOLTIP_H = 200

  let top = 0, left = 0, placement: typeof tooltipPos.value.placement = 'bottom'

  if (rect.bottom + TOOLTIP_H + 20 < vh) {
    placement = 'bottom'
    top = rect.bottom + 16
    left = Math.max(16, Math.min(rect.left, vw - TOOLTIP_W - 16))
  } else if (rect.top - TOOLTIP_H - 20 > 0) {
    placement = 'top'
    top = rect.top - TOOLTIP_H - 16
    left = Math.max(16, Math.min(rect.left, vw - TOOLTIP_W - 16))
  } else if (rect.right + TOOLTIP_W + 20 < vw) {
    placement = 'right'
    top = Math.max(16, rect.top)
    left = rect.right + 16
  } else {
    placement = 'left'
    top = Math.max(16, rect.top)
    left = rect.left - TOOLTIP_W - 16
  }

  tooltipPos.value = { top, left, placement }
}

// ── Navigation ────────────────────────────────────────────────
async function goTo(n: number) {
  if (n < 0 || n >= total) return
  step.value = n
  const s = STEPS[n]
  if (s.route) {
    const cur = router.currentRoute.value.path
    if (!cur.startsWith(s.route)) {
      await router.push(s.route).catch(() => {})
      await new Promise(r => setTimeout(r, 400))
    }
  }
  await positionTooltip()
}

async function next() {
  if (isLast.value) { close(true); return }
  await goTo(step.value + 1)
}

async function prev() {
  if (isFirst.value) return
  await goTo(step.value - 1)
}

// ── Open / Close ──────────────────────────────────────────────
async function open() {
  step.value = 0
  visible.value = true
  await nextTick()
  await positionTooltip()
  // Keep spotlight in sync during scroll/resize
  positionTimer = setInterval(positionTooltip, 300)
}

function close(completed = false) {
  visible.value = false
  if (positionTimer) clearInterval(positionTimer)
  if (completed) {
    try { localStorage.setItem(STORAGE_KEY, '1') } catch {}
  }
}

// ── Auto-start on first visit ─────────────────────────────────
onMounted(async () => {
  // Only auto-start if logged in and tour not yet completed
  if (!authStore.user) return
  let done = false
  try { done = !!localStorage.getItem(STORAGE_KEY) } catch {}
  if (!done) {
    // Small delay to let the app render first
    await new Promise(r => setTimeout(r, 1200))
    if (authStore.user) open()
  }
  // Keyboard navigation
  window.addEventListener('keydown', onKey)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  if (positionTimer) clearInterval(positionTimer)
})

function onKey(e: KeyboardEvent) {
  if (!visible.value) return
  if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next()
  if (e.key === 'ArrowLeft'  || e.key === 'ArrowUp')   prev()
  if (e.key === 'Escape') close()
}

// ── Expose open() to parent ───────────────────────────────────
defineExpose({ open })
</script>

<template>
  <!-- Trigger button — always visible after tour completion -->
  <button
    class="tour-fab"
    :class="{ 'tour-fab--pulsing': !visible }"
    @click="open"
    title="Guia de Início Rápido"
    aria-label="Abrir guia de início rápido"
  >
    ?
  </button>

  <Teleport to="body">
    <Transition name="tour-fade">
      <div v-if="visible" class="tour-root">

        <!-- Overlay with spotlight cutout -->
        <div class="tour-overlay" @click.self="close()">
          <svg
            v-if="spotlightRect"
            class="tour-spotlight-svg"
            xmlns="http://www.w3.org/2000/svg"
          >
            <defs>
              <mask id="tour-mask">
                <rect width="100%" height="100%" fill="white"/>
                <rect
                  :x="spotlightRect.x"
                  :y="spotlightRect.y"
                  :width="spotlightRect.width"
                  :height="spotlightRect.height"
                  rx="8"
                  fill="black"
                />
              </mask>
            </defs>
            <rect
              width="100%" height="100%"
              fill="rgba(0,0,0,0.72)"
              mask="url(#tour-mask)"
            />
            <!-- Spotlight border glow -->
            <rect
              :x="spotlightRect.x"
              :y="spotlightRect.y"
              :width="spotlightRect.width"
              :height="spotlightRect.height"
              rx="8"
              fill="none"
              stroke="#10B981"
              stroke-width="2"
              opacity="0.8"
            />
          </svg>
          <!-- Full dark overlay when no element targeted -->
          <div v-else class="tour-full-overlay" />
        </div>

        <!-- Tooltip / Card -->
        <Transition name="tour-slide" mode="out-in">
          <div
            :key="step"
            class="tour-card"
            :class="cardPlacementClass"
            :style="
              tooltipPos.placement === 'center'
                ? { top: '50%', left: '50%', transform: 'translate(-50%, -50%)' }
                : { top: tooltipPos.top + 'px', left: tooltipPos.left + 'px' }
            "
          >
            <!-- Header -->
            <div class="tour-card__header">
              <span class="tour-card__icon">{{ current.icon }}</span>
              <div class="tour-card__title">{{ current.title }}</div>
              <button class="tour-card__close" @click="close()" aria-label="Fechar guia">✕</button>
            </div>

            <!-- Body -->
            <div class="tour-card__body" v-html="current.body" />

            <!-- Footer: dots + actions + counter -->
            <div class="tour-card__footer">
              <!-- Progress dots -->
              <div class="tour-card__dots">
                <button
                  v-for="(_, i) in STEPS"
                  :key="i"
                  class="tour-dot"
                  :class="{ 'tour-dot--active': i === step, 'tour-dot--done': i < step }"
                  @click="goTo(i)"
                  :aria-label="'Passo ' + (i + 1)"
                />
              </div>

              <!-- Actions -->
              <div class="tour-card__actions">
                <button
                  v-if="!isFirst"
                  class="tour-btn tour-btn--ghost"
                  @click="prev"
                >
                  ← Voltar
                </button>
                <span v-else class="tour-card__skip" @click="close()">Pular tour</span>
                <button class="tour-btn tour-btn--primary" @click="next">
                  {{ current.cta }}
                </button>
              </div>

              <!-- Step counter -->
              <div class="tour-card__counter">{{ step + 1 }} de {{ total }}</div>
            </div>
          </div>
        </Transition>

      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* ── FAB trigger ─────────────────────────────────────────────── */
.tour-fab {
  position: fixed;
  bottom: 20px;
  right: 20px;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: rgba(16, 185, 129, 0.15);
  border: 1.5px solid rgba(16, 185, 129, 0.4);
  color: #10B981;
  font-size: 17px;
  font-weight: 800;
  font-family: Georgia, serif;          /* consistent ? glyph */
  cursor: pointer;
  z-index: 9000;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  transition: background 0.2s, border-color 0.2s, transform 0.2s;
  backdrop-filter: blur(8px);
}
.tour-fab:hover {
  background: rgba(16, 185, 129, 0.28);
  border-color: #10B981;
  transform: scale(1.1);
}
.tour-fab--pulsing {
  animation: tour-pulse 2.5s ease-in-out infinite;
}
@keyframes tour-pulse {
  0%, 100% { box-shadow: 0 0 0 0   rgba(16, 185, 129, 0.45); }
  50%       { box-shadow: 0 0 0 11px rgba(16, 185, 129, 0);   }
}

/* ── Root ────────────────────────────────────────────────────── */
.tour-root {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
}

/* ── Overlay ─────────────────────────────────────────────────── */
.tour-overlay {
  position: absolute;
  inset: 0;
  pointer-events: all;
}
.tour-spotlight-svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
.tour-full-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.72);
}

/* ── Card ────────────────────────────────────────────────────── */
.tour-card {
  position: fixed;
  width: 344px;
  max-height: calc(100vh - 48px);
  overflow-y: auto;
  background: #16162a;
  border: 1px solid rgba(16, 185, 129, 0.28);
  border-radius: 16px;
  padding: 0;                           /* padding via inner sections */
  box-shadow:
    0 0 0 1px rgba(16, 185, 129, 0.08),
    0 20px 56px rgba(0, 0, 0, 0.65),
    0 0 48px rgba(16, 185, 129, 0.07);
  pointer-events: all;
  z-index: 10000;
  display: flex;
  flex-direction: column;
}

/* ── Header ──────────────────────────────────────────────────── */
.tour-card__header {
  display: flex;
  align-items: center;                  /* vertically center icon, title, ✕ */
  gap: 10px;
  padding: 18px 18px 0 18px;
}
.tour-card__icon {
  font-size: 24px;
  line-height: 1;
  flex-shrink: 0;
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(16, 185, 129, 0.1);
  border-radius: 9px;
}
.tour-card__title {
  flex: 1;
  font-size: 14px;
  font-weight: 700;
  color: #f1f5f9;
  line-height: 1.35;
  min-width: 0;
}
.tour-card__close {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #64748b;
  cursor: pointer;
  font-size: 13px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s, color 0.15s;
  line-height: 1;
  align-self: flex-start;             /* pin to top when title wraps */
}
.tour-card__close:hover {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.3);
  color: #f87171;
}

/* ── Divider under header ────────────────────────────────────── */
.tour-card__header + .tour-card__body {
  margin-top: 12px;
}

/* ── Body ────────────────────────────────────────────────────── */
.tour-card__body {
  flex: 1;
  font-size: 13px;
  color: #94a3b8;
  line-height: 1.65;
  padding: 12px 18px 14px 18px;
}
.tour-card__body :deep(strong) {
  color: #e2e8f0;
  font-weight: 600;
}
.tour-card__body :deep(kbd) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #1e2040;
  border: 1px solid #3a3a5e;
  color: #cbd5e1;
  font-family: 'Courier New', monospace;
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 5px;
  vertical-align: baseline;
  line-height: 1.6;
}
.tour-card__body :deep(br) {
  display: block;
  content: '';
  margin-top: 4px;
}

/* ── Footer (dots + actions + counter) ───────────────────────── */
.tour-card__footer {
  padding: 0 18px 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  padding-top: 14px;
}

/* Progress dots */
.tour-card__dots {
  display: flex;
  align-items: center;
  gap: 5px;
  height: 8px;
}
.tour-dot {
  height: 6px;
  width: 6px;
  border-radius: 50%;
  border: none;
  cursor: pointer;
  background: #252545;
  transition: width 0.22s ease, background 0.22s ease, border-radius 0.22s ease;
  padding: 0;
  flex-shrink: 0;
}
.tour-dot--active {
  width: 22px;
  border-radius: 3px;
  background: #10B981;
}
.tour-dot--done {
  background: rgba(16, 185, 129, 0.38);
}
.tour-dot:hover:not(.tour-dot--active) {
  background: rgba(16, 185, 129, 0.55);
}

/* Action row */
.tour-card__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.tour-btn {
  height: 36px;
  padding: 0 16px;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  transition: background 0.15s, color 0.15s, opacity 0.15s;
  white-space: nowrap;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.tour-btn--primary {
  flex: 1;
  background: #10B981;
  color: #000;
  letter-spacing: 0.01em;
}
.tour-btn--primary:hover { background: #0ea572; }
.tour-btn--ghost {
  background: rgba(255, 255, 255, 0.05);
  color: #94a3b8;
  border: 1px solid rgba(255, 255, 255, 0.09);
  flex-shrink: 0;
}
.tour-btn--ghost:hover {
  background: rgba(255, 255, 255, 0.09);
  color: #e2e8f0;
}
.tour-card__skip {
  font-size: 12px;
  color: #475569;
  cursor: pointer;
  text-decoration-line: underline;
  text-underline-offset: 2px;
  flex-shrink: 0;
  transition: color 0.15s;
}
.tour-card__skip:hover { color: #94a3b8; }

/* Step counter */
.tour-card__counter {
  font-size: 11px;
  color: #475569;
  text-align: right;
  line-height: 1;
}

/* ── Transitions ─────────────────────────────────────────────── */
.tour-fade-enter-active, .tour-fade-leave-active { transition: opacity 0.25s ease; }
.tour-fade-enter-from, .tour-fade-leave-to { opacity: 0; }

.tour-slide-enter-active { transition: opacity 0.18s ease, transform 0.18s ease; }
.tour-slide-leave-active  { transition: opacity 0.14s ease, transform 0.14s ease; }
.tour-slide-enter-from { opacity: 0; transform: translateY(6px);  }
.tour-slide-leave-to   { opacity: 0; transform: translateY(-5px); }
</style>
