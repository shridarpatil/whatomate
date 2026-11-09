import type { LocationQuery } from 'vue-router'
import type { OAuthConsentRequest } from '@/services/api'

/** OAuth authorize parameters forwarded by the backend to the consent page. */
export interface ConsentParams {
  client_id: string
  redirect_uri: string
  state: string
  code_challenge: string
  code_challenge_method: string
  scope?: string
  resource?: string
}

export interface ConsentOrganization {
  organization_id: string
  name: string
  is_default?: boolean
}

function first(value: LocationQuery[string] | undefined): string {
  const v = Array.isArray(value) ? value[0] : value
  return typeof v === 'string' ? v : ''
}

/** Reads the consent params from the route query; null when a required one is missing. */
export function readConsentParams(query: LocationQuery): ConsentParams | null {
  const params: ConsentParams = {
    client_id: first(query.client_id),
    redirect_uri: first(query.redirect_uri),
    state: first(query.state),
    code_challenge: first(query.code_challenge),
    code_challenge_method: first(query.code_challenge_method),
  }
  if (!params.client_id || !params.redirect_uri || !params.code_challenge || !params.code_challenge_method) {
    return null
  }
  const scope = first(query.scope)
  const resource = first(query.resource)
  if (scope) params.scope = scope
  if (resource) params.resource = resource
  return params
}

export function buildConsentBody(
  params: ConsentParams,
  organizationId: string,
  approve: boolean,
): OAuthConsentRequest {
  return { ...params, organization_id: organizationId, approve }
}

/** Picks the organisation to preselect: the active one, then the default, then the first. */
export function pickInitialOrganization(
  orgs: ConsentOrganization[],
  preferredIds: Array<string | null | undefined>,
): string {
  for (const id of preferredIds) {
    if (id && orgs.some(o => o.organization_id === id)) return id
  }
  return orgs.find(o => o.is_default)?.organization_id || orgs[0]?.organization_id || ''
}

/** Full-page navigation back to the AI app (kept separate so tests can stub it). */
export function navigateTo(url: string) {
  window.location.href = url
}
