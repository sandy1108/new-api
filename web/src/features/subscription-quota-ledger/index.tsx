import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getSubjects, quotaKey } from './api'
import { QuotaHistory } from './components/history'
import { QuotaOverview } from './components/overview'
import { QuotaSelect } from './components/quota-select'
import { beijingMonth } from './lib/forms'
import type { View } from './types'

export function SubscriptionQuotaLedger(props: {
  view: View
  onViewChange: (view: View) => void
}) {
  const { t } = useTranslation()
  const [subjectPage, setSubjectPage] = useState(1)
  const [selectedSubject, setSelectedSubject] = useState(0)
  const [month, setMonth] = useState(beijingMonth)
  const subjects = useQuery({
    queryKey: [...quotaKey, 'subjects', subjectPage],
    queryFn: () => getSubjects(subjectPage),
    refetchInterval: 60_000,
  })
  // 周额度是唯一累计单位；5小时仅在概览中展示状态，不提供份数切换。
  // 不使用placeholderData；切账号/月时先展示加载，绝不沿用另一账号结果。
  const subject =
    subjects.data?.items.find((item) => item.id === selectedSubject) ??
    subjects.data?.items[0]
  const filters = {
    subject_id: subject?.id ?? 0,
    month,
    window_seconds: 604800,
  }
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Subscription quota ledger')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          role='link'
          variant='outline'
          render={<Link to='/system-settings/subscription-quota' />}
        >
          {t('Subscription quota ledger settings')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Tabs
          value={props.view}
          onValueChange={(value) => {
            if (value === 'overview' || value === 'history') {
              props.onViewChange(value)
            }
          }}
          className='space-y-4'
        >
          <TabsList aria-label={t('Subscription quota ledger')}>
            <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
            <TabsTrigger value='history'>{t('Cycles and events')}</TabsTrigger>
          </TabsList>
          {subjects.isPending && <LoadingState />}
          {subjects.isError && (
            <ErrorState onRetry={() => void subjects.refetch()} />
          )}
          {subjects.data && (
            <>
              {!subject && (
                <EmptyState
                  title={t('No monitored subscriptions')}
                  action={
                    <Button
                      role='link'
                      render={<Link to='/system-settings/subscription-quota' />}
                    >
                      {t('Subscription quota ledger settings')}
                    </Button>
                  }
                />
              )}
              {subject && (
                <>
                  <div className='flex min-w-0 flex-wrap items-center gap-3'>
                    <QuotaSelect
                      label={t('Subscription')}
                      value={String(subject.id)}
                      onChange={(value) => setSelectedSubject(Number(value))}
                      options={subjects.data.items.map((item) => ({
                        value: String(item.id),
                        label: item.name,
                      }))}
                    />
                    <Input
                      aria-label={t('Month (Asia/Shanghai)')}
                      type='month'
                      min='2000-01'
                      max='2100-12'
                      className='w-auto'
                      value={month}
                      onChange={(event) => {
                        if (
                          /^\d{4}-(0[1-9]|1[0-2])$/.test(event.target.value)
                        ) {
                          setMonth(event.target.value)
                        }
                      }}
                    />
                    {subjects.data.total > 100 && (
                      <>
                        <Button
                          variant='outline'
                          disabled={subjectPage === 1}
                          onClick={() => setSubjectPage(subjectPage - 1)}
                        >
                          {t('Previous')}
                        </Button>
                        <Button
                          variant='outline'
                          disabled={subjectPage * 100 >= subjects.data.total}
                          onClick={() => setSubjectPage(subjectPage + 1)}
                        >
                          {t('Next')}
                        </Button>
                      </>
                    )}
                  </div>
                  <TabsContent value='overview'>
                    <QuotaOverview
                      key={`${subject.id}/${month}`}
                      filters={filters}
                    />
                  </TabsContent>
                  <TabsContent value='history'>
                    <QuotaHistory
                      key={`${subject.id}/${month}`}
                      filters={filters}
                    />
                  </TabsContent>
                </>
              )}
            </>
          )}
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
