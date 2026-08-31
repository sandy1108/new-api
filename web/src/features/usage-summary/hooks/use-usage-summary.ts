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

import { useQuery } from '@tanstack/react-query'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getUsageSummary } from '../api'
import { USAGE_SUMMARY_STALE_TIME } from '../constants'
import type { UsageSummaryRange, UsageSummaryScope } from '../types'

export function useUsageSummary(
  range: UsageSummaryRange,
  requestedScope: UsageSummaryScope
) {
  const user = useAuthStore((state) => state.auth.user)
  const canManageScope = (user?.role ?? ROLE.GUEST) >= ROLE.ADMIN
  const scope: UsageSummaryScope =
    canManageScope && requestedScope === 'all' ? 'all' : 'self'

  const query = useQuery({
    queryKey: [
      'usage-summary',
      scope,
      range.id,
      range.startTimestamp,
      range.endTimestamp,
    ],
    queryFn: () => getUsageSummary({ scope, range }),
    enabled: Boolean(user),
    staleTime: USAGE_SUMMARY_STALE_TIME,
    refetchOnWindowFocus: false,
    placeholderData: (previousData) => previousData,
  })

  return {
    ...query,
    scope,
    canManageScope,
  }
}
