import { request } from '@playwright/test'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

interface CreateUser {
  email: string
  password: string
  full_name: string
  role_name: string
}

/**
 * Extract the whm_csrf cookie value from Set-Cookie response headers.
 */
function extractCSRFToken(response: { headersArray: () => Array<{ name: string; value: string }> }): string | null {
  const cookieHeaders = response.headersArray().filter(h => h.name.toLowerCase() === 'set-cookie')
  for (const header of cookieHeaders) {
    const match = header.value.match(/whm_csrf=([^;]+)/)
    if (match) return match[1]
  }
  return null
}

async function globalSetup() {
  console.log('\n🔧 Global Setup: Creating test users...')

  const context = await request.newContext({
    baseURL: BASE_URL,
  })

  // Step 1: Login as the default superadmin (created by migrations)
  // This user has IsSuperAdmin=true and can create users in any org
  const defaultAdmin = {
    email: 'admin@admin.com',
    password: 'admin',
  }

  let csrfToken: string | null = null

  try {
    const loginResponse = await context.post('/api/auth/login', {
      data: defaultAdmin,
    })

    if (loginResponse.ok()) {
      // Auth cookies are auto-persisted by Playwright's APIRequestContext
      csrfToken = extractCSRFToken(loginResponse)
      console.log(`  ✅ Logged in as superadmin: ${defaultAdmin.email}`)
    } else {
      console.log(`  ❌ Failed to login as superadmin: ${await loginResponse.text()}`)
      console.log(`  ℹ️  Make sure migrations have run (./whatomate server -migrate)`)
    }
  } catch (error) {
    console.log(`  ❌ Error logging in as superadmin:`, error)
  }

  // Step 2: Get the roles to find admin, manager and agent role IDs
  const roleIds: Record<string, string> = {}

  try {
    // GET requests don't need CSRF token — cookies auto-sent by Playwright
    const rolesResponse = await context.get('/api/roles')

    if (rolesResponse.ok()) {
      const data = await rolesResponse.json()
      const roles = data.data?.roles || []
      for (const role of roles) {
        roleIds[role.name] = role.id
      }
      console.log(`  ✅ Found roles: ${Object.keys(roleIds).join(', ')}`)
    } else {
      console.log(`  ⚠️  Could not fetch roles: ${rolesResponse.status()}`)
    }
  } catch (error) {
    console.log(`  ⚠️  Error fetching roles:`, error)
  }

  // Step 3: Create test users in the default organization
  const usersToCreate: CreateUser[] = [
    { email: 'admin@test.com', password: 'password', full_name: 'Test Admin', role_name: 'admin' },
    { email: 'manager@test.com', password: 'password', full_name: 'Test Manager', role_name: 'manager' },
    { email: 'agent@test.com', password: 'password', full_name: 'Test Agent', role_name: 'agent' },
  ]

  // Get existing users to check for duplicates
  let existingEmails: Set<string> = new Set()
  try {
    const listResponse = await context.get('/api/users')
    if (listResponse.ok()) {
      const data = await listResponse.json()
      const users = data.data?.users || []
      existingEmails = new Set(users.map((u: { email: string }) => u.email))
    }
  } catch (error) {
    console.log(`  ⚠️  Error fetching existing users:`, error)
  }

  const csrfHeaders: Record<string, string> = csrfToken ? { 'X-CSRF-Token': csrfToken } : {}

  for (const user of usersToCreate) {
    if (existingEmails.has(user.email)) {
      console.log(`  ⏭️  User already exists: ${user.email}`)
      continue
    }

    try {
      const roleId = roleIds[user.role_name] || null

      const createResponse = await context.post('/api/users', {
        headers: csrfHeaders,
        data: {
          email: user.email,
          password: user.password,
          full_name: user.full_name,
          role_id: roleId,
          is_active: true,
        },
      })

      if (createResponse.ok()) {
        console.log(`  ✅ Created user: ${user.email} (${user.role_name})`)
      } else {
        const body = await createResponse.text()
        if (body.includes('already') || createResponse.status() === 409) {
          console.log(`  ⏭️  User already exists: ${user.email}`)
        } else {
          console.log(`  ⚠️  Could not create ${user.email}: ${createResponse.status()} - ${body}`)
        }
      }
    } catch (error) {
      console.log(`  ❌ Error creating ${user.email}:`, error)
    }
  }

  await context.dispose()
  console.log('🔧 Global Setup: Complete\n')
}

export default globalSetup
