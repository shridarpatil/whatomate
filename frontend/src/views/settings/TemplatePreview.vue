<script setup lang="ts">
import { computed, ref } from "vue";
import DOMPurify from "dompurify";
import {
  Reply,
  ExternalLink,
  Phone,
  X,
  Image as ImageIcon,
  Video,
  FileIcon,
} from "lucide-vue-next";

interface TemplateButton {
  type: "QUICK_REPLY" | "URL" | "PHONE_NUMBER";
  text: string;
  url?: string;
  phone_number?: string;
}

interface SampleValue {
  component: string;
  index?: number;
  param_name?: string;
  value: string;
}

interface TemplatePreviewProps {
  headerType?: "NONE" | "TEXT" | "IMAGE" | "VIDEO" | "DOCUMENT";
  headerContent?: string;
  bodyContent?: string;
  footerContent?: string;
  buttons?: TemplateButton[];
  sampleValues?: SampleValue[];
  contained?: boolean;
}

const props = withDefaults(defineProps<TemplatePreviewProps>(), {
  headerType: "NONE",
  headerContent: "",
  bodyContent: "",
  footerContent: "",
  buttons: () => [],
  sampleValues: () => [],
  contained: false,
});

const showAllOptions = ref(false);

const visibleButtons = computed(() =>
  (props.buttons || []).length > 3
    ? props.buttons.slice(0, 2)
    : props.buttons || [],
);
const hasSeeAll = computed(() => (props.buttons || []).length > 3);

const HTML_ESCAPES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  "'": "&#39;",
  '"': "&quot;",
};

function escapeHtml(text: string): string {
  return text.replace(/[&<>'"]/g, (tag) => HTML_ESCAPES[tag] || tag);
}

// Samples are scoped per component, so a header sample must not fill a body
// variable. Positional vars ({{1}}) resolve on index, named vars on param_name.
function findSample(component: string, token: string) {
  const name = token.trim();
  const isPositional = /^\d+$/.test(name);
  const index = isPositional ? parseInt(name, 10) : 0;

  return (props.sampleValues || []).find((s) => {
    if (s.component !== component) return false;
    if (isPositional) return s.index === index || s.param_name === name;
    return s.param_name === name;
  });
}

function formatText(text: string, component: string): string {
  if (!text) return "";

  let result = escapeHtml(text);

  // WhatsApp formatting
  result = result.replace(/\*([^*\n]+)\*/g, "<strong>$1</strong>");
  result = result.replace(/_([^\n_]+)_/g, "<em>$1</em>");
  result = result.replace(/~([^~\n]+)~/g, "<del>$1</del>");
  result = result.replace(/```([^`]+)```/g, "<code>$1</code>");

  // Filled variables render as a green pill, unfilled ones stay a yellow
  // placeholder. A function replacer returns its string verbatim, so sample
  // values containing $& or $1 cannot inject replacement tokens.
  result = result.replace(/\{\{([^}]+)\}\}/g, (match, token: string) => {
    const sample = findSample(component, token);
    if (sample?.value) {
      return `<span class="bg-[#d4f1c7] dark:bg-green-900/50 px-0.5 rounded text-[#1a7a3c] dark:text-green-300">${escapeHtml(String(sample.value))}</span>`;
    }
    return `<span class="bg-yellow-100 dark:bg-yellow-900/40 px-0.5 rounded text-yellow-700 dark:text-yellow-300">${match}</span>`;
  });

  return DOMPurify.sanitize(result, {
    ALLOWED_TAGS: ["strong", "em", "del", "code", "span"],
    ALLOWED_ATTR: ["class"],
  });
}

const formattedBody = computed(() => formatText(props.bodyContent || "", "body"));
const formattedHeader = computed(() =>
  formatText(props.headerContent || "", "header"),
);
const currentTime = computed(() =>
  new Date().toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  }),
);
</script>

<template>
  <div class="flex justify-start w-full">
    <div class="w-full max-w-[320px]">
      <!-- Message Bubble -->
      <div
        class="bg-white dark:bg-[#202c33] rounded-lg rounded-tl-none shadow-sm overflow-hidden border border-gray-100 dark:border-zinc-800"
      >
        <!-- HEADER -->
        <div v-if="headerType && headerType !== 'NONE'">
          <!-- Text Header -->
          <div
            v-if="headerType === 'TEXT'"
            class="px-3 pt-3 pb-1 text-[14px] font-bold text-[#111b21] dark:text-gray-100"
            v-html="formattedHeader || 'Header Text...'"
          />

          <!-- Image Header -->
          <div v-else-if="headerType === 'IMAGE'">
            <img
              v-if="headerContent && !headerContent.startsWith('4')"
              :src="headerContent"
              class="w-full h-[160px] object-cover"
              alt="Header"
            />
            <div
              v-else
              class="w-full h-[160px] bg-gray-100 dark:bg-zinc-800 flex flex-col items-center justify-center gap-2"
            >
              <ImageIcon class="w-8 h-8 text-zinc-400" />
              <span class="text-[10px] text-zinc-400 font-medium tracking-wider"
                >IMAGE</span
              >
            </div>
          </div>

          <!-- Video Header -->
          <div
            v-else-if="headerType === 'VIDEO'"
            class="w-full h-[160px] bg-gray-100 dark:bg-zinc-800 flex flex-col items-center justify-center gap-2"
          >
            <Video class="w-8 h-8 text-zinc-400" />
            <span class="text-[10px] text-zinc-400 font-medium tracking-wider"
              >VIDEO</span
            >
          </div>

          <!-- Document Header -->
          <div
            v-else-if="headerType === 'DOCUMENT'"
            class="mx-3 mt-3 mb-1 bg-gray-100 dark:bg-zinc-800 rounded-lg p-3 flex items-center gap-3"
          >
            <FileIcon class="w-8 h-8 text-zinc-400 flex-shrink-0" />
            <div>
              <p
                class="text-[12px] font-medium text-[#111b21] dark:text-gray-200 truncate"
              >
                Document
              </p>
              <p class="text-[10px] text-zinc-400">PDF</p>
            </div>
          </div>
        </div>

        <!-- BODY -->
        <div class="px-3 pt-2 pb-1">
          <p
            v-if="bodyContent"
            class="text-[13.5px] text-[#111b21] dark:text-gray-200 whitespace-pre-wrap leading-relaxed"
            v-html="formattedBody"
          />
          <p
            v-else
            class="text-[13.5px] text-gray-400 dark:text-zinc-500 italic"
          >
            Your message body...
          </p>

          <!-- FOOTER -->
          <p
            v-if="footerContent"
            class="text-[11px] text-gray-400 dark:text-zinc-500 mt-1"
          >
            {{ footerContent }}
          </p>

          <!-- Timestamp -->
          <p class="text-[10px] text-gray-400 text-right mt-1">
            {{ currentTime }}
          </p>
        </div>

        <!-- BUTTONS -->
        <div
          v-if="visibleButtons.length > 0 || hasSeeAll"
          class="flex flex-col border-t border-gray-100 dark:border-zinc-800"
        >
          <div
            v-for="(btn, i) in visibleButtons"
            :key="i"
            class="flex items-center justify-center gap-1.5 py-2.5 px-4 text-[#00A884] text-[13px] font-semibold cursor-pointer hover:bg-gray-50 dark:hover:bg-white/5 transition-colors border-b border-gray-100 dark:border-zinc-800"
          >
            <Reply v-if="btn.type === 'QUICK_REPLY'" class="w-3.5 h-3.5" />
            <ExternalLink v-else-if="btn.type === 'URL'" class="w-3.5 h-3.5" />
            <Phone
              v-else-if="btn.type === 'PHONE_NUMBER'"
              class="w-3.5 h-3.5"
            />
            <span class="truncate">{{ btn.text || "Button" }}</span>
          </div>

          <!-- See all options -->
          <div
            v-if="hasSeeAll"
            class="flex items-center justify-center gap-1.5 py-2.5 px-4 text-[#00A884] text-[13px] font-semibold cursor-pointer hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
            @click="showAllOptions = true"
          >
            <svg viewBox="0 0 24 24" class="w-4 h-4 fill-current">
              <path d="M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z" />
            </svg>
            <span>See all options</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Bottom Sheet -->
    <Teleport to="body" :disabled="contained">
      <Transition name="overlay">
        <div
          v-if="showAllOptions"
          :class="
            contained
              ? 'absolute inset-0 bg-black/40 z-10'
              : 'fixed inset-0 bg-black/40 z-50'
          "
          @click="showAllOptions = false"
        />
      </Transition>

      <Transition name="sheet">
        <div
          v-if="showAllOptions"
          :class="
            contained
              ? 'absolute bottom-0 left-0 right-0 z-10 bg-white dark:bg-[#1f2c34] rounded-t-2xl shadow-2xl max-h-[70%] overflow-y-auto'
              : 'fixed bottom-0 left-0 right-0 z-50 bg-white dark:bg-[#1f2c34] rounded-t-2xl shadow-2xl max-h-[70vh] overflow-y-auto'
          "
        >
          <!-- Sheet Header -->
          <div
            class="flex items-center justify-between px-4 py-3 border-b border-gray-100 dark:border-zinc-800"
          >
            <button
              class="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
              @click="showAllOptions = false"
            >
              <X class="w-5 h-5" />
            </button>
            <span
              class="text-[15px] font-semibold text-[#111b21] dark:text-gray-200"
              >All options</span
            >
            <div class="w-5" />
          </div>

          <!-- All Buttons -->
          <div class="flex flex-col py-1">
            <div
              v-for="(btn, i) in buttons"
              :key="i"
              class="flex items-center gap-3 px-5 py-3.5 cursor-pointer hover:bg-gray-50 dark:hover:bg-white/5 transition-colors border-b border-gray-100 dark:border-zinc-800 last:border-b-0"
            >
              <Reply
                v-if="btn.type === 'QUICK_REPLY'"
                class="w-4 h-4 text-[#00A884]"
              />
              <ExternalLink
                v-else-if="btn.type === 'URL'"
                class="w-4 h-4 text-[#00A884]"
              />
              <Phone
                v-else-if="btn.type === 'PHONE_NUMBER'"
                class="w-4 h-4 text-[#00A884]"
              />
              <span class="text-[14px] text-[#111b21] dark:text-gray-200">{{
                btn.text || "Button"
              }}</span>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.overlay-enter-active,
.overlay-leave-active {
  transition: opacity 0.25s ease;
}
.overlay-enter-from,
.overlay-leave-to {
  opacity: 0;
}
.sheet-enter-active,
.sheet-leave-active {
  transition: transform 0.3s cubic-bezier(0.32, 0.72, 0, 1);
}
.sheet-enter-from,
.sheet-leave-to {
  transform: translateY(100%);
}

/* WhatsApp text formatting */
:deep(strong) {
  font-weight: 700;
}
:deep(em) {
  font-style: italic;
}
:deep(del) {
  text-decoration: line-through;
}
:deep(code) {
  font-family: monospace;
  font-size: 13px;
}
</style>
