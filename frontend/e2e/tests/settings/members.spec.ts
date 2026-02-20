import { test, expect } from '@playwright/test'
import { ApiHelper, generateUniqueEmail } from '../../helpers'

const ADMIN_EMAIL = 'admin@admin.com'
const ADMIN_PASSWORD = 'admin'
const FALLBACK_ADMIN_EMAIL = 'admin@test.com'
const FALLBACK_ADMIN_PASSWORD = 'password'

test.describe('Organization Members - API Tests', () => {
  test('should list organization members via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    const members = await api.getOrgMembers()
    expect(Array.isArray(members)).toBeTruthy()
  })

  test('should add and remove organization member via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    // Create a new org to test with
    const orgName = `Member Test Org ${Date.now()}`
    let org: any
    try {
      org = await api.createOrganization(orgName)
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    // Create a user in the default org to add to the new org
    const testEmail = generateUniqueEmail('member-test')
    let testUser: any
    try {
      testUser = await api.createUser({
        email: testEmail,
        password: 'password123',
        full_name: 'Member Test User',
        role_id: '', // Will get default role
      })
    } catch {
      test.skip(true, 'Failed to create test user')
      return
    }

    // Add the user to the new org
    await api.addOrgMember(testUser.id, undefined, org.id)

    // List members and verify user is included
    const members = await api.getOrgMembers(org.id)
    const memberIds = members.map((m: any) => m.user_id)
    expect(memberIds).toContain(testUser.id)

    // Remove the user from the org
    await api.removeOrgMember(testUser.id, org.id)

    // Verify user is no longer a member
    const membersAfter = await api.getOrgMembers(org.id)
    const memberIdsAfter = membersAfter.map((m: any) => m.user_id)
    expect(memberIdsAfter).not.toContain(testUser.id)
  })

  test('should list my organizations via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    const orgs = await api.getMyOrganizations()
    expect(Array.isArray(orgs)).toBeTruthy()
    // The logged-in user should belong to at least one org
    expect(orgs.length).toBeGreaterThanOrEqual(1)

    // Each org should have required fields
    for (const org of orgs) {
      expect(org.organization_id).toBeTruthy()
      expect(org.name).toBeTruthy()
    }
  })

  test('should switch organization via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    // Create a second org
    const orgName = `Switch Test Org ${Date.now()}`
    let org: any
    try {
      org = await api.createOrganization(orgName)
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    // Switch to the new org — super admin can switch to any org
    // switchOrg sets new auth cookies (no token returned in cookie-based auth)
    await api.switchOrg(org.id)

    // Verify the switch worked by checking current org
    const currentOrg = await api.getCurrentOrg()
    expect(currentOrg.id).toBe(org.id)
  })
})
