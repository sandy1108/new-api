import { createFileRoute, redirect } from '@tanstack/react-router'

import { ReportAccessSettings } from '@/features/report-access'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute(
  '/_authenticated/system-settings/report-access/'
)({
  beforeLoad: () => {
    if (useAuthStore.getState().auth.user?.role !== ROLE.SUPER_ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: ReportAccessSettings,
})
