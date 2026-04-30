import { test, expect } from '@playwright/test'
import { ApiHelper, generateUniqueEmail, generateUniqueName, verifyAuditLogged } from '../../helpers'

/**
 * Audit-trail regression check.
 *
 * The backend's LogAudit (internal/audit/audit.go) is fired from many handlers.
 * This spec exercises CRUD on the highest-traffic resources and asserts that
 * /api/audit-logs records a (resource_type, resource_id, action) tuple for
 * each — guarding against regressions where someone adds a new CRUD path and
 * forgets to wire up the audit hook.
 *
 * If a new resource is added, copy one of the cases below. The verifyAuditLogged
 * helper will retry briefly because LogAudit creates entries asynchronously.
 */
test.describe('Audit trail — CRUD writes audit log entries', () => {
  test('user create/update/delete', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const roles = (await (await api.get('/api/roles')).json()).data?.roles ?? []
    const agentRole = roles.find((r: { name: string }) => r.name.toLowerCase() === 'agent')
    const managerRole = roles.find((r: { name: string }) => r.name.toLowerCase() === 'manager')
    expect(agentRole, 'agent role must exist').toBeDefined()

    const created = await api.createUser({
      email: generateUniqueEmail('audit-trail-user'),
      password: 'Password123!',
      full_name: generateUniqueName('AuditUser'),
      role_id: agentRole.id,
    })
    await verifyAuditLogged(request, 'user', created.id, 'created')

    if (managerRole) {
      await api.updateUserRole(created.id, managerRole.id)
      // Note: the diff records the change under "role" (the preloaded relation's
      // JSON tag) rather than "role_id" — both reflect the same change.
      await verifyAuditLogged(request, 'user', created.id, 'updated')
    }

    await api.deleteUser(created.id)
    await verifyAuditLogged(request, 'user', created.id, 'deleted')
  })

  test('contact create/update/delete', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const phone = `+1555${Date.now().toString().slice(-7)}`
    const created = await api.createContact(phone, generateUniqueName('AuditCt'))
    await verifyAuditLogged(request, 'contact', created.id, 'created')

    await api.updateContact(created.id, { profile_name: generateUniqueName('AuditCtUpd') })
    await verifyAuditLogged(request, 'contact', created.id, 'updated', { expectedFields: ['profile_name'] })

    const delResp = await api.del(`/api/contacts/${created.id}`)
    expect(delResp.ok()).toBe(true)
    await verifyAuditLogged(request, 'contact', created.id, 'deleted')
  })

  test('whatsapp account create/update/delete', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const acc = await api.createWhatsAppAccount({
      name: generateUniqueName('AuditAcct').replace(/\s/g, '-').toLowerCase(),
      phone_id: `phone-${Date.now()}`,
      business_id: `biz-${Date.now()}`,
      access_token: 'test-token-e2e',
    })
    await verifyAuditLogged(request, 'account', acc.id, 'created')

    const updResp = await api.put(`/api/accounts/${acc.id}`, { auto_read_receipt: true })
    expect(updResp.ok()).toBe(true)
    await verifyAuditLogged(request, 'account', acc.id, 'updated')

    const delResp = await api.del(`/api/accounts/${acc.id}`)
    expect(delResp.ok()).toBe(true)
    await verifyAuditLogged(request, 'account', acc.id, 'deleted')
  })

  test('template create/update/delete', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    // Templates need a WhatsApp account to be linked to.
    const acc = await api.createWhatsAppAccount({
      name: generateUniqueName('TplAcct').replace(/\s/g, '-').toLowerCase(),
      phone_id: `phone-tpl-${Date.now()}`,
      business_id: `biz-tpl-${Date.now()}`,
      access_token: 'test-token-e2e',
    })

    try {
      const tpl = await api.createTemplate({
        name: `audit_tpl_${Date.now()}`,
        body_content: 'Hello {{1}}',
        whatsapp_account: acc.name,
      })
      await verifyAuditLogged(request, 'template', tpl.id, 'created')

      const updResp = await api.put(`/api/templates/${tpl.id}`, { body_content: 'Hello {{1}}, updated' })
      expect(updResp.ok()).toBe(true)
      await verifyAuditLogged(request, 'template', tpl.id, 'updated', { expectedFields: ['body_content'] })

      const delResp = await api.del(`/api/templates/${tpl.id}`)
      expect(delResp.ok()).toBe(true)
      await verifyAuditLogged(request, 'template', tpl.id, 'deleted')
    } finally {
      await api.del(`/api/accounts/${acc.id}`)
    }
  })

  test('webhook create/update/delete', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const createResp = await api.post('/api/webhooks', {
      name: generateUniqueName('AuditHook'),
      url: 'https://webhook.site/audit-trail-test',
      events: ['message.received'],
      is_active: true,
    })
    expect(createResp.ok()).toBe(true)
    const wh = (await createResp.json()).data
    await verifyAuditLogged(request, 'webhook', wh.id, 'created')

    const updResp = await api.put(`/api/webhooks/${wh.id}`, {
      url: 'https://webhook.site/audit-trail-test-updated',
    })
    expect(updResp.ok()).toBe(true)
    await verifyAuditLogged(request, 'webhook', wh.id, 'updated')

    const delResp = await api.del(`/api/webhooks/${wh.id}`)
    expect(delResp.ok()).toBe(true)
    await verifyAuditLogged(request, 'webhook', wh.id, 'deleted')
  })
})
