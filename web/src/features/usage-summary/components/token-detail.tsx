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
import { ChevronRight, Hash, Radio, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
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
import { selectDefaultChannel } from '../lib/selectors'
import type { UsageChannelBucket, UsageTokenBucket } from '../types'

export interface TokenDetailProps {
  token: UsageTokenBucket | null
  selectedChannelKey?: string
  onChannelSelect?: (key: string) => void
}

function MetricLine(props: { label: string; value: string; title?: string }) {
  return (
    <span
      className='text-muted-foreground inline-flex items-center gap-1 text-xs'
      title={props.title ?? props.value}
    >
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

  const selectedChannel = selectDefaultChannel(
    props.token,
    props.selectedChannelKey
  )

  const renderChannel = (channel: UsageChannelBucket) => {
    const isSelected = channel.key === selectedChannel?.key
    return (
      <button
        key={channel.key}
        type='button'
        aria-pressed={isSelected}
        data-selected={isSelected}
        className='hover:bg-muted/45 focus-visible:ring-ring/50 data-[selected=true]:border-primary/30 data-[selected=true]:bg-primary/5 flex w-full items-center gap-3 rounded-xl border border-transparent px-3 py-2.5 text-left transition-colors outline-none focus-visible:ring-3'
        onClick={() => props.onChannelSelect?.(channel.key)}
      >
        <IconBadge
          tone={isSelected ? 'chart-1' : 'neutral'}
          size='sm'
          className='rounded-lg'
        >
          <Radio />
        </IconBadge>
        <div className='min-w-0 flex-1'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='truncate text-sm font-medium'>{channel.name}</span>
            {isSelected && (
              <span className='text-primary shrink-0 text-[10px] font-semibold'>
                {t('Selected')}
              </span>
            )}
          </div>
          <div className='text-muted-foreground mt-1 truncate text-[11px]'>
            {t('Click to see models in this channel.')}
          </div>
          <div className='text-muted-foreground/70 mt-1 truncate text-[10px] sm:hidden'>
            <span title={formatUsageNumber(channel.inputTokens)}>
              {t('Input')}: {formatUsageCompactNumber(channel.inputTokens)}
            </span>{' '}
            ·{' '}
            <span title={formatUsageNumber(channel.outputTokens)}>
              {t('Output')}: {formatUsageCompactNumber(channel.outputTokens)}
            </span>
          </div>
        </div>
        <div className='hidden shrink-0 items-center gap-4 text-right sm:flex'>
          <MetricLine
            label={t('Requests')}
            value={formatUsageCompactNumber(channel.requests)}
            title={formatUsageNumber(channel.requests)}
          />
          <MetricLine
            label={t('Total Tokens')}
            value={formatUsageCompactNumber(channel.totalTokens)}
            title={formatUsageNumber(channel.totalTokens)}
          />
          <MetricLine
            label={t('Quota')}
            value={formatUsageQuota(channel.quota)}
            title={formatUsageQuota(channel.quota)}
          />
        </div>
        <ChevronRight
          className='text-muted-foreground size-4 shrink-0'
          aria-hidden='true'
        />
      </button>
    )
  }

  const renderModel = (model: UsageChannelBucket['models'][number]) => (
    <TableRow key={model.name}>
      <TableCell className='max-w-[180px] truncate font-medium'>
        <div className='flex min-w-0 items-center gap-2'>
          <IconBadge tone='chart-2' size='xs'>
            <Sparkles />
          </IconBadge>
          <span className='truncate'>{model.name}</span>
        </div>
      </TableCell>
      <TableCell className='text-right font-mono tabular-nums'>
        <span title={formatUsageNumber(model.requests)}>
          {formatUsageCompactNumber(model.requests)}
        </span>
      </TableCell>
      <TableCell className='text-right font-mono tabular-nums'>
        <span title={formatUsageNumber(model.inputTokens)}>
          {formatUsageCompactNumber(model.inputTokens)}
        </span>
      </TableCell>
      <TableCell className='text-right font-mono tabular-nums'>
        <span title={formatUsageNumber(model.outputTokens)}>
          {formatUsageCompactNumber(model.outputTokens)}
        </span>
      </TableCell>
      <TableCell className='text-right font-mono tabular-nums'>
        <span title={formatUsageNumber(model.totalTokens)}>
          {formatUsageCompactNumber(model.totalTokens)}
        </span>
      </TableCell>
      <TableCell className='text-right font-mono tabular-nums'>
        <span title={formatUsageQuota(model.quota)}>
          {formatUsageQuota(model.quota)}
        </span>
      </TableCell>
    </TableRow>
  )

  return (
    <div className='space-y-3'>
      <Card size='sm' className='overflow-hidden'>
        <CardHeader className='border-b p-3 sm:p-4'>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div className='flex min-w-0 items-center gap-3'>
              <IconBadge tone='chart-1' size='lg'>
                <Hash />
              </IconBadge>
              <div className='min-w-0'>
                <CardTitle className='truncate text-sm sm:text-base'>
                  {t('Channel Summary')}
                </CardTitle>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {props.token.name} · {t('Click a channel to see its models.')}
                </p>
              </div>
            </div>
            <div className='flex flex-wrap gap-x-4 gap-y-1'>
              <MetricLine
                label={t('Requests')}
                value={formatUsageCompactNumber(props.token.requests)}
                title={formatUsageNumber(props.token.requests)}
              />
              <MetricLine
                label={t('Total Tokens')}
                value={formatUsageCompactNumber(props.token.totalTokens)}
                title={formatUsageNumber(props.token.totalTokens)}
              />
              <MetricLine
                label={t('Quota')}
                value={formatUsageQuota(props.token.quota)}
                title={formatUsageQuota(props.token.quota)}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent className='p-2 sm:p-3'>
          <div className='space-y-1'>
            {props.token.channels.map(renderChannel)}
          </div>
        </CardContent>
      </Card>

      <Card size='sm' className='overflow-hidden'>
        <CardHeader className='border-b p-3 sm:p-4'>
          <div className='flex flex-wrap items-start justify-between gap-2'>
            <div className='flex min-w-0 items-center gap-2'>
              <IconBadge tone='chart-2' size='sm'>
                <Sparkles />
              </IconBadge>
              <div className='min-w-0'>
                <CardTitle className='truncate text-sm'>
                  {selectedChannel
                    ? `${selectedChannel.name} · ${t('Models in Channel')}`
                    : t('Models in Channel')}
                </CardTitle>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Only models from the selected channel are shown.')}
                </p>
              </div>
            </div>
            {selectedChannel && (
              <div className='flex flex-wrap gap-x-4 gap-y-1'>
                <MetricLine
                  label={t('Requests')}
                  value={formatUsageCompactNumber(selectedChannel.requests)}
                  title={formatUsageNumber(selectedChannel.requests)}
                />
                <MetricLine
                  label={t('Total Tokens')}
                  value={formatUsageCompactNumber(selectedChannel.totalTokens)}
                  title={formatUsageNumber(selectedChannel.totalTokens)}
                />
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent className='p-2 sm:p-3'>
          {!selectedChannel || selectedChannel.models.length === 0 ? (
            <p className='text-muted-foreground rounded-lg border border-dashed px-3 py-7 text-center text-xs'>
              {t('No model usage for this channel.')}
            </p>
          ) : (
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow className='bg-muted/40 hover:bg-muted/40'>
                    <TableHead>{t('Model')}</TableHead>
                    <TableHead className='text-right'>
                      {t('Requests')}
                    </TableHead>
                    <TableHead className='text-right'>
                      {t('Input Tokens')}
                    </TableHead>
                    <TableHead className='text-right'>
                      {t('Output Tokens')}
                    </TableHead>
                    <TableHead className='text-right'>
                      {t('Total Tokens')}
                    </TableHead>
                    <TableHead className='text-right'>{t('Quota')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>{selectedChannel.models.map(renderModel)}</TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
