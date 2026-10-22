/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { useUsageFormat } from '../hooks/use-usage-format'
import { aggregateTokenModels, selectDefaultChannel } from '../lib/selectors'
import type { UsageMetrics, UsageTokenBucket } from '../types'
import {
  UsageCost,
  UsageMetricsCells,
  UsageMetricsHeaders,
  usageTableClassName,
} from './usage-metrics'

export interface TokenDetailProps {
  token: UsageTokenBucket | null
  selectedChannelKey?: string
  onChannelSelect?: (key: string) => void
}

function BreakdownMetrics(props: {
  metrics: UsageMetrics
  totalTokens: number
}) {
  const { t } = useTranslation()
  const format = useUsageFormat()
  const percent =
    props.totalTokens > 0
      ? Math.min(
          100,
          Math.max(0, (props.metrics.totalTokens / props.totalTokens) * 100)
        )
      : 0
  return (
    <>
      <div className='mt-2 flex flex-wrap justify-between gap-x-3 gap-y-1 text-xs'>
        <span title={format.number(props.metrics.totalTokens)}>
          {format.compact(props.metrics.totalTokens)} Token
        </span>
        <span title={format.number(props.metrics.quota)}>
          {t('Consumed quota')}: {format.compact(props.metrics.quota)}
        </span>
      </div>
      <div className='text-muted-foreground mt-1 flex flex-wrap justify-between gap-x-3 gap-y-1 text-xs'>
        <span
          title={`${t('Input')}: ${format.number(props.metrics.inputTokens)} / ${t('Output')}: ${format.number(props.metrics.outputTokens)}`}
        >
          {t('Input')}: {format.compact(props.metrics.inputTokens)} /{' '}
          {t('Output')}: {format.compact(props.metrics.outputTokens)}
        </span>
        <span>
          {format.number(props.metrics.requests)} {t('Requests')}
        </span>
      </div>
      <div className='bg-muted mt-3 h-1 overflow-hidden rounded-full'>
        <div
          className='bg-chart-1 h-full rounded-full'
          style={{ width: `${percent}%` }}
        />
      </div>
    </>
  )
}

export function TokenDetail(props: TokenDetailProps) {
  const { t } = useTranslation()
  const format = useUsageFormat()
  const models = useMemo(
    () => (props.token ? aggregateTokenModels(props.token) : []),
    [props.token]
  )
  if (!props.token) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {t('Select a token to see channel details.')}
      </p>
    )
  }
  if (props.token.channels.length === 0) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {t('No channel usage for this token.')}
      </p>
    )
  }
  const selectedChannel = selectDefaultChannel(
    props.token,
    props.selectedChannelKey
  )
  const totalTokens = props.token.totalTokens

  return (
    <div className='space-y-4'>
      <div
        className='flex flex-wrap items-center justify-between gap-3'
        aria-live='polite'
      >
        <div className='min-w-0'>
          <p className='text-muted-foreground text-xs'>
            {t('Current API token')}
          </p>
          <p className='mt-1 font-medium break-all'>{props.token.name}</p>
        </div>
        <div className='text-xs sm:text-right'>
          <p className='mb-1' title={format.number(props.token.totalTokens)}>
            {format.compact(props.token.totalTokens)} Token ·{' '}
            {t('Consumed quota')}: {format.compact(props.token.quota)}
          </p>
          <UsageCost quota={props.token.quota} />
        </div>
      </div>
      <div className='grid min-w-0 gap-4 lg:grid-cols-2'>
        <Card size='sm' className='min-w-0'>
          <CardHeader>
            <CardTitle>
              <h3>{t('Channel Summary')}</h3>
            </CardTitle>
            <p className='text-muted-foreground text-xs'>
              {t('Click a channel to see its models.')}
            </p>
          </CardHeader>
          <CardContent className='space-y-2'>
            {props.token.channels.map((channel) => (
              <Button
                key={channel.key}
                variant='outline'
                aria-pressed={channel.key === selectedChannel?.key}
                className='aria-pressed:border-chart-1 aria-pressed:bg-chart-1/5 h-auto w-full min-w-0 flex-col items-stretch rounded-lg p-3 text-left whitespace-normal'
                onClick={() => props.onChannelSelect?.(channel.key)}
              >
                <div className='flex flex-wrap items-start justify-between gap-2'>
                  <span className='min-w-0 flex-1 break-all'>
                    {channel.name}
                  </span>
                  <UsageCost
                    quota={channel.quota}
                    className='shrink-0 text-right'
                  />
                </div>
                <BreakdownMetrics metrics={channel} totalTokens={totalTokens} />
              </Button>
            ))}
          </CardContent>
        </Card>
        <Card
          size='sm'
          className='min-w-0'
          role='region'
          aria-label={t('Models across channels')}
        >
          <CardHeader>
            <CardTitle>
              <h3>{t('Model Summary')}</h3>
            </CardTitle>
            <p className='text-muted-foreground text-xs'>
              {t('Current token, all channels combined')}
            </p>
          </CardHeader>
          <CardContent className='space-y-2'>
            {models.map((model) => (
              <div key={model.name} className='rounded-lg border p-3'>
                <div className='flex flex-wrap items-start justify-between gap-2 text-sm'>
                  <span className='min-w-0 flex-1 font-medium break-all'>
                    {model.name}
                  </span>
                  <UsageCost quota={model.quota} className='text-right' />
                </div>
                <BreakdownMetrics metrics={model} totalTokens={totalTokens} />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
      <Card size='sm' className='min-w-0'>
        <CardHeader>
          <CardTitle>
            <h3 className='break-all'>
              {selectedChannel?.name} · {t('Models in Channel')}
            </h3>
          </CardTitle>
          <p className='text-muted-foreground text-xs'>
            {t('Only models from the selected channel are shown.')}
          </p>
        </CardHeader>
        <CardContent>
          {!selectedChannel || selectedChannel.models.length === 0 ? (
            <p className='text-muted-foreground py-7 text-center text-xs'>
              {t('No model usage for this channel.')}
            </p>
          ) : (
            <StaticDataTable
              className='rounded-none border-0'
              tableClassName={usageTableClassName}
              tableProps={{ 'aria-label': t('Models in Channel') }}
            >
              <TableHeader>
                <TableRow className='bg-muted/40'>
                  <TableHead className='text-muted-foreground'>
                    {t('Model')}
                  </TableHead>
                  <UsageMetricsHeaders />
                </TableRow>
              </TableHeader>
              <TableBody>
                {selectedChannel.models.map((model) => (
                  <TableRow key={model.name}>
                    <TableCell className='max-w-52 break-all whitespace-normal'>
                      {model.name}
                    </TableCell>
                    <UsageMetricsCells metrics={model} />
                  </TableRow>
                ))}
              </TableBody>
            </StaticDataTable>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
