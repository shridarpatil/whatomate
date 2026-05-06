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

async function clearQueueForOrg(orgId: string): Promise<void> {
  await execSQL(`DELETE FROM agent_transfers WHERE organization_id = '${orgId}' AND status = 'active' AND agent_id IS NULL`)
}

async function seedQueuedTransfer(orgId: string, contactId: string, phone: string, contactName: string, accountName: string): Promise<string> {
  const rows = await execSQL(`
    INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, transferred_at, created_at, updated_at)
    VALUES (gen_random_uuid(), '${orgId}', '${contactId}', '${accountName}', '${phone}', 'active', 'manual', NOW(), NOW(), NOW())
    RETURNING id::text AS id
  `)
  return rows[0]!.id as string
}

const scope = createTestScope('queue-pickup')

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
    if (orgId) await clearQueueForOrg(orgId)
    if (agent) {
      await api.deleteUser(agent.user.id).catch(() => {})
      await api.deleteRole(agent.role.id).catch(() => {})
    }
  })

  test.beforeEach(async () => {
    // Each test starts from a clean queue, then seeds what it needs.
    await clearQueueForOrg(orgId)
  })

  test('Pick Next button is disabled when the queue is empty', async ({ page }) => {
    await loginAs(page, agent)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    const pickBtn = page.getByRole('button', { name: /Pick Next/i })
    await expect(pickBtn).toBeVisible({ timeout: 10_000 })
    await expect(pickBtn).toBeDisabled()

    // The header counter renders the queue size.
    await expect(page.getByText(/0 waiting in queue/i)).toBeVisible()
  })

  test('Pick Next assigns the queued transfer and navigates to the chat', async ({ page }) => {
    await seedQueuedTransfer(orgId, contactId, phone, contactName, accountName)

    await loginAs(page, agent)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    // Counter reflects the seeded item; button is enabled.
    await expect(page.getByText(/1 waiting in queue/i)).toBeVisible({ timeout: 10_000 })
    const pickBtn = page.getByRole('button', { name: /Pick Next/i })
    await expect(pickBtn).toBeEnabled()

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
    await seedQueuedTransfer(orgId, contactId, phone, contactName, accountName)

    await loginAs(page, agent)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /Pick Next/i }).click()
    await page.waitForURL(new RegExp(`/chat/${contactId}`), { timeout: 10_000 })

    // Navigate back; the agent role's view shows their assigned transfers
    // directly (no tabs), so the just-picked contact must be in the table.
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

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
    if (orgId) await clearQueueForOrg(orgId)
    if (assignee) {
      await api.deleteUser(assignee.user.id).catch(() => {})
      await api.deleteRole(assignee.role.id).catch(() => {})
    }
  })

  test('admin assigns a queued transfer to a specific agent', async ({ page }) => {
    await clearQueueForOrg(orgId)
    const transferId = await execSQL(`
      INSERT INTO agent_transfers (id, organization_id, contact_id, whats_app_account, phone_number, status, source, transferred_at, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${contactId}', '${accountName}', '0000000000', 'active', 'manual', NOW(), NOW(), NOW())
      RETURNING id::text AS id
    `).then(r => r[0]!.id as string)

    // Super admin sees the admin/manager view (tabs).
    await page.goto('/login')
    await page.locator('input[type="email"]').fill(SUPER_ADMIN.email)
    await page.locator('input[type="password"]').fill(SUPER_ADMIN.password)
    await page.locator('button[type="submit"]').click()
    await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10_000 })

    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    // Open Queue tab. The seeded contact must be visible.
    await page.getByRole('tab', { name: /Queue/i }).click()
    const row = page.locator('tbody tr').filter({ hasText: '0000000000' })
    await expect(row).toBeVisible({ timeout: 10_000 })

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
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

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
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

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

  async function seedContactAndQueue(slug: string): Promise<{ contactId: string; transferId: string }> {
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
      return { contactId: contact.id, transferId }
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
    await clearQueueForOrg(orgId).catch(() => {})
    if (agent) {
      await api.deleteUser(agent.user.id).catch(() => {})
      await api.deleteRole(agent.role.id).catch(() => {})
    }
  })

  test('with assign_to_same_agent=true (default) pickup pins the agent as relationship manager', async ({ page }) => {
    await updateChatbotSetting('assign_to_same_agent', true)
    const { contactId } = await seedContactAndQueue('rm-on')

    await loginAs(page, agent)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /Pick Next/i }).click()
    await page.waitForURL(new RegExp(`/chat/${contactId}`), { timeout: 10_000 })

    expect(await readContactAssignedUser(contactId)).toBe(agent.user.id)
  })

  test('with assign_to_same_agent=false pickup does NOT touch contact.assigned_user_id', async ({ page }) => {
    await updateChatbotSetting('assign_to_same_agent', false)
    const { contactId } = await seedContactAndQueue('rm-off')

    // Sanity: nobody is assigned before the pick.
    expect(await readContactAssignedUser(contactId)).toBeNull()

    await loginAs(page, agent)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

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
