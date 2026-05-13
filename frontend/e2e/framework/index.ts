/**
 * E2E framework public API.
 *
 * The new framework lives alongside the existing helpers/ and pages/
 * directories. Specs migrate over time; existing specs keep using the
 * older helpers until they're rewritten.
 *
 * Defaults this framework codifies:
 *   - UI-driven (`{ page }` fixture). API-only is for behaviors with no
 *     UI surface — see ARCHITECTURE.md.
 *   - Per-spec scope prefix on every name / email so test artifacts are
 *     identifiable and don't collide with each other or production-shaped
 *     data.
 *   - Login as the super admin (admin@admin.com) by default so tests
 *     don't silently land on a permission-denied page.
 *   - One-call permission-scoped user creation.
 */

export { createTestScope, type TestScope } from './scope'
export {
  loginAs,
  loginAsSuperAdmin,
  SUPER_ADMIN,
  type Credentials,
} from './auth'
export {
  createUserWithPermissions,
  type CreateUserOptions,
  type TestUserHandle,
  type PermissionRef,
} from './users'
export {
  listLoadsBody,
  expectAddButtonHidden,
  createFlowBody,
  editFlowBody,
  deleteFlowBody,
  searchListBody,
  type UserRef,
  type FieldFill,
} from './crud'
