import type { TFunction } from 'i18next'
import { ChartNoAxesCombined } from 'lucide-react'

import type { NavItem } from '../types'

// 私有控制台入口集中声明，减少同步官方侧栏代码时的改动面。
export function pongAPIConsoleItems(
  t: TFunction,
  canReadLedger: boolean
): NavItem[] {
  if (!canReadLedger) return []
  return [
    {
      title: t('Subscription quota ledger'),
      url: '/subscription-quota',
      icon: ChartNoAxesCombined,
    },
  ]
}
