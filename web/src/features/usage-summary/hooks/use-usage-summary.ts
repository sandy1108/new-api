/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

import { useQueryClient, useQuery } from '@tanstack/react-query'
import { CanceledError } from 'axios'

import {
  useReportAccess,
  reportIdentity,
  reportReadFailure,
} from '@/features/report-access/hooks'
import { useAuthStore } from '@/stores/auth-store'

import { getUsageSummary } from '../api'
import { USAGE_SUMMARY_STALE_TIME } from '../constants'
import type { UsageSummaryRange, UsageSummaryScope } from '../types'

export function useUsageSummary(
  range: UsageSummaryRange,
  requestedScope: UsageSummaryScope
) {
  const user = useAuthStore((state) => state.auth.user)
  const access = useReportAccess()
  const client = useQueryClient()
  const canManageScope = access.canReadUsageAll
  const scope: UsageSummaryScope =
    canManageScope && requestedScope === 'all' ? 'all' : 'self'

  const query = useQuery({
    queryKey: [
      'pong-report-data',
      access.identity,
      access.revision,
      'usage-summary',
      scope,
      range.id,
      range.startTimestamp,
      range.endTimestamp,
    ],
    queryFn: async ({ signal }) => {
      try {
        const result = await getUsageSummary({
          scope,
          range,
          observer: access.canReadUsageAll,
          signal,
        })
        if (reportIdentity() !== access.identity) throw new CanceledError()
        return result
      } catch (error) {
        if (reportIdentity() !== access.identity) throw new CanceledError()
        return reportReadFailure(client, error)
      }
    },
    enabled: Boolean(user) && !access.isPending,
    staleTime: USAGE_SUMMARY_STALE_TIME,
    refetchOnWindowFocus: false,
    retry: false,
  })

  return {
    ...query,
    isLoading: query.isLoading || access.isPending,
    data: !query.isError && !access.isPending ? query.data : undefined,
    scope,
    canManageScope,
  }
}
