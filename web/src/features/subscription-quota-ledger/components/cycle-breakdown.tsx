import { useQuery } from '@tanstack/react-query'
import { Info } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { toIntlLocale } from '@/i18n/languages'

import { getCycles, quotaKey } from '../api'
import { beijingMonth, formatWeeklyUnits, ledgerTime } from '../lib/forms'
import type { Filters, WindowSummary } from '../types'

export function CycleBreakdown(props: {
  filters: Filters
  weekly: WindowSummary
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [page, setPage] = useState(1)
  const cycles = useQuery({
    queryKey: [...quotaKey, 'cycles', props.filters, page],
    queryFn: () => getCycles(props.filters, page),
    refetchInterval: 60_000,
  })
  return (
    <Card className='min-w-0'>
      <CardHeader>
        <CardTitle>{t('Usage and unused quota by cycle')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t('Monthly unused total')}:{' '}
          <strong className='text-foreground text-lg font-bold tabular-nums'>
            {props.weekly.evaluated_cycles
              ? formatWeeklyUnits(props.weekly.unused_units, locale)
              : '—'}
          </strong>
        </p>
        {cycles.isPending && <LoadingState />}
        {cycles.isError && <ErrorState onRetry={() => void cycles.refetch()} />}
        {cycles.data && (
          <>
            {cycles.data.items.length === 0 && (
              <EmptyState title={t('No Data')} />
            )}
            {cycles.data.items.map((cycle) => {
              // 与后端汇总沿用相同排除条件；这里只展示来源，不改写账本。
              const included =
                cycle.ended_at > 0 &&
                beijingMonth(new Date(cycle.ended_at * 1000)) ===
                  props.filters.month &&
                !cycle.partial &&
                !cycle.has_gap &&
                !cycle.has_correction &&
                cycle.ended_by !== 'segment_change'
              return (
                <div
                  key={`${cycle.window_seconds}/${cycle.started_at}`}
                  className='space-y-2 border-b pb-3 last:border-b-0'
                >
                  <div className='flex flex-wrap items-center justify-between gap-2 text-sm'>
                    <span>
                      {ledgerTime(cycle.started_at, locale)} →{' '}
                      {ledgerTime(cycle.ended_at, locale)}
                    </span>
                    <span className='text-muted-foreground'>
                      {cycle.ended_at
                        ? t('Cycle ended')
                        : t('Current cycle is still in progress')}
                    </span>
                  </div>
                  <Progress
                    value={Math.min(
                      100,
                      Math.max(0, cycle.high_water_basis_points / 100)
                    )}
                    aria-label={t('Highest observed weekly usage')}
                  />
                  <p className='text-muted-foreground text-sm'>
                    {t('Highest observed weekly usage')}:{' '}
                    <strong className='text-foreground font-semibold tabular-nums'>
                      {formatWeeklyUnits(
                        cycle.high_water_basis_points / 100,
                        locale
                      )}
                      %
                    </strong>
                  </p>
                  <div className='text-muted-foreground flex flex-wrap justify-between gap-2 text-sm tabular-nums'>
                    <span>
                      {t('Observed weekly units')}:{' '}
                      <strong className='text-foreground text-base font-bold'>
                        {formatWeeklyUnits(
                          cycle.observed_basis_points / 10000,
                          locale
                        )}
                      </strong>
                    </span>
                    <span>
                      {t('Quota left unused at this reset (units)')}:{' '}
                      <strong className='text-foreground text-base font-bold'>
                        {included
                          ? formatWeeklyUnits(
                              Math.max(
                                0,
                                10000 - cycle.high_water_basis_points
                              ) / 10000,
                              locale
                            )
                          : '—'}
                      </strong>
                    </span>
                  </div>
                  {cycle.partial && (
                    <p className='text-muted-foreground flex items-start gap-1 text-xs'>
                      <Info className='size-3.5 shrink-0' aria-hidden='true' />
                      {t('Joined midway; unused quota is not included')}
                    </p>
                  )}
                  {(cycle.has_gap ||
                    cycle.has_correction ||
                    cycle.ended_by === 'segment_change') && (
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Incomplete or corrected cycle; unused quota is not included'
                      )}
                    </p>
                  )}
                </div>
              )
            })}
            {cycles.data.total > 20 && (
              <div className='flex items-center justify-between gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page === 1}
                  onClick={() => setPage(page - 1)}
                >
                  {t('Previous')}
                </Button>
                <span className='text-muted-foreground text-xs'>
                  {t('Cycle details on this page')}
                </span>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page * 20 >= cycles.data.total}
                  onClick={() => setPage(page + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
