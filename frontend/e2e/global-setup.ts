import { request } from '@playwright/test'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

interface RegisterUser {
  email: string
  password: string
  full_name: string
  organization_name: string
}

interface CreateUser {
  email: string
  password: string
  full_name: string
  role_name: string
}

async function globalSetup() {
  console.log('\n🔧 Global Setup: Creating test users...')

  const context = await request.newContext({
    baseURL: BASE_URL,
  })

  // Step 1: Register the admin user (creates organization + admin role)
  const adminUser: RegisterUser = {
    email: 'admin@test.com',
    password: 'password',
    full_name: 'Test Admin',
    organization_name: 'Test Organization',
  }

  let accessToken: string | null = null

  try {
    // Try to register admin
    const registerResponse = await context.post('/api/auth/register', {
      data: adminUser,
    })

    if (registerResponse.ok()) {
      const data = await registerResponse.json()
      accessToken = data.data?.access_token
      console.log(`  ✅ Created admin user: ${adminUser.email}`)
    } else {
      const body = await registerResponse.text()
      if (body.includes('already') || registerResponse.status() === 409) {
        console.log(`  ⏭️  Admin already exists, logging in...`)
        // Login to get token
        const loginResponse = await context.post('/api/auth/login', {
          data: { email: adminUser.email, password: adminUser.password },
        })
        if (loginResponse.ok()) {
          const data = await loginResponse.json()
          accessToken = data.data?.access_token
          console.log(`  ✅ Logged in as admin: ${adminUser.email}`)
        } else {
          console.log(`  ❌ Failed to login as admin: ${await loginResponse.text()}`)
        }
      } else {
        console.log(`  ⚠️  Could not create admin: ${registerResponse.status()} - ${body}`)
      }
    }
  } catch (error) {
    console.log(`  ❌ Error creating admin:`, error)
  }

  if (!accessToken) {
    console.log('  ❌ No access token, cannot create other users')
    await context.dispose()
    return
  }

  // Step 2: Get the roles to find manager and agent role IDs
  let managerRoleId: string | null = null
  let agentRoleId: string | null = null

  try {
    const rolesResponse = await context.get('/api/roles', {
      headers: { Authorization: `Bearer ${accessToken}` },
    })

    if (rolesResponse.ok()) {
      const data = await rolesResponse.json()
      const roles = data.data?.roles || []
      for (const role of roles) {
        if (role.name === 'manager') managerRoleId = role.id
        if (role.name === 'agent') agentRoleId = role.id
      }
      console.log(`  ✅ Found roles - manager: ${managerRoleId ? 'yes' : 'no'}, agent: ${agentRoleId ? 'yes' : 'no'}`)
    } else {
      console.log(`  ⚠️  Could not fetch roles: ${rolesResponse.status()}`)
    }
  } catch (error) {
    console.log(`  ⚠️  Error fetching roles:`, error)
  }

  // Step 3: Create manager and agent users in the same organization
  const usersToCreate: CreateUser[] = [
    { email: 'manager@test.com', password: 'password', full_name: 'Test Manager', role_name: 'manager' },
    { email: 'agent@test.com', password: 'password', full_name: 'Test Agent', role_name: 'agent' },
  ]

  for (const user of usersToCreate) {
    try {
      // First check if user already exists
      const listResponse = await context.get('/api/users', {
        headers: { Authorization: `Bearer ${accessToken}` },
      })

      let userExists = false
      if (listResponse.ok()) {
        const data = await listResponse.json()
        const users = data.data?.users || []
        userExists = users.some((u: { email: string }) => u.email === user.email)
      }

      if (userExists) {
        console.log(`  ⏭️  User already exists: ${user.email}`)
        continue
      }

      // Get role ID
      let roleId: string | null = null
      if (user.role_name === 'manager') roleId = managerRoleId
      if (user.role_name === 'agent') roleId = agentRoleId

      // Create user via users API
      const createResponse = await context.post('/api/users', {
        headers: { Authorization: `Bearer ${accessToken}` },
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
