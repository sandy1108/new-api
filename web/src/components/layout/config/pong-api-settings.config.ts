import type { TFunction } from 'i18next'
import { KeyRound, ChartNoAxesCombined } from 'lucide-react'

import { ROLE } from '@/lib/roles'

import type { NavGroup } from '../types'

// 私有设置独立分组，旧凭据URL保持不变；令牌用量统计不移入这里。
export function pongAPISettingsGroup(t: TFunction, role: number): NavGroup[] {
  if (role < ROLE.SUPER_ADMIN) return []
  return [
    {
      id: 'pong-api-settings',
      title: t('PongAPI custom settings'),
      items: [
        {
          title: t('Client access credentials'),
          url: '/system-settings/client-access',
          icon: KeyRound,
          requiredRole: ROLE.SUPER_ADMIN,
        },
        {
          title: t('Subscription quota ledger'),
          url: '/system-settings/subscription-quota',
          icon: ChartNoAxesCombined,
          requiredRole: ROLE.SUPER_ADMIN,
        },
      ],
    },
  ]
}
