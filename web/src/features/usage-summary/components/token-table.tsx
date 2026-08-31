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

import { formatUsageNumber, formatUsageQuota } from '../lib/format'
import type { UsageTokenBucket } from '../types'

export interface TokenTableProps {
  tokens: UsageTokenBucket[]
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

  return (
    <div className='overflow-hidden rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow className='bg-muted/40 hover:bg-muted/40'>
            <TableHead>{t('Token')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Total Tokens')}</TableHead>
            <TableHead className='text-right'>{t('Quota')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.tokens.map((token) => {
            const isSelected = token.key === props.selectedKey
            return (
              <TableRow key={token.key} aria-selected={isSelected}>
                <TableCell className='min-w-[180px]'>
                  <button
                    type='button'
                    className='focus-visible:ring-ring/50 inline-flex max-w-full rounded-md text-left font-medium outline-none focus-visible:ring-3'
                    aria-selected={isSelected}
                    onClick={() => props.onSelect(token.key)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault()
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
                  {formatUsageNumber(token.requests)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatUsageNumber(token.totalTokens)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatUsageQuota(token.quota)}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
