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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { useUsageFormat } from '../hooks/use-usage-format'
import type { UsageSummaryTrendPoint } from '../types'

export interface UsageTrendProps {
  points: UsageSummaryTrendPoint[]
}

export function UsageTrend(props: UsageTrendProps) {
  const { t } = useTranslation()
  const format = useUsageFormat()
  const chart = useMemo(() => {
    const points = props.points.map((point) => {
      const input = Number.isFinite(point.input_tokens)
        ? Math.max(0, point.input_tokens)
        : 0
      const output = Number.isFinite(point.output_tokens)
        ? Math.max(0, point.output_tokens)
        : 0
      return { ...point, total: input + output }
    })
    return { points, max: Math.max(1, ...points.map((point) => point.total)) }
  }, [props.points])
  const dateFormat = new Intl.DateTimeFormat(format.locale, {
    month: 'numeric',
    day: 'numeric',
  })
  return (
    <Card
      size='sm'
      className='h-full min-w-0'
      data-testid='usage-summary-trend'
    >
      <CardHeader className='flex flex-wrap items-center justify-between gap-2'>
        <CardTitle>
          <h3>{t('Usage Trend')}</h3>
        </CardTitle>
        <span className='text-muted-foreground text-xs'>
          {t('Daily total tokens')}
        </span>
      </CardHeader>
      <CardContent>
        {chart.points.length === 0 ? (
          <div className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
            {t('No trend data in this range.')}
          </div>
        ) : (
          <div
            className='flex h-40 gap-3'
            role='img'
            aria-label={t('Daily total token trend')}
          >
            <div className='text-muted-foreground flex w-12 shrink-0 flex-col justify-between pb-6 text-right text-xs'>
              <span>{format.compact(chart.max)}</span>
              <span>{format.compact(chart.max / 2)}</span>
              <span>0</span>
            </div>
            <div className='min-w-0 flex-1 overflow-x-auto'>
              <div
                className='flex h-full min-w-full gap-2'
                // 日期也需要可读宽度；窄屏只在图内滚动，不裁掉首尾日期。
                style={{ minWidth: chart.points.length * 40 }}
              >
                {chart.points.map((point, index) => {
                  const date = new Date(point.timestamp * 1000)
                  const dateLabel = Number.isNaN(date.getTime())
                    ? '—'
                    : dateFormat.format(date)
                  const showDate =
                    chart.points.length <= 7 ||
                    index === 0 ||
                    index === Math.floor((chart.points.length - 1) / 2) ||
                    index === chart.points.length - 1
                  return (
                    <div
                      key={point.timestamp}
                      className='flex min-w-0 flex-1 flex-col'
                    >
                      <div className='border-border flex min-h-0 flex-1 items-end justify-center border-b'>
                        <span
                          className='bg-chart-1/75 block w-full max-w-8 rounded-t-sm'
                          style={{
                            height: `${(point.total / chart.max) * 100}%`,
                          }}
                          data-testid={`usage-trend-total-${point.timestamp}`}
                          title={format.number(point.total)}
                          aria-label={`${dateLabel}: ${format.number(point.total)} Token`}
                        />
                      </div>
                      <span className='text-muted-foreground mt-1 h-5 text-center text-[11px] whitespace-nowrap'>
                        {showDate ? dateLabel : ''}
                      </span>
                    </div>
                  )
                })}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
