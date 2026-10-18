import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { getSummary, getTrend, quotaKey } from '../api'
import { ledgerTime } from '../lib/forms'
import type { Filters } from '../types'

export function QuotaOverview(props: { filters: Filters }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const summary = useQuery({
    queryKey: [...quotaKey, 'summary', props.filters],
    queryFn: () => getSummary(props.filters),
    refetchInterval: 60_000,
  })
  const trend = useQuery({
    queryKey: [...quotaKey, 'trend', props.filters],
    queryFn: () => getTrend(props.filters),
    refetchInterval: 60_000,
  })
  if (summary.isPending || trend.isPending) return <LoadingState />
  if (summary.isError || trend.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void summary.refetch()
          void trend.refetch()
        }}
      />
    )
  }
  const data = summary.data
  const window =
    props.filters.window_seconds === 604800
      ? data.summary.seven_day
      : data.summary.five_hour
  const current = data.current[String(props.filters.window_seconds)]
  const cards = [
    {
      title: t('Observed quota units'),
      value: formatNumber(window.observed_units, locale),
    },
    {
      title: t('Current usage'),
      value:
        current?.used_basis_points == null
          ? '—'
          : `${formatNumber(current.used_basis_points / 100, locale)}%`,
    },
    {
      title: t('Evaluable unused units'),
      value: window.evaluated_cycles
        ? formatNumber(window.unused_units, locale)
        : '—',
    },
    {
      title: t('Observed resets'),
      value: formatNumber(window.reset_count, locale),
    },
  ]
  return (
    <div className='space-y-4' aria-label={t('Quota overview')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Quota observations are estimates, not billing. Initial usage is a baseline; weekly and five-hour units are separate.'
        )}
      </p>
      {!data.last_observed_at && (
        <EmptyState
          title={t('Waiting for first observation')}
          description={t(
            'Enable collection and bind a Codex channel in collection settings.'
          )}
        />
      )}
      <div className='grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-4'>
        {cards.map((card) => (
          <Card key={card.title} className='min-w-0'>
            <CardHeader>
              <CardTitle>{card.title}</CardTitle>
            </CardHeader>
            <CardContent className='text-3xl font-semibold tabular-nums'>
              {card.value}
            </CardContent>
          </Card>
        ))}
      </div>
      <div
        className='bg-muted/40 space-y-1 rounded-lg border p-4 text-sm'
        role='status'
      >
        <p>
          {t('Last observation')}:{' '}
          {ledgerTime(current?.fetched_at ?? 0, locale)} · {t('Expected reset')}
          : {ledgerTime(current?.reset_at ?? 0, locale)} (Asia/Shanghai)
        </p>
        {(!data.collection_enabled ||
          current?.stale ||
          !data.subject.enabled ||
          data.subject.error_code) && (
          <p className='text-amber-600'>
            {t('Collection is paused, stale or incomplete')}
            {data.subject.error_code && ` · ${data.subject.error_code}`}
          </p>
        )}
        {current?.quality_code && (
          <p>
            {t('Unknown observation')}: {current.quality_code}
          </p>
        )}
        <p>
          {t('Pending events')}: {formatNumber(window.pending_events, locale)} ·{' '}
          {t('Partial cycles')}: {formatNumber(window.partial_cycles, locale)} ·{' '}
          {t('Excluded cycles')}: {formatNumber(window.excluded_cycles, locale)}
        </p>
        <p>
          {t('Observation gaps (seconds)')}:{' '}
          {formatNumber(data.summary.gap_seconds, locale)} ·{' '}
          {t('Invalid observations')}:{' '}
          {formatNumber(data.summary.invalid_samples, locale)}
        </p>
        <p>
          {t('Coverage starts')}:{' '}
          {ledgerTime(data.summary.coverage_start, locale)} ·{' '}
          {t('Evaluated cycles')}:{' '}
          {formatNumber(window.evaluated_cycles, locale)}
        </p>
      </div>
      <div className='grid min-w-0 gap-4 lg:grid-cols-3'>
        <Card className='min-w-0 lg:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Daily observed quota units')}</CardTitle>
          </CardHeader>
          <CardContent>
            <ChartContainer
              className='h-64 w-full'
              config={{
                observed_units: {
                  label: t('Observed quota units'),
                  color: 'var(--primary)',
                },
              }}
            >
              <LineChart data={trend.data.items} accessibilityLayer>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey='date'
                  tickFormatter={(value: string) => value.slice(8)}
                />
                <YAxis
                  tickFormatter={(value: number) => formatNumber(value, locale)}
                />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Line
                  dataKey='observed_units'
                  stroke='var(--color-observed_units)'
                  connectNulls={false}
                  dot={false}
                />
              </LineChart>
            </ChartContainer>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Missing days are unknown, not zero. Gaps and pending resets can undercount usage.'
              )}
            </p>
          </CardContent>
        </Card>
        <Card className='min-w-0'>
          <CardHeader>
            <CardTitle>{t('Reset distribution')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            <p>
              {t('Early reset')}: {formatNumber(window.early_resets, locale)}
            </p>
            <p>
              {t('Expired reset')}:{' '}
              {formatNumber(window.expired_resets, locale)}
            </p>
            <p>
              {t('Manual reset decision')}:{' '}
              {formatNumber(window.manual_resets, locale)}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
