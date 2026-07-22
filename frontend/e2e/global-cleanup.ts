import { Client } from 'pg'

/**
 * Wipe leftover E2E test data before the suite runs.
 *
 * Why: tests don't currently clean up after themselves (no per-test DB
 * isolation, CI gives a fresh DB but local dev runs accumulate). Stale
 * rows cause two problems:
 *   1. Strict-mode violations when a substring filter (e.g. picking
 *      role "Agent") matches both the system role and leftover rows
 *      named "E2E Transfer Agent 1777979885793" or
 *      "E2E-queue-pickup-XXX-pickup-agent-role".
 *   2. Slow-growing table sizes that make local list-view tests fragile.
 *
 * What we delete: anything whose name / email matches a known E2E prefix.
 * Patterns:
 *   - 'E2E-%'           — current framework prefix (createTestScope)
 *   - 'E2E %'           — legacy 'E2E Transfer Agent <ts>' style
 *   - 'e2e-%@e2e.test'  — current framework email
 *   - 'e2e-%@test.com'  — legacy generateUniqueEmail() before .e2e.test domain
 *   - 'e2e_%'           — templates seeded by SQL in template-sending.spec.ts
 *
 * Ordering: child tables first so FKs don't reject parent deletes. Within
 * each table we use a single statement; failures (FK violation, unknown
 * column, …) are logged and the cleanup continues — better to over-clean
 * than to leave half a state.
 */

const E2E_NAME_PREDICATE = `(name LIKE 'E2E-%' OR name LIKE 'E2E %')`
// @e2e.test is reserved for the framework's email factory — anything in
// that domain is safe to delete. Legacy generateUniqueEmail() landed on
// @test.com with an `e2e-` prefix; we match those too. Stable seeded users
// (admin@test.com, manager@test.com, agent@test.com) don't have the
// `e2e-` prefix so they're never touched.
const E2E_USER_EMAIL_PREDICATE = `(email LIKE '%@e2e.test' OR email LIKE 'e2e-%@test.com')`

// Statements run sequentially. Each is best-effort: a failure logs and
// the loop continues. Phrased as "DELETE ... USING <child>" or scoped to
// the prefix so stable rows (admin@test.com, system roles) are untouched.
const CLEANUP_STATEMENTS: Array<{ label: string; sql: string }> = [
  // Messages first — they reference contacts and users.
  {
    label: 'messages of E2E contacts',
    sql: `DELETE FROM messages WHERE contact_id IN (SELECT id FROM contacts WHERE profile_name LIKE 'E2E-%' OR profile_name LIKE 'E2E %')`,
  },
  // Notes are scoped to contact + user.
  {
    label: 'conversation_notes for E2E contacts',
    sql: `DELETE FROM conversation_notes WHERE contact_id IN (SELECT id FROM contacts WHERE profile_name LIKE 'E2E-%' OR profile_name LIKE 'E2E %')`,
  },
  // Agent transfers reference contacts + users + teams.
  {
    label: 'agent_transfers for E2E contacts',
    sql: `DELETE FROM agent_transfers WHERE contact_id IN (SELECT id FROM contacts WHERE profile_name LIKE 'E2E-%' OR profile_name LIKE 'E2E %')`,
  },
  {
    label: 'agent_transfers assigned to E2E users',
    sql: `DELETE FROM agent_transfers WHERE agent_id IN (SELECT id FROM users WHERE ${E2E_USER_EMAIL_PREDICATE})`,
  },
  // User org memberships first — covers both E2E users (so we can delete
  // them) and memberships pointing at E2E roles or orgs (so the role / org
  // delete below isn't blocked by FK).
  {
    label: 'user_organizations for E2E users',
    sql: `DELETE FROM user_organizations WHERE user_id IN (SELECT id FROM users WHERE ${E2E_USER_EMAIL_PREDICATE})`,
  },
  {
    label: 'user_organizations referencing E2E roles',
    sql: `DELETE FROM user_organizations WHERE role_id IN (SELECT id FROM custom_roles WHERE ${E2E_NAME_PREDICATE})`,
  },
  {
    label: 'user_organizations in E2E orgs',
    sql: `DELETE FROM user_organizations WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE})`,
  },
  {
    label: 'team_members for E2E users',
    sql: `DELETE FROM team_members WHERE user_id IN (SELECT id FROM users WHERE ${E2E_USER_EMAIL_PREDICATE})`,
  },
  // canned_responses created by an E2E user keep the user row pinned via
  // FK; nuke them first.
  {
    label: 'canned_responses authored by E2E users',
    sql: `DELETE FROM canned_responses WHERE created_by_id IN (SELECT id FROM users WHERE ${E2E_USER_EMAIL_PREDICATE})`,
  },
  // canned_responses authored by users in E2E orgs (covers users we'll
  // delete via the org-membership path below).
  {
    label: 'canned_responses authored by users in E2E orgs',
    sql: `DELETE FROM canned_responses WHERE created_by_id IN (SELECT id FROM users WHERE role_id IN (SELECT id FROM custom_roles WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE})))`,
  },
  // Now the entity tables.
  {
    label: 'E2E users (by email pattern)',
    sql: `DELETE FROM users WHERE ${E2E_USER_EMAIL_PREDICATE}`,
  },
  // Catch users in E2E orgs that didn't match the email predicate (e.g.
  // legacy `org1-admin-<ts>@test.com` pattern from older spec versions).
  // Match by their role pointing at a role in an E2E org.
  {
    label: 'users with roles in E2E orgs',
    sql: `DELETE FROM users WHERE role_id IN (SELECT id FROM custom_roles WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE}))`,
  },
  {
    label: 'role_permissions for E2E roles',
    sql: `DELETE FROM role_permissions WHERE custom_role_id IN (SELECT id FROM custom_roles WHERE ${E2E_NAME_PREDICATE})`,
  },
  {
    label: 'E2E custom roles',
    sql: `DELETE FROM custom_roles WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E contacts',
    sql: `DELETE FROM contacts WHERE profile_name LIKE 'E2E-%' OR profile_name LIKE 'E2E %'`,
  },
  {
    label: 'E2E teams',
    sql: `DELETE FROM teams WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E tags',
    sql: `DELETE FROM tags WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E keyword rules',
    sql: `DELETE FROM keyword_rules WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E AI contexts',
    sql: `DELETE FROM ai_contexts WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E custom actions',
    sql: `DELETE FROM custom_actions WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E webhooks',
    sql: `DELETE FROM webhooks WHERE ${E2E_NAME_PREDICATE}`,
  },
  {
    label: 'E2E templates',
    sql: `DELETE FROM templates WHERE name LIKE 'e2e_%' OR display_name LIKE 'E2E-%' OR display_name LIKE 'E2E %'`,
  },
  {
    label: 'E2E whatsapp_accounts',
    sql: `DELETE FROM whatsapp_accounts WHERE name LIKE 'e2e-%' OR name LIKE 'E2E-%'`,
  },
  {
    label: 'chatbot_settings for E2E orgs',
    sql: `DELETE FROM chatbot_settings WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE})`,
  },
  // custom_roles in E2E orgs may not match name-based cleanup (a default
  // role auto-created with the org carries the system name). Strip them
  // by org_id so the organization delete can succeed.
  {
    label: 'role_permissions for roles in E2E orgs',
    sql: `DELETE FROM role_permissions WHERE custom_role_id IN (SELECT id FROM custom_roles WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE}))`,
  },
  {
    label: 'custom_roles in E2E orgs',
    sql: `DELETE FROM custom_roles WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE})`,
  },
  {
    label: 'widgets in E2E orgs',
    sql: `DELETE FROM widgets WHERE organization_id IN (SELECT id FROM organizations WHERE ${E2E_NAME_PREDICATE})`,
  },
  {
    label: 'E2E organizations',
    sql: `DELETE FROM organizations WHERE ${E2E_NAME_PREDICATE}`,
  },
]

export async function cleanupE2EData(connectionString: string): Promise<void> {
  const client = new Client({ connectionString })
  try {
    await client.connect()
  } catch (err) {
    console.log(`  ⚠️  Skipping E2E cleanup — couldn't connect to DB: ${(err as Error).message}`)
    return
  }

  let totalDeleted = 0
  for (const { label, sql } of CLEANUP_STATEMENTS) {
    try {
      const result = await client.query(sql)
      const count = result.rowCount ?? 0
      if (count > 0) {
        console.log(`  🧹 Removed ${count} ${label}`)
      }
      totalDeleted += count
    } catch (err) {
      // Don't abort: an unknown column or FK glitch shouldn't block the
      // rest of the cleanup. Log so a curious dev sees what's left.
      console.log(`  ⚠️  Cleanup of ${label} failed: ${(err as Error).message}`)
    }
  }

  await client.end()

  if (totalDeleted === 0) {
    console.log('  ✨ No leftover E2E rows found')
  } else {
    console.log(`  ✨ Cleaned up ${totalDeleted} stale E2E rows`)
  }
}
