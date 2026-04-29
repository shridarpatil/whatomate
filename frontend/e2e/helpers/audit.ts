import { expect, type APIRequestContext } from '@playwright/test'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

export interface AuditLogEntry {
  id: string
  resource_type: string
  resource_id: string
  user_id: string
  user_name: string
  action: 'created' | 'updated' | 'deleted'
  changes: Array<{ field: string; old_value: unknown; new_value: unknown }>
  created_at: string
}

interface AuditListResponse {
  audit_logs: AuditLogEntry[]
  total: number
}

interface VerifyOpts {
  /** How long to keep polling for the entry (ms). LogAudit is async (goroutine in
   *  the production code), so a short retry window is safest. */
  timeoutMs?: number
  /** Subset of fields the changes array must mention (matched by field name). */
  expectedFields?: string[]
}

/** Polls /api/audit-logs for an entry matching the (resource_type, resource_id, action)
 *  triple. Asserts that one is found within the timeout — fails the test otherwise. */
export async function verifyAuditLogged(
  request: APIRequestContext,
  resourceType: string,
  resourceID: string,
  action: 'created' | 'updated' | 'deleted',
  opts: VerifyOpts = {},
): Promise<AuditLogEntry> {
  const timeoutMs = opts.timeoutMs ?? 3000
  const deadline = Date.now() + timeoutMs

  let lastResponseText = ''
  let lastEntries: AuditLogEntry[] = []

  while (Date.now() < deadline) {
    const params = new URLSearchParams({
      resource_type: resourceType,
      resource_id: resourceID,
      action,
      limit: '50',
    })
    const response = await request.get(`${BASE_URL}/api/audit-logs?${params.toString()}`)

    if (response.ok()) {
      const body = (await response.json()) as { data: AuditListResponse }
      lastEntries = body.data.audit_logs ?? []
      const match = lastEntries.find(
        (e) => e.resource_type === resourceType && e.resource_id === resourceID && e.action === action,
      )
      if (match) {
        if (opts.expectedFields && opts.expectedFields.length > 0) {
          const changedFields = new Set(match.changes.map((c) => c.field))
          for (const f of opts.expectedFields) {
            expect(
              changedFields.has(f),
              `audit log for ${resourceType}/${resourceID} (${action}) should record a change to "${f}". Recorded fields: ${[...changedFields].join(', ') || '(none)'}`,
            ).toBe(true)
          }
        }
        return match
      }
    } else {
      lastResponseText = await response.text()
    }

    await new Promise((r) => setTimeout(r, 100))
  }

  throw new Error(
    `verifyAuditLogged: no audit log entry found for resource_type=${resourceType} resource_id=${resourceID} action=${action} within ${timeoutMs}ms.\n` +
      `Last entries returned (${lastEntries.length}):\n${JSON.stringify(lastEntries, null, 2)}\n` +
      (lastResponseText ? `Last error response body: ${lastResponseText}` : ''),
  )
}

/** Like verifyAuditLogged but returns null instead of throwing — for "should NOT have logged" assertions. */
export async function findAuditEntry(
  request: APIRequestContext,
  resourceType: string,
  resourceID: string,
  action: 'created' | 'updated' | 'deleted',
): Promise<AuditLogEntry | null> {
  const params = new URLSearchParams({
    resource_type: resourceType,
    resource_id: resourceID,
    action,
    limit: '50',
  })
  const response = await request.get(`${BASE_URL}/api/audit-logs?${params.toString()}`)
  if (!response.ok()) return null
  const body = (await response.json()) as { data: AuditListResponse }
  return body.data.audit_logs?.find((e) => e.action === action) ?? null
}
