import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { SubscriptionQuotaSettings } from '@/features/subscription-quota-ledger/settings-page'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute(
  '/_authenticated/system-settings/subscription-quota/'
)({
  validateSearch: z.object({
    view: z
      .enum(['overview', 'history', 'settings'])
      .catch('settings')
      .optional(),
  }),
  beforeLoad: ({ search }) => {
    if (useAuthStore.getState().auth.user?.role !== ROLE.SUPER_ADMIN) {
      throw redirect({ to: '/403' })
    }
    // 兼容旧书签；配置入口及旧settings链接留在系统设置。
    if (search.view === 'overview' || search.view === 'history') {
      throw redirect({
        to: '/subscription-quota',
        search: { view: search.view },
        replace: true,
      })
    }
  },
  component: SubscriptionQuotaSettings,
})
