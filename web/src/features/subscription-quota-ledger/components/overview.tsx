import { useTranslation } from 'react-i18next'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { useLedgerRead } from '@/features/report-access/hooks'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { getSummary, getTrend } from '../api'
import { formatWeeklyUnits, ledgerTime } from '../lib/forms'
import type { Filters } from '../types'
import { CycleBreakdown } from './cycle-breakdown'
import { QuotaMetrics } from './quota-metrics'

export function QuotaOverview(props: { filters: Filters }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const summary = useLedgerRead(['summary', props.filters], (signal) =>
    getSummary(props.filters, signal)
  )
  const trend = useLedgerRead(['trend', props.filters], (signal) =>
    getTrend(props.filters, signal)
  )
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
  if (!summary.data || !trend.data) return <ErrorState />
  const data = summary.data
  const window = data.summary.seven_day
  const current = data.current['604800']
  const fiveHour = data.current['18000']
  let collectionStatus = t('Collecting')
  if (!data.collection_enabled || !data.subject.enabled) {
    collectionStatus = t('Paused')
  } else if (!current) {
    collectionStatus = t('Waiting for first observation')
  } else if (
    data.subject.error_code ||
    current?.stale ||
    current?.quality_code
  ) {
    collectionStatus = t('Collection needs attention')
  }
  return (
    <div className='space-y-4' aria-label={t('Quota overview')}>
      <div
        className='text-muted-foreground flex flex-wrap items-center justify-between gap-2 text-xs'
        role='status'
      >
        <span>
          {t('Collection status')}: {collectionStatus}
        </span>
        <span>
          {t('Last observation')}:{' '}
          {ledgerTime(current?.fetched_at ?? 0, locale)}
        </span>
      </div>
      {!current && (
        <EmptyState
          title={t('Waiting for first observation')}
          description={t(
            'Enable collection and bind a Codex channel in collection settings.'
          )}
        />
      )}
      <QuotaMetrics data={data} />
      {window.pending_events > 0 && (
        <Alert>
          <AlertTitle>{t('Pending events')}</AlertTitle>
          <AlertDescription>
            <p>
              {t(
                'Weekly accounting is paused at an unresolved event. Later observations are retained but are not yet included. Review it in History; do not confirm a reset just to increase the total.'
              )}
            </p>
            <p>
              {t('Accounting paused since')}:{' '}
              {ledgerTime(window.pending_since, locale)}
            </p>
          </AlertDescription>
        </Alert>
      )}
      <Card role='region' aria-label={t('Five-hour quota status')}>
        <CardHeader>
          <CardTitle>{t('Five-hour quota status')}</CardTitle>
        </CardHeader>
        <CardContent className='flex flex-col gap-2 text-sm'>
          <p className='text-2xl font-semibold tabular-nums'>
            {fiveHour?.used_basis_points == null
              ? '—'
              : `${formatNumber((10000 - fiveHour.used_basis_points) / 100, locale)}%`}
          </p>
          <p>
            {t('Expected reset')}: {ledgerTime(fiveHour?.reset_at ?? 0, locale)}{' '}
            (Asia/Shanghai)
          </p>
          <p>
            {t('Last observation')}:{' '}
            {ledgerTime(fiveHour?.fetched_at ?? 0, locale)}
          </p>
          {(!fiveHour || fiveHour.stale || fiveHour.quality_code) && (
            <p className='text-muted-foreground'>
              {t(
                'Five-hour status is stale or unavailable; weekly accounting is independent.'
              )}
            </p>
          )}
          <p className='text-muted-foreground'>
            {t(
              'Status only. Five-hour quota is not counted as weekly usage or unused units.'
            )}
          </p>
        </CardContent>
      </Card>
      <div className='grid min-w-0 gap-4 lg:grid-cols-3'>
        <Card className='min-w-0 lg:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Daily observed weekly units')}</CardTitle>
          </CardHeader>
          <CardContent>
            <ChartContainer
              className='h-64 w-full'
              config={{
                observed_units: {
                  label: t('Observed weekly units'),
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
                  tickFormatter={(value: number) =>
                    formatWeeklyUnits(value, locale)
                  }
                />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      formatter={(value) =>
                        formatWeeklyUnits(Number(value), locale)
                      }
                    />
                  }
                />
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
            <CardTitle>{t('Weekly reset distribution')}</CardTitle>
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
      <div className='grid min-w-0 items-start gap-4 lg:grid-cols-2'>
        <CycleBreakdown filters={props.filters} weekly={window} />
        <Collapsible className='text-muted-foreground space-y-2 rounded-lg border p-4 text-xs'>
          <CollapsibleTrigger className='text-foreground text-sm'>
            {t('Accounting rules and collection details')}
          </CollapsibleTrigger>
          <CollapsibleContent className='space-y-2 pt-3'>
            <p>
              {t(
                'One unit equals 100% of a seven-day quota. Initial usage is a baseline; five-hour quota is status only.'
              )}
            </p>
            <p>
              {t(
                'Increases count after each successful observation, not only at resets. This page refreshes every minute; collection follows the configured interval.'
              )}
            </p>
            <p>
              {t(
                'Four weekly units are a reference, not the actual monthly allowance or unused quota. Unused estimates may overstate waste between observations.'
              )}
            </p>
            <p>
              {t('Last observation')}:{' '}
              {ledgerTime(current?.fetched_at ?? 0, locale)} ·{' '}
              {t('Expected reset')}:{' '}
              {ledgerTime(current?.reset_at ?? 0, locale)} (Asia/Shanghai)
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
              {t('Pending events')}:{' '}
              {formatNumber(window.pending_events, locale)} ·{' '}
              {t('Partial cycles')}:{' '}
              {formatNumber(window.partial_cycles, locale)} ·{' '}
              {t('Excluded cycles')}:{' '}
              {formatNumber(window.excluded_cycles, locale)}
            </p>
            <p>
              {t('Observation gaps (seconds)')}:{' '}
              {formatNumber(window.gap_seconds, locale)} ·{' '}
              {t('Invalid observations')}:{' '}
              {formatNumber(window.invalid_samples, locale)}
            </p>
            <p>
              {t('Coverage starts')}:{' '}
              {ledgerTime(data.summary.coverage_start, locale)} ·{' '}
              {t('Evaluated cycles')}:{' '}
              {formatNumber(window.evaluated_cycles, locale)}
            </p>
          </CollapsibleContent>
        </Collapsible>
      </div>
    </div>
  )
}
