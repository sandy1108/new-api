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

export interface TokenDetailProps {
  token: UsageTokenBucket | null
}

function MetricLine(props: { label: string; value: string }) {
  return (
    <span className='text-muted-foreground inline-flex items-center gap-1 text-xs'>
      <span>{props.label}</span>
      <span className='text-foreground font-mono tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function TokenDetail(props: TokenDetailProps) {
  const { t } = useTranslation()

  if (!props.token) {
    return (
      <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
        {t('Select a token to see channel details.')}
      </div>
    )
  }

  if (props.token.channels.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
        {t('No channel usage for this token.')}
      </div>
    )
  }

  return (
    <div className='grid gap-3 lg:grid-cols-2'>
      {props.token.channels.map((channel) => (
        <Card key={channel.key} size='sm'>
          <CardHeader className='p-3 pb-0'>
            <div className='flex flex-wrap items-start justify-between gap-2'>
              <CardTitle className='truncate text-sm'>{channel.name}</CardTitle>
              <div className='flex flex-wrap justify-end gap-x-3 gap-y-1'>
                <MetricLine
                  label={t('Requests')}
                  value={formatUsageNumber(channel.requests)}
                />
                <MetricLine
                  label={t('Total Tokens')}
                  value={formatUsageNumber(channel.totalTokens)}
                />
                <MetricLine
                  label={t('Quota')}
                  value={formatUsageQuota(channel.quota)}
                />
              </div>
            </div>
          </CardHeader>
          <CardContent className='p-3 pt-2'>
            {channel.models.length === 0 ? (
              <p className='text-muted-foreground rounded-md border border-dashed px-3 py-5 text-center text-xs'>
                {t('No model usage for this channel.')}
              </p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow className='bg-muted/40 hover:bg-muted/40'>
                    <TableHead>{t('Model')}</TableHead>
                    <TableHead className='text-right'>{t('Requests')}</TableHead>
                    <TableHead className='text-right'>{t('Input Tokens')}</TableHead>
                    <TableHead className='text-right'>{t('Output Tokens')}</TableHead>
                    <TableHead className='text-right'>{t('Total Tokens')}</TableHead>
                    <TableHead className='text-right'>{t('Quota')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {channel.models.map((model) => (
                    <TableRow key={model.name}>
                      <TableCell className='max-w-[180px] truncate font-medium'>
                        {model.name}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatUsageNumber(model.requests)}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatUsageNumber(model.inputTokens)}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatUsageNumber(model.outputTokens)}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatUsageNumber(model.totalTokens)}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatUsageQuota(model.quota)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
