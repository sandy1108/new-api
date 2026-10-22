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

import { StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import type { UsageTokenBucket } from '../types'
import {
  UsageMetricsCells,
  UsageMetricsHeaders,
  usageTableClassName,
} from './usage-metrics'

export interface TokenTableProps {
  tokens: UsageTokenBucket[]
  selectedKey?: string
  onSelect: (key: string) => void
}

export function TokenTable(props: TokenTableProps) {
  const { t } = useTranslation()
  if (props.tokens.length === 0) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {t('No tokens in this range.')}
      </p>
    )
  }
  return (
    <StaticDataTable
      className='rounded-none border-0'
      tableClassName={usageTableClassName}
      tableProps={{ 'aria-label': t('API Token Summary') }}
    >
      <TableHeader>
        <TableRow className='bg-muted/40'>
          <TableHead className='text-muted-foreground'>
            {t('API Token')}
          </TableHead>
          <UsageMetricsHeaders />
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.tokens.map((token) => {
          const selected = token.key === props.selectedKey
          return (
            <TableRow
              key={token.key}
              aria-selected={selected}
              data-state={selected ? 'selected' : undefined}
              className='data-[state=selected]:bg-primary/5 cursor-pointer'
              onClick={() => props.onSelect(token.key)}
            >
              <TableCell className='max-w-52 whitespace-normal'>
                <Button
                  variant='link'
                  className='h-auto max-w-full justify-start p-0 text-left whitespace-normal'
                  aria-pressed={selected}
                  onClick={(event) => {
                    event.stopPropagation()
                    props.onSelect(token.key)
                  }}
                >
                  <span className='break-all'>{token.name}</span>
                </Button>
                <div
                  className='text-muted-foreground text-xs'
                  data-table-text='secondary'
                >
                  {token.username || '—'}
                  {selected ? ` · ${t('Selected')}` : ''}
                </div>
              </TableCell>
              <UsageMetricsCells metrics={token} />
            </TableRow>
          )
        })}
      </TableBody>
    </StaticDataTable>
  )
}
