import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import type { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { useReportAccess, useLedgerRead } from '@/features/report-access/hooks'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import { decideEvent, getCycles, getEvents, quotaKey } from '../api'
import { decisionSchema, formatWeeklyUnits, ledgerTime } from '../lib/forms'
import type { Filters, QuotaEvent } from '../types'
import { QuotaSelect } from './quota-select'

export function QuotaHistory(props: { filters: Filters }) {
  const { t, i18n } = useTranslation()
  const access = useReportAccess()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [page, setPage] = useState(1)
  const [event, setEvent] = useState<QuotaEvent | null>(null)
  const cycles = useLedgerRead(['cycles', props.filters, page], (signal) =>
    getCycles(props.filters, page, signal)
  )
  const events = useLedgerRead(['events', props.filters, page], (signal) =>
    getEvents(props.filters, page, signal)
  )
  if (cycles.isPending || events.isPending) return <LoadingState />
  if (cycles.isError || events.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void cycles.refetch()
          void events.refetch()
        }}
      />
    )
  }
  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Quota cycles')}</CardTitle>
        </CardHeader>
        <CardContent>
          <StaticDataTable
            data={cycles.data?.items ?? []}
            emptyContent={t('No Data')}
            columns={[
              {
                id: 'start',
                header: t('Cycle started'),
                cell: (row) => ledgerTime(row.started_at, locale),
              },
              {
                id: 'end',
                header: t('Cycle ended'),
                cell: (row) => ledgerTime(row.ended_at, locale),
              },
              {
                id: 'usage',
                header: t('Observed weekly units'),
                cell: (row) =>
                  formatWeeklyUnits(row.observed_basis_points / 10000, locale),
              },
              {
                id: 'unused',
                header: t('Quota left unused at this reset (units)'),
                cell: (row) =>
                  row.partial ||
                  row.has_gap ||
                  row.has_correction ||
                  !row.ended_at ||
                  row.ended_by === 'segment_change'
                    ? '—'
                    : formatWeeklyUnits(
                        (10000 - row.high_water_basis_points) / 10000,
                        locale
                      ),
              },
              {
                id: 'quality',
                header: t('Status'),
                cell: (row) => (
                  <span>
                    {!row.ended_at && t('Current cycle is still in progress')}
                    {row.partial && `${t('Partial cycles')} · `}
                    {row.has_gap && `${t('Observation gap')} · `}
                    {row.has_correction && `${t('Plan correction')} · `}
                    {row.ended_by && eventLabel(row.ended_by, t)}
                  </span>
                ),
              },
            ]}
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('Reset events')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <StaticDataTable
            data={events.data?.items ?? []}
            getRowKey={(row) => row.id}
            emptyContent={t('No Data')}
            columns={[
              {
                id: 'time',
                header: t('Observation interval'),
                cell: (row) =>
                  `${ledgerTime(row.observed_from, locale)} – ${ledgerTime(row.observed_to, locale)}`,
              },
              {
                id: 'kind',
                header: t('Type'),
                cell: (row) => eventLabel(row.kind, t),
              },
              {
                id: 'status',
                header: t('Status'),
                cell: (row) => eventLabel(row.status, t),
              },
              {
                id: 'reason',
                header: t('Decision history'),
                cell: (row) => (
                  <div className='max-w-80 break-words whitespace-normal'>
                    {row.decisions.map((decision) => (
                      <p key={decision.id}>
                        {eventLabel(decision.decision, t)}: {decision.reason}
                      </p>
                    ))}
                  </div>
                ),
              },
              {
                id: 'action',
                header: t('Actions'),
                cell: (row) => (
                  <Button
                    variant='outline'
                    disabled={!Object.keys(row.previews).length}
                    onClick={() => setEvent(row)}
                  >
                    {t('Review event')} #{row.id}
                  </Button>
                ),
              },
            ]}
          />
          <div className='flex gap-2'>
            <Button
              variant='outline'
              disabled={page === 1}
              onClick={() => setPage(page - 1)}
            >
              {t('Previous')}
            </Button>
            <Button
              variant='outline'
              disabled={
                page * 20 >=
                Math.max(cycles.data?.total ?? 0, events.data?.total ?? 0)
              }
              onClick={() => setPage(page + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </CardContent>
      </Card>
      {event && access.canManage && (
        <EventDecision
          key={`${event.id}/${event.version}`}
          event={event}
          revision={events.data?.ledger_revision ?? 0}
          onClose={() => setEvent(null)}
        />
      )}
      {event && !access.canManage && (
        <ReadonlyEvent event={event} onClose={() => setEvent(null)} />
      )}
    </div>
  )
}

// 观察者只能查看既有本地预览；此组件不导入或调用任何保存方法。
function ReadonlyEvent(props: { event: QuotaEvent; onClose: () => void }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Review quota event')}
      description={t('Read-only report access')}
    >
      <p>
        {t('Expected reset')}: {ledgerTime(props.event.old_reset_at, locale)} →{' '}
        {ledgerTime(props.event.new_reset_at, locale)}
      </p>
      {Object.entries(props.event.previews).map(([decision, preview]) => (
        <div key={decision} className='mt-4 space-y-1'>
          <h3 className='font-medium'>{eventLabel(decision, t)}</h3>
          <p>
            {t('Observed weekly units')}:{' '}
            {formatWeeklyUnits(preview?.seven_day.observed_units, locale)}
          </p>
          <p>
            {t('Observed weekly resets')}:{' '}
            {formatNumber(preview?.seven_day.reset_count, locale)}
          </p>
          <p>
            {t('Estimated unused weekly units')}:{' '}
            {preview?.seven_day.evaluated_cycles
              ? formatWeeklyUnits(preview.seven_day.unused_units, locale)
              : '—'}
          </p>
        </div>
      ))}
      {props.event.decisions.map((item) => (
        <p key={item.id} className='mt-3 break-words'>
          {eventLabel(item.decision, t)}: {item.reason}
        </p>
      ))}
    </Dialog>
  )
}

// 状态翻译保持字面量键，避免动态key漏进i18n提取。
function eventLabel(value: string, t: (key: string) => string): string {
  switch (value) {
    case 'early_reset':
      return t('Early reset')
    case 'expired_reset':
      return t('Expired reset')
    case 'manual_reset':
    case 'confirm_reset':
      return t('Manual reset decision')
    case 'plan_change':
    case 'usage_correction':
    case 'plan_correction':
      return t('Plan correction')
    case 'segment_change':
      return t('Start new segment')
    case 'pending':
      return t('Pending')
    case 'accepted':
      return t('Accepted')
    case 'corrected':
      return t('Corrected')
    case 'inconclusive':
      return t('Inconclusive')
    case 'superseded':
      return t('Superseded')
    default:
      return value
  }
}

function EventDecision(props: {
  event: QuotaEvent
  revision: number
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const client = useQueryClient()
  const form = useForm<z.infer<typeof decisionSchema>>({
    resolver: zodResolver(decisionSchema),
    defaultValues: { decision: 'confirm_reset', reason: '' },
  })
  const decision = form.watch('decision')
  let decisionExplanation = t(
    'Confirm that a reset occurred. This starts a new local cycle and counts its observed usage; it does not reset upstream quota or add a whole unit.'
  )
  if (decision === 'plan_correction') {
    decisionExplanation = t(
      'Treat this as a time or usage correction, not a reset. Keep the current cycle and count only growth above its previous high-water mark.'
    )
  } else if (decision === 'segment_change') {
    decisionExplanation = t(
      'Use only when the account or quota basis is no longer comparable. Start from a new baseline without counting its initial usage or a reset.'
    )
  }
  const preview = props.event.previews[decision]
  const window = preview?.seven_day
  const mutation = useMutation({
    mutationFn: decideEvent,
    onSuccess: async () => {
      props.onClose()
      await client.invalidateQueries({ queryKey: quotaKey })
      await client.invalidateQueries({ queryKey: ['pong-report-data'] })
    },
    onError: (error) => handleServerError(error),
  })
  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
      title={t('Review quota event')}
      desc={t(
        'This changes only local accounting, not upstream quota. Existing evidence and decisions are retained.'
      )}
      confirmText={t('Save decision')}
      isLoading={mutation.isPending}
      disabled={!preview || mutation.isError}
      handleConfirm={() =>
        void form.handleSubmit((input) =>
          mutation.mutate({
            ...input,
            id: props.event.id,
            event_version: props.event.version,
            subject_version: props.revision,
          })
        )()
      }
    >
      <Form {...form}>
        <div className='space-y-4'>
          <p className='text-sm'>
            {t('Expected reset')}:{' '}
            {ledgerTime(props.event.old_reset_at, locale)} →{' '}
            {ledgerTime(props.event.new_reset_at, locale)}
          </p>
          <FormField
            control={form.control}
            name='decision'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Decision')}</FormLabel>
                <QuotaSelect
                  label={t('Decision')}
                  value={field.value}
                  onChange={field.onChange}
                  options={[
                    {
                      value: 'confirm_reset',
                      label: t('Manual reset decision'),
                    },
                    { value: 'plan_correction', label: t('Plan correction') },
                    { value: 'segment_change', label: t('Start new segment') },
                  ].filter((option) =>
                    Object.hasOwn(props.event.previews, option.value)
                  )}
                />
                <FormMessage />
              </FormItem>
            )}
          />
          <p
            role='note'
            aria-label={t('Decision explanation')}
            className='text-muted-foreground text-sm'
          >
            {decisionExplanation}
          </p>
          <p className='text-sm' role='status'>
            {t('Preview after decision')}: {t('Observed weekly units')}{' '}
            {formatWeeklyUnits(window?.observed_units, locale)} ·{' '}
            {t('Observed weekly resets')}{' '}
            {formatNumber(window?.reset_count, locale)} ·{' '}
            {t('Estimated unused weekly units')}{' '}
            {window?.evaluated_cycles
              ? formatWeeklyUnits(window.unused_units, locale)
              : '—'}
          </p>
          <FormField
            control={form.control}
            name='reason'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Decision reason')}</FormLabel>
                <FormControl>
                  <Textarea maxLength={500} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          {mutation.isError && (
            <p role='alert'>
              {t(
                'Save failed. Close this dialog and reload the latest event before retrying.'
              )}
            </p>
          )}
        </div>
      </Form>
    </ConfirmDialog>
  )
}
