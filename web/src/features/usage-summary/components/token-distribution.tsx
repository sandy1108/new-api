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
import { KeyRound } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'

import { formatUsageCompactNumber, formatUsageNumber } from '../lib/format'
import type { UsageTokenBucket } from '../types'

const DISTRIBUTION_TONES: IconBadgeTone[] = [
  'chart-1',
  'chart-2',
  'chart-3',
  'chart-4',
  'chart-5',
]

const DISTRIBUTION_COLORS: Record<IconBadgeTone, string> = {
  neutral: 'var(--muted-foreground)',
  primary: 'var(--primary)',
  success: 'var(--success)',
  warning: 'var(--warning)',
  info: 'var(--info)',
  destructive: 'var(--destructive)',
  'chart-1': 'var(--chart-1)',
  'chart-2': 'var(--chart-2)',
  'chart-3': 'var(--chart-3)',
  'chart-4': 'var(--chart-4)',
  'chart-5': 'var(--chart-5)',
}

export interface TokenDistributionProps {
  tokens: UsageTokenBucket[]
  totalTokens: number
}

function formatPercentage(value: number): string {
  if (value <= 0) return '0%'
  if (value < 1) return '<1%'
  return `${new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(value)}%`
}

export function TokenDistribution(props: TokenDistributionProps) {
  const { t } = useTranslation()
  const totalTokens = Number.isFinite(props.totalTokens)
    ? Math.max(0, props.totalTokens)
    : 0
  const distribution = useMemo(() => {
    const percentages = props.tokens.map((token) =>
      totalTokens > 0 ? (Math.max(0, token.totalTokens) / totalTokens) * 100 : 0
    )
    const cumulativePercentages = percentages.map((_, index) =>
      percentages.slice(0, index + 1).reduce((sum, value) => sum + value, 0)
    )

    return props.tokens.map((token, index) => {
      const percentage = percentages[index]
      const start = cumulativePercentages[index - 1] ?? 0
      return {
        token,
        percentage,
        tone: DISTRIBUTION_TONES[index % DISTRIBUTION_TONES.length],
        start,
        end: cumulativePercentages[index],
      }
    })
  }, [props.tokens, totalTokens])

  const donutBackground =
    distribution.length === 0 || totalTokens === 0
      ? 'var(--muted)'
      : `conic-gradient(${distribution
          .map(
            (item) =>
              `${DISTRIBUTION_COLORS[item.tone]} ${item.start}% ${item.end}%`
          )
          .join(', ')})`

  return (
    <Card
      size='sm'
      className='h-full overflow-hidden'
      data-testid='usage-summary-distribution'
    >
      <CardHeader className='border-b p-3 sm:p-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <IconBadge tone='chart-4' size='sm'>
              <KeyRound />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='truncate text-sm sm:text-base'>
                {t('API Token Distribution')}
              </CardTitle>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('Share of total tokens by API token')}
              </p>
            </div>
          </div>
          <span className='text-muted-foreground bg-muted/60 shrink-0 rounded-md px-2 py-1 font-mono text-[10px] font-medium tabular-nums'>
            {props.tokens.length} {t('Tokens')}
          </span>
        </div>
      </CardHeader>
      <CardContent className='p-3 sm:p-4'>
        {props.tokens.length === 0 ? (
          <div className='text-muted-foreground flex h-48 items-center justify-center rounded-xl border border-dashed text-sm'>
            {t('No token distribution in this range.')}
          </div>
        ) : (
          <div className='space-y-4'>
            <div className='flex flex-col items-center gap-4 sm:flex-row sm:items-center'>
              <div
                className='relative size-32 shrink-0 rounded-full p-4 shadow-inner sm:size-36'
                style={{ background: donutBackground }}
                role='img'
                aria-label={t('API Token Distribution')}
              >
                <div className='bg-card flex size-full flex-col items-center justify-center rounded-full text-center'>
                  <strong
                    className='text-foreground max-w-full truncate px-2 font-mono text-base font-semibold tabular-nums'
                    title={formatUsageNumber(totalTokens)}
                  >
                    {formatUsageCompactNumber(totalTokens)}
                  </strong>
                  <span className='text-muted-foreground mt-1 text-[10px]'>
                    {t('Total Tokens')}
                  </span>
                </div>
              </div>
              <div className='grid w-full min-w-0 gap-3'>
                {distribution.map((item) => (
                  <div
                    key={item.token.key}
                    className='grid min-w-0 grid-cols-[8px_minmax(0,1fr)_auto] items-center gap-2 text-xs'
                  >
                    <span
                      className='size-2 rounded-[3px]'
                      style={{
                        backgroundColor: DISTRIBUTION_COLORS[item.tone],
                      }}
                    />
                    <span
                      className='text-muted-foreground truncate font-medium'
                      title={item.token.name}
                    >
                      {item.token.name}
                    </span>
                    <span
                      className='text-foreground font-mono font-semibold tabular-nums'
                      title={formatUsageNumber(item.token.totalTokens)}
                    >
                      {formatPercentage(item.percentage)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
            <p className='text-muted-foreground text-[11px] leading-relaxed'>
              {t('Distribution is calculated at API token level only.')}
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
