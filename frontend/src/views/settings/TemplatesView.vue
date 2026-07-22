<script setup lang="ts">
<<<<<<< HEAD
import { ref, onMounted, watch, computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  PageHeader,
  SearchInput,
  DataTable,
  DeleteConfirmDialog,
  type Column,
} from "@/components/shared";
=======
import { ref, onMounted, watch, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, DataTable, IconButton, DeleteConfirmDialog, ErrorState, type Column } from '@/components/shared'
import { api, templatesService } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import { Plus, RefreshCw, FileText, Pencil, Trash2, Loader2, MessageSquare, Image, FileIcon, Video } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { useSearchPagination } from '@/composables/useSearchPagination'
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449

import TemplateEditor from "./TemplateEditor.vue";
import TemplatePreview from "./TemplatePreview.vue";
import { api, templatesService } from "@/services/api";
import { useOrganizationsStore } from "@/stores/organizations";
import { toast } from "vue-sonner";
import {
  Plus,
  RefreshCw,
  FileText,
  Eye,
  Pencil,
  Trash2,
  Loader2,
  MessageSquare,
  Image,
  FileIcon,
  Video,
  Send,
} from "lucide-vue-next";
import { getErrorMessage } from "@/lib/api-utils";
import { useDebounceFn } from "@vueuse/core";

const { t } = useI18n();

interface WhatsAppAccount {
  id: string;
  name: string;
  phone_id: string;
}
interface Template {
  id: string;
  whatsapp_account: string;
  meta_template_id: string;
  name: string;
  display_name: string;
  language: string;
  category: string;
  status: string;
  header_type: "NONE" | "TEXT" | "IMAGE" | "VIDEO" | "DOCUMENT";
  header_content: string;
  body_content: string;
  footer_content: string;
  buttons: any[];
  sample_values: any[];
}

const organizationsStore = useOrganizationsStore();
const templates = ref<Template[]>([]);
const accounts = ref<WhatsAppAccount[]>([]);
const isLoading = ref(true);
const isSyncing = ref(false);
const searchQuery = ref("");
const selectedAccount = ref<string>(
  localStorage.getItem("templates_selected_account") || "all",
);

<<<<<<< HEAD
// --- Original Dialog States ---
const isDialogOpen = ref(false);
const isSubmitting = ref(false);
const editingTemplate = ref<Template | null>(null);
const isPreviewOpen = ref(false);
const previewTemplate = ref<Template | null>(null);
const deleteDialogOpen = ref(false);
const templateToDelete = ref<Template | null>(null);
const publishDialogOpen = ref(false);
const templateToPublish = ref<Template | null>(null);
const publishingTemplateId = ref<string | null>(null);
const editorRef = ref<InstanceType<typeof TemplateEditor> | null>(null);

// --- Form Data ---
const formData = ref({
  whatsapp_account: "",
  name: "",
  display_name: "",
  language: "en",
  category: "UTILITY",
  header_type: "NONE",
  header_content: "",
  body_content: "",
  footer_content: "",
  buttons: [],
  sample_values: [],
});

// --- Pagination & Sorting (Restored) ---
const currentPage = ref(1);
const totalItems = ref(0);
const pageSize = 20;
const sortKey = ref("name");
const sortDirection = ref<"asc" | "desc">("asc");

const columns = computed<Column<Template>[]>(() => [
  { key: "name", label: t("templates.name"), sortable: true },
  { key: "category", label: t("templates.category"), sortable: true },
  { key: "status", label: t("templates.status"), sortable: true },
  { key: "language", label: t("templates.language"), sortable: true },
  { key: "header_type", label: t("templates.header") },
  { key: "actions", label: t("common.actions"), align: "right" },
]);
=======
const templates = ref<Template[]>([])
const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const error = ref<string | null>(null)
const isSyncing = ref(false)
const selectedAccount = ref<string>(localStorage.getItem('templates_selected_account') || 'all')

// Delete dialog state
const deleteDialogOpen = ref(false)
const templateToDelete = ref<Template | null>(null)
const isDeleting = ref(false)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchTemplates(),
})

const columns = computed<Column<Template>[]>(() => [
  { key: 'name', label: t('templates.name'), sortable: true },
  { key: 'category', label: t('templates.category'), sortable: true },
  { key: 'status', label: t('templates.status'), sortable: true },
  { key: 'language', label: t('templates.language'), sortable: true },
  { key: 'header_type', label: t('templates.header') },
  { key: 'actions', label: '', align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const languages = [
  { code: 'af', name: 'Afrikaans' },
  { code: 'sq', name: 'Albanian' },
  { code: 'ar', name: 'Arabic' },
  { code: 'az', name: 'Azerbaijani' },
  { code: 'bn', name: 'Bengali' },
  { code: 'bg', name: 'Bulgarian' },
  { code: 'ca', name: 'Catalan' },
  { code: 'zh_CN', name: 'Chinese (CHN)' },
  { code: 'zh_HK', name: 'Chinese (HKG)' },
  { code: 'zh_TW', name: 'Chinese (TAI)' },
  { code: 'hr', name: 'Croatian' },
  { code: 'cs', name: 'Czech' },
  { code: 'da', name: 'Danish' },
  { code: 'nl', name: 'Dutch' },
  { code: 'en', name: 'English' },
  { code: 'en_GB', name: 'English (UK)' },
  { code: 'en_US', name: 'English (US)' },
  { code: 'et', name: 'Estonian' },
  { code: 'fil', name: 'Filipino' },
  { code: 'fi', name: 'Finnish' },
  { code: 'fr', name: 'French' },
  { code: 'ka', name: 'Georgian' },
  { code: 'de', name: 'German' },
  { code: 'el', name: 'Greek' },
  { code: 'gu', name: 'Gujarati' },
  { code: 'ha', name: 'Hausa' },
  { code: 'he', name: 'Hebrew' },
  { code: 'hi', name: 'Hindi' },
  { code: 'hu', name: 'Hungarian' },
  { code: 'id', name: 'Indonesian' },
  { code: 'ga', name: 'Irish' },
  { code: 'it', name: 'Italian' },
  { code: 'ja', name: 'Japanese' },
  { code: 'kn', name: 'Kannada' },
  { code: 'kk', name: 'Kazakh' },
  { code: 'rw_RW', name: 'Kinyarwanda' },
  { code: 'ko', name: 'Korean' },
  { code: 'ky_KG', name: 'Kyrgyz (Kyrgyzstan)' },
  { code: 'lo', name: 'Lao' },
  { code: 'lv', name: 'Latvian' },
  { code: 'lt', name: 'Lithuanian' },
  { code: 'mk', name: 'Macedonian' },
  { code: 'ms', name: 'Malay' },
  { code: 'ml', name: 'Malayalam' },
  { code: 'mr', name: 'Marathi' },
  { code: 'nb', name: 'Norwegian (Bokm\u00e5l)' },
  { code: 'fa', name: 'Persian' },
  { code: 'pl', name: 'Polish' },
  { code: 'pt_BR', name: 'Portuguese (BR)' },
  { code: 'pt_PT', name: 'Portuguese (POR)' },
  { code: 'pa', name: 'Punjabi' },
  { code: 'ro', name: 'Romanian' },
  { code: 'ru', name: 'Russian' },
  { code: 'sr', name: 'Serbian' },
  { code: 'sk', name: 'Slovak' },
  { code: 'sl', name: 'Slovenian' },
  { code: 'es', name: 'Spanish' },
  { code: 'es_AR', name: 'Spanish (ARG)' },
  { code: 'es_MX', name: 'Spanish (MEX)' },
  { code: 'es_ES', name: 'Spanish (SPA)' },
  { code: 'sw', name: 'Swahili' },
  { code: 'sv', name: 'Swedish' },
  { code: 'ta', name: 'Tamil' },
  { code: 'te', name: 'Telugu' },
  { code: 'th', name: 'Thai' },
  { code: 'tr', name: 'Turkish' },
  { code: 'uk', name: 'Ukrainian' },
  { code: 'ur', name: 'Urdu' },
  { code: 'uz', name: 'Uzbek' },
  { code: 'vi', name: 'Vietnamese' },
  { code: 'zu', name: 'Zulu' },
]

function getLanguageName(code: string): string {
  return languages.find(l => l.code === code)?.name || code
}

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, async () => {
  await fetchAccounts()
  await fetchTemplates()
})
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449

// --- API Methods (Exactly as you wrote them) ---
watch(
  () => organizationsStore.selectedOrgId,
  async () => {
    await fetchAccounts();
    await fetchTemplates();
  },
);
onMounted(async () => {
  await fetchAccounts();
  await fetchTemplates();
});

async function fetchAccounts() {
  try {
    const response = await api.get("/accounts");
    accounts.value = response.data.data?.accounts || [];
    if (
      selectedAccount.value !== "all" &&
      !accounts.value.some((a) => a.name === selectedAccount.value)
    ) {
      selectedAccount.value = "all";
      localStorage.setItem("templates_selected_account", "all");
    }
  } catch (error) {
    console.error("Failed to fetch accounts:", error);
  }
}

function onAccountChange(
  value: string | number | bigint | Record<string, any> | null,
) {
  if (typeof value !== "string") return;
  localStorage.setItem("templates_selected_account", value);
  currentPage.value = 1;
  fetchTemplates();
}

async function fetchTemplates() {
<<<<<<< HEAD
  isLoading.value = true;
=======
  isLoading.value = true
  error.value = null
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
  try {
    const response = await templatesService.list({
      account:
        selectedAccount.value !== "all" ? selectedAccount.value : undefined,
      search: searchQuery.value || undefined,
      page: currentPage.value,
<<<<<<< HEAD
      limit: pageSize,
    });
    const data = (response.data as any).data || response.data;
    templates.value = data.templates || [];
    totalItems.value = data.total ?? templates.value.length;
  } catch (error: any) {
    toast.error(t("common.failedLoad", { resource: t("resources.templates") }));
    templates.value = [];
=======
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    templates.value = data.templates || []
    totalItems.value = data.total ?? templates.value.length
  } catch (err: any) {
    console.error('Failed to fetch templates:', err)
    error.value = t('templates.errorLoadingTemplates')
    toast.error(t('common.failedLoad', { resource: t('resources.templates') }))
    templates.value = []
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
  } finally {
    isLoading.value = false;
  }
}

<<<<<<< HEAD
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1;
  fetchTemplates();
}, 300);
watch(searchQuery, () => debouncedSearch());
function handlePageChange(page: number) {
  currentPage.value = page;
  fetchTemplates();
}

=======
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
async function syncTemplates() {
  if (!selectedAccount.value || selectedAccount.value === "all")
    return toast.error(t("templates.selectAccountFirst"));
  isSyncing.value = true;
  try {
    const response = await api.post("/templates/sync", {
      whatsapp_account: selectedAccount.value,
    });
    toast.success(response.data.data.message || t("templates.syncSuccess"));
    await fetchTemplates();
  } catch (error) {
    toast.error(getErrorMessage(error, t("templates.syncFailed")));
  } finally {
    isSyncing.value = false;
  }
}

<<<<<<< HEAD
// --- Action Logic ---
function openCreateDialog() {
  //Force the editor to wipe its local variables clean
  editorRef.value?.resetState();
  editingTemplate.value = null;
  formData.value = {
    whatsapp_account:
      selectedAccount.value && selectedAccount.value !== "all"
        ? selectedAccount.value
        : accounts.value[0]?.name || "",
    name: "",
    display_name: "",
    language: "en",
    category: "UTILITY",
    header_type: "NONE",
    header_content: "",
    body_content: "",
    footer_content: "",
    buttons: [],
    sample_values: [],
  };
  isDialogOpen.value = true;
}

function openEditDialog(template: Template) {
  editorRef.value?.resetState();
  editingTemplate.value = template;

  console.group("📥 API RECEIVE (Edit Mode)");
  console.log("Raw Template Object:", JSON.parse(JSON.stringify(template)));
  console.table(template.buttons || []);
  console.groupEnd();
  // ------------------------------

  formData.value = JSON.parse(JSON.stringify(template));
  isDialogOpen.value = true;
}

function openPreview(template: Template) {
  previewTemplate.value = template;
  isPreviewOpen.value = true;
}

async function saveTemplate() {
  if (!formData.value.name.trim() || !formData.value.body_content.trim())
    return toast.error(t("templates.nameBodyRequired"));
  if (!formData.value.whatsapp_account)
    return toast.error(t("templates.selectAccountRequired"));

  // Sanitize internal UI IDs before API call
  const payload = JSON.parse(JSON.stringify(formData.value));
  payload.buttons = payload.buttons.map((btn: any, idx: number) => {
    const cleanBtn = { ...btn };
    delete cleanBtn.id;
    delete cleanBtn.urlType;
    delete cleanBtn.originalIndex;

    if (btn.urlType === "DYNAMIC" && cleanBtn.url?.includes("{{")) {
      const sample = payload.sample_values?.find(
        (s: any) =>
          s.component === "button_url" && s.param_name === String(idx),
      );
      if (sample?.value) cleanBtn.example = sample.value;
    }

    return cleanBtn;
  });
  // _____
  console.group("📤 API SEND (Save Mode)");
  console.log("Final Payload:", JSON.parse(JSON.stringify(payload)));
  console.table(payload.buttons || []);
  console.groupEnd();
  // ------
  isSubmitting.value = true;
  try {
    if (editingTemplate.value) {
      await api.put(`/templates/${editingTemplate.value.id}`, payload);
      toast.success(
        t("common.updatedSuccess", { resource: t("resources.Template") }),
      );
    } else {
      await api.post("/templates", payload);
      toast.success(
        t("common.createdSuccess", { resource: t("resources.Template") }),
      );
    }
    isDialogOpen.value = false;
    await fetchTemplates();
  } catch (error) {
    toast.error(
      getErrorMessage(
        error,
        t("common.failedSave", { resource: t("resources.template") }),
      ),
    );
  } finally {
    isSubmitting.value = false;
  }
}

=======
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
function openDeleteDialog(template: Template) {
  templateToDelete.value = template;
  deleteDialogOpen.value = true;
}
async function confirmDeleteTemplate() {
<<<<<<< HEAD
  if (!templateToDelete.value) return;
=======
  if (!templateToDelete.value) return

  isDeleting.value = true
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
  try {
    await api.delete(`/templates/${templateToDelete.value.id}`);
    toast.success(
      t("common.deletedSuccess", { resource: t("resources.Template") }),
    );
    deleteDialogOpen.value = false;
    templateToDelete.value = null;
    await fetchTemplates();
  } catch (error) {
<<<<<<< HEAD
    toast.error(
      getErrorMessage(
        error,
        t("common.failedDelete", { resource: t("resources.template") }),
      ),
    );
  }
}

function openPublishDialog(template: Template) {
  templateToPublish.value = template;
  publishDialogOpen.value = true;
}
async function confirmPublishTemplate() {
  if (!templateToPublish.value) return;
  publishingTemplateId.value = templateToPublish.value.id;
  try {
    const response = await api.post(
      `/templates/${templateToPublish.value.id}/publish`,
    );
    toast.success(response.data.data?.message || t("templates.publishSuccess"));
    publishDialogOpen.value = false;
    templateToPublish.value = null;
    await fetchTemplates();
  } catch (error) {
    toast.error(getErrorMessage(error, t("templates.publishFailed")), {
      duration: 8000,
    });
  } finally {
    publishingTemplateId.value = null;
=======
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.template') })))
  } finally {
    isDeleting.value = false
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
  }
}

// --- Style Helpers (Restored) ---
function getStatusBadgeClass(status: string) {
  switch (status) {
    case "APPROVED":
      return "bg-green-900 text-green-300 light:bg-green-100 light:text-green-800";
    case "PENDING":
      return "bg-yellow-900 text-yellow-300 light:bg-yellow-100 light:text-yellow-800";
    case "REJECTED":
      return "bg-red-900 text-red-300 light:bg-red-100 light:text-red-800";
    default:
      return "bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800";
  }
}

function getCategoryBadgeClass(category: string) {
  switch (category) {
    case "UTILITY":
      return "bg-blue-900 text-blue-300 light:bg-blue-100 light:text-blue-800";
    case "MARKETING":
      return "bg-purple-900 text-purple-300 light:bg-purple-100 light:text-purple-800";
    case "AUTHENTICATION":
      return "bg-orange-900 text-orange-300 light:bg-orange-100 light:text-orange-800";
    default:
      return "bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800";
  }
}

function getHeaderIcon(type: string) {
  switch (type) {
    case "IMAGE":
      return Image;
    case "VIDEO":
      return Video;
    case "DOCUMENT":
      return FileIcon;
    default:
      return MessageSquare;
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('templates.title')"
      :subtitle="$t('templates.subtitle')"
      :icon="FileText"
      icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20"
    >
      <template #actions>
        <Button
          variant="outline"
          size="sm"
          @click="syncTemplates"
          :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'"
        >
          <Loader2 v-if="isSyncing" class="h-4 w-4 mr-2 animate-spin" />
          <RefreshCw v-else class="h-4 w-4 mr-2" />
          {{ $t("templates.syncFromMeta") }}
        </Button>
<<<<<<< HEAD
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" /> {{ $t("templates.createTemplate") }}
        </Button>
=======
        <RouterLink to="/templates/new">
          <Button variant="outline" size="sm">
            <Plus class="h-4 w-4 mr-2" />
            {{ $t('templates.createTemplate') }}
          </Button>
        </RouterLink>
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div>
          <ErrorState
            v-if="error && !isLoading"
            :title="$t('common.loadErrorTitle')"
            :description="error"
            :retry-label="$t('common.retry')"
            @retry="fetchTemplates"
          />
          <Card v-else>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t("templates.yourTemplates") }}</CardTitle>
                  <CardDescription>{{
                    $t("templates.yourTemplatesDesc")
                  }}</CardDescription>
                </div>
                <div class="flex items-center gap-4 flex-wrap">
                  <div class="flex items-center gap-2">
                    <Label class="text-sm text-muted-foreground"
                      >{{ $t("templates.account") }}:</Label
                    >
                    <Select
                      v-model="selectedAccount"
                      @update:model-value="onAccountChange"
                    >
                      <SelectTrigger class="w-[180px]"
                        ><SelectValue
                          :placeholder="$t('templates.allAccounts')"
                      /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">{{
                          $t("templates.allAccounts")
                        }}</SelectItem>
                        <SelectItem
                          v-for="account in accounts"
                          :key="account.id"
                          :value="account.name"
                          >{{ account.name }}</SelectItem
                        >
                      </SelectContent>
                    </Select>
                  </div>
                  <SearchInput
                    v-model="searchQuery"
                    :placeholder="$t('templates.searchTemplates') + '...'"
                    class="w-64"
                  />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="templates"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="FileText"
                :empty-title="$t('templates.noTemplatesFound')"
                :empty-description="$t('templates.noTemplatesFoundDesc')"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="templates"
                @page-change="handlePageChange"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
              >
                <template #cell-name="{ item: template }">
<<<<<<< HEAD
                  <div>
                    <span class="font-medium">{{
                      template.display_name || template.name
                    }}</span>
                    <p class="text-xs font-mono text-muted-foreground">
                      {{ template.name }}
                    </p>
                  </div>
=======
                  <RouterLink :to="`/templates/${template.id}`" class="text-inherit no-underline hover:opacity-80">
                    <span class="font-medium">{{ template.display_name || template.name }}</span>
                    <p class="text-xs font-mono text-muted-foreground">{{ template.name }}</p>
                  </RouterLink>
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
                </template>
                <template #cell-category="{ item: template }"
                  ><Badge
                    :class="getCategoryBadgeClass(template.category)"
                    class="text-xs"
                    >{{ template.category }}</Badge
                  ></template
                >
                <template #cell-status="{ item: template }"
                  ><Badge
                    :class="getStatusBadgeClass(template.status)"
                    class="text-xs"
                    >{{ template.status }}</Badge
                  ></template
                >
                <template #cell-language="{ item: template }"
                  ><span class="text-muted-foreground">{{
                    template.language
                  }}</span></template
                >
                <template #cell-header_type="{ item: template }">
                  <div class="flex items-center gap-1">
                    <component
                      :is="getHeaderIcon(template.header_type)"
                      class="h-4 w-4 text-muted-foreground"
                    />
                    <span class="text-muted-foreground text-sm">{{
                      template.header_type || "NONE"
                    }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: template }">
                  <div class="flex items-center justify-end gap-1">
<<<<<<< HEAD
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="openPreview(template)"
                      ><Eye class="h-4 w-4"
                    /></Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="openEditDialog(template)"
                      :disabled="template.status === 'PENDING'"
                      ><Pencil class="h-4 w-4"
                    /></Button>
                    <Button
                      v-if="
                        template.status === 'DRAFT' ||
                        template.status === 'REJECTED'
                      "
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-blue-600 hover:text-blue-700"
                      @click="openPublishDialog(template)"
                      :disabled="publishingTemplateId === template.id"
                    >
                      <Loader2
                        v-if="publishingTemplateId === template.id"
                        class="h-4 w-4 animate-spin"
                      />
                      <Send v-else class="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-destructive"
                      @click="openDeleteDialog(template)"
                      ><Trash2 class="h-4 w-4"
                    /></Button>
=======
                    <RouterLink :to="`/templates/${template.id}`">
                      <IconButton
                        :icon="Pencil"
                        :label="$t('common.edit')"
                        class="h-8 w-8"
                      />
                    </RouterLink>
                    <IconButton
                      :icon="Trash2"
                      :label="$t('common.delete')"
                      class="h-8 w-8 text-destructive"
                      @click="openDeleteDialog(template)"
                    />
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
                  </div>
                </template>
                <template #empty-action>
                  <div class="flex items-center justify-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      @click="syncTemplates"
                      :disabled="!selectedAccount || selectedAccount === 'all'"
                    >
                      <RefreshCw class="h-4 w-4 mr-2" />
<<<<<<< HEAD
                      {{ $t("templates.syncFromMeta") }}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      @click="openCreateDialog"
                      ><Plus class="h-4 w-4 mr-2" />
                      {{ $t("templates.createTemplate") }}</Button
                    >
=======
                      {{ $t('templates.syncFromMeta') }}
                    </Button>
                    <RouterLink to="/templates/new">
                      <Button variant="outline" size="sm">
                        <Plus class="h-4 w-4 mr-2" />
                        {{ $t('templates.createTemplate') }}
                      </Button>
                    </RouterLink>
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

<<<<<<< HEAD
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-6xl max-h-[90vh] overflow-y-auto w-full">
        <DialogHeader>
          <DialogTitle>{{
            editingTemplate
              ? $t("templates.editDialogTitle")
              : $t("templates.createDialogTitle")
          }}</DialogTitle>
          <DialogDescription>{{
            editingTemplate
              ? $t("templates.editDialogDesc")
              : $t("templates.createDialogDesc")
          }}</DialogDescription>
        </DialogHeader>

        <TemplateEditor
          ref="editorRef"
          v-model="formData"
          :is-edit="!!editingTemplate"
          :accounts="accounts"
        />

        <DialogFooter
          class="pt-6 mt-6 border-t border-zinc-200 dark:border-zinc-800"
        >
          <Button variant="outline" size="sm" @click="isDialogOpen = false">{{
            $t("common.cancel")
          }}</Button>
          <Button size="sm" @click="saveTemplate" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{
              editingTemplate
                ? $t("templates.updateTemplate")
                : $t("templates.createTemplate")
            }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="isPreviewOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t("templates.templatePreview") }}</DialogTitle>
          <DialogDescription>{{
            previewTemplate?.display_name || previewTemplate?.name
          }}</DialogDescription>
        </DialogHeader>
        <div v-if="previewTemplate" class="py-4">
          <div
            class="bg-[#e5ddd5] dark:bg-[#111b21] rounded-lg p-4 relative overflow-hidden"
          >
            <TemplatePreview
              :header-type="previewTemplate.header_type"
              :header-content="previewTemplate.header_content"
              :body-content="previewTemplate.body_content"
              :footer-content="previewTemplate.footer_content"
              :buttons="previewTemplate.buttons"
              :sample-values="previewTemplate.sample_values"
              contained
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" @click="isPreviewOpen = false">{{
            $t("common.close")
          }}</Button>
          <Button
            v-if="
              previewTemplate?.status === 'DRAFT' ||
              previewTemplate?.status === 'REJECTED'
            "
            size="sm"
            @click="
              openPublishDialog(previewTemplate!);
              isPreviewOpen = false;
            "
            :disabled="publishingTemplateId === previewTemplate?.id"
          >
            <Loader2
              v-if="publishingTemplateId === previewTemplate?.id"
              class="h-4 w-4 mr-2 animate-spin"
            />
            <Send v-else class="h-4 w-4 mr-2" />
            {{
              previewTemplate?.meta_template_id
                ? $t("templates.republishToMeta")
                : $t("templates.publishToMeta")
            }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

=======
    <!-- Delete Confirmation Dialog -->
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('templates.deleteTemplate')"
      :item-name="templateToDelete?.display_name || templateToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteTemplate"
    />
<<<<<<< HEAD

    <AlertDialog v-model:open="publishDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{
            templateToPublish?.meta_template_id
              ? $t("templates.republishTemplate")
              : $t("templates.publishTemplate")
          }}</AlertDialogTitle>
          <AlertDialogDescription>
            <template v-if="templateToPublish?.meta_template_id">{{
              $t("templates.republishConfirm", {
                name:
                  templateToPublish?.display_name || templateToPublish?.name,
              })
            }}</template>
            <template v-else>{{
              $t("templates.publishConfirm", {
                name:
                  templateToPublish?.display_name || templateToPublish?.name,
              })
            }}</template>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t("common.cancel") }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmPublishTemplate">{{
            templateToPublish?.meta_template_id
              ? $t("templates.republish")
              : $t("templates.publish")
          }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
=======
>>>>>>> 0c566f76ccc5eb523d89b2cdfe9bbf4100575449
  </div>
</template>
