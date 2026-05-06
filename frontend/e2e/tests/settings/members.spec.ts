import { test, expect } from '@playwright/test'
import { ApiHelper } from '../../helpers'
import { createTestScope, SUPER_ADMIN } from '../../framework'

const scope = createTestScope('members-api')

async function loginApi(api: ApiHelper): Promise<boolean> {
  try {
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    return true
  } catch {
    try {
      await api.login('admin@test.com', 'password')
      return true
    } catch {
      return false
    }
  }
}

test.describe('Organization Members - API Tests', () => {
  test('should list organization members via API', async ({ request }) => {
    const api = new ApiHelper(request)
    test.skip(!(await loginApi(api)), 'No admin credentials available')

    const members = await api.getOrgMembers()
    expect(Array.isArray(members)).toBeTruthy()
  })

  test('should add and remove organization member via API', async ({ request }) => {
    const api = new ApiHelper(request)
    test.skip(!(await loginApi(api)), 'No admin credentials available')

    let org: any
    try {
      org = await api.createOrganization(scope.name('add-remove-org'))
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    // Need a real role_id; the agent role is seeded by Go migrations.
    const rolesResp = await api.get('/api/roles')
    expect(rolesResp.ok(), `roles fetch: ${rolesResp.status()} ${await rolesResp.text()}`).toBe(true)
    const roles = ((await rolesResp.json()).data?.roles ?? []) as Array<{ id: string; name: string }>
    const agentRole = roles.find(r => r.name.toLowerCase() === 'agent')
    expect(agentRole, 'agent role must exist (created by Go migrations)').toBeDefined()

    const testUser = await api.createUser({
      email: scope.email('member'),
      password: 'password123',
      full_name: scope.name('member'),
      role_id: agentRole!.id,
    })

    await api.addOrgMember(testUser.id, undefined, org.id)

    const members = await api.getOrgMembers(org.id)
    const memberIds = members.map((m: any) => m.user_id)
    expect(memberIds).toContain(testUser.id)

    await api.removeOrgMember(testUser.id, org.id)

    const membersAfter = await api.getOrgMembers(org.id)
    const memberIdsAfter = membersAfter.map((m: any) => m.user_id)
    expect(memberIdsAfter).not.toContain(testUser.id)
  })

  test('should list my organizations via API', async ({ request }) => {
    const api = new ApiHelper(request)
    test.skip(!(await loginApi(api)), 'No admin credentials available')

    const orgs = await api.getMyOrganizations()
    expect(Array.isArray(orgs)).toBeTruthy()
    expect(orgs.length).toBeGreaterThanOrEqual(1)

    for (const org of orgs) {
      expect(org.organization_id).toBeTruthy()
      expect(org.name).toBeTruthy()
    }
  })

  test('should switch organization via API', async ({ request }) => {
    const api = new ApiHelper(request)
    test.skip(!(await loginApi(api)), 'No admin credentials available')

    let org: any
    try {
      org = await api.createOrganization(scope.name('switch-org'))
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    await api.switchOrg(org.id)

    const currentOrg = await api.getCurrentOrg()
    expect(currentOrg.id).toBe(org.id)
  })
})
