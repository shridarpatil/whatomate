/**
 * Per-spec scoping primitive.
 *
 * Each spec creates a TestScope at module load — every name, email and
 * phone derived from it includes a stable prefix unique to this run.
 *
 * Why: with no automatic per-test cleanup, test artifacts persist in the
 * dev database. Prefixed names mean (a) tests within a single run can't
 * collide on uniqueness, (b) leftover rows are identifiable as test data
 * if a dev wants to clean their local DB, (c) tests that filter / list
 * by name don't accidentally match production-shaped data.
 *
 *   const scope = createTestScope('users')
 *   scope.prefix    // 'E2E-users-mh3p2x'
 *   scope.name()    // 'E2E-users-mh3p2x-a3f9b1'
 *   scope.name('admin')  // 'E2E-users-mh3p2x-admin'
 *   scope.email('agent') // 'e2e-users-mh3p2x-agent@e2e.test'
 *   scope.phone()        // '911745236847123'
 */

import { randomBytes, randomInt } from 'node:crypto'

export interface TestScope {
  /** Stable identifier for everything this spec creates. */
  readonly prefix: string
  /** Generates a unique name; `suffix` is appended if provided, otherwise random. */
  name(suffix?: string): string
  /** Generates a unique email under @e2e.test. */
  email(suffix?: string): string
  /** Generates a unique phone-shaped string (no SMS sent in test runs). */
  phone(): string
}

export function createTestScope(specName: string): TestScope {
  const safe = specName.replace(/[^a-zA-Z0-9-]/g, '-').toLowerCase()
  const runId = Date.now().toString(36)
  const prefix = `E2E-${safe}-${runId}`

  return {
    prefix,
    name(suffix) {
      return suffix ? `${prefix}-${suffix}` : `${prefix}-${randomSuffix()}`
    },
    email(suffix) {
      const local = suffix ?? randomSuffix()
      return `${prefix.toLowerCase()}-${local}@e2e.test`
    },
    phone() {
      // 91 (country) + 10-digit local suffix derived from time + random.
      // node:crypto satisfies CodeQL's js/insecure-randomness — these values
      // flow into API calls so the linter treats them as security-sensitive
      // even though the phones are throwaway test data.
      const suffix = randomInt(0, 1000).toString().padStart(3, '0')
      return `91${Date.now().toString().slice(-7)}${suffix}`
    },
  }
}

function randomSuffix(): string {
  return randomBytes(4).toString('hex').slice(0, 6)
}
