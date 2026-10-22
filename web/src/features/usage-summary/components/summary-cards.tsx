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
import { Activity, Coins, Layers, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent } from '@/components/ui/card'

import { useUsageFormat } from '../hooks/use-usage-format'
import type { UsageMetrics } from '../types'

export interface UsageSummaryCardsProps {
  totals: UsageMetrics
}

export function UsageSummaryCards(props: UsageSummaryCardsProps) {
  const { t } = useTranslation()
  const format = useUsageFormat()
  const cost = format.cost(props.totals.quota)
  const metrics = [
    {
      key: 'tokens',
      label: t('Total Tokens'),
      icon: Layers,
      value: format.compact(props.totals.totalTokens),
      title: format.number(props.totals.totalTokens),
      detail: `${t('Input')}: ${format.compact(props.totals.inputTokens)} / ${t('Output')}: ${format.compact(props.totals.outputTokens)}`,
      detailTitle: `${t('Input')}: ${format.number(props.totals.inputTokens)} / ${t('Output')}: ${format.number(props.totals.outputTokens)}`,
    },
    {
      key: 'requests',
      label: t('Requests'),
      icon: Activity,
      value: format.number(props.totals.requests),
      title: format.number(props.totals.requests),
      detail: t('Total in selected range'),
    },
    {
      key: 'quota',
      label: t('Consumed quota'),
      icon: Coins,
      value: format.compact(props.totals.quota),
      title: format.number(props.totals.quota),
      detail: t('Internal units recorded in logs'),
    },
    {
      key: 'cost',
      label: t('Equivalent cost'),
      icon: Wallet,
      value: cost.cny ?? t('Unavailable'),
      title: cost.cny ?? t('Unavailable'),
      detail: `${cost.usd ?? t('Unavailable')} · ${t('Site conversion')}`,
    },
  ]

  return (
    <div
      className='grid shrink-0 grid-cols-2 gap-3 xl:grid-cols-4'
      data-testid='usage-summary-cards'
    >
      {metrics.map((metric) => (
        <Card
          key={metric.key}
          size='sm'
          className='min-w-0'
          role='group'
          aria-label={metric.label}
        >
          <CardContent>
            <div className='text-muted-foreground flex items-center gap-2 text-xs'>
              <metric.icon className='size-3.5 shrink-0' aria-hidden='true' />
              {metric.label}
            </div>
            <div
              className='mt-2 text-xl font-semibold tracking-tight break-words tabular-nums sm:text-2xl'
              title={metric.title}
            >
              {metric.value}
            </div>
            <p
              className='text-muted-foreground mt-1 text-xs'
              title={metric.detailTitle}
            >
              {metric.detail}
            </p>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
