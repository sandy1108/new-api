import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

export type ReportCapabilities = {
  usage_read_all: boolean
  subscription_read_all: boolean
  can_manage_grants: boolean
  revision: string
}
export type ReportGrant = {
  id: number
  username: string
  display_name: string
  usage_read_all: boolean
  subscription_read_all: boolean
  version: number
  invalidated: boolean
  updated_at: number
}
export type GrantChange = Pick<
  ReportGrant,
  'usage_read_all' | 'subscription_read_all' | 'version'
>

export async function getReportCapabilities(
  signal?: AbortSignal
): Promise<ReportCapabilities> {
  const result = requireServerSuccess(
    (
      await api.get('/api/pong-reports/capabilities', {
        signal,
        skipErrorHandler: true,
      })
    ).data
  ).data as ReportCapabilities
  if (
    !result ||
    typeof result.usage_read_all !== 'boolean' ||
    typeof result.subscription_read_all !== 'boolean' ||
    typeof result.can_manage_grants !== 'boolean' ||
    typeof result.revision !== 'string'
  ) {
    throw new Error('Invalid report capabilities')
  }
  return result
}
export async function getReportGrants(
  page: number,
  search: string,
  signal?: AbortSignal
): Promise<{ items: ReportGrant[]; total: number }> {
  return requireServerSuccess(
    (
      await api.get('/api/pong-report-access/users', {
        params: { page, page_size: 20, search },
        signal,
      })
    ).data
  ).data
}
export async function saveReportGrant(
  id: number,
  input: GrantChange,
  proof: string
): Promise<{ version: number }> {
  return requireServerSuccess(
    (
      await api.put(`/api/pong-report-access/users/${id}`, input, {
        headers: { 'X-PongAPI-Request': '1', 'X-Security-Proof': proof },
        singleUseAuthorization: true,
      })
    ).data
  ).data
}
