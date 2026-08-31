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
import {
  Activity,
  ArrowDownToLine,
  ArrowUpToLine,
  Coins,
  Hash,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent } from '@/components/ui/card'

import { formatUsageNumber, formatUsageQuota } from '../lib/format'
import type { UsageMetrics } from '../types'

const metricCards = [
  { key: 'requests', labelKey: 'Requests', icon: Activity },
  { key: 'inputTokens', labelKey: 'Input Tokens', icon: ArrowDownToLine },
  { key: 'outputTokens', labelKey: 'Output Tokens', icon: ArrowUpToLine },
  { key: 'totalTokens', labelKey: 'Total Tokens', icon: Hash },
  { key: 'quota', labelKey: 'Quota', icon: Coins },
] as const

export interface UsageSummaryCardsProps {
  totals: UsageMetrics
}

export function UsageSummaryCards(props: UsageSummaryCardsProps) {
  const { t } = useTranslation()

  return (
    <div
      className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'
      data-testid='usage-summary-cards'
    >
      {metricCards.map((metric) => {
        const Icon = metric.icon
        const value =
          metric.key === 'quota'
            ? formatUsageQuota(props.totals.quota)
            : formatUsageNumber(props.totals[metric.key])

        return (
          <Card key={metric.key} size='sm'>
            <CardContent className='flex items-center gap-3 p-3'>
              <span className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg'>
                <Icon className='size-4' aria-hidden='true' />
              </span>
              <div className='min-w-0'>
                <p className='text-muted-foreground truncate text-xs'>
                  {t(metric.labelKey)}
                </p>
                <p className='text-foreground mt-0.5 truncate font-mono text-lg font-semibold tabular-nums'>
                  {value}
                </p>
              </div>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
