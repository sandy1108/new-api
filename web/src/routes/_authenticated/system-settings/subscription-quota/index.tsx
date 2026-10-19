import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { SubscriptionQuotaLedger } from '@/features/subscription-quota-ledger'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute(
  '/_authenticated/system-settings/subscription-quota/'
)({
  validateSearch: z.object({
    view: z.enum(['overview', 'history', 'settings']).catch('overview'),
  }),
  beforeLoad: () => {
    if (useAuthStore.getState().auth.user?.role !== ROLE.SUPER_ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: QuotaRoute,
})

function QuotaRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <SubscriptionQuotaLedger
      view={search.view}
      onViewChange={(view) => void navigate({ search: { view } })}
    />
  )
}
