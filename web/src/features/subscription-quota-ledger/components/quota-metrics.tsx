import {
  Activity,
  ChartColumnIncreasing,
  Gauge,
  Hourglass,
  RotateCcw,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Progress } from '@/components/ui/progress'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import {
  formatWeeklyUnits,
  ledgerTime,
  monthlyQuotaProgress,
} from '../lib/forms'
import type { Summary } from '../types'

export function QuotaMetrics(props: { data: Summary }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const weekly = props.data.summary.seven_day
  const current = props.data.current['604800']
  const progress = monthlyQuotaProgress(weekly.observed_units)
  const ratio = progress
    ? `${formatWeeklyUnits(progress.percent, locale)}%`
    : '—'
  const metrics = [
    {
      title: t('Current weekly usage'),
      icon: Activity,
      tone: 'success' as const,
      value:
        current?.used_basis_points == null
          ? '—'
          : `${formatNumber(current.used_basis_points / 100, locale)}%`,
      detail: t('Current cycle is still in progress'),
    },
    {
      title: t('Estimated unused weekly units'),
      icon: Hourglass,
      tone: 'warning' as const,
      value: weekly.evaluated_cycles
        ? formatWeeklyUnits(weekly.unused_units, locale)
        : '—',
      detail: t('{{count}} cycles included; not the current remaining quota', {
        count: weekly.evaluated_cycles,
      }),
    },
    {
      title: t('Observed weekly resets'),
      icon: RotateCcw,
      tone: 'info' as const,
      value: formatNumber(weekly.reset_count, locale),
      detail: t(
        'Automatically detected: {{automatic}} · Manually confirmed: {{manual}}',
        {
          automatic: formatNumber(
            weekly.early_resets + weekly.expired_resets,
            locale
          ),
          manual: formatNumber(weekly.manual_resets, locale),
        }
      ),
    },
  ]
  return (
    <div className='space-y-3'>
      <div className='grid min-w-0 gap-3 md:grid-cols-2'>
        <Card
          className='min-w-0 py-0'
          role='group'
          aria-label={t('Monthly observed weekly units')}
        >
          {/* 两张主卡共享四行节奏；行高只设下限，长文案可自然增高。 */}
          <CardContent className='grid grid-rows-[minmax(2rem,auto)_minmax(3rem,auto)_minmax(2rem,auto)_minmax(2.5rem,auto)] items-center gap-3 p-6'>
            <div className='flex items-center gap-2 text-base font-medium'>
              <IconBadge tone='info'>
                <ChartColumnIncreasing />
              </IconBadge>
              {t('Monthly observed weekly units')}
            </div>
            <p className='text-4xl font-bold tracking-tight break-words tabular-nums'>
              {formatWeeklyUnits(weekly.observed_units, locale)}
            </p>
            <p className='text-muted-foreground text-sm leading-5'>
              {t(
                'One unit is a full weekly allowance; counting starts after onboarding'
              )}
            </p>
            <p className='text-muted-foreground text-sm leading-5'>
              {t('Coverage starts')}:{' '}
              {ledgerTime(props.data.summary.coverage_start, locale)}
            </p>
          </CardContent>
        </Card>
        <Card
          className='min-w-0 py-0'
          role='group'
          aria-label={t('Relative to four weekly units')}
        >
          <CardContent className='grid grid-rows-[minmax(2rem,auto)_minmax(3rem,auto)_minmax(2rem,auto)_minmax(2.5rem,auto)] items-center gap-3 p-6'>
            <div className='flex items-center gap-2 text-base font-medium'>
              <IconBadge tone='info'>
                <Gauge />
              </IconBadge>
              {t('Relative to four weekly units')}
            </div>
            <p className='text-4xl font-bold tracking-tight break-words tabular-nums'>
              {ratio}
            </p>
            <div className='flex flex-wrap items-center gap-x-3 gap-y-1 text-sm'>
              {progress && (
                <>
                  <Progress
                    className='min-w-20 flex-1'
                    value={progress.progress}
                    aria-label={t('Monthly quota reference progress')}
                    aria-valuetext={t(
                      'Total {{total}}; segment {{segment}} at {{progress}}%',
                      {
                        total: ratio,
                        segment: formatNumber(progress.segment, locale),
                        progress: formatWeeklyUnits(progress.progress, locale),
                      }
                    )}
                  />
                  <span className='font-semibold tabular-nums'>
                    {t('Segment {{segment}} · {{progress}}%', {
                      segment: formatNumber(progress.segment, locale),
                      progress: formatWeeklyUnits(progress.progress, locale),
                    })}
                  </span>
                  {progress.completed > 0 && (
                    <span>
                      {t('Reached {{count}} monthly reference units', {
                        count: progress.completed,
                      })}
                    </span>
                  )}
                </>
              )}
            </div>
            <p className='text-muted-foreground text-sm leading-5'>
              {t(
                'Estimated against four weekly units per month, not an official fixed allowance'
              )}
            </p>
          </CardContent>
        </Card>
      </div>
      <div className='grid min-w-0 gap-3 md:grid-cols-3'>
        {metrics.map((metric) => (
          <Card
            key={metric.title}
            className='min-w-0 py-0'
            role='group'
            aria-label={metric.title}
          >
            <CardContent className='grid grid-rows-[minmax(2rem,auto)_minmax(2.5rem,auto)_minmax(2.5rem,auto)] items-center gap-3 p-5'>
              <div className='flex items-center gap-2 text-base font-medium'>
                <IconBadge tone={metric.tone}>
                  <metric.icon />
                </IconBadge>
                {metric.title}
              </div>
              <p className='text-3xl font-bold tracking-tight break-words tabular-nums'>
                {metric.value}
              </p>
              <p className='text-muted-foreground text-sm leading-5'>
                {metric.detail}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
