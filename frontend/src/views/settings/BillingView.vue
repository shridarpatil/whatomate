<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationsStore } from '@/stores/organizations'
import { api, organizationsService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import PixQRCode from '@/components/billing/PixQRCode.vue'

interface Plan {
  id?: string; name: string; price_cents: number; currency: string; interval_months: number
  trial_days: number; user_limit: number; account_limit: number; team_limit: number; active: boolean; public: boolean
}
interface Subscription { id: string; organization_id: string; plan_id: string; status: string; expires_at: string; plan: Plan; organization: { name: string } }
interface Billing { subscription: Subscription | null; status: string; checkout_enabled: boolean; payment_provider: string; usage: { users: number; accounts: number; teams: number } }
interface Invoice { id: string; price_cents: number; currency: string; provider: string; status: string; pix_code: string; expires_at: string; error_code: string; created_at: string }
const { t, locale } = useI18n()
const auth = useAuthStore()
const orgs = useOrganizationsStore()
const admin = computed(() => !!auth.user?.is_super_admin)
const canPay = computed(() => auth.hasPermission('settings.general', 'write'))
const billing = ref<Billing | null>(null)
const invoices = ref<Invoice[]>([])
const plans = ref<Plan[]>([])
const subscriptions = ref<Subscription[]>([])
const companies = ref<{ id: string; name: string }[]>([])
const busy = ref(false)
const error = ref('')
const success = ref('')
const autoRefreshFailed = ref(false)
let pollTimer: ReturnType<typeof setInterval> | undefined
let polling = false
let disposed = false
let dataVersion = 0
const emptyPlan = (): Plan => ({ name: '', price_cents: 0, currency: 'BRL', interval_months: 1, trial_days: 0, user_limit: -1, account_limit: -1, team_limit: -1, active: true, public: false })
const plan = ref<Plan>(emptyPlan())
const assignment = ref({ organization_id: '', plan_id: '', status: 'trialing', expires_at: '' })
const trial = ref({ name: '', owner_name: '', email: '', password: '', plan_id: '' })
const trialPlans = computed(() => plans.value.filter(p => p.active && p.trial_days > 0 && p.user_limit !== 0))
const states = ['trialing', 'active', 'suspended', 'canceled']
const money = (p: { currency: string; price_cents: number }) => new Intl.NumberFormat(locale.value, { style: 'currency', currency: p.currency }).format(p.price_cents / 100)
const limit = (n: number | undefined) => n === undefined || n === -1 ? t('saas.unlimited') : n
function failure(e: any, fallback = 'saas.load_failed') {
  const key = e.response?.data?.message
  error.value = typeof key === 'string' && key.startsWith('saas.') ? t(key)
    : e.code === 'ECONNABORTED' ? t('saas.payment_timeout') : t(fallback)
}
async function load() {
  const version = ++dataVersion
  const selectedOrg = orgs.selectedOrgId
  const [res, invoiceRes] = await Promise.all([api.get('/billing'), api.get('/billing/invoices')])
  if (disposed || version !== dataVersion || selectedOrg !== orgs.selectedOrgId) return
  billing.value = res.data.data
  invoices.value = invoiceRes.data.data
  autoRefreshFailed.value = false
  if (admin.value) {
    const [p, s, o] = await Promise.all([api.get('/saas/plans'), api.get('/saas/subscriptions'), organizationsService.list()])
    if (disposed || version !== dataVersion || selectedOrg !== orgs.selectedOrgId) return
    plans.value = p.data.data
    subscriptions.value = s.data.data
    companies.value = (o.data as any).data.organizations
  }
}
// The server settles invoices independently. This only reads local state; it
// never creates a payment or calls the gateway from the browser.
async function pollBilling() {
  if (disposed || polling || busy.value || document.hidden) return
  polling = true
  const version = dataVersion
  const selectedOrg = orgs.selectedOrgId
  try {
    const [b, i] = await Promise.all([api.get('/billing'), api.get('/billing/invoices')])
    if (disposed || busy.value || version !== dataVersion || selectedOrg !== orgs.selectedOrgId) return
    const fresh = i.data.data as Invoice[]
    const justPaid = fresh.some(item => item.status === 'paid' && invoices.value.some(old => old.id === item.id && old.status !== 'paid'))
    billing.value = b.data.data
    invoices.value = fresh
    autoRefreshFailed.value = false
    if (justPaid) {
      success.value = t('saas.payment_confirmed_automatically')
      error.value = ''
      if (admin.value) {
        const s = await api.get('/saas/subscriptions')
        if (!disposed && !busy.value && version === dataVersion && selectedOrg === orgs.selectedOrgId) subscriptions.value = s.data.data
      }
    }
  } catch {
    if (!disposed && version === dataVersion && selectedOrg === orgs.selectedOrgId) autoRefreshFailed.value = true
  } finally { polling = false }
}
async function refresh() {
  busy.value = true; error.value = ''; billing.value = null; invoices.value = []
  try { await load() } catch (e) { failure(e) } finally { busy.value = false }
}
async function paymentAction(id?: string, action = 'refresh') {
  busy.value = true; error.value = ''; success.value = ''
  try {
    if (id) await api.post(`/billing/invoices/${id}/${action}`)
    else await api.post('/billing/invoices', { subscription_expires_at: billing.value?.subscription?.expires_at })
    success.value = t('saas.payment_updated')
  } catch (e) { failure(e, 'saas.payment_unavailable') }
  finally {
    try { await load() } catch (e) { if (!error.value) failure(e) }
    busy.value = false
  }
}
async function copyPix(code: string) {
  try { await navigator.clipboard.writeText(code); success.value = t('saas.pix_copied') }
  catch { error.value = t('saas.pix_copy_failed') }
}
async function savePlan() {
  busy.value = true; error.value = ''; success.value = ''
  try {
    if (plan.value.id) await api.put(`/saas/plans/${plan.value.id}`, plan.value)
    else await api.post('/saas/plans', plan.value)
    plan.value = emptyPlan(); success.value = t('saas.saved'); await load()
  } catch (e) { failure(e) } finally { busy.value = false }
}
async function assign() {
  busy.value = true; error.value = ''; success.value = ''
  try {
    await api.put('/saas/subscriptions', { ...assignment.value, expires_at: new Date(assignment.value.expires_at).toISOString() })
    success.value = t('saas.saved'); await load()
  } catch (e) { failure(e) } finally { busy.value = false }
}
async function createTrial() {
  busy.value = true; error.value = ''; success.value = ''
  try {
    await api.post('/saas/organizations', trial.value)
    trial.value = { name: '', owner_name: '', email: '', password: '', plan_id: '' }
    success.value = t('saas.trial_created')
    await load()
  } catch (e) { failure(e) }
  finally { trial.value.password = ''; busy.value = false }
}
function editSubscription(s: Subscription) {
  const date = new Date(s.expires_at)
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  assignment.value = { organization_id: s.organization_id, plan_id: s.plan_id, status: s.status, expires_at: date.toISOString().slice(0, 16) }
}
watch(() => orgs.selectedOrgId, refresh)
onMounted(() => {
  void refresh()
  pollTimer = setInterval(() => { void pollBilling() }, 10000)
  document.addEventListener('visibilitychange', pollBilling)
})
onUnmounted(() => {
  disposed = true
  if (pollTimer) clearInterval(pollTimer)
  document.removeEventListener('visibilitychange', pollBilling)
})
</script>

<template>
  <div class="h-full overflow-y-auto">
  <div class="p-6 space-y-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between gap-3">
      <h1 class="text-2xl font-semibold">{{ t('saas.title') }}</h1>
      <Button variant="outline" :disabled="busy" @click="refresh">{{ t('common.refresh') }}</Button>
    </div>
    <p class="text-sm text-muted-foreground">{{ t('saas.stageNotice') }}</p>
    <p v-if="busy" role="status">{{ t('common.loading') }}</p>
    <p v-if="error" role="alert" class="border border-destructive rounded p-3 text-destructive">{{ error }}</p>
    <p v-if="success" role="status" class="border rounded p-3 text-green-700 dark:text-green-400">{{ success }}</p>
    <Card v-if="billing">
      <CardHeader><CardTitle>{{ t('saas.currentPlan') }}</CardTitle></CardHeader>
      <CardContent class="space-y-4">
        <p class="font-medium">{{ billing.subscription?.plan.name || t('saas.noPlan') }} — {{ t(`saas.${billing.status}`) }}</p>
        <p v-if="billing.subscription">{{ money(billing.subscription.plan) }} / {{ billing.subscription.plan.interval_months }} {{ t('saas.months') }} · {{ t('saas.expires') }}: {{ new Date(billing.subscription.expires_at).toLocaleString(locale) }}</p>
        <div class="grid sm:grid-cols-3 gap-4">
          <p>{{ t('saas.users') }}: {{ billing.usage.users }} / {{ limit(billing.subscription?.plan.user_limit) }}</p>
          <p>{{ t('saas.accounts') }}: {{ billing.usage.accounts }} / {{ limit(billing.subscription?.plan.account_limit) }}</p>
          <p>{{ t('saas.teams') }}: {{ billing.usage.teams }} / {{ limit(billing.subscription?.plan.team_limit) }}</p>
        </div>
      </CardContent>
    </Card>

    <Card v-if="billing">
      <CardHeader><CardTitle>{{ t('saas.invoices') }}</CardTitle></CardHeader>
      <CardContent class="space-y-4">
        <p class="text-sm text-muted-foreground">{{ t('saas.pix_hint') }}</p>
        <p class="text-sm" role="status">{{ t(autoRefreshFailed ? 'saas.auto_refresh_failed' : 'saas.auto_payment_hint') }}</p>
        <p>{{ t('saas.gateway') }}: {{ billing.payment_provider === 'efi' ? t('saas.efi_contingency') : 'Mercado Pago' }}</p>
        <p v-if="!billing.checkout_enabled" role="status">{{ t('saas.checkout_disabled') }}</p>
        <Button v-if="canPay && billing.checkout_enabled && billing.subscription && ['active', 'trialing'].includes(billing.subscription.status) && billing.subscription.plan.price_cents > 0" :disabled="busy" @click="paymentAction()">{{ t('saas.generate_pix') }}</Button>
        <p v-if="!invoices.length">{{ t('saas.no_invoices') }}</p>
        <article v-for="invoice in invoices" :key="invoice.id" class="rounded border p-4 space-y-3">
          <p class="font-medium">{{ money(invoice) }} · {{ invoice.provider === 'efi' ? 'Efí' : 'Mercado Pago' }} · {{ t(`saas.invoice_${invoice.status}`) }}</p>
          <p class="text-sm">{{ new Date(invoice.created_at).toLocaleString(locale) }} · {{ t('saas.expires') }}: {{ new Date(invoice.expires_at).toLocaleString(locale) }}</p>
          <p v-if="invoice.error_code" role="alert" class="text-destructive">{{ t(invoice.error_code) }}</p>
          <template v-if="invoice.pix_code && invoice.status === 'pending' && new Date(invoice.expires_at) > new Date()">
            <PixQRCode :code="invoice.pix_code" />
            <label class="block text-sm">{{ t('saas.pix_code') }}<textarea :value="invoice.pix_code" readonly class="mt-2 w-full rounded border bg-background p-2 font-mono" rows="3" /></label>
            <Button variant="outline" @click="copyPix(invoice.pix_code)">{{ t('saas.copy_pix') }}</Button>
          </template>
          <p v-else-if="invoice.status === 'pending' && new Date(invoice.expires_at) <= new Date()" class="text-sm">{{ t('saas.pix_expired') }}</p>
          <Button v-if="canPay && ['creating', 'unknown', 'pending'].includes(invoice.status)" variant="outline" :disabled="busy" @click="paymentAction(invoice.id)">{{ t('saas.check_payment') }}</Button>
          <Button v-if="canPay && invoice.status === 'pending'" variant="outline" :disabled="busy" @click="paymentAction(invoice.id, 'cancel')">{{ t('saas.cancel_pix') }}</Button>
          <Button v-if="canPay && billing.checkout_enabled && invoice.status === 'canceled'" variant="outline" :disabled="busy" @click="paymentAction(invoice.id, 'reissue')">{{ t('saas.reissue_pix') }}</Button>
        </article>
      </CardContent>
    </Card>

    <template v-if="admin">
      <Card>
        <CardHeader><CardTitle>{{ t('saas.new_trial_company') }}</CardTitle></CardHeader>
        <CardContent class="space-y-4">
          <p class="text-sm text-muted-foreground">{{ t('saas.trial_creation_hint') }}</p>
          <p v-if="!trialPlans.length">{{ t('saas.trial_plan_required') }}</p>
          <form v-else @submit.prevent="createTrial"><fieldset :disabled="busy" class="grid md:grid-cols-2 gap-4">
            <label>{{ t('saas.company') }}<Input v-model="trial.name" required maxlength="120" /></label>
            <label>{{ t('saas.owner_name') }}<Input v-model="trial.owner_name" required maxlength="120" autocomplete="off" /></label>
            <label>{{ t('saas.owner_email') }}<Input v-model="trial.email" type="email" required maxlength="255" autocomplete="off" /></label>
            <label>{{ t('saas.owner_password') }}<Input v-model="trial.password" type="password" required minlength="12" maxlength="72" autocomplete="new-password" /></label>
            <label>{{ t('saas.currentPlan') }}<select v-model="trial.plan_id" required class="block w-full rounded border bg-background p-2"><option value="" disabled>{{ t('saas.select') }}</option><option v-for="p in trialPlans" :key="p.id" :value="p.id">{{ p.name }} · {{ p.trial_days }} {{ t('saas.trial_days_label') }}</option></select></label>
            <Button type="submit">{{ t('saas.create_trial') }}</Button>
          </fieldset></form>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>{{ t('saas.catalog') }}</CardTitle></CardHeader>
        <CardContent class="space-y-5">
          <div class="overflow-auto">
            <table class="w-full text-sm text-left"><thead><tr><th class="p-2">{{ t('saas.name') }}</th><th>{{ t('saas.price') }}</th><th>{{ t('saas.status') }}</th><th>{{ t('saas.actions') }}</th></tr></thead>
              <tbody><tr v-for="p in plans" :key="p.id" class="border-t"><td class="p-2">{{ p.name }}</td><td>{{ money(p) }} / {{ p.interval_months }} {{ t('saas.months') }}</td><td>{{ t(p.active ? 'saas.active' : 'saas.inactive') }}</td><td><Button variant="outline" :disabled="busy" @click="plan = { ...p }">{{ t('common.edit') }}</Button></td></tr></tbody>
            </table>
          </div>
          <form @submit.prevent="savePlan"><fieldset :disabled="busy" class="grid md:grid-cols-3 gap-4">
            <label class="space-y-2">{{ t('saas.name') }}<Input v-model="plan.name" required maxlength="120" /></label>
            <label class="space-y-2">{{ t('saas.priceCents') }}<Input v-model.number="plan.price_cents" type="number" min="0" max="100000000" step="1" required /></label>
            <label class="space-y-2">{{ t('saas.interval') }}<select v-model.number="plan.interval_months" class="block w-full rounded border bg-background p-2"><option v-for="n in [1, 3, 6, 12]" :key="n" :value="n">{{ n }} {{ t('saas.months') }}</option></select></label>
            <label class="space-y-2">{{ t('saas.trialDays') }}<Input v-model.number="plan.trial_days" type="number" min="0" max="365" required /></label>
            <label class="space-y-2">{{ t('saas.users') }}<Input v-model.number="plan.user_limit" type="number" min="-1" max="1000000" required /></label>
            <label class="space-y-2">{{ t('saas.accounts') }}<Input v-model.number="plan.account_limit" type="number" min="-1" max="1000000" required /></label>
            <label class="space-y-2">{{ t('saas.teams') }}<Input v-model.number="plan.team_limit" type="number" min="-1" max="1000000" required /></label>
            <label><input v-model="plan.active" type="checkbox" /> {{ t('saas.active') }}</label>
            <label><input v-model="plan.public" type="checkbox" /> {{ t('saas.public') }}</label>
            <p class="md:col-span-3 text-sm text-muted-foreground">{{ t('saas.limitHint') }}</p>
            <Button type="submit">{{ t('common.save') }}</Button>
            <Button type="button" variant="outline" @click="plan = emptyPlan()">{{ t('saas.newPlan') }}</Button>
          </fieldset></form>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>{{ t('saas.assignments') }}</CardTitle></CardHeader>
        <CardContent class="space-y-5">
          <p class="text-sm text-muted-foreground">{{ t('saas.manualHint') }}</p>
          <form @submit.prevent="assign"><fieldset :disabled="busy" class="grid md:grid-cols-2 gap-4">
            <label>{{ t('saas.company') }}<select v-model="assignment.organization_id" required class="block w-full rounded border bg-background p-2"><option value="" disabled>{{ t('saas.select') }}</option><option v-for="o in companies" :key="o.id" :value="o.id">{{ o.name }}</option></select></label>
            <label>{{ t('saas.currentPlan') }}<select v-model="assignment.plan_id" required class="block w-full rounded border bg-background p-2"><option value="" disabled>{{ t('saas.select') }}</option><option v-for="p in plans.filter(p => p.active)" :key="p.id" :value="p.id">{{ p.name }}</option></select></label>
            <label>{{ t('saas.status') }}<select v-model="assignment.status" class="block w-full rounded border bg-background p-2"><option v-for="s in states" :key="s" :value="s">{{ t(`saas.${s}`) }}</option></select></label>
            <label>{{ t('saas.expires') }}<Input v-model="assignment.expires_at" type="datetime-local" required /></label>
            <Button type="submit">{{ t('saas.assign') }}</Button>
          </fieldset></form>
          <div class="overflow-auto"><table class="w-full text-sm text-left"><thead><tr><th class="p-2">{{ t('saas.company') }}</th><th>{{ t('saas.currentPlan') }}</th><th>{{ t('saas.status') }}</th><th>{{ t('saas.expires') }}</th><th>{{ t('saas.actions') }}</th></tr></thead>
            <tbody><tr v-for="s in subscriptions" :key="s.id" class="border-t"><td class="p-2">{{ s.organization.name }}</td><td>{{ s.plan.name }}</td><td>{{ t(`saas.${['active', 'trialing'].includes(s.status) && new Date(s.expires_at) <= new Date() ? 'expired' : s.status}`) }}</td><td>{{ new Date(s.expires_at).toLocaleString(locale) }}</td><td><Button :disabled="busy" variant="outline" @click="editSubscription(s)">{{ t('common.edit') }}</Button></td></tr></tbody>
          </table></div>
        </CardContent>
      </Card>
    </template>
  </div>
  </div>
</template>
