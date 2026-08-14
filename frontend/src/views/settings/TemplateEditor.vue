<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import {
  Trash2,
  Plus,
  Check,
  ChevronsUpDown,
  ChevronUp,
  ChevronDown,
  AlertCircle,
} from "lucide-vue-next";

import {
  validateButtonCombination,
  BUTTON_LIMITS,
  MAX_BUTTONS,
  MAX_CTA,
} from "@/lib/templateButtons";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { toast } from "vue-sonner";

const { t } = useI18n();

const props = defineProps<{
  modelValue: Record<string, any>;
  isEdit?: boolean;
  // Meta freezes name, language, category and account once it has the template.
  isPublished?: boolean;
  accounts: any[];
  // Published WhatsApp Flows, for FLOW buttons. Loaded by the parent.
  flows?: any[];
  disabled?: boolean;
  // Picked here, uploaded by the parent on save, then set back to null.
  mediaFile?: File | null;
}>();

const isLocked = computed(() => !!props.isPublished);

// Mirrors normalizeTemplateName() in internal/handlers/templates.go.
function normalizeName(value: string) {
  return value
    .toLowerCase()
    .replace(/[\s-]+/g, "_")
    .replace(/[^a-z0-9_]/g, "");
}

function isValidPhone(value: string) {
  return /^\+?[0-9]{7,15}$/.test(value.trim());
}

// Meta treats a URL button as dynamic when the url ends in {{1}}, so the url is
// the source of truth. The example suffix goes on the button as `example`, which
// is where the backend reads it from.
const URL_VAR = "{{1}}";
// New dynamic urls always use {{1}}, but one synced from Meta may carry a named
// variable, so detecting and stripping has to accept any of them.
const URL_VAR_PATTERN = /\{\{[^}]+\}\}/;

function isDynamicUrl(btn: any) {
  return URL_VAR_PATTERN.test(String(btn.url || ""));
}

function setUrlDynamic(btn: any, dynamic: boolean) {
  const base = urlPrefix(btn);
  if (dynamic) {
    btn.url = base + URL_VAR;
  } else {
    btn.url = base;
    btn.example = "";
  }
}

// The part of a dynamic url before the variable — what the user types, and the
// prefix the example is appended to.
function urlPrefix(btn: any) {
  return String(btn.url || "").replace(URL_VAR_PATTERN, "");
}

function setUrlPrefix(btn: any, prefix: string) {
  btn.url = prefix.replace(/[{}]/g, "") + URL_VAR;
}

const emit = defineEmits(["update:modelValue", "update:mediaFile"]);

// Buttons are keyed on id in the template. The server never returns one, so
// every copy coming from the parent needs its ids restored.
function withButtonIds(value: any) {
  if (Array.isArray(value?.buttons)) {
    value.buttons = value.buttons.map((b: any) => ({
      ...b,
      id: b.id || crypto.randomUUID(),
    }));
  }
  return value;
}

const state = ref(withButtonIds(JSON.parse(JSON.stringify(props.modelValue))));

watch(
  state,
  (newVal) => {
    emit("update:modelValue", JSON.parse(JSON.stringify(newVal)));
  },
  { deep: true },
);

watch(
  () => props.modelValue,
  (newVal) => {
    if (JSON.stringify(newVal) !== JSON.stringify(state.value)) {
      state.value = withButtonIds(JSON.parse(JSON.stringify(newVal)));
    }
  },
  { deep: true },
);

const languageSelectorOpen = ref(false);
const languages = [
  { code: "af", name: "Afrikaans" },
  { code: "sq", name: "Albanian" },
  { code: "ar", name: "Arabic" },
  { code: "az", name: "Azerbaijani" },
  { code: "bn", name: "Bengali" },
  { code: "bg", name: "Bulgarian" },
  { code: "ca", name: "Catalan" },
  { code: "zh_CN", name: "Chinese (CHN)" },
  { code: "zh_HK", name: "Chinese (HKG)" },
  { code: "zh_TW", name: "Chinese (TAI)" },
  { code: "hr", name: "Croatian" },
  { code: "cs", name: "Czech" },
  { code: "da", name: "Danish" },
  { code: "nl", name: "Dutch" },
  { code: "en", name: "English" },
  { code: "en_GB", name: "English (UK)" },
  { code: "en_US", name: "English (US)" },
  { code: "et", name: "Estonian" },
  { code: "fil", name: "Filipino" },
  { code: "fi", name: "Finnish" },
  { code: "fr", name: "French" },
  { code: "ka", name: "Georgian" },
  { code: "de", name: "German" },
  { code: "el", name: "Greek" },
  { code: "gu", name: "Gujarati" },
  { code: "ha", name: "Hausa" },
  { code: "he", name: "Hebrew" },
  { code: "hi", name: "Hindi" },
  { code: "hu", name: "Hungarian" },
  { code: "id", name: "Indonesian" },
  { code: "ga", name: "Irish" },
  { code: "it", name: "Italian" },
  { code: "ja", name: "Japanese" },
  { code: "kn", name: "Kannada" },
  { code: "kk", name: "Kazakh" },
  { code: "rw_RW", name: "Kinyarwanda" },
  { code: "ko", name: "Korean" },
  { code: "ky_KG", name: "Kyrgyz (Kyrgyzstan)" },
  { code: "lo", name: "Lao" },
  { code: "lv", name: "Latvian" },
  { code: "lt", name: "Lithuanian" },
  { code: "mk", name: "Macedonian" },
  { code: "ms", name: "Malay" },
  { code: "ml", name: "Malayalam" },
  { code: "mr", name: "Marathi" },
  { code: "nb", name: "Norwegian (Bokmål)" },
  { code: "fa", name: "Persian" },
  { code: "pl", name: "Polish" },
  { code: "pt_BR", name: "Portuguese (BR)" },
  { code: "pt_PT", name: "Portuguese (POR)" },
  { code: "pa", name: "Punjabi" },
  { code: "ro", name: "Romanian" },
  { code: "ru", name: "Russian" },
  { code: "sr", name: "Serbian" },
  { code: "sk", name: "Slovak" },
  { code: "sl", name: "Slovenian" },
  { code: "es", name: "Spanish" },
  { code: "es_AR", name: "Spanish (ARG)" },
  { code: "es_MX", name: "Spanish (MEX)" },
  { code: "es_ES", name: "Spanish (SPA)" },
  { code: "sw", name: "Swahili" },
  { code: "sv", name: "Swedish" },
  { code: "ta", name: "Tamil" },
  { code: "te", name: "Telugu" },
  { code: "th", name: "Thai" },
  { code: "tr", name: "Turkish" },
  { code: "uk", name: "Ukrainian" },
  { code: "ur", name: "Urdu" },
  { code: "uz", name: "Uzbek" },
  { code: "vi", name: "Vietnamese" },
  { code: "zu", name: "Zulu" },
];

function getLanguageName(code: string): string {
  return languages.find((l) => l.code === code)?.name || code;
}

const categories = [
  {
    value: "UTILITY",
    label: "Utility",
    description: "Order updates, account alerts",
  },
  { value: "MARKETING", label: "Marketing", description: "Promotions, offers" },
  {
    value: "AUTHENTICATION",
    label: "Authentication",
    description: "OTP, verification codes",
  },
];

const headerTypes = [
  { value: "NONE", label: "None" },
  { value: "TEXT", label: "Text" },
  { value: "IMAGE", label: "Image" },
  { value: "VIDEO", label: "Video" },
  { value: "DOCUMENT", label: "Document" },
];

// Meta's per-field character limits. Neither the API nor main's form enforced
// these, so an over-length template was only rejected once it reached Meta.
const LIMITS = {
  name: 512,
  header: 60,
  body: 1024,
  footer: 60,
  buttonText: 25,
  url: 2000,
  copyCode: 15,
};

const mediaFileName = computed(() => props.mediaFile?.name || "");

watch(
  () => state.value.header_type,
  (newType, oldType) => {
    if (newType === oldType) return;
    state.value.header_content = "";
    state.value.sample_values = (state.value.sample_values || []).filter(
      (s: any) => s.component !== "header",
    );
    clearMediaFile();
  },
);

const bodyTextareaRef = ref<any>(null);
// -1 until the textarea has been focused, so the first insert goes to the end
// of the body instead of in front of it.
const savedSelectionStart = ref(-1);
const savedSelectionEnd = ref(-1);

function saveCursorPosition(event: Event) {
  const el = event.target as HTMLTextAreaElement;
  if (el && typeof el.selectionStart === "number") {
    savedSelectionStart.value = el.selectionStart;
    savedSelectionEnd.value = el.selectionEnd ?? el.selectionStart;
  }
}

function insertAtCursor(textToInsert: string, cursorOffset: number = 0) {
  const content = state.value.body_content || "";
  const startPos =
    savedSelectionStart.value < 0 ? content.length : savedSelectionStart.value;
  const endPos =
    savedSelectionEnd.value < 0 ? content.length : savedSelectionEnd.value;

  state.value.body_content =
    content.substring(0, startPos) + textToInsert + content.substring(endPos);

  const newPos = startPos + textToInsert.length + cursorOffset;
  savedSelectionStart.value = newPos;
  savedSelectionEnd.value = newPos;

  nextTick(() => {
    const el =
      bodyTextareaRef.value?.$el?.tagName === "TEXTAREA"
        ? bodyTextareaRef.value.$el
        : bodyTextareaRef.value?.$el?.querySelector("textarea");
    if (el) {
      el.focus();
      el.setSelectionRange(newPos, newPos);
    }
  });
}

function insertFormat(format: string) {
  let insertText = "";
  let offset = 0;
  switch (format) {
    case "bold":
      insertText = "**";
      offset = -1;
      break;
    case "italic":
      insertText = "__";
      offset = -1;
      break;
    case "strikethrough":
      insertText = "~~";
      offset = -1;
      break;
    case "monospace":
      insertText = "``````";
      offset = -3;
      break;
  }
  insertAtCursor(insertText, offset);
}

function extractParamNames(content: string): string[] {
  const matches = (content || "").match(/\{\{([^}]+)\}\}/g) || [];
  const seen = new Set<string>();
  const names: string[] = [];
  for (const m of matches) {
    const name = m.replace(/[{}]/g, "").trim();
    if (name && !seen.has(name)) {
      seen.add(name);
      names.push(name);
    }
  }
  return names;
}

const bodyVariables = computed(() =>
  extractParamNames(state.value.body_content),
);
const headerVariables = computed(() => {
  if (state.value.header_type !== "TEXT") return [];
  return extractParamNames(state.value.header_content);
});

function insertHeaderVariable() {
  if (headerVariables.value.length >= 1) return;
  state.value.header_content = (state.value.header_content || "") + `{{1}}`;
}

function getNextBodyVariableNumber() {
  const content = state.value.body_content || "";
  const matches = content.match(/\{\{(\d+)\}\}/g) || [];
  let maxNum = 0;
  for (const m of matches) {
    const num = parseInt(m.replace(/[{}]/g, ""), 10);
    if (!isNaN(num) && num > maxNum) maxNum = num;
  }
  return maxNum + 1;
}

function insertVariable() {
  if (bodyVariables.value.length >= 20) {
    toast.error(t("templates.maxVariables", "Maximum 20 variables allowed"));
    return;
  }
  const nextVarNum = getNextBodyVariableNumber();
  insertAtCursor(`{{${nextVarNum}}}`);
}

// Meta fixes the body text for AUTHENTICATION templates and models code delivery
// as a single OTP button, so this category replaces the body/buttons editors.
const isAuthentication = computed(
  () => state.value.category === "AUTHENTICATION",
);

// Inline, a literal {{1}} would close the template interpolation early.
const otpCodeToken = `{{1}}`;

function otpButton() {
  return (state.value.buttons || []).find((b: any) => b.type === "OTP");
}

const authOtpType = computed(() => {
  if (!isAuthentication.value) return "COPY_CODE";
  return otpButton()?.otp_type || "COPY_CODE";
});

// On the form, not a local ref, so the parent's save() can gate on it.
function setAuthOtpType(type: string) {
  if (!type) return;
  state.value.zero_tap_accepted = false;
  const needsApps = type === "ONE_TAP" || type === "ZERO_TAP";
  const existing = otpButton();

  if (existing) {
    existing.otp_type = type;
    if (needsApps && !existing.supported_apps?.length) {
      existing.supported_apps = [{ package_name: "", signature_hash: "" }];
    }
    return;
  }

  const btn: any = {
    id: crypto.randomUUID(),
    type: "OTP",
    text: "Copy code",
    otp_type: type,
  };
  if (needsApps) {
    btn.supported_apps = [{ package_name: "", signature_hash: "" }];
  }
  state.value.buttons = [btn];
}

function addSupportedApp() {
  const btn = otpButton();
  if (!btn) return;
  if (!btn.supported_apps) btn.supported_apps = [];
  if (btn.supported_apps.length < 5) {
    btn.supported_apps.push({ package_name: "", signature_hash: "" });
  }
}

function removeSupportedApp(index: number) {
  const btn = otpButton();
  if (btn?.supported_apps?.length > 1) {
    btn.supported_apps.splice(index, 1);
  }
}

watch(isAuthentication, (isAuth) => {
  if (isAuth && !otpButton()) setAuthOtpType("COPY_CODE");
});

function componentVariables(component: string): string[] {
  if (component === "header") return headerVariables.value;
  if (component === "body") return bodyVariables.value;
  return [];
}

// Positional vars carry their index in the name; named vars take their slot.
function sampleIndexFor(component: string, paramName: string): number {
  if (/^\d+$/.test(paramName)) return parseInt(paramName, 10);
  const slot = componentVariables(component).indexOf(paramName);
  return slot >= 0 ? slot + 1 : 1;
}

function findSample(component: string, paramName: string) {
  const samples = state.value.sample_values || [];
  const index = sampleIndexFor(component, paramName);
  return samples.find((s: any) => {
    if (s.component !== component) return false;
    if (/^\d+$/.test(paramName)) {
      return s.index === index || s.param_name === paramName;
    }
    return s.param_name ? s.param_name === paramName : s.index === index;
  });
}

function getSampleValue(component: string, paramName: string): string {
  return findSample(component, paramName)?.value || "";
}

// The backend sorts positional examples on index and looks named ones up by
// param_name, so both must be written.
function setSampleValue(component: string, paramName: string, value: string) {
  if (!state.value.sample_values) state.value.sample_values = [];
  const index = sampleIndexFor(component, paramName);
  const existing = findSample(component, paramName);
  if (existing) {
    existing.value = value;
    existing.index = index;
    existing.param_name = paramName;
  } else {
    state.value.sample_values.push({
      component,
      index,
      param_name: paramName,
      value,
    });
  }
}

// Rendered as body:{{name}} / header:{{1}} to match the rest of the app.
function formatVariableLabel(component: string, paramName: string): string {
  return `${component}:{{${paramName}}}`;
}

const hasTooManyHeaderVariables = computed(() => {
  if (state.value.header_type !== "TEXT") return false;
  return new Set(headerVariables.value).size > 1;
});

type ButtonType =
  | "QUICK_REPLY"
  | "URL"
  | "PHONE_NUMBER"
  | "COPY_CODE"
  | "FLOW"
  | "VOICE_CALL";

// Default label a new button starts with, so it reads as its action instead of a blank.
const DEFAULT_BUTTON_LABEL: Record<ButtonType, string> = {
  QUICK_REPLY: "Reply",
  URL: "Visit",
  PHONE_NUMBER: "Call",
  COPY_CODE: "Copy code",
  FLOW: "View",
  VOICE_CALL: "Call",
};

// The backend drops a FLOW button that has no flow_id.
function getFlowScreens(flowId: string): string[] {
  const flow = (props.flows || []).find(
    (f: any) => f.meta_flow_id === flowId || f.id === flowId,
  );
  if (!flow?.screens) return [];
  return flow.screens
    .map((s: any) => (typeof s === "string" ? s : s?.id || s?.name))
    .filter(Boolean);
}

function addButton(type: ButtonType) {
  const buttons = [...(state.value.buttons || [])];
  if (buttons.length >= MAX_BUTTONS) {
    toast.error(
      t("templates.maxButtons", `Maximum ${MAX_BUTTONS} buttons allowed`),
    );
    return;
  }

  const countOf = (t: string) => buttons.filter((b: any) => b.type === t).length;

  // URL and phone are call-to-action buttons, capped at 2 combined.
  if (type === "URL" || type === "PHONE_NUMBER") {
    if (countOf("URL") + countOf("PHONE_NUMBER") >= MAX_CTA) {
      toast.error(
        t(
          "templates.maxCta",
          `Maximum ${MAX_CTA} call-to-action (URL/Phone) buttons allowed`,
        ),
      );
      return;
    }
  }

  const limit = BUTTON_LIMITS[type];
  if (limit && countOf(type) >= limit) {
    toast.error(
      t(
        "templates.maxOfType",
        limit === 1
          ? "Only one button of this type is allowed"
          : `Maximum ${limit} buttons of this type allowed`,
      ),
    );
    return;
  }

  // Keep the set valid as it grows: group same-type buttons together, and keep
  // quick replies after the other buttons so the two groups never interleave.
  let insertIndex: number;
  const lastSameType = buttons.map((b: any) => b.type).lastIndexOf(type);
  if (lastSameType !== -1) {
    insertIndex = lastSameType + 1;
  } else if (type === "QUICK_REPLY") {
    insertIndex = buttons.length;
  } else {
    const firstQuickReply = buttons.findIndex(
      (b: any) => b.type === "QUICK_REPLY",
    );
    insertIndex = firstQuickReply === -1 ? buttons.length : firstQuickReply;
  }

  const newButton: Record<string, any> = {
    id: crypto.randomUUID(),
    type,
    text: DEFAULT_BUTTON_LABEL[type] || "",
  };
  if (type === "URL") {
    newButton.url = "";
    newButton.example = "";
  }
  if (type === "PHONE_NUMBER") newButton.phone_number = "";
  if (type === "COPY_CODE") newButton.example = "";
  if (type === "FLOW") {
    newButton.flow_id = "";
    newButton.flow_action = "navigate";
    newButton.navigate_screen = "";
  }

  buttons.splice(insertIndex, 0, newButton);
  state.value.buttons = buttons;
}

// Buttons are sent to Meta in array order, and order decides whether quick-reply and
// other buttons stay in valid groups — so let the user move them.
function moveButton(index: number, delta: number) {
  const buttons = [...(state.value.buttons || [])];
  const target = index + delta;
  if (target < 0 || target >= buttons.length) return;
  [buttons[index], buttons[target]] = [buttons[target], buttons[index]];
  state.value.buttons = buttons;
}

// Meta's grouping and per-type limits, surfaced inline as the user builds the set.
const buttonComboWarning = computed(() =>
  validateButtonCombination(state.value.buttons || []),
);

const MEDIA_ACCEPT: Record<string, string> = {
  IMAGE: "image/jpeg,image/png",
  VIDEO: "video/mp4",
  DOCUMENT: "application/pdf",
};

const MEDIA_MAX_MB: Record<string, number> = {
  IMAGE: 5,
  VIDEO: 16,
  DOCUMENT: 100,
};

const mediaAccept = computed(
  () => MEDIA_ACCEPT[state.value.header_type as string] || "",
);

// The file is only handed up to the parent, which previews it and uploads it to
// Meta on save. Picking a file costs no API call.
function onMediaFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  const limit = MEDIA_MAX_MB[state.value.header_type as string];
  if (limit && file.size > limit * 1024 * 1024) {
    toast.error(`File must be under ${limit} MB`);
    input.value = "";
    return;
  }

  state.value.header_content = "";
  emit("update:mediaFile", file);
}

function clearMediaFile() {
  state.value.header_content = "";
  emit("update:mediaFile", null);
}

onMounted(() => {
  if (!state.value.sample_values) {
    state.value.sample_values = [];
  }
});

</script>

<template>
  <!-- fieldset[disabled] natively disables every control inside, so read-only
       users and non-editable approved templates need no per-input binding. -->
  <fieldset :disabled="disabled" class="space-y-6 min-w-0 disabled:opacity-70">
    <div
      class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-x-4 gap-y-5 pb-6 mb-2 border-b border-zinc-200 dark:border-zinc-800"
    >
      <div class="space-y-2">
        <Label
          >{{ t("templates.whatsappAccount") }}
          <span class="text-destructive">*</span></Label
        >
        <Select v-model="state.whatsapp_account" :disabled="isLocked">
          <SelectTrigger class="h-10">
            <SelectValue :placeholder="t('templates.selectAccount')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem
              v-for="account in accounts"
              :key="account.id"
              :value="account.name"
            >
              {{ account.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div class="space-y-2">
        <Label
          >{{ t("templates.templateName") }}
          <span class="text-destructive">*</span></Label
        >
        <Input
          :model-value="state.name"
          @update:model-value="state.name = normalizeName(String($event))"
          :disabled="isLocked"
          :maxlength="LIMITS.name"
          placeholder="order_confirmation"
          class="w-full h-10 font-mono"
        />
        <p class="text-[10px] text-muted-foreground">
          Lowercase letters, numbers and underscores only.
        </p>
      </div>

      <div class="space-y-2">
        <!-- Local label only; Meta never sees it, so it stays editable after publish -->
        <Label>{{ t("templates.displayName", "Display Name") }}</Label>
        <Input
          id="display-name"
          v-model="state.display_name"
          :placeholder="t('templates.displayNamePlaceholder', 'Friendly name')"
          class="w-full h-10"
        />
      </div>

      <div class="space-y-2">
        <Label
          >{{ t("templates.language") }}
          <span class="text-destructive">*</span></Label
        >
        <Popover v-model:open="languageSelectorOpen">
          <PopoverTrigger as-child>
            <Button
              variant="outline"
              role="combobox"
              class="w-full h-10 justify-between"
              :disabled="isLocked"
            >
              <span>{{ getLanguageName(state.language) }}</span>
              <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
            </Button>
          </PopoverTrigger>
          <PopoverContent class="w-[300px] p-0">
            <Command>
              <CommandInput placeholder="Search language..." />
              <CommandList>
                <CommandEmpty>No language found.</CommandEmpty>
                <CommandGroup>
                  <CommandItem
                    v-for="lang in languages"
                    :key="lang.code"
                    :value="lang.name"
                    class="flex items-center gap-2 cursor-pointer"
                    @select="
                      state.language = lang.code;
                      languageSelectorOpen = false;
                    "
                  >
                    <span class="flex-1">{{ lang.name }}</span>
                    <Check
                      v-if="state.language === lang.code"
                      class="h-4 w-4 text-primary"
                    />
                  </CommandItem>
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      </div>

      <div class="space-y-2">
        <Label
          >{{ t("templates.category") }}
          <span class="text-destructive">*</span></Label
        >
        <Select v-model="state.category" :disabled="isLocked">
          <SelectTrigger class="h-10">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem
              v-for="cat in categories"
              :key="cat.value"
              :value="cat.value"
            >
              {{ cat.label }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <div>
      <div class="space-y-6">
        <!-- Meta forces header_type NONE on AUTHENTICATION templates -->
        <div
          v-if="!isAuthentication"
          class="space-y-4 p-4 border rounded-lg bg-muted/10"
        >
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-1.5">
              <div class="flex items-center h-7">
                <Label class="text-xs font-bold text-muted-foreground"
                  >Header Type</Label
                >
              </div>
              <Select v-model="state.header_type">
                <SelectTrigger class="h-9">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="type in headerTypes"
                    :key="type.value"
                    :value="type.value"
                  >
                    {{ type.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="text-[10px] text-muted-foreground">
                Optional. A header appears above the message body.
              </p>
            </div>

            <div v-if="state.header_type === 'TEXT'" class="space-y-1.5">
              <div class="flex items-center justify-between h-7">
                <Label class="text-xs font-bold text-muted-foreground"
                  >Header Text</Label
                >
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  @click="insertHeaderVariable"
                  :disabled="headerVariables.length >= 1"
                  class="h-7 px-3 text-[10px] bg-emerald-50 text-emerald-600 hover:bg-emerald-100 border-emerald-200 disabled:opacity-50"
                >
                  <Plus class="w-3 h-3 mr-1" /> Add Variable
                </Button>
              </div>
              <Input
                id="header-content"
                v-model="state.header_content"
                :maxlength="LIMITS.header"
                class="h-9 bg-background"
                placeholder="Hello!"
              />
              <div class="flex items-center justify-between">
                <p
                  v-if="hasTooManyHeaderVariables"
                  class="text-[10px] text-red-500"
                >
                  Meta allows at most one variable in a TEXT header.
                </p>
                <span v-else />
                <span class="text-[10px] text-muted-foreground">
                  {{ (state.header_content || "").length }}/{{ LIMITS.header }}
                </span>
              </div>
            </div>
          </div>

          <div
            v-if="['IMAGE', 'VIDEO', 'DOCUMENT'].includes(state.header_type)"
            class="space-y-2 pt-2 border-t"
          >
            <Label class="text-xs font-bold text-muted-foreground"
              >Sample {{ state.header_type.toLowerCase() }}</Label
            >
            <Input
              type="file"
              :accept="mediaAccept"
              @change="onMediaFileChange"
              class="h-9 text-xs bg-background"
            />

            <div
              v-if="mediaFileName"
              class="flex items-center justify-between rounded-md border bg-muted/30 px-3 py-2"
            >
              <span class="truncate text-xs">{{ mediaFileName }}</span>
              <button
                type="button"
                @click="clearMediaFile"
                class="text-muted-foreground hover:text-red-500"
              >
                <Trash2 class="h-3.5 w-3.5" />
              </button>
            </div>
            <p v-else-if="state.header_content" class="text-xs text-emerald-600">
              Sample already uploaded. Choose a file to replace it.
            </p>

            <p class="text-[10px] text-muted-foreground">
              Meta requires a sample. It is uploaded when you save, not now.
            </p>
          </div>
        </div>

        <!-- AUTHENTICATION: Meta fixes the body text, code delivery is the OTP button -->
        <div v-if="isAuthentication" class="space-y-4">
          <div class="space-y-2">
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Code Delivery Method</Label
            >
            <select
              :value="authOtpType"
              @change="
                setAuthOtpType(($event.target as HTMLSelectElement).value)
              "
              class="w-full h-10 rounded-md border bg-background px-3 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <option value="COPY_CODE">Copy Code</option>
              <option value="ONE_TAP">One-Tap Autofill (Android only)</option>
              <option value="ZERO_TAP">Zero-Tap (Android only)</option>
            </select>
            <p class="text-[10px] text-muted-foreground">
              <span v-if="authOtpType === 'COPY_CODE'"
                >User taps a button to copy the code to their clipboard.</span
              >
              <span v-else-if="authOtpType === 'ONE_TAP'"
                >User taps a button to autofill the code in your app. Requires
                app configuration.</span
              >
              <span v-else
                >The code is delivered to your app automatically. Requires app
                configuration.</span
              >
            </p>
          </div>

          <div class="space-y-2">
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Message Body</Label
            >
            <div
              class="rounded-md border bg-muted/50 p-3 text-sm text-muted-foreground"
            >
              <span class="font-mono">{{ otpCodeToken }}</span> is your
              verification code.
              <span
                v-if="state.add_security_recommendation"
                class="block mt-1"
                >For your security, do not share this code.</span
              >
            </div>
            <p class="text-[10px] text-muted-foreground">
              Authentication templates use fixed preset text defined by Meta.
            </p>
          </div>

          <div class="flex items-center gap-2">
            <input
              id="security-rec"
              type="checkbox"
              v-model="state.add_security_recommendation"
              class="h-4 w-4 rounded border-gray-300"
            />
            <Label for="security-rec" class="text-xs cursor-pointer"
              >Add security recommendation</Label
            >
          </div>

          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <input
                id="code-expiration"
                type="checkbox"
                :checked="state.code_expiration_minutes > 0"
                @change="
                  state.code_expiration_minutes = (
                    $event.target as HTMLInputElement
                  ).checked
                    ? 10
                    : 0
                "
                class="h-4 w-4 rounded border-gray-300"
              />
              <Label for="code-expiration" class="text-xs cursor-pointer"
                >Add an expiration time for the code</Label
              >
            </div>
            <div
              v-if="state.code_expiration_minutes > 0"
              class="ml-6 space-y-1"
            >
              <div class="flex items-center gap-2">
                <Input
                  type="number"
                  :model-value="state.code_expiration_minutes"
                  @update:model-value="
                    (val: string | number) =>
                      (state.code_expiration_minutes = val
                        ? parseInt(String(val), 10)
                        : 0)
                  "
                  min="1"
                  max="90"
                  class="h-9 w-24 bg-background"
                />
                <span class="text-[10px] text-muted-foreground"
                  >minutes (1-90)</span
                >
              </div>
              <p class="text-[10px] text-muted-foreground">
                Footer: "This code expires in
                {{ state.code_expiration_minutes }} minutes."
              </p>
            </div>
          </div>

          <!-- Zero-Tap requires explicit acceptance of the WhatsApp Business Terms -->
          <div
            v-if="authOtpType === 'ZERO_TAP'"
            class="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3"
          >
            <div class="flex items-start gap-2">
              <input
                id="zero-tap-tos"
                type="checkbox"
                v-model="state.zero_tap_accepted"
                class="mt-0.5 h-4 w-4 rounded border-gray-300"
              />
              <Label
                for="zero-tap-tos"
                class="text-[11px] cursor-pointer leading-relaxed"
              >
                By selecting zero-tap, I understand that my business's use of
                zero-tap authentication is subject to the
                <a
                  href="https://www.whatsapp.com/legal/business-terms/"
                  target="_blank"
                  class="underline text-primary"
                  >WhatsApp Business Terms of Service</a
                >. It is my business's responsibility to ensure its customers
                expect that the code will be automatically filled in on their
                behalf.
              </Label>
            </div>
          </div>

          <!-- ONE_TAP / ZERO_TAP: the app that receives the autofilled code -->
          <div
            v-if="authOtpType === 'ONE_TAP' || authOtpType === 'ZERO_TAP'"
            class="space-y-3 rounded-lg border p-3"
          >
            <div v-if="authOtpType === 'ONE_TAP' && otpButton()" class="space-y-1">
              <Label class="text-xs">Autofill Text</Label>
              <Input
                v-model="otpButton().autofill_text"
                placeholder="Autofill"
                class="h-9 bg-background"
              />
            </div>
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label class="text-xs">Supported Apps *</Label>
                <Button
                  v-if="(otpButton()?.supported_apps?.length || 0) < 5"
                  type="button"
                  variant="outline"
                  size="sm"
                  class="h-7 px-2 text-[10px]"
                  @click="addSupportedApp"
                >
                  <Plus class="w-3 h-3 mr-1" /> Add App
                </Button>
              </div>
              <div
                v-for="(app, i) in otpButton()?.supported_apps || []"
                :key="i"
                class="flex items-end gap-2"
              >
                <div class="flex-1 space-y-1">
                  <Label class="text-[10px]">Package Name *</Label>
                  <Input
                    v-model="app.package_name"
                    placeholder="com.example.app"
                    class="h-9 bg-background"
                  />
                </div>
                <div class="flex-1 space-y-1">
                  <Label class="text-[10px]">Signature Hash *</Label>
                  <Input
                    v-model="app.signature_hash"
                    placeholder="K8a/AINcGX7"
                    class="h-9 bg-background"
                  />
                </div>
                <Button
                  v-if="(otpButton()?.supported_apps?.length || 0) > 1"
                  type="button"
                  variant="ghost"
                  size="sm"
                  class="h-9 w-9 shrink-0 p-0"
                  @click="removeSupportedApp(Number(i))"
                >
                  <Trash2 class="w-3.5 h-3.5 text-destructive" />
                </Button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="!isAuthentication" class="space-y-2">
          <Label class="text-xs font-bold uppercase text-muted-foreground"
            >Message Body</Label
          >
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="h-7 px-2 text-xs font-bold bg-background hover:bg-muted"
                @click="insertFormat('bold')"
                title="Bold"
                >B</Button
              >
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="h-7 px-2 text-xs italic bg-background hover:bg-muted"
                @click="insertFormat('italic')"
                title="Italic"
                >I</Button
              >
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="h-7 px-2 text-xs line-through bg-background hover:bg-muted"
                @click="insertFormat('strikethrough')"
                title="Strikethrough"
                >S</Button
              >
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="h-7 px-2 text-xs font-mono bg-background hover:bg-muted"
                @click="insertFormat('monospace')"
                title="Monospace"
                >&lt;/&gt;</Button
              >
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              @click="insertVariable"
              :disabled="bodyVariables.length >= 20"
              class="h-7 px-3 text-[10px] bg-emerald-50 text-emerald-600 hover:bg-emerald-100 border-emerald-200 disabled:opacity-50"
            >
              <Plus class="w-3 h-3 mr-1" /> Add Variable
            </Button>
          </div>
          <Textarea
            ref="bodyTextareaRef"
            v-model="state.body_content"
            :rows="6"
            :maxlength="LIMITS.body"
            @blur="saveCursorPosition"
            @keyup="saveCursorPosition"
            @mouseup="saveCursorPosition"
            class="bg-background focus:ring-emerald-500"
            placeholder="Type your message..."
          />
          <div class="flex justify-between text-[10px] text-muted-foreground">
            <span v-pre>Variables must be sequential: {{1}}, {{2}}, {{3}}…</span>
            <span
              >{{ (state.body_content || "").length }}/{{ LIMITS.body }}</span
            >
          </div>
        </div>

        <div
          v-if="
            !isAuthentication &&
            (bodyVariables.length > 0 || headerVariables.length > 0)
          "
          class="space-y-4 pt-1"
        >
          <div>
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Sample Values for Variables</Label
            >
            <p class="text-[10px] text-muted-foreground mt-1">
              Provide sample values for your variables to see how they look in
              the preview. Required for Meta submission.
            </p>
          </div>

          <div v-if="headerVariables.length > 0" class="space-y-2">
            <p class="text-[10px] font-medium text-muted-foreground">
              Header Variables
            </p>
            <div
              v-for="paramName in headerVariables"
              :key="'header-' + paramName"
              class="flex items-center gap-3"
            >
              <span
                class="text-[10px] font-mono bg-muted border text-muted-foreground px-2 py-1 rounded min-w-[50px] text-center"
                >{{ formatVariableLabel('header', paramName) }}</span
              >
              <Input
                type="text"
                :value="getSampleValue('header', paramName)"
                @input="
                  setSampleValue(
                    'header',
                    paramName,
                    ($event.target as HTMLInputElement).value,
                  )
                "
                :placeholder="'e.g. ' + paramName"
                :class="[
                  'flex-1 h-8 text-xs bg-background',
                  getSampleValue('header', paramName) ? '' : 'border-red-500',
                ].join(' ')"
              />
            </div>
          </div>

          <div v-if="bodyVariables.length > 0" class="space-y-2">
            <p class="text-[10px] font-medium text-muted-foreground">
              Body Variables
            </p>
            <div
              v-for="paramName in bodyVariables"
              :key="'body-' + paramName"
              class="flex items-center gap-3"
            >
              <span
                class="text-[10px] font-mono bg-muted border text-muted-foreground px-2 py-1 rounded min-w-[50px] text-center"
                >{{ formatVariableLabel('body', paramName) }}</span
              >
              <Input
                type="text"
                :value="getSampleValue('body', paramName)"
                @input="
                  setSampleValue(
                    'body',
                    paramName,
                    ($event.target as HTMLInputElement).value,
                  )
                "
                :placeholder="'e.g. ' + paramName"
                :class="[
                  'flex-1 h-8 text-xs bg-background',
                  getSampleValue('body', paramName) ? '' : 'border-red-500',
                ].join(' ')"
              />
            </div>
          </div>
        </div>

        <div v-if="!isAuthentication" class="space-y-2">
          <div class="flex items-center justify-between border-b pb-2">
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Footer Text</Label
            >
            <span class="text-[10px] text-muted-foreground">Optional</span>
          </div>
          <Textarea
            id="footer-content"
            v-model="state.footer_content"
            :rows="1"
            :maxlength="LIMITS.footer"
            placeholder="E.g. Reply STOP to opt out"
            class="bg-background min-h-9 py-1.5"
          />
          <div class="flex justify-end text-[10px] text-muted-foreground">
            {{ (state.footer_content || "").length }}/{{ LIMITS.footer }}
          </div>
        </div>

        <div v-if="!isAuthentication" class="space-y-4 pt-2 border-t">
          <div class="flex items-center justify-between border-b pb-2">
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Action Buttons</Label
            >
            <div class="flex flex-wrap gap-1">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="text-emerald-500"
                @click="addButton('QUICK_REPLY')"
                >+ Reply</Button
              >
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="text-emerald-500"
                @click="addButton('URL')"
                >+ URL</Button
              >
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="text-emerald-500"
                @click="addButton('PHONE_NUMBER')"
                >+ Phone</Button
              >
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="text-emerald-500"
                @click="addButton('COPY_CODE')"
                >+ Copy Code</Button
              >
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="text-emerald-500"
                @click="addButton('FLOW')"
                >+ Flow</Button
              >
              <Button
                type="button"
                variant="ghost"
                size="sm"
                class="text-emerald-500"
                @click="addButton('VOICE_CALL')"
                >+ Call</Button
              >
            </div>
          </div>

          <div class="space-y-3 mt-2">
            <template v-for="(btn, idx) in state.buttons" :key="btn.id">
              <div class="p-3 bg-muted/10 border rounded-md space-y-3">
                <div class="flex items-center justify-between">
                  <Badge variant="outline" class="text-[9px] uppercase">{{
                    btn.type
                  }}</Badge>
                  <div class="flex items-center gap-1">
                    <button
                      type="button"
                      title="Move up"
                      :disabled="Number(idx) === 0"
                      @click="moveButton(Number(idx), -1)"
                      class="text-muted-foreground hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed"
                    >
                      <ChevronUp class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      title="Move down"
                      :disabled="Number(idx) === state.buttons.length - 1"
                      @click="moveButton(Number(idx), 1)"
                      class="text-muted-foreground hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed"
                    >
                      <ChevronDown class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      title="Remove"
                      @click="state.buttons.splice(idx, 1)"
                      class="ml-1 text-muted-foreground hover:text-red-500"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>

                <div class="space-y-1.5">
                  <div class="flex items-center justify-between">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Button Label</Label
                    >
                    <span class="text-[10px] text-muted-foreground">
                      {{ (btn.text || "").length }}/{{ LIMITS.buttonText }}
                    </span>
                  </div>
                  <Input
                    v-model="btn.text"
                    :maxlength="LIMITS.buttonText"
                    placeholder="Label"
                    :class="[
                      'h-8 text-xs bg-background',
                      (btn.text || '').trim() ? '' : 'border-red-500',
                    ].join(' ')"
                  />
                </div>

                <div v-if="btn.type === 'URL'" class="space-y-1.5">
                  <div class="flex items-center justify-between">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Website URL</Label
                    >
                    <div class="flex bg-muted p-0.5 rounded border">
                      <button
                        @click="setUrlDynamic(btn, false)"
                        type="button"
                        :class="[
                          'text-[10px] font-bold px-2 py-0.5 rounded transition-all',
                          !isDynamicUrl(btn)
                            ? 'bg-background text-emerald-600 shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                        ]"
                      >
                        Static
                      </button>
                      <button
                        @click="setUrlDynamic(btn, true)"
                        type="button"
                        :class="[
                          'text-[10px] font-bold px-2 py-0.5 rounded transition-all',
                          isDynamicUrl(btn)
                            ? 'bg-background text-emerald-600 shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                        ]"
                      >
                        Dynamic
                      </button>
                    </div>
                  </div>

                  <div v-if="isDynamicUrl(btn)" class="flex items-center">
                    <Input
                      :value="urlPrefix(btn)"
                      @input="
                        setUrlPrefix(
                          btn,
                          ($event.target as HTMLInputElement).value,
                        )
                      "
                      placeholder="https://example.com/item/"
                      class="h-8 font-mono text-xs flex-1 rounded-r-none border-r-0 bg-background focus:z-10"
                    />
                    <div
                      v-pre
                      class="bg-muted text-muted-foreground px-3 h-8 flex items-center rounded-r-md border text-[11px] font-mono font-bold select-none"
                    >
                      {{ 1 }}
                    </div>
                  </div>
                  <Input
                    v-else
                    v-model="btn.url"
                    :maxlength="LIMITS.url"
                    placeholder="https://example.com"
                    class="h-8 font-mono text-xs bg-background"
                  />

                  <div v-if="isDynamicUrl(btn)" class="space-y-1.5 pt-1">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Example URL</Label
                    >
                    <div class="flex items-center">
                      <span
                        class="h-8 max-w-[45%] flex items-center px-2.5 rounded-l-md border border-r-0 bg-muted text-[11px] font-mono text-muted-foreground truncate select-none"
                      >
                        {{ urlPrefix(btn) || "https://example.com/item/" }}
                      </span>
                      <Input
                        v-model="btn.example"
                        placeholder="12345"
                        :class="[
                          'h-8 flex-1 rounded-l-none font-mono text-xs bg-background',
                          (btn.example || '').trim() ? '' : 'border-red-500',
                        ].join(' ')"
                      />
                    </div>
                    <p class="text-[10px] text-muted-foreground">
                      Meta reviews the template against this full example URL.
                    </p>
                  </div>
                </div>

                <div v-if="btn.type === 'PHONE_NUMBER'" class="space-y-1.5">
                  <Label
                    class="text-[10px] font-bold text-muted-foreground uppercase"
                    >Phone Number</Label
                  >
                  <Input
                    v-model="btn.phone_number"
                    placeholder="+1234567890"
                    class="h-8 text-xs bg-background"
                  />
                  <p
                    v-if="btn.phone_number && !isValidPhone(btn.phone_number)"
                    class="text-[10px] text-red-500"
                  >
                    Use international format, e.g. +14155551234.
                  </p>
                </div>

                <div v-if="btn.type === 'COPY_CODE'" class="space-y-1.5">
                  <div class="flex items-center justify-between">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Example Code</Label
                    >
                    <span class="text-[10px] text-muted-foreground">
                      {{ (btn.example || "").length }}/{{ LIMITS.copyCode }}
                    </span>
                  </div>
                  <Input
                    v-model="btn.example"
                    :maxlength="LIMITS.copyCode"
                    placeholder="SAVE20"
                    class="h-8 text-xs bg-background"
                  />
                </div>

                <div v-if="btn.type === 'FLOW'" class="grid grid-cols-2 gap-3">
                  <div class="space-y-1.5">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Flow</Label
                    >
                    <select
                      v-model="btn.flow_id"
                      class="w-full h-8 rounded-md border bg-background px-2 text-xs disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <option value="">Select a Flow...</option>
                      <option
                        v-for="flow in flows || []"
                        :key="flow.id"
                        :value="flow.meta_flow_id || flow.id"
                      >
                        {{ flow.name }}
                      </option>
                    </select>
                    <p
                      v-if="!(flows || []).length"
                      class="text-[10px] text-muted-foreground"
                    >
                      No published Flows available.
                    </p>
                  </div>

                  <div class="space-y-1.5">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Flow Action</Label
                    >
                    <select
                      v-model="btn.flow_action"
                      class="w-full h-8 rounded-md border bg-background px-2 text-xs disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <option value="navigate">Navigate</option>
                      <option value="data_exchange">Data Exchange</option>
                    </select>
                  </div>

                  <div
                    v-if="
                      btn.flow_action === 'navigate' &&
                      btn.flow_id &&
                      getFlowScreens(btn.flow_id).length > 0
                    "
                    class="col-span-2 space-y-1.5"
                  >
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Screen</Label
                    >
                    <select
                      v-model="btn.navigate_screen"
                      class="w-full h-8 rounded-md border bg-background px-2 text-xs disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <option value="">Select a screen...</option>
                      <option
                        v-for="screen in getFlowScreens(btn.flow_id)"
                        :key="screen"
                        :value="screen"
                      >
                        {{ screen }}
                      </option>
                    </select>
                  </div>
                </div>

                <p
                  v-if="btn.type === 'VOICE_CALL'"
                  class="text-[10px] text-muted-foreground"
                >
                  Opens a WhatsApp voice call with your business. No extra
                  configuration needed.
                </p>
              </div>
            </template>

            <div
              v-if="buttonComboWarning"
              class="flex gap-2 items-start p-3 rounded-md border border-amber-500/40 bg-amber-500/10 text-[11px] text-amber-700 dark:text-amber-400"
            >
              <AlertCircle class="h-4 w-4 shrink-0 mt-px" />
              <span>{{ buttonComboWarning }}</span>
            </div>
          </div>
        </div>

        <div
          class="flex gap-3 p-4 bg-blue-950/20 border border-blue-900/30 rounded-lg"
        >
          <AlertCircle class="h-5 w-5 text-blue-500 shrink-0" />
          <div class="space-y-1 text-sm text-blue-600 dark:text-blue-400">
            <p class="font-medium">Before you submit</p>
            <p v-pre class="text-xs opacity-80">
              Variables like {{1}} are replaced with real data when the message
              is sent. Meta reviews your sample values, so make them look like
              real customer data.
            </p>
          </div>
        </div>
      </div>

    </div>
  </fieldset>
</template>
