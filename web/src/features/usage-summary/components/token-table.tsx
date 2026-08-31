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

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import {
  formatUsageCompactNumber,
  formatUsageNumber,
  formatUsageQuota,
} from '../lib/format'
import type { UsageTokenBucket } from '../types'

function formatPercentage(value: number): string {
  if (value <= 0) return '0%'
  if (value < 1) return '<1%'
  return `${new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 }).format(value)}%`
}

export interface TokenTableProps {
  tokens: UsageTokenBucket[]
  totalTokens?: number
  selectedKey?: string
  onSelect: (key: string) => void
}

export function TokenTable(props: TokenTableProps) {
  const { t } = useTranslation()

  if (props.tokens.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
        {t('No tokens in this range.')}
      </div>
    )
  }

  const totalTokens =
    Number.isFinite(props.totalTokens) && (props.totalTokens ?? 0) > 0
      ? (props.totalTokens ?? 0)
      : props.tokens.reduce((total, token) => total + token.totalTokens, 0)

  return (
    <div className='overflow-hidden rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow className='bg-muted/40 hover:bg-muted/40'>
            <TableHead>{t('API Token')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Total Tokens')}</TableHead>
            <TableHead className='text-right'>{t('Input / Output')}</TableHead>
            <TableHead className='text-right'>{t('Quota')}</TableHead>
            <TableHead className='min-w-[130px]'>{t('Share')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.tokens.map((token) => {
            const isSelected = token.key === props.selectedKey
            const percentage =
              totalTokens > 0 ? (token.totalTokens / totalTokens) * 100 : 0
            const safePercentage = Math.min(100, Math.max(0, percentage))
            return (
              <TableRow
                key={token.key}
                aria-selected={isSelected}
                data-selected={isSelected}
                tabIndex={0}
                className='focus-visible:bg-muted/60 data-[selected=true]:bg-primary/5 cursor-pointer outline-none'
                onClick={() => props.onSelect(token.key)}
                onKeyDown={(event) => {
                  if (event.target !== event.currentTarget) return
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    props.onSelect(token.key)
                  }
                }}
              >
                <TableCell className='min-w-[180px]'>
                  <button
                    type='button'
                    className='focus-visible:ring-ring/50 inline-flex max-w-full rounded-md text-left font-medium outline-none focus-visible:ring-3'
                    aria-selected={isSelected}
                    onClick={(event) => {
                      event.stopPropagation()
                      props.onSelect(token.key)
                    }}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault()
                        event.stopPropagation()
                        props.onSelect(token.key)
                      }
                    }}
                  >
                    <span className='truncate'>{token.name}</span>
                  </button>
                </TableCell>
                <TableCell className='text-muted-foreground'>
                  {token.username || '—'}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  <span title={formatUsageNumber(token.requests)}>
                    {formatUsageCompactNumber(token.requests)}
                  </span>
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  <span title={formatUsageNumber(token.totalTokens)}>
                    {formatUsageCompactNumber(token.totalTokens)}
                  </span>
                </TableCell>
                <TableCell className='text-right font-mono text-xs tabular-nums'>
                  <span title={formatUsageNumber(token.inputTokens)}>
                    {formatUsageCompactNumber(token.inputTokens)}
                  </span>
                  <span className='text-muted-foreground mx-1'>/</span>
                  <span title={formatUsageNumber(token.outputTokens)}>
                    {formatUsageCompactNumber(token.outputTokens)}
                  </span>
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  <span title={formatUsageQuota(token.quota)}>
                    {formatUsageQuota(token.quota)}
                  </span>
                </TableCell>
                <TableCell>
                  <div className='min-w-[110px] space-y-1'>
                    <div className='text-muted-foreground flex justify-between gap-2 text-[10px]'>
                      <span>{formatPercentage(percentage)}</span>
                      <span>{t('of total')}</span>
                    </div>
                    <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
                      <div
                        className='bg-primary h-full rounded-full'
                        style={{ width: `${safePercentage}%` }}
                      />
                    </div>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
