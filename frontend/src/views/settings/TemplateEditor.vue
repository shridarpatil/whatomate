<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import {
  Trash2,
  Plus,
  Loader2,
  Check,
  ChevronsUpDown,
  AlertCircle,
} from "lucide-vue-next";

// UI Components
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

import TemplatePreview from "./TemplatePreview.vue";
import { templatesService } from "@/services/api";
import { toast } from "vue-sonner";

const { t } = useI18n();

const props = defineProps<{
  modelValue: Record<string, any>;
  isEdit?: boolean;
  accounts: any[];
}>();

const emit = defineEmits(["update:modelValue"]);

// Prevent Prop Mutation via Local State Cloning
const state = ref(JSON.parse(JSON.stringify(props.modelValue)));

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
      state.value = JSON.parse(JSON.stringify(newVal));
    }
  },
  { deep: true },
);

// Constants
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

// Header & Media State
const headerMediaUploading = ref(false);
const headerMediaFilename = ref("");

// Reset header content safely when changing types
watch(
  () => state.value.header_type,
  (newType, oldType) => {
    if (newType !== oldType) {
      state.value.header_content = "";
      if (state.value.sample_values) {
        state.value.sample_values = state.value.sample_values.filter(
          (s: any) => s.component !== "header",
        );
      }
      headerMediaFilename.value = "";
    }
  },
);

// Cursor & Formatting
const bodyTextareaRef = ref<any>(null);
const savedSelectionStart = ref(0);
const savedSelectionEnd = ref(0);

function saveCursorPosition(event: Event) {
  const el = event.target as HTMLTextAreaElement;
  if (el && typeof el.selectionStart === "number") {
    savedSelectionStart.value = el.selectionStart;
    savedSelectionEnd.value = el.selectionEnd ?? el.selectionStart;
  }
}

function insertAtCursor(textToInsert: string, cursorOffset: number = 0) {
  const startPos = savedSelectionStart.value;
  const endPos = savedSelectionEnd.value;
  const content = state.value.body_content || "";

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

// Variables & Samples
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

function getSampleValue(component: string, paramName: string): string {
  const sample = (state.value.sample_values || []).find(
    (s: any) => s.component === component && s.param_name === paramName,
  );
  return sample?.value || "";
}

function setSampleValue(component: string, paramName: string, value: string) {
  if (!state.value.sample_values) state.value.sample_values = [];
  const existingIndex = state.value.sample_values.findIndex(
    (s: any) => s.component === component && s.param_name === paramName,
  );
  if (existingIndex >= 0) {
    state.value.sample_values[existingIndex].value = value;
  } else {
    state.value.sample_values.push({ component, param_name: paramName, value });
  }
}

function formatVariableLabel(paramName: string): string {
  return `{{${paramName}}}`;
}

// Button Handling
function addButton(type: "QUICK_REPLY" | "URL" | "PHONE_NUMBER") {
  const buttons = [...(state.value.buttons || [])];
  if (buttons.length >= 10) {
    alert("Maximum 10 buttons allowed.");
    return;
  }

  const urlCount = buttons.filter((b: any) => b.type === "URL").length;
  const phoneCount = buttons.filter(
    (b: any) => b.type === "PHONE_NUMBER",
  ).length;

  if (type === "URL" || type === "PHONE_NUMBER") {
    if (urlCount + phoneCount >= 2) {
      alert("Maximum 2 Call to Action (URL/Phone) buttons allowed.");
      return;
    }
  }
  if (type === "URL" && urlCount >= 2) {
    alert("Maximum 2 Website URL buttons allowed.");
    return;
  }
  if (type === "PHONE_NUMBER" && phoneCount >= 1) {
    alert("Maximum 1 Phone Number button allowed.");
    return;
  }

  const sameTypeButtons = buttons
    .map((b, index) => ({ ...b, originalIndex: index }))
    .filter((b) => b.type === type);
  let insertIndex = buttons.length;
  if (sameTypeButtons.length > 0) {
    const lastSameType = sameTypeButtons[sameTypeButtons.length - 1];
    insertIndex = lastSameType.originalIndex + 1;
  }

  const newButton = {
    id: crypto.randomUUID(),
    type,
    text: "",
    url: "",
    urlType: "STATIC",
    phone_number: "",
  };
  buttons.splice(insertIndex, 0, newButton);
  state.value.buttons = buttons;
}

// Media Upload
async function handleMediaUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  console.log("account:", state.value.whatsapp_account);
  console.log("file:", file.name, file.type, file.size);

  if (!state.value.whatsapp_account) {
    toast.error(t("templates.selectAccountFirst"));
    return;
  }

  headerMediaFilename.value = file.name;
  headerMediaUploading.value = true;
  try {
    const response = await templatesService.uploadMedia(
      state.value.whatsapp_account,
      file,
    );
    state.value.header_content = response.data.data.handle;
    headerMediaFilename.value = response.data.data.filename;
    console.log("upload response:", response);
    toast.success(t("templates.mediaUploadedSuccess"));
  } catch (error) {
    console.error("upload error:", error);
    toast.error(t("templates.uploadFailed"));
  } finally {
    headerMediaUploading.value = false;
  }
}

onMounted(() => {
  if (state.value?.buttons) {
    state.value.buttons = state.value.buttons.map((b: any) => ({
      ...b,
      id: b.id || crypto.randomUUID(),
    }));
  }
  if (!state.value.sample_values) {
    state.value.sample_values = [];
  }
});

function resetState() {
  headerMediaUploading.value = false;
  headerMediaFilename.value = "";
  savedSelectionStart.value = 0;
  savedSelectionEnd.value = 0;
}

defineExpose({
  resetState,
});
</script>

<template>
  <div class="space-y-6">
    <div
      class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 py-4 mb-4 border-b border-zinc-200 dark:border-zinc-800 pb-6"
    >
      <div class="space-y-2">
        <Label
          >{{ t("templates.whatsappAccount") }}
          <span class="text-destructive">*</span></Label
        >
        <select
          v-model="state.whatsapp_account"
          class="w-full h-10 rounded-md border bg-background px-3 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="!!isEdit"
        >
          <option value="">{{ t("templates.selectAccount") }}...</option>
          <option
            v-for="account in accounts"
            :key="account.id"
            :value="account.name"
          >
            {{ account.name }}
          </option>
        </select>
      </div>

      <div class="space-y-2">
        <Label
          >{{ t("templates.templateName") }}
          <span class="text-destructive">*</span></Label
        >
        <Input
          v-model="state.name"
          :disabled="!!isEdit"
          :placeholder="t('templates.templateNamePlaceholder')"
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
              :disabled="!!isEdit"
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
        <select
          v-model="state.category"
          class="w-full h-10 rounded-md border bg-background px-3 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="!!isEdit"
        >
          <option v-for="cat in categories" :key="cat.value" :value="cat.value">
            {{ cat.label }}
          </option>
        </select>
      </div>
    </div>

    <div class="grid grid-cols-12 gap-8">
      <div class="col-span-12 lg:col-span-7 space-y-6">
        <div class="space-y-4 p-4 border rounded-lg bg-muted/10">
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label class="text-xs font-bold text-muted-foreground"
                >Header Type</Label
              >
              <select
                v-model="state.header_type"
                class="w-full h-9 bg-background border rounded-md px-2 text-sm"
              >
                <option value="NONE">None</option>
                <option value="TEXT">Text</option>
                <option value="IMAGE">Image</option>
                <option value="VIDEO">Video</option>
                <option value="DOCUMENT">Document</option>
              </select>
            </div>

            <div v-if="state.header_type === 'TEXT'" class="space-y-2">
              <div class="flex items-center justify-between">
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
                v-model="state.header_content"
                class="h-9 bg-background"
                placeholder="Hello!"
              />
            </div>
          </div>

          <div
            v-if="['IMAGE', 'VIDEO', 'DOCUMENT'].includes(state.header_type)"
            class="space-y-2 pt-2 border-t"
          >
            <Label class="text-[10px] text-muted-foreground"
              >Upload Media Sample (Required for Meta)</Label
            >
            <div class="flex items-center gap-3">
              <Input
                type="file"
                @change="handleMediaUpload"
                class="h-9 text-xs flex-1 bg-background"
              />
              <Loader2
                v-if="headerMediaUploading"
                class="w-4 h-4 animate-spin text-emerald-500"
              />
            </div>
            <p
              v-if="state.header_content"
              class="text-[9px] font-mono text-emerald-500 truncate"
            >
              Handle: {{ state.header_content }}
            </p>
          </div>
        </div>

        <div class="space-y-2">
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
            @blur="saveCursorPosition"
            @keyup="saveCursorPosition"
            @mouseup="saveCursorPosition"
            class="bg-background focus:ring-emerald-500"
            placeholder="Type your message..."
          />
          <div class="flex justify-between text-[10px] text-muted-foreground">
            <span>Variables must be sequential {{ 1 }}, {{ 2 }}...</span>
            <span
              :class="{
                'text-red-500': (state.body_content?.length ?? 0) > 1024,
              }"
              >{{ state.body_content?.length ?? 0 }}/1024</span
            >
          </div>
        </div>

        <div
          v-if="bodyVariables.length > 0 || headerVariables.length > 0"
          class="space-y-4 pt-1"
        >
          <div>
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Sample Values</Label
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
                >{{ formatVariableLabel(paramName) }}</span
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
                :placeholder="'Example for ' + paramName + '...'"
                class="flex-1 h-8 text-xs bg-background"
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
                >{{ formatVariableLabel(paramName) }}</span
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
                :placeholder="'Example for ' + paramName + '...'"
                class="flex-1 h-8 text-xs bg-background"
              />
            </div>
          </div>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between border-b pb-2">
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Footer Text</Label
            >
            <span class="text-[10px] text-muted-foreground"
              >Optional, max 60 chars</span
            >
          </div>
          <Input
            v-model="state.footer_content"
            placeholder="E.g. Reply STOP to opt out"
            class="h-9 bg-background"
          />
        </div>

        <div class="space-y-4 pt-2 border-t">
          <div class="flex items-center justify-between border-b pb-2">
            <Label class="text-xs font-bold uppercase text-muted-foreground"
              >Action Buttons</Label
            >
            <div class="flex gap-2">
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
            </div>
          </div>

          <div
            class="space-y-0 divide-y divide-slate-100 border-t border-zinc-100 mt-2"
          >
            <template v-for="(btn, idx) in state.buttons" :key="btn.id">
              <div class="p-3 bg-muted/10 border rounded-md mb-3">
                <div class="flex justify-between mb-2">
                  <Badge variant="outline" class="text-[9px] uppercase">{{
                    btn.type
                  }}</Badge>
                  <button
                    type="button"
                    @click="state.buttons.splice(idx, 1)"
                    class="text-muted-foreground hover:text-red-500"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>

                <div class="grid grid-cols-2 gap-3">
                  <div class="space-y-1.5">
                    <Label
                      class="text-[10px] font-bold text-muted-foreground uppercase"
                      >Button Label</Label
                    >
                    <Input
                      v-model="btn.text"
                      placeholder="Label"
                      class="h-8 text-xs bg-background"
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
                          @click="btn.urlType = 'STATIC'"
                          type="button"
                          :class="[
                            'text-[10px] font-bold px-2 py-0.5 rounded transition-all',
                            btn.urlType !== 'DYNAMIC'
                              ? 'bg-background text-emerald-600 shadow-sm'
                              : 'text-muted-foreground hover:text-foreground',
                          ]"
                        >
                          Static
                        </button>
                        <button
                          @click="btn.urlType = 'DYNAMIC'"
                          type="button"
                          :class="[
                            'text-[10px] font-bold px-2 py-0.5 rounded transition-all',
                            btn.urlType === 'DYNAMIC'
                              ? 'bg-background text-emerald-600 shadow-sm'
                              : 'text-muted-foreground hover:text-foreground',
                          ]"
                        >
                          Dynamic
                        </button>
                      </div>
                    </div>

                    <div
                      v-if="btn.urlType === 'DYNAMIC'"
                      class="flex flex-col gap-2"
                    >
                      <div class="flex items-center">
                        <Input
                          :value="(btn.url || '').replace(/\{\{1\}\}/g, '')"
                          @input="
                            btn.url =
                              ($event.target as HTMLInputElement).value.replace(
                                /\{\{|\}\}/g,
                                '',
                              ) + '{{1}}'
                          "
                          placeholder="https://example.com/item/"
                          class="h-8 font-mono text-xs flex-1 rounded-r-none border-r-0 bg-background focus:z-10"
                        />
                        <div
                          v-pre
                          class="bg-muted text-muted-foreground px-3 h-8 flex items-center rounded-r-md border border-l-0 text-[11px] font-mono font-bold select-none"
                        >
                          {{ 1 }}
                        </div>
                      </div>
                      <div
                        class="flex items-center gap-2 bg-muted/30 p-2 rounded-md border"
                      >
                        <Label
                          class="text-[10px] text-muted-foreground font-bold uppercase whitespace-nowrap"
                          >Example Suffix:</Label
                        >
                        <Input
                          :value="getSampleValue('button_url', String(idx))"
                          @input="
                            setSampleValue(
                              'button_url',
                              String(idx),
                              ($event.target as HTMLInputElement).value,
                            )
                          "
                          placeholder="e.g. 12345"
                          class="h-7 text-[11px] bg-background flex-1"
                        />
                      </div>
                      <p class="text-[10px] text-muted-foreground mt-1">
                        <span class="font-mono text-emerald-600 font-medium"
                          >Full Preview:
                          {{
                            (btn.url || "").replace(/\{\{1\}\}/g, "") ||
                            "https://..."
                          }}{{
                            getSampleValue("button_url", String(idx)) || "12345"
                          }}</span
                        >
                      </p>
                    </div>
                    <div v-else>
                      <Input
                        v-model="btn.url"
                        placeholder="https://example.com"
                        class="h-8 font-mono text-xs bg-background"
                      />
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
                  </div>
                </div>
              </div>
            </template>
          </div>
        </div>

        <div
          class="flex gap-3 p-4 bg-blue-950/20 border border-blue-900/30 rounded-lg"
        >
          <AlertCircle class="h-5 w-5 text-blue-500 shrink-0" />
          <div class="space-y-1 text-sm text-blue-600 dark:text-blue-400">
            <p class="font-medium">Template Submission Info</p>
            <p class="text-xs opacity-80">
              Variables like {{ 1 }} will be replaced with real data when
              sending messages. Make sure your samples reflect real customer
              data.
            </p>
          </div>
        </div>
      </div>

      <div class="col-span-12 lg:col-span-5">
        <div
          class="sticky top-4 bg-[#e5ddd5] dark:bg-[#111b21] rounded-2xl p-6 min-h-[450px] flex flex-col items-start shadow-sm relative overflow-hidden"
        >
          <p
            class="text-[10px] font-bold text-muted-foreground mb-4 uppercase tracking-wider"
          >
            Live Preview
          </p>
          <TemplatePreview
            :header-type="state.header_type"
            :header-content="state.header_content"
            :body-content="state.body_content"
            :footer-content="state.footer_content"
            :buttons="state.buttons"
            :sample-values="state.sample_values"
            contained
          />
        </div>
      </div>
    </div>
  </div>
</template>
