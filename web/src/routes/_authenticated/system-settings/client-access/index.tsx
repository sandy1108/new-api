import { createFileRoute, redirect } from '@tanstack/react-router'

import { ClientAccessCredentials } from '@/features/client-access-credentials'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/system-settings/client-access/')({
  beforeLoad: () => {
    if (useAuthStore.getState().auth.user?.role !== ROLE.SUPER_ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: ClientAccessCredentials,
})
