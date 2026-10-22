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

import { TableCell, TableHead } from '@/components/ui/table'

import { useUsageFormat } from '../hooks/use-usage-format'
import type { UsageMetrics } from '../types'

// 小屏按行转为两列信息块；用 tbody > tr 覆盖公共 TableBody 的固定行高，避免重叠。
export const usageTableClassName =
  'max-sm:block max-sm:[&_thead]:hidden max-sm:[&_tbody]:block max-sm:[&_tr]:grid max-sm:[&_tbody>tr]:h-auto max-sm:[&_tr]:grid-cols-2 max-sm:[&_td]:block max-sm:[&_td]:whitespace-normal max-sm:[&_td:first-child]:col-span-2'

export function UsageCost(props: { quota: number; className?: string }) {
  const { t } = useTranslation()
  const cost = useUsageFormat().cost(props.quota)
  return (
    <div className={props.className}>
      <span className='tabular-nums'>{cost.cny ?? t('Unavailable')}</span>
      <div
        className='text-muted-foreground text-xs tabular-nums'
        data-table-text='secondary'
      >
        {cost.usd ?? t('Unavailable')}
      </div>
    </div>
  )
}

export function UsageMetricsHeaders() {
  const { t } = useTranslation()
  return (
    <>
      <TableHead className='text-muted-foreground text-right'>
        {t('Requests')}
      </TableHead>
      <TableHead className='text-muted-foreground text-right'>
        {t('Token usage')}
      </TableHead>
      <TableHead className='text-muted-foreground text-right'>
        {t('Consumed quota')}
      </TableHead>
      <TableHead className='text-muted-foreground text-right'>
        {t('Equivalent cost')}
      </TableHead>
    </>
  )
}

export function UsageMetricsCells(props: { metrics: UsageMetrics }) {
  const { t } = useTranslation()
  const format = useUsageFormat()
  return (
    <>
      <TableCell className='text-right'>
        <div
          className='text-muted-foreground text-xs sm:hidden'
          data-table-text='secondary'
        >
          {t('Requests')}
        </div>
        {format.number(props.metrics.requests)}
      </TableCell>
      <TableCell className='text-right'>
        <div
          className='text-muted-foreground text-xs sm:hidden'
          data-table-text='secondary'
        >
          {t('Token usage')}
        </div>
        <span title={format.number(props.metrics.totalTokens)}>
          {format.compact(props.metrics.totalTokens)}
        </span>
        <div
          className='text-muted-foreground text-xs'
          data-table-text='secondary'
          title={`${t('Input')}: ${format.number(props.metrics.inputTokens)} / ${t('Output')}: ${format.number(props.metrics.outputTokens)}`}
        >
          {t('Input')}: {format.compact(props.metrics.inputTokens)} /{' '}
          {t('Output')}: {format.compact(props.metrics.outputTokens)}
        </div>
      </TableCell>
      <TableCell className='text-right'>
        <div
          className='text-muted-foreground text-xs sm:hidden'
          data-table-text='secondary'
        >
          {t('Consumed quota')}
        </div>
        <span title={format.number(props.metrics.quota)}>
          {format.compact(props.metrics.quota)}
        </span>
      </TableCell>
      <TableCell className='text-right'>
        <div
          className='text-muted-foreground text-xs sm:hidden'
          data-table-text='secondary'
        >
          {t('Equivalent cost')}
        </div>
        <UsageCost quota={props.metrics.quota} />
      </TableCell>
    </>
  )
}
