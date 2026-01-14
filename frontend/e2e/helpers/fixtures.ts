// Test data fixtures for E2E tests

export function generateUniqueEmail(prefix = 'test'): string {
  const timestamp = Date.now()
  const random = Math.random().toString(36).substring(7)
  return `${prefix}-${timestamp}-${random}@test.com`
}

export function generateUniqueName(prefix = 'Test'): string {
  const timestamp = Date.now()
  return `${prefix} ${timestamp}`
}

export const UserFixtures = {
  valid: {
    email: generateUniqueEmail('user'),
    fullName: 'Test User',
    role: 'agent',
    password: 'Password123!',
  },
  admin: {
    email: generateUniqueEmail('admin'),
    fullName: 'Test Admin',
    role: 'admin',
    password: 'Password123!',
  },
  manager: {
    email: generateUniqueEmail('manager'),
    fullName: 'Test Manager',
    role: 'manager',
    password: 'Password123!',
  },
}

export const TeamFixtures = {
  valid: {
    name: generateUniqueName('Team'),
    description: 'Test team description',
  },
}

export const WebhookFixtures = {
  valid: {
    name: generateUniqueName('Webhook'),
    url: 'https://webhook.site/test-endpoint',
    events: ['message.received', 'message.sent'],
  },
}

export const ContactFixtures = {
  valid: {
    name: 'Test Contact',
    phoneNumber: '+1234567890',
  },
}

// Factory functions for creating new fixtures on demand
export function createUserFixture(overrides = {}) {
  return {
    email: generateUniqueEmail('user'),
    fullName: generateUniqueName('User'),
    role: 'agent',
    password: 'Password123!',
    ...overrides,
  }
}

export function createTeamFixture(overrides = {}) {
  return {
    name: generateUniqueName('Team'),
    description: 'Test team description',
    ...overrides,
  }
}

export function createWebhookFixture(overrides = {}) {
  return {
    name: generateUniqueName('Webhook'),
    url: 'https://webhook.site/test-endpoint',
    events: ['message.received'],
    ...overrides,
  }
}
