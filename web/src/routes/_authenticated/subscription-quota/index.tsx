import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { SubscriptionQuotaLedger } from '@/features/subscription-quota-ledger'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/subscription-quota/')({
  validateSearch: z.object({
    view: z.enum(['overview', 'history']).catch('overview').optional(),
  }),
  // 从系统设置移出后仍保持原有超级管理员门槛，不能仅依赖侧栏隐藏。
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
      view={search.view ?? 'overview'}
      onViewChange={(view) => void navigate({ search: { view } })}
    />
  )
}
