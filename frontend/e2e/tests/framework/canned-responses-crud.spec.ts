import { test } from '@playwright/test'
import { ApiHelper } from '../../helpers'
import {
  createTestScope,
  createUserWithPermissions,
  listLoadsBody,
  createFlowBody,
  type TestUserHandle,
} from '../../framework'

/**
 * Canned-responses CRUD coverage assembled from framework primitives.
 * Demonstrates the dialog-based CRUD pattern (open dialog → fill fields →
 * submit → row appears).
 */
const scope = createTestScope('canned-responses-crud')

test.describe.configure({ mode: 'serial' })

test.describe('Canned Responses CRUD', () => {
  let api: ApiHelper
  let admin: TestUserHandle
  const newResponseName = scope.name('greeting')

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    admin = await createUserWithPermissions(api, scope, {
      userSlug: 'admin',
      permissions: [
        { resource: 'canned_responses', action: 'read' },
        { resource: 'canned_responses', action: 'write' },
        { resource: 'canned_responses', action: 'delete' },
      ],
    })
  })

  test.afterAll(async () => {
    await api.deleteUser(admin.user.id).catch(() => {})
    await api.deleteRole(admin.role.id).catch(() => {})
  })

  test('list loads for permitted user', listLoadsBody({
    url: '/settings/canned-responses',
    user: () => admin,
    addButton: /Add Response/i,
  }))

  test('admin can create a canned response', createFlowBody({
    url: '/settings/canned-responses',
    user: () => admin,
    addButton: /Add Response/i,
    fields: [
      // Label component in shadcn-vue isn't `for`-linked, so use placeholder.
      { placeholder: /Welcome Message/i, value: newResponseName },
      { placeholder: /Hello \{contact_name\}/i, value: 'Hi from the framework demo' },
    ],
    expectRow: newResponseName,
  }))
})
