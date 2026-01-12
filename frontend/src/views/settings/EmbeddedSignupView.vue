<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/services/api'
import { toast } from 'vue-sonner'
import {
  Plus,
  Pencil,
  Trash2,
  ExternalLink,
  Copy,
  Loader2,
  Code2,
  Users,
  CheckCircle2,
  XCircle,
  Clock,
  Link2,
  Settings2
} from 'lucide-vue-next'

interface EmbeddedSignup {
  id: string
  name: string
  whatsapp_account_id: string
  meta_app_id: string
  meta_config_id: string
  enable_coexistence: boolean
  sync_chat_history: boolean
  api_version: string
  form_fields: Record<string, any>
  required_fields: string[]
  welcome_message: string
  success_message: string
  redirect_url?: string
  webhook_url?: string
  allowed_origins: string[]
  rate_limit_per_hour: number
  is_active: boolean
  auto_create_contact: boolean
  created_at: string
  updated_at: string
}

interface Lead {
  id: string
  phone_number: string
  profile_name: string
  form_data: Record<string, any>
  status: string
  source: string
  coexistence_enabled: boolean
  chat_history_synced: boolean
  created_at: string
}

interface WhatsAppAccount {
  id: string
  name: string
}

const signups = ref<EmbeddedSignup[]>([])
const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingSignup = ref<EmbeddedSignup | null>(null)
const deleteDialogOpen = ref(false)
const signupToDelete = ref<EmbeddedSignup | null>(null)
const viewingLeads = ref<string | null>(null)
const leads = ref<Lead[]>([])
const loadingLeads = ref(false)
const codeDialogOpen = ref(false)
const selectedSignup = ref<EmbeddedSignup | null>(null)

const formData = ref({
  name: '',
  whatsapp_account_id: '',
  meta_app_id: '',
  meta_config_id: '',
  meta_app_secret: '',
  enable_coexistence: true,
  sync_chat_history: true,
  api_version: 'v24.0',
  form_fields: '{}',
  required_fields: 'phone',
  welcome_message: 'Welcome! Thank you for signing up with WhatsApp Business.',
  success_message: 'Your WhatsApp Business account has been connected successfully!',
  redirect_url: '',
  webhook_url: '',
  allowed_origins: '',
  rate_limit_per_hour: 100,
  is_active: true,
  auto_create_contact: true
})

onMounted(async () => {
  await Promise.all([loadSignups(), loadAccounts()])
})

async function loadSignups() {
  isLoading.value = true
  try {
    const response = await api.get('/embedded-signups')
    signups.value = response.data.signups || []
  } catch (error) {
    console.error('Failed to load signups:', error)
    toast.error('Failed to load embedded signups')
  } finally {
    isLoading.value = false
  }
}

async function loadAccounts() {
  try {
    const response = await api.get('/accounts')
    accounts.value = response.data.accounts || []
  } catch (error) {
    console.error('Failed to load accounts:', error)
  }
}

async function loadLeads(signupId: string) {
  loadingLeads.value = true
  viewingLeads.value = signupId
  try {
    const response = await api.get(`/embedded-signups/${signupId}/leads`)
    leads.value = response.data.leads || []
  } catch (error) {
    console.error('Failed to load leads:', error)
    toast.error('Failed to load leads')
  } finally {
    loadingLeads.value = false
  }
}

function openCreateDialog() {
  editingSignup.value = null
  resetForm()
  isDialogOpen.value = true
}

function openEditDialog(signup: EmbeddedSignup) {
  editingSignup.value = signup
  formData.value = {
    name: signup.name,
    whatsapp_account_id: signup.whatsapp_account_id,
    meta_app_id: signup.meta_app_id,
    meta_config_id: signup.meta_config_id,
    meta_app_secret: '',
    enable_coexistence: signup.enable_coexistence,
    sync_chat_history: signup.sync_chat_history,
    api_version: signup.api_version,
    form_fields: JSON.stringify(signup.form_fields, null, 2),
    required_fields: signup.required_fields.join(', '),
    welcome_message: signup.welcome_message,
    success_message: signup.success_message,
    redirect_url: signup.redirect_url || '',
    webhook_url: signup.webhook_url || '',
    allowed_origins: signup.allowed_origins.join('\n'),
    rate_limit_per_hour: signup.rate_limit_per_hour,
    is_active: signup.is_active,
    auto_create_contact: signup.auto_create_contact
  }
  isDialogOpen.value = true
}

function resetForm() {
  formData.value = {
    name: '',
    whatsapp_account_id: '',
    meta_app_id: '',
    meta_config_id: '',
    meta_app_secret: '',
    enable_coexistence: true,
    sync_chat_history: true,
    api_version: 'v24.0',
    form_fields: '{}',
    required_fields: 'phone',
    welcome_message: 'Welcome! Thank you for signing up with WhatsApp Business.',
    success_message: 'Your WhatsApp Business account has been connected successfully!',
    redirect_url: '',
    webhook_url: '',
    allowed_origins: '',
    rate_limit_per_hour: 100,
    is_active: true,
    auto_create_contact: true
  }
}

async function saveSignup() {
  isSubmitting.value = true
  try {
    // Parse form fields
    let parsedFormFields = {}
    if (formData.value.form_fields.trim()) {
      try {
        parsedFormFields = JSON.parse(formData.value.form_fields)
      } catch (e) {
        toast.error('Invalid JSON in form fields')
        isSubmitting.value = false
        return
      }
    }

    // Parse required fields
    const requiredFields = formData.value.required_fields
      .split(',')
      .map(f => f.trim())
      .filter(f => f.length > 0)

    // Parse allowed origins
    const allowedOrigins = formData.value.allowed_origins
      .split('\n')
      .map(o => o.trim())
      .filter(o => o.length > 0)

    const payload = {
      name: formData.value.name,
      whatsapp_account_id: formData.value.whatsapp_account_id,
      meta_app_id: formData.value.meta_app_id,
      meta_config_id: formData.value.meta_config_id,
      meta_app_secret: formData.value.meta_app_secret,
      enable_coexistence: formData.value.enable_coexistence,
      sync_chat_history: formData.value.sync_chat_history,
      api_version: formData.value.api_version,
      form_fields: parsedFormFields,
      required_fields: requiredFields,
      welcome_message: formData.value.welcome_message,
      success_message: formData.value.success_message,
      redirect_url: formData.value.redirect_url || null,
      webhook_url: formData.value.webhook_url || null,
      allowed_origins: allowedOrigins,
      rate_limit_per_hour: formData.value.rate_limit_per_hour,
      is_active: formData.value.is_active,
      auto_create_contact: formData.value.auto_create_contact
    }

    if (editingSignup.value) {
      await api.put(`/embedded-signups/${editingSignup.value.id}`, payload)
      toast.success('Embedded signup updated')
    } else {
      await api.post('/embedded-signups', payload)
      toast.success('Embedded signup created')
    }

    isDialogOpen.value = false
    await loadSignups()
  } catch (error: any) {
    console.error('Failed to save signup:', error)
    toast.error(error.response?.data?.message || 'Failed to save signup')
  } finally {
    isSubmitting.value = false
  }
}

function confirmDelete(signup: EmbeddedSignup) {
  signupToDelete.value = signup
  deleteDialogOpen.value = true
}

async function deleteSignup() {
  if (!signupToDelete.value) return

  try {
    await api.delete(`/embedded-signups/${signupToDelete.value.id}`)
    toast.success('Embedded signup deleted')
    await loadSignups()
  } catch (error) {
    toast.error('Failed to delete signup')
  } finally {
    deleteDialogOpen.value = false
    signupToDelete.value = null
  }
}

function showCode(signup: EmbeddedSignup) {
  selectedSignup.value = signup
  codeDialogOpen.value = true
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
  toast.success('Copied to clipboard')
}

const integrationCode = computed(() => {
  if (!selectedSignup.value) return ''

  const baseUrl = window.location.origin
  const signupId = selectedSignup.value.id

  return `<!DOCTYPE html>
<html>
<head>
    <title>WhatsApp Business Signup</title>
    <script src="https://connect.facebook.net/en_US/sdk.js"></script>
</head>
<body>
    <h1>Sign Up for WhatsApp Business API</h1>
    <button id="whatsapp-signup-btn" style="
        background: #25D366;
        color: white;
        padding: 15px 30px;
        font-size: 18px;
        border: none;
        border-radius: 5px;
        cursor: pointer;
    ">
        🔗 Sign Up with WhatsApp Business
    </button>

    <script>
        const SIGNUP_ID = '${signupId}';
        const API_URL = '${baseUrl}';

        // Fetch public config
        fetch(\`\${API_URL}/api/embedded-signup/\${SIGNUP_ID}/config\`)
            .then(res => res.json())
            .then(config => {
                // Initialize Facebook SDK
                window.fbAsyncInit = function() {
                    FB.init({
                        appId: config.meta_app_id,
                        cookie: true,
                        xfbml: true,
                        version: config.api_version
                    });
                };

                // Handle signup button click
                document.getElementById('whatsapp-signup-btn').addEventListener('click', () => {
                    // Launch Embedded Signup flow
                    FB.login(function(response) {
                        if (response.authResponse) {
                            const authCode = response.authResponse.code;

                            // Get form data
                            const phoneNumber = prompt('Enter your WhatsApp Business phone number:');
                            const companyName = prompt('Enter your company name:');

                            if (!phoneNumber) {
                                alert('Phone number is required');
                                return;
                            }

                            // Submit signup
                            fetch(\`\${API_URL}/api/embedded-signup/\${SIGNUP_ID}/submit\`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                    phone_number: phoneNumber,
                                    profile_name: companyName,
                                    meta_auth_code: authCode,
                                    form_data: {
                                        company_name: companyName
                                    },
                                    source: 'widget'
                                })
                            })
                            .then(res => res.json())
                            .then(result => {
                                if (result.success) {
                                    alert(result.message || 'Success!');
                                    if (result.redirect_url) {
                                        window.location.href = result.redirect_url;
                                    }
                                } else {
                                    alert('Signup failed: ' + (result.message || 'Unknown error'));
                                }
                            })
                            .catch(err => {
                                console.error('Signup error:', err);
                                alert('Failed to complete signup');
                            });
                        }
                    }, {
                        config_id: config.meta_config_id,
                        response_type: 'code',
                        override_default_response_type: true
                    });
                });
            })
            .catch(err => {
                console.error('Failed to load config:', err);
                alert('Failed to load signup configuration');
            });
    </script>
</body>
</html>`
})

function getStatusColor(status: string) {
  const colors: Record<string, string> = {
    confirmed: 'bg-green-100 text-green-800',
    pending: 'bg-yellow-100 text-yellow-800',
    failed: 'bg-red-100 text-red-800'
  }
  return colors[status] || 'bg-gray-100 text-gray-800'
}

function formatDate(dateString: string) {
  return new Date(dateString).toLocaleString()
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <div class="flex-1">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/settings">Settings</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Embedded Signup</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
          <h1 class="text-xl font-semibold mt-1">Embedded Signup</h1>
          <p class="text-sm text-muted-foreground">Manage WhatsApp Business embedded signup with coexistence</p>
        </div>
        <Button @click="openCreateDialog" :disabled="isLoading">
          <Plus class="h-4 w-4 mr-2" />
          Create Signup
        </Button>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4">
        <!-- Loading State -->
        <div v-if="isLoading" class="space-y-4">
          <Card v-for="i in 3" :key="i">
            <CardHeader>
              <Skeleton class="h-5 w-1/3" />
              <Skeleton class="h-4 w-2/3 mt-2" />
            </CardHeader>
            <CardContent>
              <Skeleton class="h-20 w-full" />
            </CardContent>
          </Card>
        </div>

        <!-- Empty State -->
        <Card v-else-if="signups.length === 0">
          <CardContent class="flex flex-col items-center justify-center py-16">
            <Link2 class="h-12 w-12 text-muted-foreground mb-4" />
            <h3 class="text-lg font-semibold mb-2">No Embedded Signups</h3>
            <p class="text-sm text-muted-foreground text-center max-w-md mb-4">
              Create your first embedded signup to allow users to connect their WhatsApp Business accounts directly from your website.
            </p>
            <Button @click="openCreateDialog">
              <Plus class="h-4 w-4 mr-2" />
              Create Signup
            </Button>
          </CardContent>
        </Card>

        <!-- Signups List -->
        <Card v-for="signup in signups" :key="signup.id">
          <CardHeader>
            <div class="flex items-start justify-between">
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <CardTitle>{{ signup.name }}</CardTitle>
                  <Badge :class="signup.is_active ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'">
                    {{ signup.is_active ? 'Active' : 'Inactive' }}
                  </Badge>
                  <Badge v-if="signup.enable_coexistence" variant="outline">
                    Coexistence
                  </Badge>
                </div>
                <CardDescription class="mt-1">
                  API {{ signup.api_version }} • Created {{ formatDate(signup.created_at) }}
                </CardDescription>
              </div>
              <div class="flex gap-2">
                <Button variant="ghost" size="icon" @click="showCode(signup)">
                  <Code2 class="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" @click="loadLeads(signup.id)">
                  <Users class="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" @click="openEditDialog(signup)">
                  <Pencil class="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" @click="confirmDelete(signup)">
                  <Trash2 class="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div class="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p class="text-muted-foreground">Meta App ID</p>
                <p class="font-mono">{{ signup.meta_app_id }}</p>
              </div>
              <div>
                <p class="text-muted-foreground">Config ID</p>
                <p class="font-mono">{{ signup.meta_config_id }}</p>
              </div>
              <div>
                <p class="text-muted-foreground">Rate Limit</p>
                <p>{{ signup.rate_limit_per_hour }}/hour</p>
              </div>
              <div>
                <p class="text-muted-foreground">Features</p>
                <div class="flex gap-1 mt-1">
                  <Badge v-if="signup.auto_create_contact" variant="secondary" class="text-xs">Auto Contact</Badge>
                  <Badge v-if="signup.sync_chat_history" variant="secondary" class="text-xs">Chat Sync</Badge>
                </div>
              </div>
            </div>

            <!-- Leads Preview -->
            <div v-if="viewingLeads === signup.id" class="mt-4 pt-4 border-t">
              <div class="flex items-center justify-between mb-3">
                <h4 class="font-semibold">Recent Leads</h4>
                <Button variant="ghost" size="sm" @click="viewingLeads = null">Close</Button>
              </div>
              <div v-if="loadingLeads" class="space-y-2">
                <Skeleton class="h-12 w-full" />
                <Skeleton class="h-12 w-full" />
              </div>
              <div v-else-if="leads.length === 0" class="text-center py-8 text-muted-foreground">
                No leads yet
              </div>
              <div v-else class="space-y-2">
                <Card v-for="lead in leads" :key="lead.id" class="p-3">
                  <div class="flex items-center justify-between">
                    <div class="flex-1">
                      <div class="flex items-center gap-2">
                        <p class="font-medium">{{ lead.profile_name || 'Unknown' }}</p>
                        <Badge :class="getStatusColor(lead.status)" class="text-xs">{{ lead.status }}</Badge>
                      </div>
                      <p class="text-sm text-muted-foreground">{{ lead.phone_number }}</p>
                    </div>
                    <div class="text-right text-sm text-muted-foreground">
                      {{ formatDate(lead.created_at) }}
                    </div>
                  </div>
                  <div v-if="Object.keys(lead.form_data || {}).length > 0" class="mt-2 text-xs text-muted-foreground">
                    <pre class="bg-muted p-2 rounded">{{ JSON.stringify(lead.form_data, null, 2) }}</pre>
                  </div>
                </Card>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Create/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ editingSignup ? 'Edit' : 'Create' }} Embedded Signup</DialogTitle>
          <DialogDescription>
            Configure embedded signup with Meta coexistence support (API v24.0)
          </DialogDescription>
        </DialogHeader>

        <Tabs default-value="basic" class="w-full">
          <TabsList class="grid w-full grid-cols-4">
            <TabsTrigger value="basic">Basic</TabsTrigger>
            <TabsTrigger value="meta">Meta Config</TabsTrigger>
            <TabsTrigger value="form">Form</TabsTrigger>
            <TabsTrigger value="advanced">Advanced</TabsTrigger>
          </TabsList>

          <TabsContent value="basic" class="space-y-4 mt-4">
            <div class="space-y-2">
              <Label for="name">Name *</Label>
              <Input id="name" v-model="formData.name" placeholder="Main Website Signup" />
            </div>

            <div class="space-y-2">
              <Label for="account">WhatsApp Account *</Label>
              <Select v-model="formData.whatsapp_account_id">
                <SelectTrigger>
                  <SelectValue placeholder="Select account" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="account in accounts" :key="account.id" :value="account.id">
                    {{ account.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="flex items-center justify-between">
              <div>
                <p class="font-medium">Active</p>
                <p class="text-sm text-muted-foreground">Enable this signup configuration</p>
              </div>
              <Switch v-model:checked="formData.is_active" />
            </div>

            <div class="flex items-center justify-between">
              <div>
                <p class="font-medium">Auto Create Contact</p>
                <p class="text-sm text-muted-foreground">Automatically create contact from signup</p>
              </div>
              <Switch v-model:checked="formData.auto_create_contact" />
            </div>
          </TabsContent>

          <TabsContent value="meta" class="space-y-4 mt-4">
            <div class="space-y-2">
              <Label for="meta_app_id">Meta App ID *</Label>
              <Input id="meta_app_id" v-model="formData.meta_app_id" placeholder="123456789" />
            </div>

            <div class="space-y-2">
              <Label for="meta_config_id">Meta Config ID *</Label>
              <Input id="meta_config_id" v-model="formData.meta_config_id" placeholder="config-abc-123" />
            </div>

            <div class="space-y-2">
              <Label for="meta_app_secret">Meta App Secret *</Label>
              <Input id="meta_app_secret" type="password" v-model="formData.meta_app_secret" placeholder="Enter app secret" />
              <p class="text-xs text-muted-foreground">Required for OAuth token exchange</p>
            </div>

            <div class="space-y-2">
              <Label for="api_version">API Version</Label>
              <Input id="api_version" v-model="formData.api_version" placeholder="v24.0" />
            </div>

            <div class="flex items-center justify-between">
              <div>
                <p class="font-medium">Enable Coexistence</p>
                <p class="text-sm text-muted-foreground">Use same number on App and API</p>
              </div>
              <Switch v-model:checked="formData.enable_coexistence" />
            </div>

            <div class="flex items-center justify-between">
              <div>
                <p class="font-medium">Sync Chat History</p>
                <p class="text-sm text-muted-foreground">Sync up to 6 months of chats</p>
              </div>
              <Switch v-model:checked="formData.sync_chat_history" />
            </div>
          </TabsContent>

          <TabsContent value="form" class="space-y-4 mt-4">
            <div class="space-y-2">
              <Label for="form_fields">Form Fields (JSON)</Label>
              <Textarea
                id="form_fields"
                v-model="formData.form_fields"
                placeholder='{"company_name": {"type": "text", "label": "Company Name", "required": true}}'
                rows="6"
                class="font-mono text-sm"
              />
              <p class="text-xs text-muted-foreground">Custom fields to collect during signup</p>
            </div>

            <div class="space-y-2">
              <Label for="required_fields">Required Fields</Label>
              <Input id="required_fields" v-model="formData.required_fields" placeholder="phone, company_name" />
              <p class="text-xs text-muted-foreground">Comma-separated field names</p>
            </div>

            <div class="space-y-2">
              <Label for="welcome_message">Welcome Message</Label>
              <Textarea id="welcome_message" v-model="formData.welcome_message" rows="3" />
            </div>

            <div class="space-y-2">
              <Label for="success_message">Success Message</Label>
              <Input id="success_message" v-model="formData.success_message" />
            </div>

            <div class="space-y-2">
              <Label for="redirect_url">Redirect URL (Optional)</Label>
              <Input id="redirect_url" v-model="formData.redirect_url" placeholder="https://yoursite.com/success" />
            </div>
          </TabsContent>

          <TabsContent value="advanced" class="space-y-4 mt-4">
            <div class="space-y-2">
              <Label for="allowed_origins">Allowed Origins (one per line)</Label>
              <Textarea
                id="allowed_origins"
                v-model="formData.allowed_origins"
                placeholder="https://yoursite.com&#10;https://app.yoursite.com"
                rows="4"
              />
              <p class="text-xs text-muted-foreground">CORS whitelist for embedding</p>
            </div>

            <div class="space-y-2">
              <Label for="rate_limit">Rate Limit (per hour)</Label>
              <Input id="rate_limit" type="number" v-model.number="formData.rate_limit_per_hour" />
            </div>

            <div class="space-y-2">
              <Label for="webhook_url">Webhook URL (Optional)</Label>
              <Input id="webhook_url" v-model="formData.webhook_url" placeholder="https://yoursite.com/webhooks/signup" />
              <p class="text-xs text-muted-foreground">Receive notifications for new signups</p>
            </div>
          </TabsContent>
        </Tabs>

        <DialogFooter>
          <Button variant="outline" @click="isDialogOpen = false" :disabled="isSubmitting">
            Cancel
          </Button>
          <Button @click="saveSignup" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
            {{ editingSignup ? 'Update' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Integration Code Dialog -->
    <Dialog v-model:open="codeDialogOpen">
      <DialogContent class="max-w-4xl max-h-[90vh]">
        <DialogHeader>
          <DialogTitle>Integration Code</DialogTitle>
          <DialogDescription>
            Copy this code and embed it on your website
          </DialogDescription>
        </DialogHeader>
        <div class="relative">
          <Button
            variant="outline"
            size="sm"
            class="absolute top-2 right-2 z-10"
            @click="copyToClipboard(integrationCode)"
          >
            <Copy class="h-4 w-4 mr-2" />
            Copy
          </Button>
          <ScrollArea class="h-[500px]">
            <pre class="bg-muted p-4 rounded text-xs overflow-x-auto"><code>{{ integrationCode }}</code></pre>
          </ScrollArea>
        </div>
        <p class="text-sm text-muted-foreground">
          📚 For more details, see <code>docs/embedded-signup.md</code>
        </p>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Embedded Signup?</AlertDialogTitle>
          <AlertDialogDescription>
            This will permanently delete "{{ signupToDelete?.name }}". Users will no longer be able to use this signup form.
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="deleteSignup" class="bg-destructive text-destructive-foreground hover:bg-destructive/90">
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
