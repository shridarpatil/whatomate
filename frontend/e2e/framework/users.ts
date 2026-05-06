/**
 * Permission-scoped test user creation.
 *
 * Replaces the verbose role-then-user-then-cleanup dance that was
 * duplicated across permission-sensitive specs. One call returns the
 * credentials and IDs you need to log in and assert.
 *
 *   const handle = await createUserWithPermissions(api, scope, {
 *     permissions: [
 *       { resource: 'transfers', action: 'read' },
 *       { resource: 'transfers', action: 'write' },
 *     ],
 *   })
 *   await loginAs(page, handle)
 */

import type { ApiHelper } from '../helpers/api'
import type { TestScope } from './scope'

export interface PermissionRef {
  resource: string
  action: string
}

export interface CreateUserOptions {
  permissions: PermissionRef[]
  /** Override the auto-generated role name. Default: scope.name('role'). */
  roleName?: string
  /** Override the auto-generated email local part. Default: random. */
  userSlug?: string
  /** Defaults to a strong test password. */
  password?: string
}

export interface TestUserHandle {
  /** User row created. */
  user: { id: string; email: string }
  /** Custom role created with exactly the requested permissions. */
  role: { id: string; name: string }
  /** Convenience for `loginAs(page, handle)`. */
  email: string
  password: string
}

export async function createUserWithPermissions(
  api: ApiHelper,
  scope: TestScope,
  opts: CreateUserOptions,
): Promise<TestUserHandle> {
  const password = opts.password ?? 'Password123!'
  // Default role name derives from userSlug when present so two callers
  // sharing one scope get distinct roles. Falls back to a random suffix.
  const roleName = opts.roleName ?? scope.name(opts.userSlug ? `${opts.userSlug}-role` : undefined)
  // Always randomise the email even when a slug is present. Rationale: a
  // beforeAll in a describe re-runs on flaky-test retries within the same
  // worker, but the scope's runId is fixed at module load — so a slug-based
  // email would collide with the row created on the previous attempt.
  // Random emails dodge that without breaking the (slug-readable) role name.
  const email = scope.email()

  const permissionIds = await api.findPermissionKeys(opts.permissions)

  const role = await api.createRole({
    name: roleName,
    description: `Test role for ${scope.prefix}`,
    permissions: permissionIds,
  })

  const user = await api.createUser({
    email,
    password,
    full_name: scope.name(opts.userSlug ?? 'user'),
    role_id: role.id,
  })

  return {
    user: { id: user.id, email: user.email },
    role: { id: role.id, name: role.name },
    email,
    password,
  }
}
