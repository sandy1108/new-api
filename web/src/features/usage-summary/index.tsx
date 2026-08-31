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
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { ErrorState } from '@/components/error-state'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'

import {
  DEFAULT_USAGE_SUMMARY_RANGE,
  DEFAULT_USAGE_SUMMARY_SCOPE,
} from './constants'
import { UsageSummaryFilters } from './components/filters'
import { UsageSummaryCards } from './components/summary-cards'
import { TokenDetail } from './components/token-detail'
import { TokenTable } from './components/token-table'
import { useUsageSummary } from './hooks/use-usage-summary'
import { createUsageSummaryRange } from './lib/date-range'
import { aggregateUsageItems, selectDefaultToken } from './lib/selectors'
import type {
  UsageMetrics,
  UsageSummaryRangeId,
  UsageSummaryScope,
} from './types'

function getHttpStatus(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') return undefined
  const response = (error as { response?: unknown }).response
  if (!response || typeof response !== 'object') return undefined
  const status = (response as { status?: unknown }).status
  return typeof status === 'number' ? status : undefined
}

function toServerTotals(data: {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_tokens: number
  total_quota: number
}): UsageMetrics {
  return {
    requests: data.total_requests,
    inputTokens: data.total_input_tokens,
    outputTokens: data.total_output_tokens,
    totalTokens: data.total_tokens,
    quota: data.total_quota,
  }
}

function UsageSummaryLoading() {
  const skeletonKeys = [
    'requests',
    'input-tokens',
    'output-tokens',
    'total-tokens',
    'quota',
  ]

  return (
    <div className='space-y-4' data-testid='usage-summary-loading'>
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
        {skeletonKeys.map((key) => (
          <Skeleton key={key} className='h-[76px] w-full rounded-xl' />
        ))}
      </div>
      <Skeleton className='h-48 w-full rounded-xl' />
      <Skeleton className='h-56 w-full rounded-xl' />
    </div>
  )
}

export function UsageSummary() {
  const { t } = useTranslation()
  const [rangeId, setRangeId] = useState<UsageSummaryRangeId>(
    DEFAULT_USAGE_SUMMARY_RANGE
  )
  const [requestedScope, setRequestedScope] = useState<UsageSummaryScope>(
    DEFAULT_USAGE_SUMMARY_SCOPE
  )
  const [selectedTokenKey, setSelectedTokenKey] = useState<string>()
  const range = useMemo(() => createUsageSummaryRange(rangeId), [rangeId])
  const query = useUsageSummary(range, requestedScope)
  const view = useMemo(
    () => (query.data ? aggregateUsageItems(query.data.items) : null),
    [query.data]
  )
  const selectedToken = selectDefaultToken(
    view?.tokens ?? [],
    selectedTokenKey
  )

  const handleRangeChange = (nextRangeId: UsageSummaryRangeId) => {
    setRangeId(nextRangeId)
    setSelectedTokenKey(undefined)
  }

  const handleScopeChange = (nextScope: UsageSummaryScope) => {
    setRequestedScope(nextScope)
    setSelectedTokenKey(undefined)
  }

  const showInitialLoading = query.isLoading && !query.data
  const showInitialError = query.isError && !query.data
  const errorStatus = getHttpStatus(query.error)
  const errorTitle =
    errorStatus === 403
      ? t('Access Forbidden')
      : t('Failed to load usage summary')
  let errorDescription: string | undefined
  if (errorStatus === 403) {
    errorDescription = t("You don't have necessary permission")
  } else if (query.error instanceof Error) {
    errorDescription = query.error.message
  }

  const data = query.data
  let content: ReactNode
  if (showInitialLoading) {
    content = <UsageSummaryLoading />
  } else if (showInitialError) {
    content = (
      <ErrorState
        title={errorTitle}
        description={errorDescription}
        onRetry={() => void query.refetch()}
        className='min-h-[320px]'
      />
    )
  } else if (view && data) {
    const loadedContent =
      view.tokens.length === 0 ? (
        <Empty className='min-h-[260px] border'>
          <EmptyHeader>
            <EmptyTitle>{t('No consumption logs in this range.')}</EmptyTitle>
            <EmptyDescription>
              {t('Try selecting a different time range.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <>
          <section aria-labelledby='usage-summary-token-heading'>
            <h3
              id='usage-summary-token-heading'
              className='mb-2 text-sm font-semibold'
            >
              {t('Token Usage by Token')}
            </h3>
            <TokenTable
              tokens={view.tokens}
              selectedKey={selectedToken?.key}
              onSelect={setSelectedTokenKey}
            />
          </section>
          <section aria-labelledby='usage-summary-detail-heading'>
            <h3
              id='usage-summary-detail-heading'
              className='mb-2 text-sm font-semibold'
            >
              {t('Channel and Model Details')}
            </h3>
            <TokenDetail token={selectedToken} />
          </section>
        </>
      )

    content = (
      <>
        {query.isError && (
          <Alert variant='destructive' className='shrink-0'>
            <AlertDescription className='flex flex-wrap items-center justify-between gap-2'>
              <span>
                {errorTitle}
                {errorDescription ? `: ${errorDescription}` : null}
              </span>
              <button
                type='button'
                className='font-medium underline underline-offset-4'
                onClick={() => void query.refetch()}
              >
                {t('Retry')}
              </button>
            </AlertDescription>
          </Alert>
        )}
        <UsageSummaryCards totals={toServerTotals(data)} />
        {loadedContent}
      </>
    )
  } else {
    content = <UsageSummaryLoading />
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Token Usage')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <UsageSummaryFilters
          rangeId={rangeId}
          scope={query.scope}
          canManageScope={query.canManageScope}
          refreshing={query.isFetching && !query.isLoading}
          onRangeChange={handleRangeChange}
          onScopeChange={handleScopeChange}
          onRefresh={() => void query.refetch()}
        />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div
          className='flex h-full min-h-0 flex-col gap-4 overflow-auto'
          aria-busy={query.isFetching}
        >
          {content}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
