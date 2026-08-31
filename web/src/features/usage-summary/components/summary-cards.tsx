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

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'

import {
  formatUsageCompactNumber,
  formatUsageNumber,
  formatUsageQuota,
} from '../lib/format'
import type { UsageMetrics } from '../types'

const metricCards = [
  {
    key: 'requests',
    labelKey: 'Requests',
    icon: Activity,
    iconTone: 'chart-1',
  },
  {
    key: 'inputTokens',
    labelKey: 'Input Tokens',
    icon: ArrowDownToLine,
    iconTone: 'chart-2',
  },
  {
    key: 'outputTokens',
    labelKey: 'Output Tokens',
    icon: ArrowUpToLine,
    iconTone: 'chart-3',
  },
  {
    key: 'totalTokens',
    labelKey: 'Total Tokens',
    icon: Hash,
    iconTone: 'chart-4',
  },
  {
    key: 'quota',
    labelKey: 'Quota',
    icon: Coins,
    iconTone: 'chart-5',
  },
] as const

export interface UsageSummaryCardsProps {
  totals: UsageMetrics
}

export function UsageSummaryCards(props: UsageSummaryCardsProps) {
  const { t } = useTranslation()

  return (
    <div
      className='bg-card overflow-hidden rounded-2xl border shadow-xs'
      data-testid='usage-summary-cards'
    >
      <div className='grid min-w-0 grid-cols-2 divide-x divide-y lg:grid-cols-5 lg:divide-y-0'>
        {metricCards.map((metric, index) => {
          const Icon = metric.icon
          const rawValue = props.totals[metric.key]
          const fullValue =
            metric.key === 'quota'
              ? formatUsageQuota(rawValue)
              : formatUsageNumber(rawValue)
          const value =
            metric.key === 'quota'
              ? fullValue
              : formatUsageCompactNumber(rawValue)

          return (
            <div
              key={metric.key}
              className={`group min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4 ${
                index === metricCards.length - 1
                  ? 'col-span-2 lg:col-span-1'
                  : ''
              }`}
            >
              <div className='flex min-w-0 items-center gap-1.5 sm:gap-2'>
                <IconBadge
                  tone={metric.iconTone as IconBadgeTone}
                  size='stat'
                  className='size-5 rounded-md sm:size-7 sm:rounded-lg [&>svg]:size-3 sm:[&>svg]:size-3.5'
                >
                  <Icon />
                </IconBadge>
                <div className='text-muted-foreground truncate text-[11px] leading-4 font-medium tracking-wide sm:text-xs sm:tracking-wider'>
                  {t(metric.labelKey)}
                </div>
              </div>
              <div
                className='text-foreground mt-2 truncate font-mono text-lg leading-tight font-semibold tracking-tight tabular-nums sm:mt-3 sm:text-2xl'
                title={fullValue}
              >
                {value}
              </div>
              <div className='text-muted-foreground/65 mt-1 truncate text-[10px] sm:text-xs'>
                {t('Raw value: {{value}}', { value: fullValue })}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
