import { request } from '@playwright/test'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

interface TestUser {
  email: string
  password: string
  full_name: string
}

const TEST_USERS: TestUser[] = [
  { email: 'admin@test.com', password: 'password', full_name: 'Test Admin' },
  { email: 'manager@test.com', password: 'password', full_name: 'Test Manager' },
  { email: 'agent@test.com', password: 'password', full_name: 'Test Agent' },
]

async function globalSetup() {
  console.log('\n🔧 Global Setup: Creating test users...')

  const context = await request.newContext({
    baseURL: BASE_URL,
  })

  for (const user of TEST_USERS) {
    try {
      // Try to register the user
      const response = await context.post('/api/auth/register', {
        data: {
          email: user.email,
          password: user.password,
          full_name: user.full_name,
        },
      })

      if (response.ok()) {
        console.log(`  ✅ Created user: ${user.email}`)
      } else {
        const body = await response.text()
        // User might already exist, that's ok
        if (body.includes('already exists') || body.includes('duplicate') || response.status() === 409) {
          console.log(`  ⏭️  User already exists: ${user.email}`)
        } else {
          console.log(`  ⚠️  Could not create ${user.email}: ${response.status()} - ${body}`)
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
