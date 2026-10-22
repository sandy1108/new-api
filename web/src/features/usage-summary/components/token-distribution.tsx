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
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { useUsageFormat } from '../hooks/use-usage-format'
import type { UsageTokenBucket } from '../types'

export interface TokenDistributionProps {
  tokens: UsageTokenBucket[]
  totalTokens: number
}

export function TokenDistribution(props: TokenDistributionProps) {
  const { t } = useTranslation()
  const format = useUsageFormat()
  const total = Number.isFinite(props.totalTokens)
    ? Math.max(0, props.totalTokens)
    : 0
  return (
    <Card
      size='sm'
      className='h-full min-w-0'
      data-testid='usage-summary-distribution'
    >
      <CardHeader className='flex flex-wrap items-center justify-between gap-2'>
        <CardTitle>
          <h3>{t('API Token Distribution')}</h3>
        </CardTitle>
        <span className='text-muted-foreground text-xs'>
          {t('Share of total tokens by API token')}
        </span>
      </CardHeader>
      <CardContent className='space-y-4'>
        {props.tokens.length === 0 ? (
          <p className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
            {t('No token distribution in this range.')}
          </p>
        ) : (
          props.tokens.map((token) => {
            const percent =
              total > 0
                ? Math.min(100, Math.max(0, (token.totalTokens / total) * 100))
                : 0
            return (
              <div key={token.key}>
                <div className='mb-1.5 flex flex-wrap justify-between gap-x-3 gap-y-1 text-xs'>
                  <span className='min-w-0 font-medium break-all'>
                    {token.name}
                  </span>
                  <span
                    className='text-muted-foreground tabular-nums'
                    title={format.number(token.totalTokens)}
                  >
                    {new Intl.NumberFormat(format.locale, {
                      style: 'percent',
                      maximumFractionDigits: 1,
                    }).format(percent / 100)}{' '}
                    · {format.compact(token.totalTokens)}
                  </span>
                </div>
                <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
                  <div
                    className='bg-chart-1 h-full rounded-full'
                    style={{ width: `${percent}%` }}
                  />
                </div>
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}
