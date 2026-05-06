// Test data fixtures for E2E tests

import { randomBytes } from 'node:crypto'

export function generateUniqueEmail(prefix = 'test'): string {
  const timestamp = Date.now()
  // node:crypto satisfies CodeQL's js/insecure-randomness rule — these
  // values flow into API requests in the helpers below.
  const random = randomBytes(4).toString('hex').slice(0, 6)
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

export function createRoleFixture(overrides = {}) {
  return {
    name: generateUniqueName('Role'),
    description: 'Test role for E2E testing',
    permission_ids: [] as string[],
    ...overrides,
  }
}

export const RoleFixtures = {
  chatOnly: {
    name: generateUniqueName('Chat Only Role'),
    description: 'Role with only chat permissions',
    permissionFilters: [
      { resource: 'chat', action: 'read' },
      { resource: 'contacts', action: 'read' },
    ],
  },
  readOnly: {
    name: generateUniqueName('Read Only Role'),
    description: 'Role with read-only permissions',
    permissionFilters: [
      { resource: 'contacts', action: 'read' },
      { resource: 'templates', action: 'read' },
    ],
  },
}
