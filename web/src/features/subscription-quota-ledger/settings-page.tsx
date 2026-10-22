import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'

import { getSubjects, quotaKey } from './api'
import { QuotaSettings } from './components/settings'

// 配置页与日常账本分离，复用原有采集设置和订阅管理，不改变记账与采集逻辑。
export function SubscriptionQuotaSettings() {
  const { t } = useTranslation()
  const [subjectPage, setSubjectPage] = useState(1)
  const subjects = useQuery({
    queryKey: [...quotaKey, 'subjects', subjectPage],
    queryFn: () => getSubjects(subjectPage),
    refetchInterval: 60_000,
  })
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Subscription quota ledger settings')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          role='link'
          variant='outline'
          render={<Link to='/subscription-quota' />}
        >
          {t('Subscription quota ledger')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {subjects.isPending && <LoadingState />}
        {subjects.isError && (
          <ErrorState onRetry={() => void subjects.refetch()} />
        )}
        {subjects.data && (
          <QuotaSettings
            subjects={subjects.data.items}
            subjectPage={subjectPage}
            total={subjects.data.total}
            onPage={setSubjectPage}
          />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
