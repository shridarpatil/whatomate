import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { ApiHelper } from '../../helpers'
import {
  createTestScope,
  createUserWithPermissions,
  loginAs,
  SUPER_ADMIN,
  type TestUserHandle,
} from '../../framework'

/**
 * UI-driven coverage of the "pick from queue" agent flow.
 *
 * Transfers normally land in the queue when a chatbot flow / keyword rule
 * fires the TRANSFER step or when an agent manually transfers a chat. We
 * seed agent_transfers rows directly so the queue has deterministic items
 * for the page to render. The picking itself goes through the UI: button
 * click in the page header → toast → navigation to the chat.
 */

const DB_URL = process.env.TEST_DATABASE_URL || 'postgres://whatomate:whatomate@127.0.0.1:5432/whatomate'

async function execSQL(sql: string): Promise<Record<string, unknown>[]> {
  const client = new Client({ connectionString: DB_URL })
  await client.connect()
  try {
    const result = await client.query(sql)
    return result.rows as Record<string, unknown>[]
  } finally {
    await client.end()
  }
}

// Navigates to /chatbot/transfers and waits for the transfers list GET to
// resolve before returning. Plain `waitForLoadState('networkidle')` is
// unreliable here — Vue's lazy-loaded view can fire the GET after the
// network appears idle, so the queue-counter / button-enabled assertions
// race against an empty initial store. Waiting on the response itself
// makes those assertions deterministic.
async function gotoTransfersAndWaitLoad(page: import('@playwright/test').Page): Promise<void> {
  const transfersListed = page.waitForResponse(
    r => r.url().includes('/api/chatbot/transfers') && r.request().method() === 'GET' && r.ok(),
    { timeout: 15_000 },
  )
  await page.goto('/chatbot/transfers')
  await transfersListed
}

async function seedQueuedTransfer(orgId: string, contactId: string, phone: string, contactName: string, accountName: string): Promise<string> {
  const rows = await execSQL(`
    INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, transferred_at, created_at, updated_at)
    VALUES (gen_random_uuid(), '${orgId}', '${contactId}', '${accountName}', '${phone}', 'active', 'manual', NOW(), NOW(), NOW())
    RETURNING id::text AS id
  `)
  return rows[0]!.id as string
}

// Seed → load page → if our seed got cleared by a parallel worker's
// beforeEach, re-seed and reload. Returns once the Pick Next button is
// enabled, meaning the agent's view of the queue has at least one row.
// The shared org makes literal queue-count assertions fragile under
// parallelism, but "the row I just seeded is visible in my queue" is.
async function ensureSeedVisible(
  page: import('@playwright/test').Page,
  reseed: () => Promise<void>,
  attempts = 3,
): Promise<void> {
  for (let i = 0; i < attempts; i++) {
    const pickBtn = page.getByRole('button', { name: /Pick Next/i })
    try {
      await expect(pickBtn).toBeEnabled({ timeout: 5_000 })
      return
    } catch {
      // Seed got blown away by a parallel worker — replace it and reload.
      await reseed()
      const refreshed = page.waitForResponse(
        r => r.url().includes('/api/chatbot/transfers') && r.request().method() === 'GET' && r.ok(),
        { timeout: 15_000 },
      )
      await page.reload()
      await refreshed
    }
  }
  throw new Error(`Pick Next button never enabled after ${attempts} re-seed attempts`)
}

const scope = createTestScope('queue-pickup')

// Group of describes that all touch the shared org's queue state or
// global chatbot settings. They run sequentially across this outer block
// to avoid:
//   - one describe's seed (or unassigned-after-unassign) appearing in
//     another's pickup
//   - one describe flipping assign_to_same_agent / allow_agent_queue_pickup
//     while another reads it
// "Queue tab team filter" sits outside this block and runs in parallel —
// its rows live in dedicated teams that the agents in the serial group
// can't see, so it's genuinely isolated.
test.describe.serial('Queue + settings tests (shared org state)', () => {

test.describe('Pick from queue — agent flow', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let api: ApiHelper
  let agent: TestUserHandle
  let orgId: string
  let accountName: string
  let contactId: string
  let phone: string
  let contactName: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    // Agent role: read chat + transfers, pickup but NOT transfers:write.
    // transfers:write would flip the view into admin/manager mode and hide
    // the Pick Next button under #actions.
    agent = await createUserWithPermissions(api, scope, {
      userSlug: 'pickup-agent',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
        { resource: 'contacts', action: 'read' },
      ],
    })

    // Pin org from the freshly created agent's lookup.
    const userRows = await execSQL(
      `SELECT u.id::text AS id, uo.organization_id::text AS org FROM users u
       JOIN user_organizations uo ON uo.user_id = u.id AND uo.is_default = true
       WHERE u.email = '${agent.email}' LIMIT 1`,
    )
    orgId = userRows[0]!.org as string

    // Reuse / create a WhatsApp account pinned to the org.
    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    if (accounts.length > 0) {
      accountName = accounts[0]!.name
    } else {
      const acc = await api.createWhatsAppAccount({
        name: scope.name('acc').toLowerCase().replace(/\s/g, '-'),
        phone_id: `phone-${Date.now()}`,
        business_id: `biz-${Date.now()}`,
        access_token: 'test-token-queue',
      })
      accountName = acc.name
    }

    // Create the contact that will be in queue.
    phone = scope.phone()
    contactName = scope.name('queued-contact')
    const contact = await api.createContact(phone, contactName)
    contactId = contact.id

    // Pin contact to our account.
    await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${contactId}'`)
  })

  test.afterAll(async () => {
    // (no afterAll queue clear — would race parallel describes)
    if (agent) {
      await api.deleteUser(agent.user.id).catch(() => {})
      await api.deleteRole(agent.role.id).catch(() => {})
    }
  })

  // No beforeEach clear: clearQueueForOrg blows away ALL unassigned rows
  // in the shared org, which would race with parallel workers' seeds.
  // Each test seeds + asserts on its own specific item using
  // ensureSeedVisible() to self-heal if a sibling worker clears mid-test.

  // Note: there is intentionally no "Pick Next disabled when queue is empty"
  // test — multiple specs run in parallel against the same shared org, so
  // any worker seeding a queue item can flip "queue empty" mid-assertion.
  // The negative state is impossible to assert reliably without isolating
  // the org, which would be a much bigger refactor.

  test('Pick Next assigns the queued transfer and navigates to the chat', async ({ page }) => {
    const reseed = () => seedQueuedTransfer(orgId, contactId, phone, contactName, accountName).then(() => {})
    await reseed()

    await loginAs(page, agent)
    await gotoTransfersAndWaitLoad(page)
    // Self-heal: a parallel worker's beforeEach may have wiped our seed
    // between insert and page load. Verify the button is enabled (queue
    // has ≥ 1 item) and re-seed if it isn't.
    await ensureSeedVisible(page, reseed)

    const pickBtn = page.getByRole('button', { name: /Pick Next/i })
    await pickBtn.click()

    // Success toast confirms the pick. Use a partial match because the toast
    // body includes the contact name interpolated via i18n.
    await expect(
      page.locator('[data-sonner-toast]').filter({ hasText: /Transfer picked/i }),
    ).toBeVisible({ timeout: 10_000 })

    // After picking the page navigates to the contact's chat.
    await page.waitForURL(new RegExp(`/chat/${contactId}`), { timeout: 10_000 })

    // DB sanity: the transfer is now assigned to this agent and out of queue.
    const rows = await execSQL(
      `SELECT agent_id::text AS agent_id FROM agent_transfers WHERE contact_id = '${contactId}' AND status = 'active'`,
    )
    expect(rows.length).toBe(1)
    expect(rows[0]!.agent_id).toBe(agent.user.id)
  })

  test('Picked transfer surfaces in the agent\'s My Transfers list', async ({ page }) => {
    const reseed = () => seedQueuedTransfer(orgId, contactId, phone, contactName, accountName).then(() => {})
    await reseed()

    await loginAs(page, agent)
    await gotoTransfersAndWaitLoad(page)
    await ensureSeedVisible(page, reseed)

    await page.getByRole('button', { name: /Pick Next/i }).click()
    await page.waitForURL(new RegExp(`/chat/${contactId}`), { timeout: 10_000 })

    // Navigate back; the agent role's view shows their assigned transfers
    // directly (no tabs), so the just-picked contact must be in the table.
    await gotoTransfersAndWaitLoad(page)

    // Multiple rows may match if previous tests left assigned transfers in
    // place; we just need one visible row for the contact.
    await expect(
      page.locator('table').getByText(contactName).first(),
    ).toBeVisible({ timeout: 10_000 })
  })
})

test.describe('Pick from queue — admin assign flow', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let api: ApiHelper
  let assignee: TestUserHandle
  let orgId: string
  let accountName: string
  let contactId: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    // Create an agent that the admin will assign the queued transfer to.
    assignee = await createUserWithPermissions(api, scope, {
      userSlug: 'tx-assignee',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
      ],
    })

    const userRows = await execSQL(
      `SELECT uo.organization_id::text AS org FROM users u
       JOIN user_organizations uo ON uo.user_id = u.id AND uo.is_default = true
       WHERE u.email = '${assignee.email}' LIMIT 1`,
    )
    orgId = userRows[0]!.org as string

    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    accountName = accounts[0]?.name ?? 'test-account'

    const phone = scope.phone()
    const contact = await api.createContact(phone, scope.name('admin-queued'))
    contactId = contact.id
    await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${contactId}'`)
  })

  test.afterAll(async () => {
    // (no afterAll queue clear — would race parallel describes)
    if (assignee) {
      await api.deleteUser(assignee.user.id).catch(() => {})
      await api.deleteRole(assignee.role.id).catch(() => {})
    }
  })

  test('admin assigns a queued transfer to a specific agent', async ({ page }) => {
    // Unique phone so the row stays identifiable even if other workers'
    // beforeEach clears nuke the entire general queue.
    const uniquePhone = scope.phone()
    const seedRow = async () => execSQL(`
      INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, transferred_at, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${contactId}', '${accountName}', '${uniquePhone}', 'active', 'manual', NOW(), NOW(), NOW())
      RETURNING id::text AS id
    `).then(r => r[0]!.id as string)
    let transferId = await seedRow()

    // Super admin sees the admin/manager view (tabs).
    await page.goto('/login')
    await page.locator('input[type="email"]').fill(SUPER_ADMIN.email)
    await page.locator('input[type="password"]').fill(SUPER_ADMIN.password)
    await page.locator('button[type="submit"]').click()
    await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10_000 })

    await gotoTransfersAndWaitLoad(page)
    await page.getByRole('tab', { name: /^Queue\b/i }).click()

    // Self-heal if a parallel worker cleared the queue between seed and load.
    const row = page.locator('tbody tr').filter({ hasText: uniquePhone })
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        await expect(row).toBeVisible({ timeout: 5_000 })
        break
      } catch {
        transferId = await seedRow()
        const refreshed = page.waitForResponse(
          r => r.url().includes('/api/chatbot/transfers') && r.request().method() === 'GET' && r.ok(),
          { timeout: 15_000 },
        )
        await page.reload()
        await refreshed
        await page.getByRole('tab', { name: /^Queue\b/i }).click()
      }
    }
    await expect(row).toBeVisible({ timeout: 5_000 })

    // Click Assign on that row → dialog opens → pick the agent → submit.
    await row.getByRole('button', { name: /^Assign$/i }).click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible({ timeout: 5_000 })

    // The dialog has two comboboxes (Team Queue then Assign to Agent).
    // We want the second.
    const agentSelect = dialog.getByRole('combobox').nth(1)
    await agentSelect.click()

    // The agent option is rendered with full_name = scope.name('tx-assignee'),
    // so the option text contains 'tx-assignee'.
    await page.getByRole('option').filter({ hasText: /tx-assignee/i }).first().click()

    await dialog.getByRole('button', { name: /^Save$/i }).click()

    // Toast on success.
    await expect(
      page.locator('[data-sonner-toast]').filter({ hasText: /updated|assigned/i }),
    ).toBeVisible({ timeout: 10_000 })

    // Transfer now has the agent assigned in the DB.
    const rows = await execSQL(
      `SELECT agent_id::text AS agent_id FROM agent_transfers WHERE id = '${transferId}'`,
    )
    expect(rows[0]!.agent_id).toBe(assignee.user.id)
  })
})

// Drive a settings update through the API so the Redis settings cache
// invalidates. Updating chatbot_settings directly via SQL works for the row
// but PickNextTransfer reads from a 6h-TTL cache. Each call opens a fresh
// APIRequestContext because Playwright's `request` fixture from beforeAll
// can't be reused inside test bodies.
async function updateChatbotSetting(field: string, value: boolean): Promise<void> {
  const ctx = await playwrightRequest.newContext()
  const settingsApi = new ApiHelper(ctx)
  try {
    await settingsApi.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    const resp = await settingsApi.put('/api/chatbot/settings', { [field]: value })
    if (!resp.ok()) {
      throw new Error(`Failed to set ${field}=${value}: ${resp.status()} ${await resp.text()}`)
    }
  } finally {
    await ctx.dispose()
  }
}

test.describe('Queue pickup gated by allow_agent_queue_pickup', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let api: ApiHelper
  let agent: TestUserHandle
  let orgId: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    agent = await createUserWithPermissions(api, scope, {
      userSlug: 'gated-agent',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
        { resource: 'contacts', action: 'read' },
      ],
    })

    const userRows = await execSQL(
      `SELECT uo.organization_id::text AS org FROM users u
       JOIN user_organizations uo ON uo.user_id = u.id AND uo.is_default = true
       WHERE u.email = '${agent.email}' LIMIT 1`,
    )
    orgId = userRows[0]!.org as string
  })

  test.afterAll(async () => {
    // Restore the default so other specs don't see the kill switch flipped.
    await updateChatbotSetting('allow_agent_queue_pickup', true).catch(() => {})
    if (agent) {
      await api.deleteUser(agent.user.id).catch(() => {})
      await api.deleteRole(agent.role.id).catch(() => {})
    }
  })

  test('Pick Next is disabled with a tooltip when the toggle is off', async ({ page }) => {
    await updateChatbotSetting('allow_agent_queue_pickup', false)

    await loginAs(page, agent)
    await gotoTransfersAndWaitLoad(page)

    const pickBtn = page.getByRole('button', { name: /Pick Next/i })
    await expect(pickBtn).toBeVisible({ timeout: 10_000 })
    await expect(pickBtn).toBeDisabled()

    // Tooltip explains why it's disabled. We hover the wrapping span (the
    // tooltip trigger) — disabled buttons don't receive pointer events, and
    // the trigger wraps the button precisely for this reason.
    await pickBtn.locator('xpath=ancestor::span[1]').hover()
    await expect(
      page.getByText(/Queue pickup is disabled by your administrator/i),
    ).toBeVisible({ timeout: 5_000 })
  })

  test('Pick Next is enabled again when the toggle is flipped back on', async ({ page }) => {
    await updateChatbotSetting('allow_agent_queue_pickup', true)

    await loginAs(page, agent)
    await gotoTransfersAndWaitLoad(page)

    // No queued transfers seeded → button stays disabled for the empty-queue
    // reason. We just need to confirm the kill-switch tooltip is gone.
    // <Tooltip :disabled="true"> means the TooltipContent never renders even
    // on hover, so a static count check is sufficient.
    await expect(
      page.getByText(/Queue pickup is disabled by your administrator/i),
    ).toHaveCount(0)
  })
})

test.describe('Pickup respects assign_to_same_agent', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let api: ApiHelper
  let agent: TestUserHandle
  let orgId: string
  let accountName: string

  async function seedContactAndQueue(slug: string): Promise<{ contactId: string; transferId: string; reseed: () => Promise<void> }> {
    // Fresh request context: the beforeAll-bound `api` can't be reused inside
    // test bodies. Login each time — cheaper than passing the auth state around.
    const ctx = await playwrightRequest.newContext()
    const localApi = new ApiHelper(ctx)
    try {
      await localApi.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
      const phone = scope.phone()
      const contact = await localApi.createContact(phone, scope.name(slug))
      await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${contact.id}'`)
      const transferId = await seedQueuedTransfer(orgId, contact.id, phone, scope.name(slug), accountName)
      // The reseed closure re-inserts a queue row for the same contact if a
      // parallel worker's beforeEach blew it away before the page loaded.
      const reseed = async () => {
        await seedQueuedTransfer(orgId, contact.id, phone, scope.name(slug), accountName)
      }
      return { contactId: contact.id, transferId, reseed }
    } finally {
      await ctx.dispose()
    }
  }

  async function readContactAssignedUser(contactId: string): Promise<string | null> {
    const rows = await execSQL(
      `SELECT assigned_user_id::text AS assigned_user_id FROM contacts WHERE id = '${contactId}'`,
    )
    return (rows[0]!.assigned_user_id as string | null) ?? null
  }

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    agent = await createUserWithPermissions(api, scope, {
      userSlug: 'assign-toggle-agent',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
        { resource: 'contacts', action: 'read' },
      ],
    })

    const userRows = await execSQL(
      `SELECT uo.organization_id::text AS org FROM users u
       JOIN user_organizations uo ON uo.user_id = u.id AND uo.is_default = true
       WHERE u.email = '${agent.email}' LIMIT 1`,
    )
    orgId = userRows[0]!.org as string

    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    accountName = accounts[0]?.name ?? 'test-account'
  })

  test.afterAll(async () => {
    await updateChatbotSetting('assign_to_same_agent', true).catch(() => {})
    // (no afterAll queue clear — would race parallel describes)
    if (agent) {
      await api.deleteUser(agent.user.id).catch(() => {})
      await api.deleteRole(agent.role.id).catch(() => {})
    }
  })

  test('with assign_to_same_agent=true (default) pickup pins the agent as relationship manager', async ({ page }) => {
    await updateChatbotSetting('assign_to_same_agent', true)
    const { contactId, reseed } = await seedContactAndQueue('rm-on')

    await loginAs(page, agent)
    await gotoTransfersAndWaitLoad(page)
    await ensureSeedVisible(page, reseed)

    await page.getByRole('button', { name: /Pick Next/i }).click()
    await page.waitForURL(new RegExp(`/chat/${contactId}`), { timeout: 10_000 })

    expect(await readContactAssignedUser(contactId)).toBe(agent.user.id)
  })

  test('with assign_to_same_agent=false pickup does NOT touch contact.assigned_user_id', async ({ page }) => {
    await updateChatbotSetting('assign_to_same_agent', false)
    const { contactId, reseed } = await seedContactAndQueue('rm-off')

    // Sanity: nobody is assigned before the pick.
    expect(await readContactAssignedUser(contactId)).toBeNull()

    await loginAs(page, agent)
    await gotoTransfersAndWaitLoad(page)
    await ensureSeedVisible(page, reseed)

    await page.getByRole('button', { name: /Pick Next/i }).click()
    await page.waitForURL(new RegExp(`/chat/${contactId}`), { timeout: 10_000 })

    // After pickup the agent has visibility through the active transfer
    // (agent_transfers.agent_id), but the relationship-manager pointer must
    // stay nil so the chat doesn't stick to them after resume.
    expect(await readContactAssignedUser(contactId)).toBeNull()

    // Wait until the chat view has hydrated the active transfer state. The
    // "Paused" badge in the chat header is gated on activeTransferId, so its
    // visibility means the transfers store has loaded.
    await expect(page.getByText('Paused').first()).toBeVisible({ timeout: 10_000 })

    // Click the standalone Resume button in the chat header. The button is
    // icon-only (Play icon, no aria-label). Lucide-vue-next renders icons
    // with a class attribute like "lucide lucide-play" on the SVG, but
    // Playwright's :has(svg.X) selector struggles with SVG namespacing —
    // use attribute matching instead.
    const resumeBtn = page.locator('main button').filter({
      has: page.locator('[class*="lucide-play"]'),
    }).first()
    await expect(resumeBtn).toBeVisible({ timeout: 10_000 })
    await resumeBtn.click()

    await expect(
      page.locator('[data-sonner-toast]').filter({ hasText: /resumed/i }),
    ).toBeVisible({ timeout: 10_000 })

    // Re-read: assigned_user_id remains nil, transfer is resumed.
    expect(await readContactAssignedUser(contactId)).toBeNull()
    const transferRows = await execSQL(
      `SELECT status FROM agent_transfers WHERE contact_id = '${contactId}' ORDER BY transferred_at DESC LIMIT 1`,
    )
    expect(transferRows[0]!.status).toBe('resumed')
  })
})

test.describe('Admin reassign and unassign flows', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let api: ApiHelper
  let agentA: TestUserHandle
  let agentB: TestUserHandle
  let orgId: string
  let accountName: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    // Two pickup-eligible agents. Slugs deliberately avoid the substring
    // "agent" / "manager" / "admin" so the assign-dialog option lookup
    // (which matches by hasText) doesn't collide with the seeded system
    // roles (see DialogPage.selectOption history).
    agentA = await createUserWithPermissions(api, scope, {
      userSlug: 'reassign-pickup-a',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
      ],
    })
    agentB = await createUserWithPermissions(api, scope, {
      userSlug: 'reassign-pickup-b',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
      ],
    })

    const userRows = await execSQL(
      `SELECT uo.organization_id::text AS org FROM users u
       JOIN user_organizations uo ON uo.user_id = u.id AND uo.is_default = true
       WHERE u.email = '${agentA.email}' LIMIT 1`,
    )
    orgId = userRows[0]!.org as string

    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    accountName = accounts[0]?.name ?? 'test-account'
  })

  test.afterAll(async () => {
    // (no afterAll queue clear — would race parallel describes)
    if (agentA) {
      await api.deleteUser(agentA.user.id).catch(() => {})
      await api.deleteRole(agentA.role.id).catch(() => {})
    }
    if (agentB) {
      await api.deleteUser(agentB.user.id).catch(() => {})
      await api.deleteRole(agentB.role.id).catch(() => {})
    }
  })

  // Login the super-admin into the browser for the dialog-driven actions.
  async function loginSuperAdmin(page: import('@playwright/test').Page) {
    await page.goto('/login')
    await page.locator('input[type="email"]').fill(SUPER_ADMIN.email)
    await page.locator('input[type="password"]').fill(SUPER_ADMIN.password)
    await page.locator('button[type="submit"]').click()
    await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10_000 })
  }

  async function seedTransferAssignedTo(agent: TestUserHandle, slug: string): Promise<{ contactId: string; transferId: string; phone: string; contactName: string }> {
    const ctx = await playwrightRequest.newContext()
    const localApi = new ApiHelper(ctx)
    try {
      await localApi.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
      const phone = scope.phone()
      const contactName = scope.name(slug)
      const contact = await localApi.createContact(phone, contactName)
      await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${contact.id}'`)
      const rows = await execSQL(`
        INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, agent_id, transferred_at, created_at, updated_at)
        VALUES (gen_random_uuid(), '${orgId}', '${contact.id}', '${accountName}', '${phone}', 'active', 'manual', '${agent.user.id}', NOW(), NOW(), NOW())
        RETURNING id::text AS id
      `)
      return { contactId: contact.id, transferId: rows[0]!.id as string, phone, contactName }
    } finally {
      await ctx.dispose()
    }
  }

  test('admin reassigns a transfer from agent A to agent B', async ({ page }) => {
    // No clearQueueForOrg: this test seeds with agent_id set, so the row
    // isn't subject to clearQueueForOrg from sibling tests (which only
    // touches agent_id IS NULL). Conversely, calling it ourselves would
    // race with siblings' seeds.
    const seeded = await seedTransferAssignedTo(agentA, 'reassign-target')

    await loginSuperAdmin(page)
    await gotoTransfersAndWaitLoad(page)

    // Reassign happens from the All Active tab where assigned transfers
    // surface. The admin/manager view is the only one with this tab.
    await page.getByRole('tab', { name: /All Active/i }).click()
    const row = page.locator('tbody tr').filter({ hasText: seeded.phone })
    await expect(row).toBeVisible({ timeout: 10_000 })
    await row.getByRole('button', { name: /^Assign$/i }).click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible({ timeout: 5_000 })

    // Second combobox is the agent select. Pick agent B by slug.
    await dialog.getByRole('combobox').nth(1).click()
    await page.getByRole('option').filter({ hasText: /reassign-pickup-b/i }).first().click()
    await dialog.getByRole('button', { name: /^Save$/i }).click()

    await expect(
      page.locator('[data-sonner-toast]').filter({ hasText: /updated|assigned/i }),
    ).toBeVisible({ timeout: 10_000 })

    const rows = await execSQL(
      `SELECT agent_id::text AS agent_id FROM agent_transfers WHERE id = '${seeded.transferId}'`,
    )
    expect(rows[0]!.agent_id).toBe(agentB.user.id)
  })

  test('admin unassigns a transfer back to the queue', async ({ page }) => {
    const seeded = await seedTransferAssignedTo(agentA, 'unassign-target')

    await loginSuperAdmin(page)
    await gotoTransfersAndWaitLoad(page)

    await page.getByRole('tab', { name: /All Active/i }).click()
    const row = page.locator('tbody tr').filter({ hasText: seeded.phone })
    await expect(row).toBeVisible({ timeout: 10_000 })
    await row.getByRole('button', { name: /^Assign$/i }).click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible({ timeout: 5_000 })
    await dialog.getByRole('combobox').nth(1).click()
    // The "Unassigned (in queue)" option returns the transfer to the queue.
    await page.getByRole('option').filter({ hasText: /Unassigned/i }).first().click()
    await dialog.getByRole('button', { name: /^Save$/i }).click()

    await expect(
      page.locator('[data-sonner-toast]').filter({ hasText: /updated|assigned/i }),
    ).toBeVisible({ timeout: 10_000 })

    const rows = await execSQL(
      `SELECT agent_id::text AS agent_id, status FROM agent_transfers WHERE id = '${seeded.transferId}'`,
    )
    expect(rows[0]!.agent_id).toBeNull()
    expect(rows[0]!.status).toBe('active')
  })
})

}) // end of serial group: Queue + settings tests (shared org state)

test.describe('Queue tab team filter', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let api: ApiHelper
  let orgId: string
  let accountName: string
  let teamAlphaId: string
  let teamBetaId: string
  let alphaContactName: string
  let betaContactName: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    // Derive the super admin's default org rather than hardcoding a name —
    // CI seeds something like "Test Org <hash>", not a stable label.
    const orgRows = await execSQL(`
      SELECT uo.organization_id::text AS id
      FROM user_organizations uo
      JOIN users u ON u.id = uo.user_id
      WHERE u.email = '${SUPER_ADMIN.email}' AND uo.is_default = true
      LIMIT 1
    `)
    orgId = orgRows[0]!.id as string

    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    accountName = accounts[0]?.name ?? 'test-account'

    // Create two teams via API.
    const tA = await api.post('/api/teams', { name: scope.name('team-alpha'), description: 'team filter alpha' })
    expect(tA.ok()).toBe(true)
    teamAlphaId = (await tA.json()).data.team.id
    const tB = await api.post('/api/teams', { name: scope.name('team-beta'), description: 'team filter beta' })
    expect(tB.ok()).toBe(true)
    teamBetaId = (await tB.json()).data.team.id

    // Seed one queued transfer in each team.
    const phoneA = scope.phone()
    alphaContactName = scope.name('alpha-contact')
    const ctA = await api.createContact(phoneA, alphaContactName)
    await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${ctA.id}'`)
    await execSQL(`
      INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, team_id, transferred_at, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${ctA.id}', '${accountName}', '${phoneA}', 'active', 'manual', '${teamAlphaId}', NOW(), NOW(), NOW())
    `)

    const phoneB = scope.phone()
    betaContactName = scope.name('beta-contact')
    const ctB = await api.createContact(phoneB, betaContactName)
    await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${ctB.id}'`)
    await execSQL(`
      INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, team_id, transferred_at, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${ctB.id}', '${accountName}', '${phoneB}', 'active', 'manual', '${teamBetaId}', NOW(), NOW(), NOW())
    `)
  })

  test.afterAll(async () => {
    if (orgId) {
      await execSQL(
        `DELETE FROM agent_transfers WHERE organization_id = '${orgId}' AND status = 'active' AND agent_id IS NULL AND team_id IN ('${teamAlphaId}', '${teamBetaId}')`,
      )
      await execSQL(`DELETE FROM teams WHERE id IN ('${teamAlphaId}', '${teamBetaId}')`)
    }
  })

  test('admin can filter the queue to a single team', async ({ page }) => {
    await page.goto('/login')
    await page.locator('input[type="email"]').fill(SUPER_ADMIN.email)
    await page.locator('input[type="password"]').fill(SUPER_ADMIN.password)
    await page.locator('button[type="submit"]').click()
    await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10_000 })

    await gotoTransfersAndWaitLoad(page)
    // The Queue tab's accessible name includes the badge count, e.g. "Queue 2".
    await page.getByRole('tab', { name: /^Queue\b/i }).click()

    const queueTable = page.locator('[role="tabpanel"]').filter({ hasText: /Transfer Queue/i }).locator('table')
    await expect(queueTable).toBeVisible({ timeout: 10_000 })

    // Without filter both contacts should be visible.
    await expect(queueTable.getByText(alphaContactName)).toBeVisible()
    await expect(queueTable.getByText(betaContactName)).toBeVisible()

    // Apply alpha team filter — the team-filter combobox is the
    // "Filter by team" select on the Queue tab header.
    await page.getByRole('combobox').filter({ hasText: /All Queues|Filter by team/i }).first().click()
    await page.getByRole('option').filter({ hasText: scope.name('team-alpha') }).first().click()

    await expect(queueTable.getByText(alphaContactName)).toBeVisible({ timeout: 5_000 })
    await expect(queueTable.getByText(betaContactName)).toHaveCount(0)
  })
})
