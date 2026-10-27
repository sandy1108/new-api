import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { getReportCapabilities } from '@/features/report-access/api'
import { SubscriptionQuotaLedger } from '@/features/subscription-quota-ledger'

export const Route = createFileRoute('/_authenticated/subscription-quota/')({
  validateSearch: z.object({
    view: z.enum(['overview', 'history']).catch('overview').optional(),
  }),
  // 独立报表能力只开放此页；系统设置父路由仍保持Root门槛。
  beforeLoad: async () => {
    try {
      if ((await getReportCapabilities()).subscription_read_all) return
    } catch {
      /* 能力不可用时关闭入口，绝不回退管理员身份。 */
    }
    throw redirect({ to: '/403' })
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
