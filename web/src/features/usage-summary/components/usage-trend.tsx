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
import { Activity } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'

import { formatUsageCompactNumber, formatUsageNumber } from '../lib/format'
import type { UsageSummaryTrendPoint } from '../types'

const trendDateFormatter = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
})

function toFiniteValue(value: number): number {
  return Number.isFinite(value) ? Math.max(0, value) : 0
}

function formatTrendDate(timestamp: number): string {
  const date = new Date(timestamp * 1000)
  return Number.isNaN(date.getTime()) ? '—' : trendDateFormatter.format(date)
}

export interface UsageTrendProps {
  points: UsageSummaryTrendPoint[]
}

export function UsageTrend(props: UsageTrendProps) {
  const { t } = useTranslation()
  const chart = useMemo(() => {
    const points = props.points.map((point) => ({
      ...point,
      input: toFiniteValue(point.input_tokens),
      output: toFiniteValue(point.output_tokens),
    }))
    const maxValue = Math.max(
      1,
      ...points.flatMap((point) => [point.input, point.output])
    )
    const peakDailyTotal = Math.max(
      0,
      ...points.map((point) => point.input + point.output)
    )

    return { points, maxValue, peakDailyTotal }
  }, [props.points])

  return (
    <Card
      size='sm'
      className='h-full overflow-hidden'
      data-testid='usage-summary-trend'
    >
      <CardHeader className='border-b p-3 sm:p-4'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <IconBadge tone='chart-1' size='sm'>
              <Activity />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='truncate text-sm sm:text-base'>
                {t('Usage Trend')}
              </CardTitle>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('Daily input and output tokens')}
              </p>
            </div>
          </div>
          <div className='flex flex-wrap items-center gap-3 text-xs'>
            <span className='text-muted-foreground inline-flex items-center gap-1.5'>
              <span className='bg-chart-1 size-1.5 rounded-sm' />
              {t('Input Tokens')}
            </span>
            <span className='text-muted-foreground inline-flex items-center gap-1.5'>
              <span className='bg-chart-2 size-1.5 rounded-sm' />
              {t('Output Tokens')}
            </span>
          </div>
        </div>
      </CardHeader>
      <CardContent className='p-3 sm:p-4'>
        {chart.points.length === 0 ? (
          <div className='text-muted-foreground flex h-48 items-center justify-center rounded-xl border border-dashed text-sm'>
            {t('No trend data in this range.')}
          </div>
        ) : (
          <div className='space-y-2'>
            <div
              className='bg-muted/20 flex h-48 gap-2 overflow-hidden rounded-xl p-2 sm:h-56 sm:gap-3 sm:p-3'
              role='img'
              aria-label={t('Daily input and output token trend')}
            >
              <div className='text-muted-foreground flex w-9 shrink-0 flex-col justify-between pt-1 pb-7 text-right text-[10px]'>
                {[chart.maxValue, chart.maxValue / 2, 0].map((value) => (
                  <span key={value}>{formatUsageCompactNumber(value)}</span>
                ))}
              </div>
              <div className='min-w-0 flex-1 overflow-x-auto'>
                <div className='relative flex h-full min-w-[420px] flex-col'>
                  <div className='pointer-events-none absolute inset-x-0 top-1 bottom-7 flex flex-col justify-between'>
                    {[0, 1, 2].map((line) => (
                      <span
                        key={line}
                        className='border-border/70 border-t border-dashed'
                      />
                    ))}
                  </div>
                  <div className='relative z-10 flex min-h-0 flex-1 items-end justify-between gap-1 px-1 pb-7 sm:gap-2'>
                    {chart.points.map((point) => {
                      const inputHeight =
                        point.input > 0
                          ? Math.max(3, (point.input / chart.maxValue) * 100)
                          : 0
                      const outputHeight =
                        point.output > 0
                          ? Math.max(3, (point.output / chart.maxValue) * 100)
                          : 0

                      return (
                        <div
                          key={point.timestamp}
                          className='flex h-full min-w-3 flex-1 items-end justify-center gap-0.5 sm:gap-1'
                        >
                          <span
                            className='bg-chart-1/85 max-w-5 min-w-1.5 rounded-t-sm shadow-[0_4px_9px_color-mix(in_oklch,var(--chart-1)_18%,transparent)]'
                            style={{ height: `${inputHeight}%` }}
                            data-testid={`usage-trend-input-${point.timestamp}`}
                            title={formatUsageNumber(point.input)}
                            aria-label={`${formatTrendDate(point.timestamp)} ${t('Input Tokens')}: ${formatUsageNumber(point.input)}`}
                          />
                          <span
                            className='bg-chart-2/75 max-w-5 min-w-1.5 rounded-t-sm'
                            style={{ height: `${outputHeight}%` }}
                            data-testid={`usage-trend-output-${point.timestamp}`}
                            title={formatUsageNumber(point.output)}
                            aria-label={`${formatTrendDate(point.timestamp)} ${t('Output Tokens')}: ${formatUsageNumber(point.output)}`}
                          />
                        </div>
                      )
                    })}
                  </div>
                  <div className='text-muted-foreground absolute inset-x-1 bottom-0 flex h-5 justify-between gap-1 overflow-hidden text-[10px]'>
                    {chart.points.map((point, index) => {
                      const shouldShowLabel =
                        chart.points.length <= 4 ||
                        index === 0 ||
                        index === Math.floor((chart.points.length - 1) / 2) ||
                        index === chart.points.length - 1
                      return (
                        <span
                          key={point.timestamp}
                          className='min-w-3 flex-1 truncate text-center'
                        >
                          {shouldShowLabel
                            ? formatTrendDate(point.timestamp)
                            : ''}
                        </span>
                      )
                    })}
                  </div>
                </div>
              </div>
            </div>
            <div className='text-muted-foreground flex flex-wrap justify-between gap-x-4 gap-y-1 text-[11px]'>
              <span>
                {t('Peak daily total')}:{' '}
                {formatUsageCompactNumber(chart.peakDailyTotal)}
              </span>
              <span>
                {t('Days shown')}: {chart.points.length}
              </span>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
